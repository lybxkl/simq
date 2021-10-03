package mysql

import (
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
)

// NewMysqlCluster 构建DB集群服务
// period 获取数据周期，单位ms
// size 每次获取数据量
func NewMysqlCluster(curNodeName string, clusterInToPub colong.ClusterInToPub,
	clusterInToPubShare colong.ClusterInToPubShare, clusterInToPubSys colong.ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode, taskPoolSize int, period, size int64,
	mysqlUrl string) (colong.NodeServerFace,
	colong.NodeClientFace, error) {
	client := NewMysqlClusterClient(curNodeName, mysqlUrl)
	server := RunMysqlClusterServer(curNodeName, clusterInToPub, clusterInToPubShare, clusterInToPubSys,
		shareTopicMapNode, taskPoolSize, period, size, mysqlUrl)
	return client, server, nil
}
