package service

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
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

func (this *stat) increment(n int64) {
	atomic.AddInt64(&this.bytes, n)
	atomic.AddInt64(&this.msgs, 1)
}

var (
	gsvcid uint64 = 0
)

type service struct {
	clusterBelong bool // 集群特殊使用的标记

	clusterOpen bool // 是否开启了集群

	// The ID of this service, it's not related to the Client ID, just a number that's
	// incremented for every new service.
	//这个服务的ID，它与客户ID无关，只是一个数字而已
	//每增加一个新服务。
	id   uint64
	ccid string
	// Is this a client or server. It's set by either Connect (client) or
	// HandleConnection (server).
	//这是客户端还是服务器?它是由Connect (client)或
	// HandleConnection(服务器)。
	// 用来表示该是服务端的还是客户端的
	client bool

	// The number of seconds to keep the connection live if there's no data.
	// If not set then default to 5 mins.
	//如果没有数据，保持连接有效的秒数。
	//如果没有设置，则默认为5分钟。1.5倍该值 作为读超时
	keepAlive    int
	writeTimeout int // 写超时，1.5
	// The number of seconds to wait for the CONNACK messagev5 before disconnecting.
	// If not set then default to 2 seconds.
	//断开连接前等待CONNACK消息的秒数。
	//如果没有设置，则默认为2秒。
	connectTimeout int

	// The number of seconds to wait for any ACK messages before failing.
	// If not set then default to 20 seconds.
	//在失败之前等待任何ACK消息的秒数。
	//如果没有设置，则默认为20秒。
	ackTimeout int

	// The number of times to retry sending a packet if ACK is not received.
	// If no set then default to 3 retries.
	//如果没有收到ACK，重试发送数据包的次数。
	//如果没有设置，则默认为3次重试。
	timeoutRetries int

	// Network connection for this service
	//此服务的网络连接
	conn io.Closer

	// Session manager for tracking all the clients
	//会话管理器，用于跟踪所有客户端
	sessMgr sessions.Provider

	// Topics manager for all the client subscriptions
	//所有客户端订阅的主题管理器
	topicsMgr topics.Manager

	// sess is the session object for this MQTT session. It keeps track session variables
	// such as ClientId, KeepAlive, Username, etc
	// sess是这个MQTT会话的会话对象。它跟踪会话变量
	//比如ClientId, KeepAlive，用户名等
	sess sessions.Session

	// Wait for the various goroutines to finish starting and stopping
	//等待各种goroutines完成启动和停止
	wgStarted sync.WaitGroup
	wgStopped sync.WaitGroup

	// writeMessage mutex - serializes writes to the outgoing buffer.
	// writeMessage互斥锁——序列化输出缓冲区的写操作。
	wmu sync.Mutex

	// Whether this is service is closed or not.
	//这个服务是否关闭。
	closed int64

	// Quit signal for determining when this service should end. If channel is closed,
	// then exit.
	//退出信号，用于确定此服务何时结束。如果通道关闭，
	//然后退出。
	done chan struct{}

	// Incoming data buffer. Bytes are read from the connection and put in here.
	//输入数据缓冲区。从连接中读取字节并放在这里。
	in *buffer

	// Outgoing data buffer. Bytes written here are in turn written out to the connection.
	//输出数据缓冲区。这里写入的字节依次写入连接。
	out *buffer

	//onpub方法，将其添加到主题订阅方列表
	// processSubscribe()方法。当服务器完成一个发布消息的ack循环时
	//它将调用订阅者，也就是这个方法。
	//
	//对于服务器，当这个方法被调用时，它意味着有一个消息
	//应该发布到连接另一端的客户端。所以我们
	//将调用publish()发送消息。
	onpub   OnPublishFunc
	inStat  stat  // 输入的记录
	outStat stat  // 输出的记录
	sign    *Sign // 信号
	quota   int64 // 配额
	limit   int

	intmp  []byte
	outtmp []byte

	rmsgs []*message.PublishMessage

	shareTopicMapNode cluster.ShareTopicMapNode

	SessionStore store.SessionStore
	MessageStore store.MessageStore
	EventStore   store.EventStore
	conFig       *config.SIConfig
}

