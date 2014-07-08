package apnsd

import (
	"reflect"
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

func testRedisPool(socket string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: time.Second * 30,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("unix", socket)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func testRetriver() (*Retriver, *redistest.Server, error) {
	redisserver, err := redistest.NewServer(true, redistest.Config{})

	if err != nil {
		return nil, nil, err
	}

	msgChan := make(chan *apns.Msg, 10)
	shutdownChan := make(chan struct{}, 0)

	retriver := &Retriver{
		c:            msgChan,
		redisPool:    testRedisPool(redisserver.Config["unixsocket"]),
		shutdownChan: shutdownChan,
		key:          Key,
		timeout:      "3",
	}

	return retriver, redisserver, nil
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
		time.Sleep(time.Second * 2)
		retriverErr = retriver.Start()
	}()

	retriver.shutdownChan <- struct{}{}
	wg.Wait()

	if retriverErr != nil {
		t.Error("if recieved shutdown chan, err should be nil", retriverErr)
	}
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

	conn := retriver.redisPool.Get()
	if err := conn.Err(); err != nil {
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
				Badge: apns.Int(1),
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
		if !reflect.DeepEqual(orgMsg, newMsg) {
			t.Log("orgMsg:", orgMsg)
			t.Log("newMsg:", newMsg)
			t.Error("org msg and new msg is not same")
		}
	}()

	retriver.shutdownChan <- struct{}{}
	wg.Wait()
}

func TestRetriverCancelByShutdown(t *testing.T) {
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

	conn := retriver.redisPool.Get()
	if err := conn.Err(); err != nil {
		t.Fatal(err)
	}

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
