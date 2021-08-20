package logger

import (
	"bytes"
	logs2 "gitee.com/Ljolan/si-mqtt/logger/logs"
	"github.com/buguang01/util"
)

// buffer holds a byte Buffer for reuse. The zero value is ready for use.
type buffer struct {
	bytes.Buffer
	tmp  [64]byte // temporary byte array for creating headers.
	next *buffer
}

var Logger *logs2.AdamLog

func LogInit(level string) {
	util.SetLocation(util.BeiJing)
	logs2.LogInit(level)
	Logger = logs2.GetLogger()
}
