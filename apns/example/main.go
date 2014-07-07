package main

import (
	"crypto/tls"
	"encoding/hex"
	"flag"
	"github.com/makeitreal/apnsd/apns"
	"log"
	"net"
	"sync"
)

var (
	certFile = flag.String("cert", "", "cert")
	keyFile  = flag.String("key", "", "key")
	token    = flag.String("token", "", "token")
)

func main() {
	flag.Parse()

	cer, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := tls.Dial("tcp", net.JoinHostPort(apns.SandboxGateway, apns.Port), &tls.Config{
		Certificates: []tls.Certificate{cer},
	})

	if err != nil {
		log.Fatal(err)
	}

	connection := apns.NewConnection(conn)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		errMsg, err := connection.ReadError()
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
	if err := connection.Write(msg); err != nil {
		log.Fatal("send error", err)
	}
	wg.Wait()
}
