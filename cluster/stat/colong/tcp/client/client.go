package client

import (
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/tcp"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	getty "github.com/apache/dubbo-getty"
)

import (
	"github.com/dubbogo/gost/sync"
)

type Client struct {
	serverName string
	c          getty.Client
	taskPool   gxsync.GenericTaskPool
}

func (this *Client) Close() {
	this.c.Close()
	colong.RemoveSession(this.serverName)
}

func RunClient(curName, serverName string, serverAddr string, connectNum int, taskPoolMode bool, taskPoolSize int) *Client {
	util.SetLimit()

	//util.Profiling(*pprofPort)

	cl := &Client{
		serverName: serverName,
	}
	if taskPoolMode {
		cl.taskPool = gxsync.NewTaskPoolSimple(taskPoolSize)
	}

	client := getty.NewTCPClient(
		getty.WithServerAddress(serverAddr),
		getty.WithConnectionNumber(connectNum),
		getty.WithClientTaskPool(cl.taskPool),
		getty.WithReconnectInterval(30000),
	)

	client.RunEventLoop(newHelloClientSession(curName, serverName))

	cl.c = client

	//util.WaitCloseSignals(client)
	//colong.RemoveSession(serverName)
	//taskPool.Close()
	return cl
}

// NewHelloClientSession use for init client session
func newHelloClientSession(curName, nodeName string) func(session getty.Session) error {
	return func(session getty.Session) error {
		colong.SetSessionOnOpen(tcp.EventListener, func(name string, session getty.Session) {
			colong.AddSession(nodeName, session)
		})
		colong.SetCurName(tcp.EventListener, curName)
		err := tcp.InitialSession(nodeName, session)
		if err != nil {
			return err
		}
		return nil
	}
}
