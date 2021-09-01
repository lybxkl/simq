package colong

import (
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
	"github.com/panjf2000/ants/v2"
	"sync"
)

const (
	CronPeriod      = 20e9
	WritePkgTimeout = 1e8
)

var log = getty.GetLogger()

func SetLoggerLevelInfo() {
	getty.SetLoggerLevel(getty.LoggerLevelInfo)
}

// 更新getty内部日志，虽然都是zap
func UpdateLogger(lg getty.Logger) {
	getty.SetLogger(lg)
	log = getty.GetLogger()
}

var (
	pingresp      []byte
	ack           []byte
	cs            sync.Map
	Cname         = "name"
	Caddr         = "addr"
	sharePrefix   = []byte("$share/")
	taskGPool     *ants.Pool
	taskGPoolSize = 1000
)

func init() {
	ps := messagev5.NewPingrespMessage()
	pingresp, _ = wrapperPub(ps)

	ackM := messagev5.NewConnackMessage()
	ackM.SetReasonCode(messagev5.Success)
	ack, _ = wrapperPub(ackM)

}
func submit(f func()) {
	dealAntsErr(taskGPool.Submit(f))
}
func InitClusterTaskPool(poolSize int) (close func()) {
	if poolSize < 100 {
		poolSize = 100
	}
	taskGPool, _ = ants.NewPool(poolSize, ants.WithPanicHandler(func(i interface{}) {
		fmt.Println("协程池处理错误：", i)
	}), ants.WithMaxBlockingTasks(poolSize*2))
	taskGPoolSize = poolSize
	return closeTaskPool
}
func closeTaskPool() {
	taskGPool.Release()
}
func dealAntsErr(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, ants.ErrPoolClosed) {
		fmt.Println("协程池错误：", err.Error())
		taskGPool.Reboot()
	}
	if errors.Is(err, ants.ErrPoolOverload) {
		fmt.Println("协程池超载：", err.Error())
		taskGPool.Tune(int(float64(taskGPoolSize) * 1.25))
	}
	fmt.Println("线程池处理异常：", err)
}
