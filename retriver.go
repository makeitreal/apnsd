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
	redisPool    *redis.Pool
	shutdownChan chan struct{}
	isShutdown   bool
	key          string
	timeout      string
	redisConn    redis.Conn
	m            sync.Mutex
}

func (r *Retriver) Name() string {
	return "retriver"
}

func (r *Retriver) Start() error {

	go func() {
		<-r.shutdownChan
		r.log("recieved shutdown chan")
		r.shutdown()
		if r.redisConn != nil {
			err := r.redisConn.Close()
			r.log(err)
		}
	}()

	for {
		if r.isShutdown {
			return nil
		}
		msg, err := r.retrive()
		if err != nil {
			r.log(err)
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
	r.log("set redis conn")
	defer r.m.Unlock()
	r.m.Lock()

	conn := r.redisPool.Get()
	if err := conn.Err(); err != nil {
		conn.Close()
		return err
	}

	r.redisConn = conn
	return nil
}

func (r *Retriver) closeRedisConn() error {
	r.log("close redis conn")
	defer r.m.Unlock()
	r.m.Lock()

	if r.redisConn != nil {
		err := r.redisConn.Close()
		r.redisConn = nil
		return err
	}

	return nil
}

//TODO: when recieved shutdown, force close stop connection.
// current implementation, wait to finish blocking command  when call close to pooled connection.
func (r *Retriver) retrive() (*apns.Msg, error) {
	if err := r.setRedisConn(); err != nil {
		return nil, err
	}

	defer r.closeRedisConn()

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

	return DecodeMsg(byt)
}

func (r *Retriver) shutdown() {
	defer r.m.Unlock()
	r.m.Lock()

	r.isShutdown = true
}

func (r *Retriver) log(v ...interface{}) {
	log.Println("[retriver]", fmt.Sprint(v))
}