func (this *service) start(resp *message.ConnackMessage) error {
	var err error
	this.ccid = fmt.Sprintf("%d/%s", this.id, this.sess.IDs())
	// Create the incoming ring buffer
	this.in, err = newBuffer(defaultBufferSize)
	if err != nil {
		return err
	}

	// Create the outgoing ring buffer
	this.out, err = newBuffer(defaultBufferSize)
	if err != nil {
		return err
	}
	var pkid uint32 = 1
	var max uint32 = math.MaxUint16 * 4 / 5

	this.sign = NewSign(this.quota, this.limit)
	// If this is a server
	if !this.client {
		// Creat the onPublishFunc so it can be used for published messages
		// 这个是发送给订阅者的，是每个订阅者都有一份的方法
		this.onpub = func(msg *message.PublishMessage, sub topics.Sub, sender string, shareMsg bool) error {
			if msg.QoS() > 0 && !this.sign.ReqQuota() {
				// 超过配额
				return nil
			}
			if !shareMsg && sub.NoLocal && this.cid() == sender {
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
			if alice, exist := this.sess.GetTopicAlice(msg.Topic()); exist {
				msg.SetNilTopicAndAlias(alice) // 直接替换主题为空了，用主题别名来表示
			}
			if err := this.publish(msg, func(msg, ack message.Message, err error) error {
				logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
				return nil
			}); err != nil {
				logger.Logger.Errorf("service/onPublish: Error publishing messagev5: %v", err)
				return err
			}

			return nil
		}
		// If this is a recovered session, then add any topics it subscribed before
		//如果这是一个恢复的会话，那么添加它之前订阅的任何主题
		tpc, err := this.sess.Topics()
		if err != nil {
			return err
		} else {
			for _, t := range tpc {
				if this.conFig.Broker.CloseShareSub && len(t.Topic) > 6 && reflect.DeepEqual(t.Topic[:6], []byte{'$', 's', 'h', 'a', 'r', 'e'}) {
					continue
				}
				_, _ = this.topicsMgr.Subscribe(topics.Sub{
					Topic:             t.Topic,
					Qos:               t.Qos,
					NoLocal:           t.NoLocal,
					RetainAsPublished: t.RetainAsPublished,
					RetainHandling:    t.RetainHandling,
					SubIdentifier:     t.SubIdentifier,
				}, &this.onpub)
			}
		}
	}

	if resp != nil {
		if err = writeMessage(this.conn, resp); err != nil {
			return err
		}
		this.outStat.increment(int64(resp.Len()))
	}

	// Processor is responsible for reading messages out of the buffer and processing
	// them accordingly.
	//处理器负责从缓冲区读取消息并进行处理
	//他们。
	this.wgStarted.Add(1)
	this.wgStopped.Add(1)
	go this.processor()

	// Receiver is responsible for reading from the connection and putting data into
	// a buffer.
	//接收端负责从连接中读取数据并将数据放入
	//一个缓冲区。
	this.wgStarted.Add(1)
	this.wgStopped.Add(1)
	go this.receiver()

	// Sender is responsible for writing data in the buffer into the connection.
	//发送方负责将缓冲区中的数据写入连接。
	this.wgStarted.Add(1)
	this.wgStopped.Add(1)
	go this.sender()

	if !this.client {
		offline := this.sess.OfflineMsg()   //  发送获取到的离线消息
		for i := 0; i < len(offline); i++ { // 依次处理离线消息
			pub := offline[i].(*message.PublishMessage)
			// topics.Sub 获取
			var (
				subs []interface{}
				qoss []topics.Sub
			)
			_ = this.topicsMgr.Subscribers(pub.Topic(), pub.QoS(), &subs, &qoss, false, "", false)
			tag := false
			for j := 0; j < len(subs); j++ {
				if utils.Equal(subs[i], &this.onpub) {
					tag = true
					_ = this.onpub(pub, qoss[j], "", false)
					break
				}
			}
			if !tag {
				_ = this.onpub(pub, topics.Sub{}, "", false)
			}
		}
		// FIXME 是否主动发送未完成确认的过程消息，还是等客户端操作
	}
	// Wait for all the goroutines to start before returning
	this.wgStarted.Wait()

	return nil
}

// FIXME: The order of closing here causes panic sometimes. For example, if receiver
// calls this, and closes the buffers, somehow it causes buffer.go:476 to panid.
func (this *service) stop() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Logger.Errorf("(%s) Recovering from panic: %v", this.cid(), r)
		}
	}()

	doit := atomic.CompareAndSwapInt64(&this.closed, 0, 1)
	if !doit {
		return
	}

	// Close quit channel, effectively telling all the goroutines it's time to quit
	if this.done != nil {
		logger.Logger.Debugf("(%s) closing this.done", this.cid())
		close(this.done)
	}

	// Close the network connection
	if this.conn != nil {
		logger.Logger.Debugf("(%s) closing this.conn", this.cid())
		this.conn.Close()
	}

	this.in.Close()
	this.out.Close()

	// Wait for all the goroutines to stop.
	this.wgStopped.Wait()

	//打印该客户端生命周期内的接收字节与消息条数、发送字节与消息条数
	logger.Logger.Debugf("(%s) Received %d bytes in %d messages.", this.cid(), this.inStat.bytes, this.inStat.msgs)
	logger.Logger.Debugf("(%s) Sent %d bytes in %d messages.", this.cid(), this.outStat.bytes, this.outStat.msgs)

	// Unsubscribe from all the topics for this client, only for the server side though
	// 取消订阅该客户机的所有主题，但只针对服务器端
	if !this.client && this.sess != nil {
		tpc, err := this.sess.Topics()
		if err != nil {
			logger.Logger.Errorf("(%s/%d): %v", this.cid(), this.id, err)
		} else {
			for _, t := range tpc {
				if err := this.topicsMgr.Unsubscribe(t.Topic, &this.onpub); err != nil {
					logger.Logger.Errorf("(%s): Error unsubscribing topic %q: %v", this.cid(), t, err)
				}
			}
		}
	}
	//如果设置了遗嘱消息，则发送遗嘱消息
	// Publish will messagev5 if WillFlag is set. Server side only.
	if !this.client && this.sess.Cmsg().WillFlag() {
		logger.Logger.Infof("(%s) service/stop: connection unexpectedly closed. Sending Will：.", this.cid())
		this.onPublish(this.sess.Will())
	}

	// 直接删除session，重连时重新初始化
	this.sess.SetStatus(sessions.OFFLINE)
	if this.sessMgr != nil { // this.sess.Cmsg().CleanSession() &&
		this.sessMgr.Del(this.sess.ID())
	}

	this.conn = nil
	this.in = nil
	this.out = nil
}

