package middleware

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/message"
)

type Options []Option

func (ops *Options) Apply(option Option) {
	*ops = append(*ops, option)
}

type Option interface {
	// Apply 返回bool true: 表示后面的非空error是可忽略的错误，不影响后面的中间件执行
	Apply(message message.Message) (canSkipErr bool, err error)
}

type console struct {
}

func (c *console) Apply(msg message.Message) (bool, error) {
	fmt.Println(msg.Name())
	return true, nil
}

func WithConsole() Option {
	return &console{}
}
