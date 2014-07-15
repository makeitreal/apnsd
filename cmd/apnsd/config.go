package main

import (
	"encoding/json"
	"errors"
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
		Num             int
		ShutdownTimeout int
	}
	Redis struct {
		Key            string
		BrpopTimeout   string
		ReconnectSleep int
		DialTimeout    int
		Network        string
		Host           string
		Port           string
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

func (c *Config) Validate() error {
	if c.Client.Buffer < 1 {
		return errors.New("Client.Buffer should more than 0")
	}

	if c.Sender.Num < 1 {
		return errors.New("Sender.Num should more than 0")
	}

	if c.Sender.ErrorTimeout < 1 {
		return errors.New("Sender.ErrorTimeout should more than 0")
	}

	if c.Sender.ReconnectSleep < 1 {
		return errors.New("Sender.ReconnectSleep should more than 0")
	}

	if c.Apns.Host == "" {
		return errors.New("Apns.Host should not empty")
	}

	if c.Apns.Port == "" {
		return errors.New("Apns.Port should not empty")
	}

	if c.Retriver.Num < 1 {
		return errors.New("Retriver.Num should more than 0")
	}

	if c.Retriver.ShutdownTimeout < 1 {
		return errors.New("Retriver.ShutdownTimeout should more than 0")
	}

	if c.Redis.Key == "" {
		return errors.New("Redis.Key should not empty")
	}

	if c.Redis.BrpopTimeout == "" {
		return errors.New("Redis.BrpopTimeout should not empty")
	}

	if c.Redis.DialTimeout < 1 {
		return errors.New("Redis.DialTimeout should more than 0")
	}

	if c.Redis.ReconnectSleep < 1 {
		return errors.New("Redis.ReconnectSleep should more than 0")
	}

	if c.Redis.Network == "" {
		return errors.New("Redis.Network should not empty")
	}

	if c.Redis.Port == "" {
		return errors.New("Redis.Port should not empty")
	}

	return nil
}
