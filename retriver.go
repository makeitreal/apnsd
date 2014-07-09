package apnsd

import (
	"fmt"
	"log"
	"sync"

	"github.com/garyburd/redigo/redis"
	"github.com/makeitreal/apnsd/apns"
)

type Retriver struct {
	c            chan *apns.Msg
	shutdownChan chan struct{}
	key          string
	timeout      string
	redisConn    redis.Conn
	redisNetwork string
	redisAddr    string
	m            sync.Mutex
}

func (r *Retriver) Name() string {
	return "retriver"
}

//TODO: redis reconnect with sleep when EOF recieved
func (r *Retriver) Start() error {

	defer r.closeRedisConn()

	go func() {
		<-r.shutdownChan
		r.log("recieved shutdown chan")
		r.log("close connection", r.closeRedisConn())
	}()

	for {
		msg, err := r.retrive()
		if err != nil {
			r.log("retrive err", err)
			return err
		}
		if msg == nil {
			continue
		}
		r.log("decoded msg and send to channel", msg)
		r.c <- msg
	}
}

func (r *Retriver) setRedisConn() error {
	defer r.m.Unlock()
	r.m.Lock()

	if r.redisConn == nil {
		conn, err := redis.Dial(r.redisNetwork, r.redisAddr)
		if err != nil {
			return err
		}

		r.log("set redis conn")
		r.redisConn = conn
		return nil
	}

	if err := r.redisConn.Err(); err != nil {
		r.redisConn = nil
		return err
	}

	return nil
}

func (r *Retriver) closeRedisConn() error {
	r.log("close redis conn")
	defer r.m.Unlock()
	r.m.Lock()

	if r.redisConn == nil {
		return nil
	}

	return r.redisConn.Close()
}

//TODO: when recieved shutdown, force close stop connection.
// current implementation, wait to finish blocking command  when call close to pooled connection.
func (r *Retriver) retrive() (*apns.Msg, error) {
	if err := r.setRedisConn(); err != nil {
		return nil, err
	}

	r.log("BRPOP", r.key, r.timeout)
	reply, err := redis.Values(r.redisConn.Do("BRPOP", r.key, r.timeout))
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

func (r *Retriver) log(v ...interface{}) {
	log.Println("[retriver]", fmt.Sprint(v))
}
