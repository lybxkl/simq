package mysql

import (
	"errors"
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/logger"
	"github.com/panjf2000/ants/v2"
	"time"
)

type dbRcv struct {
	curNodeName         string
	clusterInToPub      colong.ClusterInToPub
	clusterInToPubShare colong.ClusterInToPubShare
	clusterInToPubSys   colong.ClusterInToPubSys
	shareTopicMapNode   cluster.ShareTopicMapNode
	c                   *mysqlOrm
	stop                chan struct{}
	taskPool            *ants.Pool
	poolSize            int
}

// RunMysqlClusterServer 启动DB集群服务
// period 获取数据周期，单位ms
// size 每次获取数据量
func RunMysqlClusterServer(curNodeName string, clusterInToPub colong.ClusterInToPub,
	clusterInToPubShare colong.ClusterInToPubShare, clusterInToPubSys colong.ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode, taskPoolSize int, period, size int64, mysqlUrl string) colong.NodeServerFace {
	dbServer := &dbRcv{}
	dbServer.curNodeName = curNodeName
	dbServer.clusterInToPubShare = clusterInToPubShare
	dbServer.clusterInToPub = clusterInToPub
	dbServer.clusterInToPubSys = clusterInToPubSys
	dbServer.shareTopicMapNode = shareTopicMapNode
	dbServer.stop = make(chan struct{})
	if taskPoolSize <= 100 {
		taskPoolSize = 100
	}
	dbServer.poolSize = taskPoolSize
	dbServer.taskPool, _ = ants.NewPool(taskPoolSize, ants.WithPanicHandler(func(i interface{}) {
		logger.Logger.Errorf("协程池处理错误：%v", i)
	}), ants.WithMaxBlockingTasks(taskPoolSize*10))

	dbServer.c = newMysqlOrm(curNodeName, mysqlUrl)

	dbServer.run(period, size)
	return dbServer
}
func (this *dbRcv) Close() error {
	close(this.stop)
	return nil
}
func (this *dbRcv) submit(f func()) {
	this.dealAntsErr(this.taskPool.Submit(f))
}
func (this *dbRcv) dealAntsErr(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, ants.ErrPoolClosed) {
		logger.Logger.Errorf("协程池错误：%v", err)
		this.taskPool.Reboot()
	} else if errors.Is(err, ants.ErrPoolOverload) {
		logger.Logger.Errorf("协程池超载,进行扩容：%v", err)
		// TODO 需要缩
		this.taskPool.Tune(int(float64(this.poolSize) * 1.25))
	} else {
		logger.Logger.Errorf("线程池处理异常：%v", err)
	}
}

var sharePrefix = []byte("$share/")

// period 获取数据周期，单位ms
// size 每次获取数据量
func (s *dbRcv) run(period, size int64) {
	go func() {
		for {
			select {
			case <-time.After(time.Duration(period) * time.Millisecond):
			case _, ok := <-s.stop:
				if !ok {
					return
				}
			}
			sub, err := s.c.GetSubBatch(size / 2)
			if err != nil {
				logger.Logger.Error(err)
			}
			for i := 0; i < len(sub); i++ {
				if sub[i].SubOrUnSub == 1 {
					tpk := poToBytes(&sub[i])
					node := sub[i].Sender
					s.submit(func() {
						for j := 0; j < len(tpk); j++ {
							// 解析share name
							shareName, top := shareTopic(tpk[j])
							if shareName != "" {
								err = s.shareTopicMapNode.AddTopicMapNode(top, shareName, node)
								if err != nil {
									logger.Logger.Errorf("%s,共享订阅节点新增失败, shareName:%v , err: %v", node, shareName, err)
								} else {
									logger.Logger.Debugf("收到节点：%s 发来的 共享订阅：topic-%s, shareName-%s", node, top, shareName)
								}
							} else {
								logger.Logger.Warnf("收到非法订阅：%s", tpk[j])
							}
						}
					})
				} else if sub[i].SubOrUnSub == 2 {
					tpk := poToBytes(&sub[i])
					node := sub[i].Sender
					s.submit(func() {
						for j := 0; j < len(tpk); j++ {
							// 解析share name
							shareName, top := shareTopic(tpk[j])
							if shareName != "" {
								err = s.shareTopicMapNode.RemoveTopicMapNode(top, shareName, node)
								if err != nil {
									logger.Logger.Errorf("%s,共享订阅节点减少失败, shareName:%v , err: %v", node, shareName, err)
								} else {
									logger.Logger.Debugf("收到节点：%s 发来的 取消共享订阅：topic-%s, shareName-%s", node, top, shareName)
								}
							} else {
								logger.Logger.Warnf("收到非法取消订阅：%s", string(tpk[j]))
							}
						}
					})
				}
			}
		}
	}()
	//var nums int64
	//go func() {
	//	for {
	//		time.Sleep(3 * time.Second)
	//		println(nums)
	//	}
	//}()
	go func() {
		for {
			select {
			case <-time.After(time.Duration(period) * time.Millisecond):
			case _, ok := <-s.stop:
				if !ok {
					return
				}
			}

			msg, err := s.c.GetPubBatch(size)
			if err != nil {
				logger.Logger.Error(err)
			}
			//nums += int64(len(msg))
			for k := 0; k < len(msg); k++ {
				mg := msg[k]
				if mg.Target != "" && mg.Target != s.curNodeName {
					continue
				}
				if mg.ShareName != "" {
					msg1 := poToVo(&mg)
					s.submit(func() {
						err = s.clusterInToPubShare(msg1, mg.ShareName, true)
						if err != nil {
							logger.Logger.Errorf("clusterInToPubShare: err %v", err)
						} else {
							logger.Logger.Debugf("收到节点：%s 发来的 共享消息：%s", mg.Sender, msg1)
						}
					})
				} else {
					msg1 := poToVo(&mg)
					s.submit(func() {
						err = s.clusterInToPub(msg1)
						if err != nil {
							logger.Logger.Errorf("clusterInToPub: err %v", err)
						} else {
							logger.Logger.Debugf("收到节点：%s 发来的 普通消息：%s", mg.Sender, msg1)
						}
					})
				}
			}
		}
	}()
}

// 共享组和topic
func shareTopic(b []byte) (string, []byte) {
	if len(b) < len(sharePrefix) {
		return "", b
	}
	for i := 0; i < len(sharePrefix); i++ {
		if b[i] != sharePrefix[i] {
			return "", b
		}
	}
	for i := len(sharePrefix); i < len(b); i++ {
		if b[i] == '/' {
			return string(b[len(sharePrefix):i]), b[i+1:]
		}
	}
	return "", b
}
