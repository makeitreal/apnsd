package main

import (
	"crypto/tls"
	"encoding/hex"
	"flag"
	"log"
	"net"
	"sync"

	"github.com/makeitreal/apnsd/apns"
)

var (
	cerFile = flag.String("cer", "", "cer")
	keyFile = flag.String("key", "", "key")
	token   = flag.String("token", "", "token")
)

func main() {
	flag.Parse()

	cert, err := tls.LoadX509KeyPair(*cerFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := tls.Dial("tcp", net.JoinHostPort(apns.SandboxGateway, apns.Port), &tls.Config{
		Certificates: []tls.Certificate{cert},
	})

	if err != nil {
		log.Fatal(err)
	}

	connection := apns.NewConnection(conn, true)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		errMsg, err := connection.ReadErrorMsg()
		if err != nil {
			log.Fatal("read error: ", err)
		}
		log.Println(errMsg, err)
	}()

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
					Body: apns.String("hello!"),
				},
				Badge: apns.Int(1),
			},
		},
		Identifier: 1,
	}
	if err := connection.WriteMsg(msg); err != nil {
		log.Fatal("send error", err)
	}
	wg.Wait()
}
