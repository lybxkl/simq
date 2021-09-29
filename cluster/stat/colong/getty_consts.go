package colong

import (
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
	"github.com/panjf2000/ants/v2"
	"io"
	"sync"
)

const (
	CronPeriod      = 20e9
	WritePkgTimeout = 1e8

	AllNodeName = "@all" // TODO 表示所有节点都发送失败使用的名称，所以节点名称不要使用此
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
	cs            = &sync.Map{} // nodename --> sync.Map( remoteaddr --> session)
	Cname         = "name"
	CremoteIp     = "remote"
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
func InitClusterTaskPool(poolSize int) (close io.Closer) {
	if poolSize < 1000 {
		poolSize = 1000
	}
	taskGPool, _ = ants.NewPool(poolSize, ants.WithPanicHandler(func(i interface{}) {
		fmt.Println("协程池处理错误：", i)
	}), ants.WithMaxBlockingTasks(poolSize*10))
	taskGPoolSize = poolSize
	return &closer{}
}

type closer struct {
}

func (closer closer) Close() error {
	taskGPool.Release()
	return nil
}
func dealAntsErr(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, ants.ErrPoolClosed) {
		fmt.Println("协程池错误：", err.Error())
		taskGPool.Reboot()
	} else if errors.Is(err, ants.ErrPoolOverload) {
		fmt.Println("协程池超载,进行扩容：", err.Error())
		// TODO 需要缩
		taskGPool.Tune(int(float64(taskGPoolSize) * 1.25))
	} else {
		fmt.Println("线程池处理异常：", err)
	}
}
