package mysql

import (
	"errors"
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	autocompress "gitee.com/Ljolan/si-mqtt/cluster/stat/colong/auto_compress_sub"
	"gitee.com/Ljolan/si-mqtt/logger"
	"github.com/panjf2000/ants/v2"
	"sync"
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
	shareTopicMapNode cluster.ShareTopicMapNode, taskPoolSize int, period, size int64, mysqlUrl string, maxConn, subMinNum, autoPeriod int) colong.NodeServerFace {
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

	dbServer.c = newMysqlOrm(curNodeName, mysqlUrl, maxConn, subMinNum, autoPeriod)

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
	var _once = &sync.Once{}
	go func() {
		for {
			select {
			case <-time.After(time.Duration(period) * time.Millisecond):
			case _, ok := <-s.stop:
				if !ok {
					return
				}
			}
			_once.Do(func() {
				// 启动时，需要获取全部来初始化本地完整集群共享订阅数据
				// 因为我们采用的是本地收到的订阅，消息都直接在本地处理了，然后发送到集群消息
				// 所以，正常处理时，只会获取非自己发送的消息
				sub, err := s.c.GetSubBatch(size/2, true)
				if err != nil {
					logger.Logger.Error(err)
				}
				for i := 0; i < len(sub); i++ {
					if sub[i].SubOrUnSub == 1 {
						s.sub(sub[i])
					} else if sub[i].SubOrUnSub == 2 {
						s.unSub(sub[i])
					}
				}
				time.Sleep(time.Duration(period) * time.Millisecond)
			})
			sub, err := s.c.GetSubBatch(size/2, false)
			if err != nil {
				logger.Logger.Error(err)
			}
			for i := 0; i < len(sub); i++ {
				if sub[i].SubOrUnSub == 1 {
					s.sub(sub[i])
				} else if sub[i].SubOrUnSub == 2 {
					s.unSub(sub[i])
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
					s.share(mg)
				} else {
					s.pub(mg)
				}
			}
		}
	}()
}

func (s *dbRcv) pub(mg Message) {
	msg1 := poToVo(&mg)
	s.submit(func() {
		err := s.clusterInToPub(msg1)
		if err != nil {
			logger.Logger.Errorf("clusterInToPub: err %v", err)
		} else {
			logger.Logger.Debugf("收到节点：%s 发来的 普通消息：%s", mg.Sender, msg1)
		}
	})
}

func (s *dbRcv) share(mg Message) {
	msg1 := poToVo(&mg)
	s.submit(func() {
		err := s.clusterInToPubShare(msg1, mg.ShareName, true)
		if err != nil {
			logger.Logger.Errorf("clusterInToPubShare: err %v", err)
		} else {
			logger.Logger.Debugf("收到节点：%s 发来的 共享消息：%s", mg.Sender, msg1)
		}
	})
}

func (s *dbRcv) unSub(sub autocompress.Sub) {
	tpk := poToBytes(&sub)
	node := sub.Sender
	for j := 0; j < len(tpk); j++ {
		// 解析share name
		shareName, top := shareTopic(tpk[j])
		if shareName != "" {
			err := s.shareTopicMapNode.RemoveTopicMapNode(top, shareName, node)
			if err != nil {
				logger.Logger.Errorf("%s,共享订阅节点减少失败, shareName:%v , err: %v", node, shareName, err)
			} else {
				logger.Logger.Debugf("收到节点：%s 发来的 取消共享订阅：topic-%s, shareName-%s", node, top, shareName)
			}
		} else {
			logger.Logger.Warnf("收到非共享取消订阅：%s", string(tpk[j]))
		}
	}
}

func (s *dbRcv) sub(subi autocompress.Sub) {
	tpk := poToBytes(&subi)
	node := subi.Sender
	for j := 0; j < len(tpk); j++ {
		// 解析share name
		shareName, top := shareTopic(tpk[j])
		if shareName != "" {
			err := s.shareTopicMapNode.AddTopicMapNode(top, shareName, node, subi.Num+1) // 因为num是从0开始的，这里需要+1
			if err != nil {
				logger.Logger.Errorf("%s,共享订阅节点新增失败, shareName:%v , err: %v", node, shareName, err)
			} else {
				logger.Logger.Debugf("收到节点：%s 发来的 共享订阅：topic-%s, shareName-%s", node, top, shareName)
			}
		} else {
			logger.Logger.Warnf("收到非共享订阅：%s", string(tpk[j]))
		}
	}
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
