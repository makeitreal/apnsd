package main

import (
	"encoding/hex"
	"flag"
	"log"
	"net"

	"github.com/garyburd/redigo/redis"
	"github.com/makeitreal/apnsd"
	"github.com/makeitreal/apnsd/apns"
)

var (
	token        = flag.String("token", "", "token")
	body         = flag.String("body", "hello!", "body")
	badge        = flag.Int("badge", 1, "badge")
	redisKey     = flag.String("redisKey", "", "redisKey")
	redisNetwork = flag.String("redisNetwork", "tcp", "redisNetwork")
	redisAddr    = flag.String("redisAddr", net.JoinHostPort("127.0.0.1", "6379"), "redisAddr")
)

func main() {
	flag.Parse()

	hexToken, err := hex.DecodeString(*token)
	if err != nil {
		log.Fatal(err)
	}

	msg := &apns.Msg{
		Token:    hexToken,
		Priority: 10,
		Expire:   0,
		Payload: apns.Payload{
			"aps": &apns.Aps{
				Alert: &apns.Alert{
					Body: apns.String(*body),
				},
				Badge: apns.Int(*badge),
			},
		},
	}

	byt, err := apnsd.EncodeMsg(msg)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := redis.Dial(*redisNetwork, *redisAddr)

	if err != nil {
		log.Fatal(err)
	}

	if _, err := conn.Do("LPUSH", *redisKey, byt); err != nil {
		log.Fatal(err)
	}
}
