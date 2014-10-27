package apnsd

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/hashicorp/golang-lru"
	"github.com/ugorji/go/codec"
)

const (
	ApnsPort              = "2195"
	ApnsProductionGateway = "gateway.push.apple.com"
	ApnsSandboxGateway    = "gateway.sandbox.push.apple.com"
)

type Retriver struct {
	shutdown chan struct{}
	msgChan  chan []byte
	apnsd    *Apnsd
}

func NewRetriver(shutdown chan struct{}, msgChan chan []byte, apnsd *Apnsd) *Retriver {
	return &Retriver{
		shutdown: shutdown,
		msgChan:  msgChan,
		apnsd:    apnsd,
	}
}

func (r *Retriver) Start() error {
	// if redis connection closed, re connect.
	connChan := make(chan struct{}, 1)
	reconnectTimer := time.AfterFunc(-1, func() {
		connChan <- struct{}{}
	})

	brpop := func(conn redis.Conn) {
		defer conn.Close()

		mh := &codec.MsgpackHandle{}
		for {
			reply, err := redis.Values(conn.Do("BRPOP", r.apnsd.RetriveKey, "30"))

			if err != nil {
				if err == redis.ErrNil {
					continue
				}
				log.Printf("retriver redis err: %s. try to reconnect\n", err)
				reconnectTimer.Reset(time.Second)
				return
			}

			var key string
			var byt []byte
			if _, err := redis.Scan(reply, &key, &byt); err != nil {
				log.Printf("redis scan err")
				continue
			}

			var msg Msg
			if err := codec.NewDecoder(bytes.NewReader(byt), mh).Decode(&msg); err != nil {
				log.Printf("failed decode. skip it. %s\n", string(byt))
				continue
			}

			if msg.Identifier == 0 {
				// fill identifier
				msg.Identifier = r.apnsd.NextIdentifier()
			}

			var b bytes.Buffer
			if err := msg.WriteWithAutotrim(&b); err != nil {
				log.Println("msg write with autotrim err:", err)
				continue
			}

			r.apnsd.msgCache.Add(msg.Identifier, &msg)

			r.msgChan <- b.Bytes()
		}
	}

	var conn redis.Conn

	for {
		select {
		case <-connChan:
			log.Println("retriver connect to redisserver")
			var err error
			conn, err = redis.DialTimeout(
				r.apnsd.RetriverRedisNetwork,
				r.apnsd.RetriverRedisAddr,
				r.apnsd.RetriverDialTimeout,
				0,
				0,
			)
			if err != nil {
				log.Println("redis dial err:", err)
				reconnectTimer.Reset(time.Second)
			} else {
				go brpop(conn)
			}
		case <-r.shutdown:
			log.Println("retriver receive shutdown")
			if conn != nil {
				log.Println("close redis connection")
				conn.Close()
			}
			return nil
		}
	}

	panic("not reach")
}

type Sender struct {
	shutdown chan struct{}
	msgChan  chan []byte

	apnsd *Apnsd
}

func NewSender(shutdown chan struct{}, msgChan chan []byte, apnsd *Apnsd) *Sender {
	return &Sender{
		shutdown: shutdown,
		msgChan:  msgChan,
		apnsd:    apnsd,
	}
}

