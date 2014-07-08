package apnsd

import (
	"crypto/tls"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/makeitreal/apnsd/apns"
)

type Starter interface {
	Name() string
	Start() error
}

type Client struct {
	MsgBufferNum int
	ShutdownChan chan struct{}

	// sender
	Certificates         []tls.Certificate
	SenderNum            int
	ApnsAddr             string
	SenderErrorTimeout   time.Duration
	SenderReconnectSleep time.Duration

	// retriver
	RetriverNum     int
	RetriverKey     string
	RetriverTimeout string

	// redis
	RedisMaxIdle     int
	RedisIdleTimeout time.Duration
	RedisNetwork     string
	RedisAddr        string
}

func (c *Client) Start() int {

	identifier := &Identifier{}
	msgChan := make(chan *apns.Msg, c.MsgBufferNum)
	shutdownChan := make(chan struct{}, 0)

	starters := make([]Starter, 0)

	for i := 0; i < c.SenderNum; i++ {
		starters = append(starters, &Sender{
			c: msgChan,
			tlsConfig: &tls.Config{
				Certificates: c.Certificates,
			},
			apnsAddr:       c.ApnsAddr,
			identifier:     identifier,
			errorTimeout:   c.SenderErrorTimeout,
			reconnectSleep: c.SenderReconnectSleep,
			shutdownChan:   shutdownChan,
		})
	}

	redisPool := c.redisPool()
	defer redisPool.Close()

	for i := 0; i < c.RetriverNum; i++ {
		starters = append(starters, &Retriver{
			c:            msgChan,
			redisPool:    redisPool,
			shutdownChan: shutdownChan,
			key:          c.RetriverKey,
			timeout:      c.RetriverTimeout,
		})
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(starters)) // non blocking notify

	//TODO: sender should die after all retrivers are died
	for _, starter := range starters {
		wg.Add(1)
		go func(starter Starter) {
			defer wg.Done()
			c.log("starter", starter.Name(), "start")
			if err := starter.Start(); err != nil {
				c.log("starter", starter.Name(), "receive err:", err)
				errorChan <- err
			}
			c.log("starter", starter.Name(), "is died")
		}(starter)
	}

	select {
	case <-errorChan:
		c.log("error chan recieved")
	case <-c.ShutdownChan:
		c.log("shutdownchan received notify")
	}

	close(shutdownChan)

	wg.Wait()

	c.log("all starter are died")

	return 1
}

func (c *Client) redisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     c.RedisMaxIdle,
		IdleTimeout: c.RedisIdleTimeout,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(c.RedisNetwork, c.RedisAddr)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func (s *Client) log(v ...interface{}) {
	log.Println("[client]", fmt.Sprint(v))
}
