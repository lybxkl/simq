package colong

import "gitee.com/Ljolan/si-mqtt/cluster"

type NodeServerFace interface {
	Close()
}
type NodeClientFace interface {
	Close()
}

// NewStaticCluster 构建集群服务
func NewStaticCluster(curName string, curAddr string, clusterInToPub ClusterInToPub,
	clusterInToPubShare ClusterInToPubShare, clusterInToPubSys ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode,
	staticNodes map[string]cluster.Node, connectNum int, taskPoolMode bool, taskPoolSize int) (NodeServerFace, NodeClientFace, error) {

	svr := RunClusterGettyServer(curName, curAddr, clusterInToPub, clusterInToPubShare, clusterInToPubSys, shareTopicMapNode)
	cli := RunStaticGettyNodeClients(staticNodes, curName, connectNum, taskPoolMode, taskPoolSize)
	return svr, cli, nil
}
