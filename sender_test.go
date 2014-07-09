package apnsd

import (
	"crypto/tls"
	"sync"
	"testing"
	"time"

	"github.com/makeitreal/apnsd/apns"
)

func TestSenderNormal(t *testing.T) {
	mock, err := newTestMockApnsServer("")
	if err != nil {
		t.Fatal(err)
	}

	defer mock.l.Close()
	go mock.handle()

	msgChan := make(chan *apns.Msg, 10)
	shutdownChan := make(chan struct{}, 0)

	s := &Sender{
		c: msgChan,
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		apnsAddr:       mock.l.Addr().String(),
		identifier:     &Identifier{},
		errorTimeout:   time.Second * 3,
		reconnectSleep: time.Second * 3,
		shutdownChan:   shutdownChan,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Start(); err != nil {
			t.Log(err)
		}
	}()

	orgMsg := testNewApnsMsg()

	msgChan <- orgMsg

	newMsg := <-mock.msgChan

	if string(orgMsg.Token) != string(newMsg.Token) {
		t.Error("diffrent token")
	}

	shutdownChan <- struct{}{}

	wg.Wait()

}

func TestSenderClosedFromApnsAndReconnect(t *testing.T) {
	mock, err := newTestMockApnsServer("")
	if err != nil {
		t.Fatal(err)
	}

	defer mock.l.Close()
	go mock.handle()

	msgChan := make(chan *apns.Msg, 10)
	shutdownChan := make(chan struct{}, 0)

	s := &Sender{
		c: msgChan,
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		apnsAddr:       mock.l.Addr().String(),
		identifier:     &Identifier{},
		errorTimeout:   time.Second * 3,
		reconnectSleep: time.Second * 3,
		shutdownChan:   shutdownChan,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Start(); err != nil {
			t.Log(err)
		}
	}()

	// wait connect to mock server
	time.Sleep(time.Second * 1)
	if s.connectCount != 1 {
		t.Error("connect count should be 1. but", s.connectCount)
	}

	// shutdown
	t.Log("shutdown mock server")
	time.Sleep(time.Second * 1)
	close(mock.shutdownChan)
	mock.l.Close()
	time.Sleep(time.Second * 1)

	// re launch mock server
	mock, err = newTestMockApnsServer(s.apnsAddr)
	if err != nil {
		t.Fatal(err)
	}

	defer mock.l.Close()
	go mock.handle()

	time.Sleep(s.reconnectSleep)

	if s.connectCount != 2 {
		t.Error("connect count should be 2. but", s.connectCount)
	}

	shutdownChan <- struct{}{}

	wg.Wait()

}

//TODO errorMsgChan test
