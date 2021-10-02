package mongo

import (
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
)

// NewDBCluster 构建DB集群服务
// period 获取数据周期，单位ms
// size 每次获取数据量
func NewDBCluster(curNodeName string, clusterInToPub colong.ClusterInToPub,
	clusterInToPubShare colong.ClusterInToPubShare, clusterInToPubSys colong.ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode, taskPoolSize int, period, size int64,
	mongoUrl string, mongoMinPool, mongoMaxPool, mongoMaxConnIdle uint64) (colong.NodeServerFace,
	colong.NodeClientFace, error) {
	client := NewDBClusterClient(curNodeName, mongoUrl, mongoMinPool, mongoMaxPool, mongoMaxConnIdle)
	server := RunDBClusterServer(curNodeName, clusterInToPub, clusterInToPubShare, clusterInToPubSys,
		shareTopicMapNode, taskPoolSize, period, size, mongoUrl, mongoMinPool, mongoMaxPool, mongoMaxConnIdle)
	return client, server, nil
}
