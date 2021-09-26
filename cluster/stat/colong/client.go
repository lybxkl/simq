package colong

import (
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	getty "github.com/apache/dubbo-getty"
)

import (
	"github.com/dubbogo/gost/sync"
)

type Client struct {
	serverName string
	getty.Client
	taskPool gxsync.GenericTaskPool
	*Pool
}

func (this *Client) Close() {
	RemoveSession(this.serverName)
	this.Client.Close()
	this.Pool.Close()
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
	cl.Pool = NewPool(connectNum*2, 20)
	client := getty.NewTCPClient(
		getty.WithServerAddress(serverAddr),
		getty.WithConnectionNumber(connectNum),
		getty.WithClientTaskPool(cl.taskPool),
		getty.WithReconnectInterval(30000),
	)

	client.RunEventLoop(cl.newHelloClientSession(curName, serverName))

	cl.Client = client

	AddSession(serverName, cl)
	//util.WaitCloseSignals(client)
	//colong.RemoveSession(serverName)
	//taskPool.Close()
	return cl
}

// NewHelloClientSession use for init client session
func (this *Client) newHelloClientSession(curName, nodeName string) func(session getty.Session) error {
	return func(session getty.Session) error {
		SetSessionOnOpen(EventListener, func(name string, session getty.Session) {
			this.Put(session)
		})
		SetCurName(EventListener, curName)
		err := InitialSession(nodeName, session)
		if err != nil {
			return err
		}
		return nil
	}
}
