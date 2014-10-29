package apnsd

import (
	"bufio"
	"bytes"
	"container/list"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/ugorji/go/codec"
)

const (
	ApnsPort              = "2195"
	ApnsProductionGateway = "gateway.push.apple.com"
	ApnsSandboxGateway    = "gateway.sandbox.push.apple.com"
)

var ErrMsgClosed = errors.New("msg chan seems to be closed")

type Retriver struct {
	shutdown chan struct{}
	msgChan  chan *Msg
	apnsd    *Apnsd
}

func NewRetriver(shutdown chan struct{}, msgChan chan *Msg, apnsd *Apnsd) *Retriver {
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
				log.Printf("retriver redis err: %s\n", err)
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

			r.msgChan <- &msg
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
	msgChan  chan *Msg

	apnsd      *Apnsd
	msgs       *list.List
	mx         sync.Mutex
	identifier uint32
}

func NewSender(shutdown chan struct{}, msgChan chan *Msg, apnsd *Apnsd) *Sender {

	return &Sender{
		shutdown: shutdown,
		msgChan:  msgChan,
		apnsd:    apnsd,
	}
}

func (s *Sender) Start() error {

	connChan := make(chan struct{}, 1)

	connChan <- struct{}{}

	var senderConn *SenderConn
	var connClosedChan <-chan struct{}

	for {
		select {
		case <-connClosedChan:
			switch senderConn.err {
			case ErrMsgClosed:
				log.Println("sender detect msg chan seems to be closed")
				senderConn.Close() // block
				return nil
			case nil:
				log.Println("previous sender conn seems to be closed. try to reconnect.")
				connClosedChan = nil
				senderConn.Close() // block
				senderConn = nil
				time.Sleep(time.Second * 1)
				go func() { connChan <- struct{}{} }()
			default:
				panic("not expected err received:" + senderConn.err.Error())
			}
		case <-connChan:
			if senderConn != nil {
				panic("sender conn should be nil")
			}

			log.Println("sender connecting to apns server")
			conn, err := s.apnsd.SenderDialFunc()
			if err != nil {
				log.Println("sender dial err:", err)
				conn = nil
				time.Sleep(time.Second * 1)
				go func() { connChan <- struct{}{} }()
			} else {
				atomic.AddInt32(&s.apnsd.numOfConnectToApns, 1)
				senderConn = NewSenderConn(conn)
				connClosedChan = senderConn.Start(s.msgChan)
			}
		case <-s.shutdown:
			log.Println("sender receive shutdown")
			if senderConn != nil {
				senderConn.Close()
			}
			return nil
		}
	}

	panic("not reach")
}

type SenderConn struct {
	conn           net.Conn
	msgs           *list.List
	mx             sync.Mutex
	writeCloseChan chan struct{}
	closedChan     chan struct{}
	identifier     uint32
	err            error
}

func NewSenderConn(conn net.Conn) *SenderConn {
	return &SenderConn{
		conn:           conn,
		msgs:           list.New(),
		writeCloseChan: make(chan struct{}, 1),
		closedChan:     make(chan struct{}),
		identifier:     0,
	}
}

func (s *SenderConn) Start(msgChan chan *Msg) <-chan struct{} {

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.readError()
		go func() { s.writeCloseChan <- struct{}{} }()
		log.Println("read err done")
	}()

	go func() {
		defer wg.Done()
		s.err = s.writeLoop(msgChan)
		log.Println("werite loop done")
	}()

	go func() {
		wg.Wait()
		close(s.closedChan)
	}()

	return s.closedChan
}

//TODO: block
func (s *SenderConn) Close() {
	s.conn.Close()
	go func() { s.writeCloseChan <- struct{}{} }()
	<-s.closedChan
}

func (s *SenderConn) readError() {
	defer s.conn.Close()

	errMsg := &ErrorMsg{}
	if err := errMsg.Read(s.conn); err != nil {
		log.Println("read err msg err:", err)
	}

	if errMsg.Identifier != 0 {
		log.Printf("sender got err from apns server. command:%d status:%d identifier:%d\n", errMsg.Command, errMsg.Status, errMsg.Identifier)
		s.mx.Lock()
		for e := s.msgs.Front(); e != nil; e = e.Next() {
			if e.Value.(*Msg).Identifier == errMsg.Identifier {
				msg := e.Value.(*Msg)
				token := hex.EncodeToString(msg.Token)
				log.Printf("err msg: token:%s payload:%s expire:%d priority:%d identifier:%d", token, msg.Payload, msg.Expire, msg.Priority, msg.Identifier)
				break
			}
		}
		s.mx.Unlock()
	}
}

func (s *SenderConn) writeLoop(msgChan chan *Msg) error {
	defer s.conn.Close()
	w := bufio.NewWriter(s.conn)

	for {
		select {
		case msg, ok := <-msgChan:
			if !ok { // closed
				return ErrMsgClosed
			}

			msg.Identifier = atomic.AddUint32(&s.identifier, 1)

			s.mx.Lock()
			s.msgs.PushFront(msg)
			if s.msgs.Len() >= 100 {
				if e := s.msgs.Back(); e != nil {
					s.msgs.Remove(e)
				}
			}
			s.mx.Unlock()

			var b bytes.Buffer
			if err := msg.WriteWithAutotrim(&b); err != nil {
				log.Println("msg write with autotrim err:", err)
				continue
			}

			if _, err := w.Write(b.Bytes()); err != nil {
				log.Println("sender write err:", err)
				return nil
			}

			if err := w.Flush(); err != nil {
				log.Println("sender write flush error:", err)
				return nil
			}
		case <-s.writeCloseChan:
			return nil
		}
	}
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
}

func NewApnsd(apnsDialFunc func() (net.Conn, error)) *Apnsd {
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
	msgChan := make(chan *Msg, a.MsgChanBufferNum)

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

func (a *Apnsd) spawnRetriver(wg *sync.WaitGroup, shutdown chan struct{}, msgChan chan *Msg) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := NewRetriver(shutdown, msgChan, a)
		r.Start() // block
		r.apnsd = nil
	}()
}

func (a *Apnsd) spawnSender(wg *sync.WaitGroup, shutdown chan struct{}, msgChan chan *Msg) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		s := NewSender(shutdown, msgChan, a)
		s.Start() // block
		s.apnsd = nil
	}()
}
