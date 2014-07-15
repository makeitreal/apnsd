package apnsd

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/makeitreal/apnsd/apns"
)

type Retriver struct {
	c                   chan *apns.Msg
	shutdownChan        chan struct{}
	isShutdown          bool
	key                 string
	shutdownTimeout     time.Duration
	redisNetwork        string
	redisAddr           string
	redisDialTimeout    time.Duration
	redisReconnectSleep time.Duration
	redisBrpopTimeout   string
	connectCount        int
	m                   sync.Mutex
}

func (r *Retriver) Name() string {
	return "retriver"
}

//TODO: redis reconnect with sleep when EOF recieved
func (r *Retriver) Start() error {

	var reconnectChan <-chan time.Time
	var shutdownTimeoutChan <-chan time.Time

	var redisConn redis.Conn
	defer func() {
		if redisConn != nil {
			redisConn.Close()
		}
	}()

	redisConnChan := make(chan redis.Conn, 1)
	redisErrChan := make(chan error, 1)
	launchChan := make(chan struct{}, 1)

	launchChan <- struct{}{}

	for {
		select {
		case <-r.shutdownChan:
			r.log("recieved shutdown")
			r.shutdown()
			if redisConn != nil {
				return errors.New("recieved shutdown")
				redisConn.Close()
				redisConn = nil
			}
			//TODO: add option
			shutdownTimeoutChan = time.After(r.shutdownTimeout)
		case <-shutdownTimeoutChan:
			r.log("detect long shutdown.... try force shutdown")
			return errors.New("force shutdown")
		case err := <-redisErrChan:
			r.log("got redis err:", err)
			if r.isShutdown {
				return err
			} else {
				if redisConn != nil {
					if _, err := redisConn.Do("PING"); err != nil {
						r.log("seems to be disconnected. reconnect after", r.redisReconnectSleep)
						reconnectChan = time.After(r.redisReconnectSleep)
					} else {
						// seems to be redis command err
						r.log("seems to be redis command err. try soon without reconnecting")
						redisConnChan <- redisConn
					}
				} else {
					r.log("does not have redis connection. reconect after", r.redisReconnectSleep)
					reconnectChan = time.After(r.redisReconnectSleep)
				}
			}
		case <-reconnectChan:
			r.log("reconnect")
			launchChan <- struct{}{}
		case <-launchChan:
			r.log("launch. get connection...")
			go r.getRedisConn(redisConnChan, redisErrChan)
		case conn := <-redisConnChan:
			r.log("start retrive loop")
			redisConn = conn
			go r.retriveLoop(conn, redisErrChan)
		}
	}
}

func (r *Retriver) getRedisConn(redisConnChan chan redis.Conn, errChan chan error) {
	//TODO: option for dialtimeout. timeout should be less than shutdown timeout
	conn, err := redis.DialTimeout(r.redisNetwork, r.redisAddr, time.Second, 0, 0)
	if err != nil {
		errChan <- err
		return
	}

	if err := conn.Err(); err != nil {
		errChan <- err
		return
	}

	r.incrConnectCount()
	redisConnChan <- conn
}

func (r *Retriver) shutdown() {
	r.log("shutdown")
	defer r.m.Unlock()
	r.m.Lock()

	r.isShutdown = true
}

func (r *Retriver) incrConnectCount() {
	defer r.m.Unlock()
	r.m.Lock()
	r.connectCount++
}

func (r *Retriver) retrive(conn redis.Conn) (*apns.Msg, error) {

	r.log("BRPOP", r.key, r.redisBrpopTimeout)
	reply, err := redis.Values(conn.Do("BRPOP", r.key, r.redisBrpopTimeout))
	if err == redis.ErrNil {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var key string
	var byt []byte
	if _, err := redis.Scan(reply, &key, &byt); err != nil {
		return nil, err
	}

	return apns.DecodeMsg(byt)
}

func (r *Retriver) retriveLoop(conn redis.Conn, retriveErrChan chan error) {
	for {
		msg, err := r.retrive(conn)
		if err != nil {
			retriveErrChan <- err
			return
		}
		if msg == nil {
			continue
		}

		//TODO: block ok ?
		r.log("send to channel", msg)
		r.c <- msg
	}
}

func (r *Retriver) log(v ...interface{}) {
	log.Println("[retriver]", fmt.Sprint(v))
}
