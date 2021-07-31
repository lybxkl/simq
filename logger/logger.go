package logger

import (
	"SI-MQTT/config"
	"SI-MQTT/logger/logs"
	"bytes"
	"fmt"
	"github.com/buguang01/util"
)

// buffer holds a byte Buffer for reuse. The zero value is ready for use.
type buffer struct {
	bytes.Buffer
	tmp  [64]byte // temporary byte array for creating headers.
	next *buffer
}

var (
	infoOpen  = true
	debugOpen = true
)

var Logger *logs.AdamLog

func init() {
	consts := config.ConstConf.Logger
	infoOpen = consts.InfoOpen
	debugOpen = consts.DebugOpen
	util.SetLocation(util.BeiJing)
	if !infoOpen {
		fmt.Println("日志系统 未开启 info日志打印记录")
	}
	if !debugOpen {
		fmt.Println("日志系统 未开启 debug日志打印记录")
	}
	if consts.LogPath != "" {

	}
	Logger = logs.GetLogger()
}
