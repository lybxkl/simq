package main

import (
	"flag"
	"gitee.com/Ljolan/si-mqtt/colang/stat/colong"
	"gitee.com/Ljolan/si-mqtt/colang/stat/colong/tcp"
	"gitee.com/Ljolan/si-mqtt/colang/stat/util"
	getty "github.com/apache/dubbo-getty"
)

import (
	"github.com/dubbogo/gost/sync"
)

var (
	ip          = flag.String("ip", "127.0.0.1", "server IP")
	connections = flag.Int("conn", 1, "number of tcp connections")

	taskPoolMode = flag.Bool("taskPool", false, "task pool mode")
	taskPoolSize = flag.Int("task_pool_size", 2000, "task poll size")
	pprofPort    = flag.Int("pprof_port", 65431, "pprof http port")
)

var taskPool gxsync.GenericTaskPool

func main() {
	flag.Parse()

	util.SetLimit()

	util.Profiling(*pprofPort)

	if *taskPoolMode {
		taskPool = gxsync.NewTaskPoolSimple(*taskPoolSize)
	}

	client := getty.NewTCPClient(
		getty.WithServerAddress(*ip+":8090"),
		getty.WithConnectionNumber(*connections),
		getty.WithClientTaskPool(taskPool),
	)

	client.RunEventLoop(NewHelloClientSession)

	go colong.ClientRequest()

	util.WaitCloseSignals(client)
	taskPool.Close()
}

// NewHelloClientSession use for init client session
func NewHelloClientSession(session getty.Session) (err error) {
	tcp.EventListener.SessionOnOpen = func(session getty.Session) {
		colong.Sessions = append(colong.Sessions, session)
	}
	err = tcp.InitialSession(session)
	if err != nil {
		return
	}
	return
}
