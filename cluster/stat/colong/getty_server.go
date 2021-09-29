package colong

import (
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	getty "github.com/apache/dubbo-getty"
)

import (
	gxsync "github.com/dubbogo/gost/sync"
)

type server struct {
	name string
	s    getty.Server
}

func (this *server) Close() {
	this.s.Close()
}

var taskPool gxsync.GenericTaskPool

// RunClusterGettyServer 提供集群节点连接服务
func RunClusterGettyServer(name string, addr string, clusterInToPub ClusterInToPub,
	clusterInToPubShare ClusterInToPubShare, clusterInToPubSys ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode) NodeServerFace {
	util.SetLimit()

	//util.Profiling(*pprofPort)

	options := []getty.ServerOption{getty.WithLocalAddress(addr)}

	taskPool = gxsync.NewTaskPoolSimple(10000)
	options = append(options, getty.WithServerTaskPool(taskPool))

	sev := getty.NewTCPServer(options...)

	go sev.RunEventLoop(newHelloServerSession(name, clusterInToPub, clusterInToPubShare,
		clusterInToPubSys, shareTopicMapNode))
	return &server{
		name: name,
		s:    sev,
	}
}

func newHelloServerSession(name string, clusterInToPub ClusterInToPub,
	clusterInToPubShare ClusterInToPubShare, clusterInToPubSys ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode) func(session getty.Session) error {
	return func(session getty.Session) error {
		err := InitialSessionServer(name, session, clusterInToPub, clusterInToPubShare,
			clusterInToPubSys, shareTopicMapNode)
		if err != nil {
			return err
		}
		return nil
	}
}
