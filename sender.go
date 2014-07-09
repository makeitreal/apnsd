package apnsd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/makeitreal/apnsd/apns"
)

var shutdownError = errors.New("shutdown event received")

type Sender struct {
	c              <-chan *apns.Msg
	tlsConfig      *tls.Config
	apnsAddr       string
	identifier     *Identifier
	errorTimeout   time.Duration
	reconnectSleep time.Duration
	connectCount   int
	shutdownChan   <-chan struct{}
	isShutdown     bool
	m              sync.Mutex
}

func (s *Sender) Name() string {
	return "sender"
}

func (s *Sender) Start() error {
	for {
		if err := s.work(); err != nil {
			return err
		}
		//TODO: backoff
		s.log("sleep", s.reconnectSleep, "for reconnecting")
		time.Sleep(s.reconnectSleep)
	}
}

func (s *Sender) work() error {

	context, err := s.newSenderContext(2) // non blocking notify

	if err != nil {
		return err
	}

	defer context.conn.Close()

	var wg sync.WaitGroup

	go context.readConnectionError()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case msg := <-s.c:
				s.log("receive msg", msg, "and try to send")
				identifier := s.identifier.NextIdentifier()
				msg.Identifier = identifier

				if err := context.conn.WriteMsg(msg); err != nil {
					s.log("connection write error:", err)
					context.errorChan <- err
					return
				}
			case <-context.closeChan:
				s.log("close chan received")
				return
			case <-s.shutdownChan:
				s.log("shutdown event received")
				s.shutdown()
				context.conn.Close()
				context.errorChan <- nil
				return
			}
		}
	}()

	context.wait(s.errorTimeout)

	wg.Wait()

	if s.isShutdown {
		return shutdownError
	} else {
		return nil
	}
}

func (s *Sender) newConnection() (*apns.Connection, error) {
	//TODO: lazy connect
	tlsConn, err := tls.Dial("tcp", s.apnsAddr, s.tlsConfig)

	if err != nil {
		return nil, err
	}

	s.log("success to connect ", s.apnsAddr)
	s.incrConnectCount()

	return apns.NewConnection(tlsConn), nil
}

func (s *Sender) shutdown() {
	defer s.m.Unlock()
	s.m.Lock()
	s.isShutdown = true
}

func (s *Sender) incrConnectCount() {
	defer s.m.Unlock()
	s.m.Lock()
	s.connectCount++
}

func (s *Sender) log(v ...interface{}) {
	log.Println("[sender]", fmt.Sprint(v))
}

func (s *Sender) newSenderContext(errorChanSize int) (*senderContext, error) {

	conn, err := s.newConnection()
	if err != nil {
		return nil, err
	}

	errorChan := make(chan error, errorChanSize) // non blocking notify
	errorMsgChan := make(chan *apns.ErrorMsg, 1)
	closeChan := make(chan struct{}, 0)

	return &senderContext{
		conn:         conn,
		errorChan:    errorChan,
		errorMsgChan: errorMsgChan,
		closeChan:    closeChan,
	}, nil
}

type senderContext struct {
	conn         *apns.Connection
	errorChan    chan error
	errorMsgChan chan *apns.ErrorMsg
	closeChan    chan struct{}
}

// receive only networking error
func (s *senderContext) readConnectionError() {
	errMsg, err := s.conn.ReadErrorMsg()
	s.log("read error errMsg:", errMsg, "err:", err)
	s.errorChan <- err
	if errMsg != nil {
		s.errorMsgChan <- errMsg
	}
}

func (s *senderContext) wait(timeout time.Duration) {
	<-s.errorChan      // receive err but i dont know what is networking error
	close(s.closeChan) // notify goroutine for exiting

	// read errMsg with timeout.
	// when happen non networking error, errMsg does not come and timeout.
	select {
	case <-time.After(timeout):
		s.log("timeout error msg chan!")
	case errMsg := <-s.errorMsgChan:
		//TODO: more detail error
		s.log("err msg chan:", errMsg)
	}
}

func (s *senderContext) log(v ...interface{}) {
	log.Println("[sender context]", fmt.Sprint(v))
}
