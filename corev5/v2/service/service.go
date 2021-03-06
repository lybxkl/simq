package service

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	getty "github.com/apache/dubbo-getty"
	"io"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
)

type (
	//完成的回调方法
	OnCompleteFunc func(msg, ack message.Message, err error) error
	// 发布的func类型 sender表示当前发送消息的客户端是哪个，shareMsg=true表示是共享消息，不能被Local 操作
	OnPublishFunc func(msg *message.PublishMessage, sub topics.Sub, sender string, shareMsg bool) error
)

type stat struct {
	bytes int64 // bytes数量
	msgs  int64 // 消息数量
}

func (svc *stat) increment(n int64) {
	atomic.AddInt64(&svc.bytes, n)
	atomic.AddInt64(&svc.msgs, 1)
}

var (
	gsvcid uint64 = 0
)

type service struct {
	clusterBelong bool // 集群特殊使用的标记
	clusterOpen   bool // 是否开启了集群

	gettySession getty.Session // getty 启动

	id   uint64 //这个服务的ID，它与客户ID无关，只是一个数字而已
	ccid string // 客户端id

	// 这是客户端还是服务器?它是由Connect (client)或 HandleConnection(服务器)。
	// 用来表示该是服务端的还是客户端的
	client bool

	//客户端最大可接收包大小，在connect包内，但broker不处理，因为超过限制的报文将导致协议错误，客户端发送包含原因码0x95（报文过大）的DISCONNECT报文给broker
	// 共享订阅的情况下，如果一条消息对于部分客户端来说太长而不能发送，服务端可以选择丢弃此消息或者把消息发送给剩余能够接收此消息的客户端。
	// 非规范：服务端可以把那些没有发送就被丢弃的报文放在死信队列 上，或者执行其他诊断操作。具体的操作超出了5.0规范的范围。
	// maxPackageSize int

	conn io.Closer

	// sess是这个MQTT会话的会话对象。它跟踪会话变量
	//比如ClientId, KeepAlive，用户名等
	sess        sessions.Session
	hasSendWill bool // 防止重复发送遗嘱使用

	//等待各种goroutines完成启动和停止
	wgStarted sync.WaitGroup
	wgStopped sync.WaitGroup

	// writeMessage互斥锁——序列化输出缓冲区的写操作。
	wmu sync.Mutex

	//这个服务是否关闭。
	closed int64

	//退出信号，用于确定此服务何时结束。如果通道关闭，则退出。
	done chan struct{}

	//输入数据缓冲区。从连接中读取字节并放在这里。
	in *buffer

	//输出数据缓冲区。这里写入的字节依次写入连接。
	out *buffer

	//onpub方法，将其添加到主题订阅方列表
	// processSubscribe()方法。当服务器完成一个发布消息的ack循环时 它将调用订阅者，也就是这个方法。
	//对于服务器，当这个方法被调用时，它意味着有一个消息应该发布到连接另一端的客户端。所以我们 将调用publish()发送消息。
	onpub   OnPublishFunc
	inStat  stat  // 输入的记录
	outStat stat  // 输出的记录
	sign    *Sign // 信号
	quota   int64 // 配额
	limit   int

	intmp  []byte
	outtmp []byte

	rmsgs []*message.PublishMessage

	server *Server
}

func (server *Server) NewGettyService(gettySession getty.Session) (*service, *message.ConnectMessage, *message.ConnackMessage, error) {
	// 等待连接认证
	req, resp, err := server.conAuth(gettySession.Conn())
	if err != nil || resp == nil {
		return nil, nil, nil, err
	}

	id := atomic.AddUint64(&gsvcid, 1)

	sess, err := server.getSession(id, req, resp)
	if err != nil {
		return nil, nil, nil, err
	}
	ccId := fmt.Sprintf("%s%d/%s", server.cfg.AutoIdPrefix, id, sess.IDs())

	svc := &service{
		clusterBelong: false,
		clusterOpen:   server.cfg.Cluster.Enabled,
		id:            atomic.AddUint64(&gsvcid, 1),
		ccid:          ccId,
		sess:          sess,
		gettySession:  gettySession,
		conn:          gettySession.Conn(),
		closed:        0,
		done:          make(chan struct{}),
		sign:          NewSign(server.cfg.Quota, server.cfg.QuotaLimit),
		quota:         server.cfg.Quota,
		limit:         server.cfg.QuotaLimit,
		server:        server,
	}

	// 这个是发送给订阅者的，是每个订阅者都有一份的方法
	svc.onpub = svc.onPub()
	return svc, req, resp, nil
}

