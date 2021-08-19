package logger

import (
	"bytes"
	"gitee.com/Ljolan/si-mqtt/corev5/logger/logs"
	"github.com/buguang01/util"
)

// buffer holds a byte Buffer for reuse. The zero value is ready for use.
type buffer struct {
	bytes.Buffer
	tmp  [64]byte // temporary byte array for creating headers.
	next *buffer
}

var Logger *logs.AdamLog

func LogInit(level string) {
	util.SetLocation(util.BeiJing)
	logs.LogInit(level)
	Logger = logs.GetLogger()
}