func (this *service) publish(msg *message.PublishMessage, onComplete OnCompleteFunc) (err error) {
	//glog.Logger.Debug("service/publish: Publishing %s", msg)
	defer func() {
		if err != nil {
			return
		}
		_, err = this.writeMessage(msg)
		if err != nil {
			err = fmt.Errorf("(%s) Error sending %s messagev5: %v", this.cid(), msg.Name(), err)
		}
	}()

	switch msg.QoS() {
	case message.QosAtMostOnce:
		if onComplete != nil {
			return onComplete(msg, nil, nil)
		}

		return nil

	case message.QosAtLeastOnce:
		return this.sess.Pub1ack().Wait(msg, onComplete)

	case message.QosExactlyOnce:
		return this.sess.Pub2out().Wait(msg, onComplete)
	}

	return nil
}

func (this *service) subscribe(msg *message.SubscribeMessage, onComplete OnCompleteFunc, onPublish OnPublishFunc) error {
	if onPublish == nil {
		return fmt.Errorf("onPublish function is nil. No need to subscribe.")
	}

	_, err := this.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", this.cid(), msg.Name(), err)
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
				err = this.sess.AddTopic(topics.Sub{
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
				_, err = this.topicsMgr.Subscribe(topics.Sub{
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

	return this.sess.Suback().Wait(msg, onc)
}

func (this *service) unsubscribe(msg *message.UnsubscribeMessage, onComplete OnCompleteFunc) error {
	_, err := this.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", this.cid(), msg.Name(), err)
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
			// Remove all subscribers, which basically it's just this client, since
			// each client has it's own topic tree.
			err := this.topicsMgr.Unsubscribe(tb, nil)
			if err != nil {
				err2 = fmt.Errorf("%v\n%v", err2, err)
			}

			this.sess.RemoveTopic(string(tb))
		}

		if onComplete != nil {
			return onComplete(msg, ack, err2)
		}

		return err2
	}

	return this.sess.Unsuback().Wait(msg, onc)
}

func (this *service) ping(onComplete OnCompleteFunc) error {
	msg := message.NewPingreqMessage()

	_, err := this.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", this.cid(), msg.Name(), err)
	}

	return this.sess.Pingack().Wait(msg, onComplete)
}

func (this *service) isDone() bool {
	select {
	case <-this.done:
		return true

	default:
		if this.closed > 0 {
			return true
		}
	}

	return false
}

func (this *service) cid() string {
	return this.ccid
}
