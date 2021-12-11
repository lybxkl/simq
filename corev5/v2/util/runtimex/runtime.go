package runtimex

import (
	"gitee.com/Ljolan/si-mqtt/logger"
)

func Recover() {
	if err := recover(); err != nil {
		logger.Logger.Errorf("recover: %+v", err)
	}
}
