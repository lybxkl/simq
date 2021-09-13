package server

import (
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/tcp"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	getty "github.com/apache/dubbo-getty"
)

import (
	gxsync "github.com/dubbogo/gost/sync"
)

type Server struct {
	name string
	s    getty.Server
}

func (this *Server) Close() {
	this.s.Close()
}

var taskPool gxsync.GenericTaskPool

func RunClusterServer(name string, addr string, clusterInToPub colong.ClusterInToPub,
	clusterInToPubShare colong.ClusterInToPubShare, clusterInToPubSys colong.ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode) *Server {
	util.SetLimit()

	//util.Profiling(*pprofPort)

	options := []getty.ServerOption{getty.WithLocalAddress(addr)}

	taskPool = gxsync.NewTaskPoolSimple(1000)
	options = append(options, getty.WithServerTaskPool(taskPool))

	server := getty.NewTCPServer(options...)

	go server.RunEventLoop(newHelloServerSession(name, clusterInToPub, clusterInToPubShare,
		clusterInToPubSys, shareTopicMapNode))

	return &Server{
		name: name,
		s:    server,
	}
}

func newHelloServerSession(name string, clusterInToPub colong.ClusterInToPub,
	clusterInToPubShare colong.ClusterInToPubShare, clusterInToPubSys colong.ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode) func(session getty.Session) error {
	return func(session getty.Session) error {
		err := tcp.InitialSessionServer(name, session, clusterInToPub, clusterInToPubShare,
			clusterInToPubSys, shareTopicMapNode)
		if err != nil {
			return err
		}
		return nil
	}
}
