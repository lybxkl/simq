package main

import (
	"flag"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/tls"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	"path/filepath"
)

import (
	gxsync "github.com/dubbogo/gost/sync"
)

import (
	"github.com/apache/dubbo-getty"
)

var (
	taskPoolMode = flag.Bool("taskPool", false, "task pool mode")
	taskPoolSize = flag.Int("task_pool_size", 2000, "task poll size")
	pprofPort    = flag.Int("pprof_port", 65432, "pprof http port")
	Sessions     []getty.Session
)

var taskPool gxsync.GenericTaskPool

func main() {
	flag.Parse()

	util.SetLimit()

	util.Profiling(*pprofPort)
	serverPemPath, _ := filepath.Abs("./demo/colong/tls/certs/server0.pem")
	serverKeyPath, _ := filepath.Abs("./demo/colong/tls/certs/server0.key")
	caPemPath, _ := filepath.Abs("./demo/colong/tls/certs/ca.pem")

	c := &getty.ServerTlsConfigBuilder{
		ServerKeyCertChainPath:        serverPemPath,
		ServerPrivateKeyPath:          serverKeyPath,
		ServerTrustCertCollectionPath: caPemPath,
	}

	if *taskPoolMode {
		taskPool = gxsync.NewTaskPoolSimple(*taskPoolSize)
	}
	options := []getty.ServerOption{
		getty.WithLocalAddress(":8090"),
		getty.WithServerSslEnabled(true),
		getty.WithServerTlsConfigBuilder(c),
		getty.WithServerTaskPool(taskPool),
	}

	server := getty.NewTCPServer(options...)

	go server.RunEventLoop(NewHelloServerSession)
	util.WaitCloseSignals(server)
}

func NewHelloServerSession(session getty.Session) (err error) {
	err = tls.InitialSessionServer(session)
	Sessions = append(Sessions, session)
	if err != nil {
		return
	}

	return
}