// 运行接入的连接，会产生三个协程异步逻辑处理，当前不会阻塞
func (svc *service) start(resp *message.ConnackMessage) error {
	var err error
	svc.ccid = fmt.Sprintf("%s%d/%s", svc.server.cfg.Broker.AutoIdPrefix, svc.id, svc.sess.IDs())

	// Create the incoming ring buffer
	svc.in, err = newBuffer(defaultBufferSize)
	if err != nil {
		return err
	}
	// Create the outgoing ring buffer
	svc.out, err = newBuffer(defaultBufferSize)
	if err != nil {
		return err
	}

	var pkid uint32 = 1
	var max uint32 = math.MaxUint16 * 4 / 5

	svc.sign = NewSign(svc.quota, svc.limit)
	// If svc is a server
	if !svc.client {
		// Creat the onPublishFunc so it can be used for published messages
		// 这个是发送给订阅者的，是每个订阅者都有一份的方法
		svc.onpub = func(msg *message.PublishMessage, sub topics.Sub, sender string, isShareMsg bool) error {
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
				pid := atomic.AddUint32(&pkid, 1) // FIXME 这里只是简单的处理pkid
				if pid > max {
					atomic.StoreUint32(&pkid, 1)
				}
				msg.SetPacketId(uint16(pid))
			}
			if sub.SubIdentifier > 0 {
				msg.SetSubscriptionIdentifier(sub.SubIdentifier) // 订阅标识符
			}
			if alice, exist := svc.sess.GetTopicAlice(msg.Topic()); exist {
				msg.SetNilTopicAndAlias(alice) // 直接替换主题为空了，用主题别名来表示
			}
			if err = svc.publish(msg, func(msg, ack message.Message, err error) error {
				logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
				return nil
			}); err != nil {
				logger.Logger.Errorf("service/onPublish: Error publishing message: %v", err)
				return err
			}

			return nil
		}
		// If svc is a recovered session, then add any topics it subscribed before
		//如果这是一个恢复的会话，那么添加它之前订阅的任何主题
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
	}

	if resp != nil {
		if err = writeMessage(svc.conn, resp); err != nil {
			return err
		}
		svc.outStat.increment(int64(resp.Len()))
	}

	//处理器负责从缓冲区读取消息并进行处理
	svc.wgStarted.Add(1)
	svc.wgStopped.Add(1)
	go svc.processor()

	//接收端负责从连接中读取数据并将数据放入 一个缓冲区。
	svc.wgStarted.Add(1)
	svc.wgStopped.Add(1)
	go svc.receiver()

	//发送方负责将缓冲区中的数据写入连接。
	svc.wgStarted.Add(1)
	svc.wgStopped.Add(1)
	go svc.sender()

	if !svc.client {
		offline := svc.sess.OfflineMsg()    //  发送获取到的离线消息
		for i := 0; i < len(offline); i++ { // 依次处理离线消息
			pub := offline[i].(*message.PublishMessage)
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
		// FIXME 是否主动发送未完成确认的过程消息，还是等客户端操作
	}
	// Wait for all the goroutines to start before returning
	svc.wgStarted.Wait()

	return nil
}