func (s *Sender) Start() error {
	connChan := make(chan struct{}, 1)
	reconnectTimer := time.AfterFunc(-1, func() {
		connChan <- struct{}{}
	})

	msgClosedChan := make(chan struct{})

	writeLoop := func(conn net.Conn, msgChan chan []byte, closeChan chan struct{}) {
		defer conn.Close()
		w := bufio.NewWriter(conn)

		for {
			select {
			case msg, ok := <-msgChan:
				if !ok { // closed
					msgClosedChan <- struct{}{}
					return
				}

				if _, err := w.Write(msg); err != nil {
					log.Println("sender write err:", err)
					reconnectTimer.Reset(time.Second)
					return
				}

				if err := w.Flush(); err != nil {
					log.Println("sender write flush error:", err)
					reconnectTimer.Reset(time.Second)
					return
				}
			case <-closeChan:
				return
			}
		}
	}

	read := func(conn net.Conn) {
		defer conn.Close()

		errMsg := &ErrorMsg{}
		if err := errMsg.Read(conn); err != nil {
		}
		log.Println("sender got err from apns server:", errMsg)

		if errMsg.Identifier != 0 {
			i, ok := s.apnsd.msgCache.Get(errMsg.Identifier)
			if ok {
				msg := i.(*Msg)
				log.Printf("err msg: token:%s payload:%s expire:%d priority:%d identifier:%d", string(msg.Token), msg.Payload, msg.Expire, msg.Priority, msg.Identifier)
			}
		}

		reconnectTimer.Reset(time.Second)
	}

	var conn net.Conn
	var writeCloseChan chan struct{}

	for {
		select {
		case <-connChan:
			log.Println("sender connecting to apns server")
			var err error
			conn, err = s.apnsd.SenderDialFunc()
			if err != nil {
				log.Println("sender dial err:", err)
				reconnectTimer.Reset(time.Second)
			} else {
				if writeCloseChan != nil {
					close(writeCloseChan)
				}
				writeCloseChan = make(chan struct{})
				atomic.AddInt32(&s.apnsd.numOfConnectToApns, 1)
				go read(conn)
				go writeLoop(conn, s.msgChan, writeCloseChan)
			}
		case <-msgClosedChan:
			log.Println("sender detect msg chan seems to be closed")
			return nil
		case <-s.shutdown:
			log.Println("sender receive shutdown")
			//TODO: wait msgClosedChan for graceful shutdown.
			conn.Close()
			return nil
		}
	}

	panic("not reach")
}

type Apnsd struct {
	// Redis ( retriver )
	RetriverRedisNetwork string
	RetriverRedisAddr    string
	RetriverNum          int
	RetriveKey           string
	RetriverDialTimeout  time.Duration

	// Apns ( sender )
	SenderNum      int
	SenderDialFunc func() (net.Conn, error)

	// other
	MsgChanBufferNum   int
	shutdownChan       chan struct{}
	identifier         uint32
	numOfConnectToApns int32
	msgCache           *lru.Cache
}

func NewApnsd(apnsDialFunc func() (net.Conn, error)) *Apnsd {
	msgCache, err := lru.New(1024 * 100)
	if err != nil {
		panic(err)
	}

	a := &Apnsd{
		RetriverRedisNetwork: "tcp",
		RetriverRedisAddr:    "127.0.0.1:6379",
		RetriverNum:          1,
		RetriveKey:           "APNSD:MSG_QUEUE",
		RetriverDialTimeout:  time.Second * 5,

		SenderNum:      1,
		SenderDialFunc: apnsDialFunc,

		MsgChanBufferNum: 10,

		shutdownChan: make(chan struct{}),
		identifier:   0,
		msgCache:     msgCache,
	}

	return a
}

func (a *Apnsd) Start() {
	// shutdown flow
	// 1. receive shutdown chan
	// 2. retriver close
	// 3. send remain msg to send channel
	// 4. write to apns
	// 5. sender close
	// 6. shutdown

	var retriverWg sync.WaitGroup
	retriverShutdown := make(chan struct{})
	msgChan := make(chan []byte, a.MsgChanBufferNum)

	for i := 0; i < a.RetriverNum; i++ {
		log.Printf("spawn retriver %d\n", i)
		a.spawnRetriver(&retriverWg, retriverShutdown, msgChan)
	}

	var senderWg sync.WaitGroup
	senderShutdown := make(chan struct{})

	for i := 0; i < a.SenderNum; i++ {
		log.Printf("spawn sender %d\n", i)
		a.spawnSender(&senderWg, senderShutdown, msgChan)
	}

	select {
	case <-a.shutdownChan:
		// stop all retriver
		close(retriverShutdown)
		retriverWg.Wait()

		// close msg chan
		close(msgChan)

		// stop all sender
		close(senderShutdown)
		senderWg.Wait()

		return
	}
}

func (a *Apnsd) Shutdown() {
	a.shutdownChan <- struct{}{}
}

func (a *Apnsd) spawnRetriver(wg *sync.WaitGroup, shutdown chan struct{}, msgChan chan []byte) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := NewRetriver(shutdown, msgChan, a)
		r.Start() // block
		r.apnsd = nil
	}()
}

func (a *Apnsd) spawnSender(wg *sync.WaitGroup, shutdown chan struct{}, msgChan chan []byte) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		s := NewSender(shutdown, msgChan, a)
		s.Start() // block
		s.apnsd = nil
	}()
}

func (a *Apnsd) NextIdentifier() uint32 {
	atomic.AddUint32(&a.identifier, 1)
	return a.identifier
}
