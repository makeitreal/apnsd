package apnsd

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/makeitreal/apnsd/apns"
	"github.com/soh335/go-test-redisserver"
)

var (
	Key = "foo"
)

func testRetriver() (*Retriver, *redistest.Server, error) {
	redisserver, err := redistest.NewServer(true, redistest.Config{})

	if err != nil {
		return nil, nil, err
	}

	msgChan := make(chan *apns.Msg, 10)
	shutdownChan := make(chan struct{}, 0)

	retriver := &Retriver{
		c:            msgChan,
		redisNetwork: "unix",
		redisAddr:    redisserver.Config["unixsocket"],
		shutdownChan: shutdownChan,
		key:          Key,
		timeout:      "3",
	}

	return retriver, redisserver, nil
}

func testRetriverRedisConn(s *redistest.Server) (redis.Conn, error) {
	return redis.Dial("unix", s.Config["unixsocket"])
}

func TestRetriverDeque(t *testing.T) {
	retriver, redisserver, err := testRetriver()
	if err != nil {
		t.Fatal(err)
	}
	defer redisserver.Stop()

	var retriverErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		retriverErr = retriver.Start()
	}()

	conn, err := testRetriverRedisConn(redisserver)
	if err != nil {
		t.Fatal(err)
	}

	orgMsg := &apns.Msg{
		Token:    []byte("foo"),
		Priority: 10,
		Expire:   0,
		Payload: apns.Payload{
			"aps": &apns.Aps{
				Alert: &apns.Alert{
					Body: apns.String("hi"),
				},
				Badge: apns.Int(0),
			},
		},
	}

	byt, err := EncodeMsg(orgMsg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := conn.Do("LPUSH", Key, byt); err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		newMsg := <-retriver.c
		if string(orgMsg.Token) != string(newMsg.Token) {
			t.Error("orgMsg.token:", string(orgMsg.Token), "newMsg.Token", string(newMsg.Token))
		}

		if orgMsg.Priority != newMsg.Priority {
			t.Error("orgMsg.Priority", orgMsg.Priority, "newMsg.Priority", newMsg.Priority)
		}

		if orgMsg.Expire != newMsg.Expire {
			t.Error("orgMsg.Expire", orgMsg.Expire, "newMsg.Expire", newMsg.Expire)
		}

		orgMsgJson, _ := json.Marshal(orgMsg.Payload)
		newMsgJson, _ := json.Marshal(newMsg.Payload)

		t.Log("orgMsg.Payload:", string(orgMsgJson))
		t.Log("newMsg.Payload:", string(newMsgJson))
		if string(orgMsgJson) != string(newMsgJson) {
			t.Error("org msg and new msg is not same")
		}
		retriver.shutdownChan <- struct{}{}
	}()

	wg.Wait()
}

func TestRetriverShutdown(t *testing.T) {
	retriver, redisserver, err := testRetriver()
	if err != nil {
		t.Fatal(err)
	}
	defer redisserver.Stop()

	var retriverErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		retriverErr = retriver.Start()
	}()

	retriver.timeout = "5" // long timeout

	before := time.Now()

	time.Sleep(time.Second * 1)
	retriver.shutdownChan <- struct{}{}

	wg.Wait()

	after := time.Now()

	if after.Sub(before).Seconds() >= 3 {
		t.Error("long timeout")
	}
}
