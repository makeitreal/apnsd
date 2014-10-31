package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/makeitreal/apnsd"
)

var (
	prod = flag.Bool("prod", false, "connect to prod apns server")

	redisKeyName       = flag.String("redisKeyName", "", "key name of redis")
	redisFailedKeyName = flag.String("redisFailedKeyName", "", "failed key name of redis")

	redisNetwork = flag.String("redisNetwork", "tcp", "network of redis")
	redisAddr    = flag.String("redisAddr", "127.0.0.1:6379", "address of redis")

	apnsCer = flag.String("apnsCer", "", "path to apns cer file")
	apnsKey = flag.String("apnsKey", "", "path to apns key file")

	numOfRetriver = flag.Int("numOfRetriver", 1, "num of retriver")
	numOfSender   = flag.Int("numOfSender", 1, "num of sender")
)

func main() {
	flag.Parse()

	cer, err := tls.LoadX509KeyPair(*apnsCer, *apnsKey)
	if err != nil {
		log.Fatal("load certificate err:", err)
	}

	a := apnsd.NewApnsd(func() (net.Conn, error) {
		var addr string
		if *prod {
			addr = net.JoinHostPort(apnsd.ApnsProductionGateway, apnsd.ApnsPort)
		} else {
			addr = net.JoinHostPort(apnsd.ApnsSandboxGateway, apnsd.ApnsPort)
		}
		dialer := &net.Dialer{
			Timeout: time.Second * 5,
		}
		return tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{Certificates: []tls.Certificate{cer}})
	})

	if *redisKeyName != "" {
		a.RetriveKey = *redisKeyName
	}

	if *redisFailedKeyName != "" {
		a.SenderFailedMsgKey = *redisFailedKeyName
	}

	if *redisNetwork != "" {
		a.RetriverRedisNetwork = *redisNetwork
	}

	if *redisAddr != "" {
		a.RetriverRedisAddr = *redisAddr
	}

	if *numOfRetriver != 0 {
		a.RetriverNum = *numOfRetriver
	}

	if *numOfSender != 0 {
		a.SenderNum = *numOfSender
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	go func() {
		sig := <-signalChan
		log.Println("got signal:", sig)
		a.Shutdown()
	}()

	a.Start()
}
