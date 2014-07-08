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
	configFileName = flag.String("config", "config.json", "config fo apnsd")
)

func main() {
	flag.Parse()

	config, err := NewConfig(*configFileName)
	if err != nil {
		log.Fatal("[apnsd]", "config load error:", err, *configFileName)
	}

	log.Println(config)
	cer, err := tls.LoadX509KeyPair(
		config.Certificate.Cer,
		config.Certificate.Key,
	)
	if err != nil {
		log.Fatal("[apnsd]", "load key error:", err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	shutdownChan := make(chan struct{}, 1)

	go func() {
		<-signalChan
		log.Println("[apnsd]", "signal recieved")
		shutdownChan <- struct{}{}
	}()

	c := &apnsd.Client{
		MsgBufferNum: config.Client.Buffer,
		ShutdownChan: shutdownChan,

		Certificates:         []tls.Certificate{cer},
		SenderNum:            config.Sender.Num,
		ApnsAddr:             net.JoinHostPort(config.Apns.Host, config.Apns.Port),
		SenderErrorTimeout:   time.Second * time.Duration(config.Sender.ErrorTimeout),
		SenderReconnectSleep: time.Second * time.Duration(config.Sender.ReconnectSleep),

		RetriverNum:     config.Retriver.Num,
		RetriverKey:     config.Redis.Key,
		RetriverTimeout: config.Redis.Timeout,

		RedisMaxIdle:     config.Redis.MaxIdle,
		RedisIdleTimeout: time.Second * time.Duration(config.Redis.IdleTimeout),
		RedisNetwork:     config.Redis.Network,
		RedisAddr:        net.JoinHostPort(config.Redis.Host, config.Redis.Port),
	}

	log.Fatal("[apnsd]", c.Start())
}
