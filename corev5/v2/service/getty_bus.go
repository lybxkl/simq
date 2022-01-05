package service

import (
	"gitee.com/Ljolan/si-mqtt/logger"
	"github.com/panjf2000/ants/v2"
)

var busTaskPool *ants.Pool

func newBus(poolSize int) error {
	var err error
	busTaskPool, err = ants.NewPool(poolSize, ants.WithPanicHandler(func(recover interface{}) {
		if er, ok := recover.(error); ok {
			logger.Logger.Error(er.Error())
		}
	}), ants.WithMaxBlockingTasks(poolSize*5), ants.WithNonblocking(true), ants.WithPreAlloc(false))
	if err != nil {
		return err
	}
	return nil
}

func submitInitEvTask(f func()) {
	dealAntsErr(busTaskPool.Submit(f))
}
