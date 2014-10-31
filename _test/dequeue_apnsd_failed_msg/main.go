package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"log"
	"net"

	"github.com/garyburd/redigo/redis"
	"github.com/makeitreal/apnsd"
	"github.com/ugorji/go/codec"
)

var (
	redisKey     = flag.String("redisKey", "APNSD:FAILED_MSG_QUEUE", "redisKey")
	redisNetwork = flag.String("redisNetwork", "tcp", "redisNetwork")
	redisAddr    = flag.String("redisAddr", net.JoinHostPort("127.0.0.1", "6379"), "redisAddr")
)

func main() {
	flag.Parse()

	if err := _main(); err != nil {
		log.Fatal(err)
	}
}

func _main() error {
	conn, err := redis.Dial(*redisNetwork, *redisAddr)
	defer conn.Close()

	if err != nil {
		return err
	}

	reply, err := redis.Values(conn.Do("BRPOP", *redisKey, "30"))

	if err != nil {
		if err == redis.ErrNil {
			return nil
		} else {
			return err
		}
	}

	var key string
	var byt []byte
	if _, err := redis.Scan(reply, &key, &byt); err != nil {
		return err
	}

	var f apnsd.FailedMsg
	if err := codec.NewDecoder(bytes.NewReader(byt), &codec.MsgpackHandle{}).Decode(&f); err != nil {
		return err
	}

	log.Printf("token:%s payload:%s expire:%d priority:%d identifier:%d status:%d", hex.EncodeToString(f.Token), f.Payload, f.Expire, f.Priority, f.Identifier, f.Status)

	return nil
}
