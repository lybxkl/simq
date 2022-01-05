package service

import (
	"fmt"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	getty "github.com/apache/dubbo-getty"
	gxsync "github.com/dubbogo/gost/sync"
	"math"
	"net/url"
	"reflect"
	"sync/atomic"
	"time"
)

const (
	ServAttr   = "_ServiceAttr_"
	CronPeriod = "_CronPeriod_"
)

func (server *Server) ListenAndServeByGetty(uri string, taskPoolSize int) error {
	defer atomic.CompareAndSwapInt32(&server.running, 1, 0)

	// 防止重复启动
	if !atomic.CompareAndSwapInt32(&server.running, 0, 1) {
		return fmt.Errorf("server/ListenAndServe: Server is already running")
	}
	serverName = server.cfg.Cluster.ClusterName

	server.quit = make(chan struct{})

	//这个是配置各种钩子，比如账号认证钩子
	err := server.checkAndInitConfiguration()
	if err != nil {
		return err
	}

	getty.SetLogger(logger.Logger)

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	options := []getty.ServerOption{getty.WithLocalAddress(u.Host)}

	options = append(options, getty.WithServerTaskPool(gxsync.NewTaskPoolSimple(taskPoolSize)))

	err = newBus(server.cfg.GettyServerTaskPoolSize)
	if err != nil {
		return err
	}

	sev := getty.NewTCPServer(options...)
	server.gServer = sev

	sev.RunEventLoop(func(session getty.Session) error {
		session.SetPkgHandler(&packageHandler{})
		session.SetEventListener(server)
		session.SetReadTimeout(time.Second * time.Duration(server.cfg.ReadTimeout))
		session.SetWriteTimeout(time.Second * time.Duration(server.cfg.WriteTimeout))
		session.SetCronPeriod(1000) // 单位millisecond
		session.SetAttribute(CronPeriod, 10)
		return nil
	})
	return nil
}

func (server *Server) OnOpen(session getty.Session) error {

	svc, req, resp, err := server.NewGettyService(session)
	if err != nil {
		return err
	}

	session.SetAttribute(ServAttr, svc)
	session.SetAttribute(CronPeriod, int64(req.KeepAlive()))

	resp.SetReasonCode(messagev2.Success)
	_, _, err = session.WritePkg(resp, time.Second*time.Duration(svc.server.cfg.WriteTimeout))
	if err != nil {
		return err
	}

	submitInitEvTask(func() {
		//如果这是一个恢复的会话，那么添加它之前订阅的任何主题
		err = svc.reloadSub()
		if err != nil {
			logger.Logger.Error(err.Error())
		}
		// 发送离线消息
		svc.sendOfflineMsg()
	})

	// FIXME 是否主动发送未完成确认的过程消息，还是等客户端操作
	return nil
}

func (server *Server) OnError(session getty.Session, err error) {
	// 出错处理
	svc := session.GetAttribute(ServAttr).(*service)
	logger.Logger.Errorf("(%s) %s, error: %s.", svc.cid(), session.Stat(), err.Error())
}

func (server *Server) OnClose(session getty.Session) {
	// session 断线处理
	svc := session.GetAttribute(ServAttr).(*service)
	svc.stop()
}

func (server *Server) OnMessage(session getty.Session, m interface{}) {
	svs := session.GetAttribute(ServAttr)
	if svs == nil {
		logger.Logger.Warnf("%s", m)
		return
	}
	svc := svs.(*service)

	submitInitEvTask(func() {
		err := svc.processIncoming(m.(messagev2.Message))
		if err != nil {
			if reasonErr, ok := err.(*messagev2.Code); ok {
				_, err = svc.writeMessage(messagev2.NewDiscMessageWithCodeInfo(reasonErr.ReasonCode, []byte(reasonErr.Error())))
				if err != nil {
					logger.Logger.Error(err)
				}
			}
			session.Close()
			return
		}
		svc.middlewareHandle(m.(messagev2.Message))
	})
}

