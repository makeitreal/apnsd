package main

import (
	"testing"
)

func TestConfigSampleJson(t *testing.T) {
	c, err := NewConfig("./config.json.sample")
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Validate(); err != nil {
		t.Error("config.json.sample is broken", err)
	}
}
