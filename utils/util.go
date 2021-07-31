package utils

import (
	"os"
	"strings"
)

const (
	addAck byte = iota //添加topic
	delAck             //删除topic
)

var TopicAddTag = addAck
var TopicDelTag = delAck
var MQTTMsgAck byte = 0x99

/*
获取程序运行路径
*/
func GetCurrentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}
