package mongo

import (
	"context"
	"errors"
	"fmt"
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
	c                   *mongoOrm
	stop                chan struct{}
	taskPool            *ants.Pool
	poolSize            int
}

// RunDBClusterServer 启动DB集群服务
// period 获取数据周期，单位ms
// size 每次获取数据量
func RunDBClusterServer(curNodeName string, clusterInToPub colong.ClusterInToPub,
	clusterInToPubShare colong.ClusterInToPubShare, clusterInToPubSys colong.ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode, taskPoolSize int, period, size int64, mongoUrl string, mongoMinPool, mongoMaxPool, mongoMaxConnIdle uint64) colong.NodeServerFace {
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
		fmt.Println("协程池处理错误：", i)
	}), ants.WithMaxBlockingTasks(taskPoolSize*10))
	var e error
	dbServer.c, e = newMongoOrm(curNodeName, mongoUrl, mongoMinPool, mongoMaxPool, mongoMaxConnIdle)
	if e != nil {
		panic(e)
	}
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
		fmt.Println("协程池错误：", err.Error())
		this.taskPool.Reboot()
	} else if errors.Is(err, ants.ErrPoolOverload) {
		fmt.Println("协程池超载,进行扩容：", err.Error())
		// TODO 需要缩
		this.taskPool.Tune(int(float64(this.poolSize) * 1.25))
	} else {
		fmt.Println("线程池处理异常：", err)
	}
}

var sharePrefix = []byte("$share/")

// period 获取数据周期，单位ms
// size 每次获取数据量
func (s *dbRcv) run(period, size int64) {
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
			case <-s.stop:
				return
			}
			ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
			go func() {
				defer func() {
					if e := recover(); e != nil {
						println(e)
					}
					cancel()
				}()
				select {
				case <-time.After(10 * time.Second):
					cancel()
				case <-ctx.Done():
				}
			}()
			// FIXME 当并发新增数据速度和当前获取数据速度差不太多时，会因为mongo 新增的objectId乱序导致漏获取部分数据
			msg, err := s.c.GetBatch(ctx, "cluster_msg", size)
			if err != nil {
				cancel()
				logger.Logger.Error(err)
			}
			for k := 0; k < len(msg); k++ {
				mg := msg[k]
				if mg.Target != "" && mg.Target != s.curNodeName {
					continue
				}
				//nums++
				if mg.IsSub() {
					s.sub(mg)
				} else if mg.IsUnSub() {
					s.unSub(mg)
				} else if mg.IsShare() {
					s.share(mg)
				} else if mg.IsPub() {
					s.pub(mg)
				} else {
					logger.Logger.Warnf("OnMessage: %+v", mg)
				}
			}
		}
	}()
}

func (s *dbRcv) pub(mg MessagePo) {
	s.submit(func() {
		msg1 := poToVo(mg.Msg)
		err := s.clusterInToPub(msg1)
		if err != nil {
			logger.Logger.Errorf("clusterInToPub: err %v", err)
		} else {
			logger.Logger.Debugf("收到节点：%s 发来的 普通消息：%s", mg.Sender, msg1)
		}
	})
}

func (s *dbRcv) share(mg MessagePo) {
	s.submit(func() {
		msg1 := poToVo(mg.Msg)
		err := s.clusterInToPubShare(msg1, mg.ShareName, true)
		if err != nil {
			logger.Logger.Errorf("clusterInToPubShare: err %v", err)
		} else {
			logger.Logger.Debugf("收到节点：%s 发来的 共享消息：%s", mg.Sender, msg1)
		}
	})
}

func (s *dbRcv) unSub(mg MessagePo) {
	s.submit(func() {
		tpk := mg.Sub.Topic
		node := mg.Sender
		for i := 0; i < len(tpk); i++ {
			// 解析share name
			shareName, top := shareTopic([]byte(tpk[i]))
			if shareName != "" {
				err := s.shareTopicMapNode.RemoveTopicMapNode(top, shareName, node)
				if err != nil {
					logger.Logger.Errorf("%s,共享订阅节点减少失败, shareName:%v , err: %v", node, shareName, err)
				} else {
					logger.Logger.Debugf("收到节点：%s 发来的 取消共享订阅：topic-%s, shareName-%s", node, top, shareName)
				}
			} else {
				logger.Logger.Warnf("收到非法取消订阅：%s", string(tpk[i]))
			}
		}
	})
}

func (s *dbRcv) sub(mg MessagePo) {
	s.submit(func() {
		tpk := mg.Sub.Topic
		node := mg.Sender
		for i := 0; i < len(tpk); i++ {
			// 解析share name
			shareName, top := shareTopic([]byte(tpk[i]))
			if shareName != "" {
				err := s.shareTopicMapNode.AddTopicMapNode(top, shareName, node)
				if err != nil {
					logger.Logger.Errorf("%s,共享订阅节点新增失败, shareName:%v , err: %v", node, shareName, err)
				} else {
					logger.Logger.Debugf("收到节点：%s 发来的 共享订阅：topic-%s, shareName-%s", node, top, shareName)
				}
			} else {
				logger.Logger.Warnf("收到非法订阅：%s", tpk[i])
			}
		}
	})
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
