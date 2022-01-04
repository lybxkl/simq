package service

import (
	config2 "gitee.com/Ljolan/si-mqtt/config"
	"testing"
)

func TestServer(t *testing.T) {
	cfg := config2.GetConfig()
	svc := NewServer(cfg)
	svc.ListenAndServeByGetty(":1883", 1000)
	select {}
}
