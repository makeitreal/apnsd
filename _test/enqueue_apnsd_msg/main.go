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
	token        = flag.String("token", "", "token")
	body         = flag.String("body", "hello!", "body")
	badge        = flag.Int("badge", 1, "badge")
	redisKey     = flag.String("redisKey", "APNSD:MSG_QUEUE", "redisKey")
	redisNetwork = flag.String("redisNetwork", "tcp", "redisNetwork")
	redisAddr    = flag.String("redisAddr", net.JoinHostPort("127.0.0.1", "6379"), "redisAddr")
)

func main() {
	flag.Parse()

	hexToken, err := hex.DecodeString(*token)
	if err != nil {
		log.Fatal(err)
	}

	p := apnsd.NewPayload(&apnsd.Aps{
		Alert: &apnsd.Alert{
			Body: apnsd.String(*body),
		},
		Badge: apnsd.Int(*badge),
	})

	msg, err := apnsd.NewMsg(hexToken, p, 0, 10, 0)
	if err != nil {
		log.Println(err)
	}

	var b bytes.Buffer
	if err := codec.NewEncoder(&b, &codec.MsgpackHandle{}).Encode(msg); err != nil {
		log.Fatal(err)
	}

	conn, err := redis.Dial(*redisNetwork, *redisAddr)

	if err != nil {
		log.Fatal(err)
	}

	if _, err := conn.Do("LPUSH", *redisKey, b.Bytes()); err != nil {
		log.Fatal(err)
	}
}
