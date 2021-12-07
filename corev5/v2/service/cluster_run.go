package service

import (
	"gitee.com/Ljolan/si-mqtt/cluster"
	autocompress "gitee.com/Ljolan/si-mqtt/cluster/stat/colong/auto_compress_sub"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/db/mongo"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/db/mysql"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/static_getty"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/logger"
	"strconv"
)

// GettyClusterRun 需要与其它节点连接成功才会成功启动
func GettyClusterRun(this *Server, cfg *config.SIConfig) {
	this.AddCloser(static_getty.InitClusterTaskPool(int(cfg.Cluster.TaskClusterPoolSize)))
	this.AddCloser(InitServiceTaskPool(int(cfg.Cluster.TaskServicePoolSize)))

	static_getty.UpdateLogger(logger.Logger) // 可以替换为通用日志
	//colong.SetLoggerLevelInfo() // 设置集群服务的日志等级

	staticDisc := make(map[string]cluster.Node)
	for _, v := range cfg.Cluster.StaticNodeList {
		if v.Name == cfg.Cluster.ClusterName { // 跳过自己，这样就不用在配置文件中单独设置不同的数据了
			continue
		}
		staticDisc[v.Name] = cluster.Node{
			NNA:  v.Name,
			Addr: v.Addr,
		}
	}
	this.ClusterDiscover = cluster.NewStaticNodeDiscover(staticDisc)
	this.ShareTopicMapNode = cluster.NewShareMap()

	// 静态方式启动
	svc := this.NewService() // 单独service用来处理集群来的消息
	this.ClusterServer, this.ClusterClient, _ = static_getty.NewStaticCluster(cfg.Cluster.ClusterName,
		cfg.Cluster.ClusterHost+":"+strconv.Itoa(cfg.Cluster.ClusterPort),
		svc.ClusterInToPub, svc.ClusterInToPubShare, svc.ClusterInToPubSys,
		this.ShareTopicMapNode, this.ClusterDiscover.GetNodeMap(),
		int(cfg.Cluster.ClientConNum), true, int(cfg.Cluster.TaskServicePoolSize), int(cfg.Cluster.TaskClusterPoolSize))
	this.AddCloser(this.ClusterServer)
	this.AddCloser(this.ClusterClient)
}

// DBMongoClusterRun DB方式集群
func DBMongoClusterRun(this *Server, cfg *config.SIConfig) {
	this.AddCloser(InitServiceTaskPool(int(cfg.Cluster.TaskServicePoolSize)))
	this.ShareTopicMapNode = cluster.NewShareMap()
	svc := this.NewService() // 单独service用来处理集群来的消息
	this.ClusterServer, this.ClusterClient, _ = mongo.NewDBCluster(cfg.Cluster.ClusterName,
		svc.ClusterInToPub, svc.ClusterInToPubShare, svc.ClusterInToPubSys, this.ShareTopicMapNode,
		int(cfg.Cluster.TaskClusterPoolSize), int64(cfg.Cluster.Period), int64(cfg.Cluster.BatchSize),
		cfg.Cluster.MongoUrl, cfg.Cluster.MongoMinPool, cfg.Cluster.MongoMaxPool, cfg.Cluster.MongoMaxConnIdleTime)
	this.AddCloser(this.ClusterServer)
	this.AddCloser(this.ClusterClient)
}

// DBMysqlClusterRun DB方式集群
func DBMysqlClusterRun(this *Server, cfg *config.SIConfig) {
	this.AddCloser(InitServiceTaskPool(int(cfg.Cluster.TaskServicePoolSize)))
	this.ShareTopicMapNode = cluster.NewShareMap()
	svc := this.NewService() // 单独service用来处理集群来的消息
	compressCfg := autocompress.CompressCfg{
		Min:                int(cfg.Cluster.SubMinNum),
		Period:             int(cfg.Cluster.AutoPeriod),
		LockTimeOut:        int(cfg.Cluster.LockTimeOut),
		LockAddLive:        int(cfg.Cluster.LockAddLive),
		CompressProportion: cfg.Cluster.CompressProportion,
	}
	this.ClusterServer, this.ClusterClient, _ = mysql.NewMysqlCluster(cfg.Cluster.ClusterName,
		svc.ClusterInToPub, svc.ClusterInToPubShare, svc.ClusterInToPubSys, this.ShareTopicMapNode,
		int(cfg.Cluster.TaskClusterPoolSize), int64(cfg.Cluster.Period), int64(cfg.Cluster.BatchSize),
		cfg.Cluster.MysqlUrl, int(cfg.Cluster.MysqlMaxPool), compressCfg)
	this.AddCloser(this.ClusterServer)
	this.AddCloser(this.ClusterClient)
}