func (server *Server) OnCron(session getty.Session) {
	// 更新活跃状态，broker 应该判断最后一次收到消息
	active := session.GetActive()
	cron := session.GetAttribute(CronPeriod).(int64)
	if cron <= 0 {
		return
	}
	if cron < int64(time.Since(active).Seconds()) {
		session.Close()
	}
}

func (svc *service) sendOfflineMsg() {
	offline := svc.sess.OfflineMsg()    //  发送获取到的离线消息
	for i := 0; i < len(offline); i++ { // 依次处理离线消息
		pub := offline[i].(*messagev2.PublishMessage)
		// topics.Sub 获取
		var (
			subs   []interface{}
			subOpt []topics.Sub
		)
		_ = svc.server.topicsMgr.Subscribers(pub.Topic(), pub.QoS(), &subs, &subOpt, false, "", false)
		tag := false
		for j := 0; j < len(subs); j++ {
			if utils.Equal(subs[i], &svc.onpub) {
				tag = true
				_ = svc.onpub(pub, subOpt[j], "", false)
				break
			}
		}
		if !tag {
			_ = svc.onpub(pub, topics.Sub{}, "", false)
		}
	}
}

func (svc *service) reloadSub() error {
	tpc, err := svc.sess.Topics()
	if err != nil {
		return err
	} else {
		for _, t := range tpc {
			if svc.server.cfg.CloseShareSub && len(t.Topic) > 6 && reflect.DeepEqual(t.Topic[:6], []byte{'$', 's', 'h', 'a', 'r', 'e'}) {
				continue
			}
			_, _ = svc.server.topicsMgr.Subscribe(topics.Sub{
				Topic:             t.Topic,
				Qos:               t.Qos,
				NoLocal:           t.NoLocal,
				RetainAsPublished: t.RetainAsPublished,
				RetainHandling:    t.RetainHandling,
				SubIdentifier:     t.SubIdentifier,
			}, &svc.onpub)
		}
	}
	return nil
}

func (svc *service) unSubAll() {
	if svc.sess != nil {
		tpc, err := svc.sess.Topics()
		if err != nil {
			logger.Logger.Errorf("(%s/%d): %v", svc.cid(), svc.id, err)
		} else {
			for _, t := range tpc {
				if err = svc.server.topicsMgr.Unsubscribe(t.Topic, &svc.onpub); err != nil {
					logger.Logger.Errorf("(%s): Error unsubscribing topic %q: %v", svc.cid(), t, err)
				}
			}
		}
	}
}

func (svc *service) onPub() OnPublishFunc {
	var (
		pkID uint32 = 1
		max  uint32 = math.MaxUint16 * 4 / 5
	)
	return func(msg *messagev2.PublishMessage, sub topics.Sub, sender string, isShareMsg bool) error {
		if msg.QoS() > 0 && !svc.sign.ReqQuota() {
			// 超过配额
			return nil
		}
		if !isShareMsg && sub.NoLocal && svc.cid() == sender {
			logger.Logger.Debugf("no send  NoLocal option msg")
			return nil
		}
		if !sub.RetainAsPublished { //为1，表示向此订阅转发应用消息时保持消息被发布时设置的保留（RETAIN）标志
			msg.SetRetain(false)
		}
		if msg.QoS() > 0 {
			pid := atomic.AddUint32(&pkID, 1) // FIXME 这里只是简单的处理pkid
			if pid > max {
				atomic.StoreUint32(&pkID, 1)
			}
			msg.SetPacketId(uint16(pid))
		}
		if sub.SubIdentifier > 0 {
			msg.SetSubscriptionIdentifier(sub.SubIdentifier) // 订阅标识符
		}
		if alice, exist := svc.sess.GetTopicAlice(msg.Topic()); exist {
			msg.SetNilTopicAndAlias(alice) // 直接替换主题为空了，用主题别名来表示
		}
		if err := svc.publish(msg, func(msg, ack messagev2.Message, err error) error {
			logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
			return nil
		}); err != nil {
			logger.Logger.Errorf("service/onPublish: Error publishing message: %v", err)
			return err
		}

		return nil
	}
}
