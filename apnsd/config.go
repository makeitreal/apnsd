package main

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	Certificate struct {
		Key string
		Cer string
	}
	Client struct {
		Buffer int
	}
	Sender struct {
		Num            int
		ErrorTimeout   int
		ReconnectSleep int
	}
	Apns struct {
		Host string
		Port string
	}
	Retriver struct {
		Num int
	}
	Redis struct {
		Key         string
		Timeout     string
		Network     string
		Host        string
		Port        string
		MaxIdle     int
		IdleTimeout int
	}
}

func NewConfig(filename string) (*Config, error) {
	byt, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c := &Config{}
	if err := json.Unmarshal(byt, &c); err != nil {
		return nil, err
	}

	return c, nil
}

//TODO: validate
//func (c *Config) Validate() error {
//}