// FIXME: The order of closing here causes panic sometimes. For example, if receiver
// calls svc, and closes the buffers, somehow it causes buffer.go:476 to panid.
func (svc *service) stop() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Logger.Errorf("(%s) Recovering from panic: %v", svc.cid(), r)
		}
	}()

	doit := atomic.CompareAndSwapInt64(&svc.closed, 0, 1)
	if !doit {
		return
	}

	// Close quit channel, effectively telling all the goroutines it's time to quit
	if svc.done != nil {
		logger.Logger.Debugf("(%s) closing svc.done", svc.cid())
		close(svc.done)
	}

	// Close the network connection
	if svc.conn != nil {
		logger.Logger.Debugf("(%s) closing svc.conn", svc.cid())
		svc.conn.Close()
	}

	if svc.in != nil {
		svc.in.Close()
	}
	if svc.out != nil {
		svc.out.Close()
	}

	// Wait for all the goroutines to stop.
	if svc.gettySession == nil {
		svc.wgStopped.Wait()
	}

	//打印该客户端生命周期内的接收字节与消息条数、发送字节与消息条数
	logger.Logger.Debugf("(%s) Received %d bytes in %d messages.", svc.cid(), svc.inStat.bytes, svc.inStat.msgs)
	logger.Logger.Debugf("(%s) Sent %d bytes in %d messages.", svc.cid(), svc.outStat.bytes, svc.outStat.msgs)

	// 取消订阅该客户机的所有主题
	svc.unSubAll()

	//如果设置了遗嘱消息，则发送遗嘱消息，当是收到正常DisConnect消息产生的发送遗嘱消息行为，会在收到消息处处理
	if svc.sess.Cmsg().WillFlag() {
		logger.Logger.Infof("(%s) service/stop: connection unexpectedly closed. Sending Will：.", svc.cid())
		svc.sendWillMsg()
	}

	// 直接删除session，重连时重新初始化
	svc.sess.SetStatus(sessions.OFFLINE)
	if svc.server.sessMgr != nil { // svc.sess.Cmsg().CleanSession() &&
		svc.server.sessMgr.Del(svc.sess.ID())
	}

	svc.conn = nil
	svc.in = nil
	svc.out = nil
	svc.gettySession = nil
}

// 发布消息给客户端
func (svc *service) publish(msg *message.PublishMessage, onComplete OnCompleteFunc) (err error) {

	switch msg.QoS() {
	case message.QosAtMostOnce:
		_, err = svc.writeMessage(msg)
		if err != nil {
			err = message.NewCodeErr(message.ServiceBusy, fmt.Sprintf("(%s) Error sending %s message: %v", svc.cid(), msg.Name(), err))
		}
		if onComplete != nil {
			err = onComplete(msg, nil, nil)
			if err != nil {
				return message.NewCodeErr(message.ServerUnavailable, err.Error())
			}
			return nil
		}
	case message.QosAtLeastOnce:
		err = svc.sess.Pub1ack().Wait(msg, onComplete)
		if err != nil {
			return message.NewCodeErr(message.ServerUnavailable, err.Error())
		}
		_, err = svc.writeMessage(msg)
		if err != nil {
			err = message.NewCodeErr(message.ServiceBusy, fmt.Sprintf("(%s) Error sending %s message: %v", svc.cid(), msg.Name(), err))
		}
	case message.QosExactlyOnce:
		err = svc.sess.Pub2out().Wait(msg, onComplete)
		if err != nil {
			return message.NewCodeErr(message.ServerUnavailable, err.Error())
		}
		_, err = svc.writeMessage(msg)
		if err != nil {
			err = message.NewCodeErr(message.ServiceBusy, fmt.Sprintf("(%s) Error sending %s message: %v", svc.cid(), msg.Name(), err))
		}
	}
	return nil
}

