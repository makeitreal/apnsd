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
	//TODO: lazy connect
	tlsConn, err := tls.Dial("tcp", s.apnsAddr, s.tlsConfig)

	if err != nil {
		return err
	}

	s.log("success to connect ", s.apnsAddr)
	conn := apns.NewConnection(tlsConn)
	defer conn.Close()

	errorChan := make(chan error, 2) // non blocking notify
	errorMsgChan := make(chan *apns.ErrorMsg, 1)
	closeChan := make(chan struct{}, 0)
	var wg sync.WaitGroup

	// receive only networking error
	go func() {
		errMsg, err := conn.ReadErrorMsg()
		s.log("read error errMsg:", errMsg, "err:", err)
		errorChan <- err
		if errMsg != nil {
			errorMsgChan <- errMsg
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case msg := <-s.c:
				s.log("receive msg", msg, "and try to send")
				identifier := s.identifier.NextIdentifier()
				msg.Identifier = identifier

				if err := conn.WriteMsg(msg); err != nil {
					s.log("connection write error:", err)
					errorChan <- err
					return
				}
			case <-closeChan:
				s.log("close chan received")
				return
			case <-s.shutdownChan:
				s.log("shutdown event received")
				s.shutdown()
				errorChan <- nil
				return
			}
		}
	}()

	<-errorChan      // receive err but i dont know what is networking error
	close(closeChan) // notify goroutine for exiting

	// read errMsg with timeout.
	// when happen non networking error, errMsg does not come and timeout.
	select {
	case <-time.After(s.errorTimeout):
		s.log("timeout error msg chan!")
	case errMsg := <-errorMsgChan:
		s.log("err msg chan:", errMsg)
	}

	wg.Wait()

	if s.isShutdown {
		return shutdownError
	} else {
		return nil
	}
}

func (s *Sender) shutdown() {
	defer s.m.Unlock()
	s.m.Lock()
	s.isShutdown = true
}

func (s *Sender) log(v ...interface{}) {
	log.Println("[sender]", fmt.Sprint(v))
}