func (svc *service) subscribe(msg *message.SubscribeMessage, onComplete OnCompleteFunc, onPublish OnPublishFunc) error {
	if onPublish == nil {
		return fmt.Errorf("onPublish function is nil. No need to subscribe.")
	}

	_, err := svc.writeMessage(msg)
	if err != nil {
		return message.NewCodeErr(message.ServiceBusy, fmt.Sprintf("(%s) Error sending %s messagev5: %v", svc.cid(), msg.Name(), err))
	}

	var onc OnCompleteFunc = func(msg, ack message.Message, err error) error {
		onComplete := onComplete
		onPublish := onPublish

		if err != nil {
			if onComplete != nil {
				return onComplete(msg, ack, err)
			}
			return err
		}

		sub, ok := msg.(*message.SubscribeMessage)
		if !ok {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Invalid SubscribeMessage received"))
			}
			return nil
		}

		suback, ok := ack.(*message.SubackMessage)
		if !ok {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Invalid SubackMessage received"))
			}
			return nil
		}

		if sub.PacketId() != suback.PacketId() {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Sub and Suback packet ID not the same. %d != %d.", sub.PacketId(), suback.PacketId()))
			}
			return nil
		}

		retcodes := suback.ReasonCodes()
		tps := sub.Topics()

		if len(tps) != len(retcodes) {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Incorrect number of return codes received. Expecting %d, got %d.", len(tps), len(retcodes)))
			}
			return nil
		}

		var err2 error = nil

		for i, t := range tps {
			c := retcodes[i]

			if c == message.QosFailure {
				err2 = fmt.Errorf("Failed to subscribe to '%s'\n%v", string(t), err2)
			} else {
				tp1 := make([]byte, len(t))
				copy(tp1, t)
				err = svc.sess.AddTopic(topics.Sub{
					Topic:             tp1,
					Qos:               c,
					NoLocal:           sub.TopicNoLocal(tp1),
					RetainAsPublished: sub.TopicRetainAsPublished(tp1),
					RetainHandling:    sub.TopicRetainHandling(tp1),
					SubIdentifier:     sub.SubscriptionIdentifier(),
				})
				if err != nil {
					err2 = fmt.Errorf("Failed to subscribe to '%s' (%v)\n%v", string(t), err, err2)
				}
				tp := make([]byte, len(t))
				copy(tp, t)
				_, err = svc.server.topicsMgr.Subscribe(topics.Sub{
					Topic:             tp,
					Qos:               c,
					NoLocal:           sub.TopicNoLocal(tp),
					RetainAsPublished: sub.TopicRetainAsPublished(tp),
					RetainHandling:    sub.TopicRetainHandling(tp),
					SubIdentifier:     sub.SubscriptionIdentifier(),
				}, &onPublish)
				if err != nil {
					err2 = fmt.Errorf("Failed to subscribe to '%s' (%v)\n%v", string(t), err, err2)
				}
			}
		}

		if onComplete != nil {
			return onComplete(msg, ack, err2)
		}

		return err2
	}

	return svc.sess.Suback().Wait(msg, onc)
}

func (svc *service) unsubscribe(msg *message.UnsubscribeMessage, onComplete OnCompleteFunc) error {
	_, err := svc.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", svc.cid(), msg.Name(), err)
	}

	var onc OnCompleteFunc = func(msg, ack message.Message, err error) error {
		onComplete := onComplete

		if err != nil {
			if onComplete != nil {
				return onComplete(msg, ack, err)
			}
			return err
		}

		unsub, ok := msg.(*message.UnsubscribeMessage)
		if !ok {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Invalid UnsubscribeMessage received"))
			}
			return nil
		}

		unsuback, ok := ack.(*message.UnsubackMessage)
		if !ok {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Invalid UnsubackMessage received"))
			}
			return nil
		}

		if unsub.PacketId() != unsuback.PacketId() {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Unsub and Unsuback packet ID not the same. %d != %d.", unsub.PacketId(), unsuback.PacketId()))
			}
			return nil
		}

		var err2 error = nil

		for _, tb := range unsub.Topics() {
			// Remove all subscribers, which basically it's just svc client, since
			// each client has it's own topic tree.
			err := svc.server.topicsMgr.Unsubscribe(tb, nil)
			if err != nil {
				err2 = fmt.Errorf("%v\n%v", err2, err)
			}

			svc.sess.RemoveTopic(string(tb))
		}

		if onComplete != nil {
			return onComplete(msg, ack, err2)
		}

		return err2
	}

	return svc.sess.Unsuback().Wait(msg, onc)
}

func (svc *service) ping(onComplete OnCompleteFunc) error {
	msg := message.NewPingreqMessage()

	_, err := svc.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", svc.cid(), msg.Name(), err)
	}

	return svc.sess.Pingack().Wait(msg, onComplete)
}

func (svc *service) isDone() bool {
	select {
	case <-svc.done:
		return true

	default:
		if svc.closed > 0 {
			return true
		}
	}

	return false
}

func (svc *service) cid() string {
	return svc.ccid
}
