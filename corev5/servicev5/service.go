package servicev5

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5"
	"gitee.com/Ljolan/si-mqtt/logger"
	"io"
	"sync"
	"sync/atomic"
)

type (
	//完成的回调方法
	OnCompleteFunc func(msg, ack messagev5.Message, err error) error
	OnPublishFunc  func(msg *messagev5.PublishMessage) error
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
	sessMgr *sessionsv5.Manager

	// Topics manager for all the client subscriptions
	//所有客户端订阅的主题管理器
	topicsMgr *topicsv5.Manager

	// sess is the session object for this MQTT session. It keeps track session variables
	// such as ClientId, KeepAlive, Username, etc
	// sess是这个MQTT会话的会话对象。它跟踪会话变量
	//比如ClientId, KeepAlive，用户名等
	sess sessionsv5.Session

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
	inStat  stat // 输入的记录
	outStat stat // 输出的记录

	intmp  []byte
	outtmp []byte

	subs  []interface{}
	qoss  []byte
	rmsgs []*messagev5.PublishMessage
}

func (this *service) start() error {
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

	// If this is a server
	if !this.client {
		// Creat the onPublishFunc so it can be used for published messages
		// 这个是发送给订阅者的，是每个订阅者都有一份的方法
		this.onpub = func(msg *messagev5.PublishMessage) error {
			if err := this.publish(msg, func(msg, ack messagev5.Message, err error) error {
				logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
				return nil
			}); err != nil {
				logger.Logger.Errorf("service/onPublish: Error publishing messagev5: %v", err)
				return err
			}

			return nil
		}

		// If this is a recovered session, then add any topicsv5 it subscribed before
		//如果这是一个恢复的会话，那么添加它之前订阅的任何主题
		tpc, qoss, err := this.sess.Topics()
		if err != nil {
			return err
		} else {
			for i, t := range tpc {
				this.topicsMgr.Subscribe([]byte(t), qoss[i], &this.onpub)
			}
		}
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

	// Unsubscribe from all the topicsv5 for this client, only for the server side though
	// 取消订阅该客户机的所有主题，但只针对服务器端
	if !this.client && this.sess != nil {
		tpc, _, err := this.sess.Topics()
		if err != nil {
			logger.Logger.Errorf("(%s/%d): %v", this.cid(), this.id, err)
		} else {
			for _, t := range tpc {
				if err := this.topicsMgr.Unsubscribe([]byte(t), &this.onpub); err != nil {
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
	//移除这个客户端的主题管理
	// Remove the client topicsv5 manager
	if this.client {
		topicsv5.Unregister(this.sess.ID())
	}
	//如果该客户端支持清除session，则清除
	// Remove the session from session store if it's suppose to be clean session
	if this.sess.Cmsg().CleanSession() && this.sessMgr != nil {
		this.sessMgr.Del(this.sess.ID())
	}

	this.conn = nil
	this.in = nil
	this.out = nil
}

func (this *service) publish(msg *messagev5.PublishMessage, onComplete OnCompleteFunc) error {
	//glog.Logger.Debug("service/publish: Publishing %s", msg)
	_, err := this.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", this.cid(), msg.Name(), err)
	}

	switch msg.QoS() {
	case messagev5.QosAtMostOnce:
		if onComplete != nil {
			return onComplete(msg, nil, nil)
		}

		return nil

	case messagev5.QosAtLeastOnce:
		return this.sess.Pub1ack().Wait(msg, onComplete)

	case messagev5.QosExactlyOnce:
		return this.sess.Pub2out().Wait(msg, onComplete)
	}

	return nil
}

func (this *service) subscribe(msg *messagev5.SubscribeMessage, onComplete OnCompleteFunc, onPublish OnPublishFunc) error {
	if onPublish == nil {
		return fmt.Errorf("onPublish function is nil. No need to subscribe.")
	}

	_, err := this.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", this.cid(), msg.Name(), err)
	}

	var onc OnCompleteFunc = func(msg, ack messagev5.Message, err error) error {
		onComplete := onComplete
		onPublish := onPublish

		if err != nil {
			if onComplete != nil {
				return onComplete(msg, ack, err)
			}
			return err
		}

		sub, ok := msg.(*messagev5.SubscribeMessage)
		if !ok {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Invalid SubscribeMessage received"))
			}
			return nil
		}

		suback, ok := ack.(*messagev5.SubackMessage)
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
		topicsv5 := sub.Topics()

		if len(topicsv5) != len(retcodes) {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Incorrect number of return codes received. Expecting %d, got %d.", len(topicsv5), len(retcodes)))
			}
			return nil
		}

		var err2 error = nil

		for i, t := range topicsv5 {
			c := retcodes[i]

			if c == messagev5.QosFailure {
				err2 = fmt.Errorf("Failed to subscribe to '%s'\n%v", string(t), err2)
			} else {
				this.sess.AddTopic(string(t), c)
				_, err := this.topicsMgr.Subscribe(t, c, &onPublish)
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

func (this *service) unsubscribe(msg *messagev5.UnsubscribeMessage, onComplete OnCompleteFunc) error {
	_, err := this.writeMessage(msg)
	if err != nil {
		return fmt.Errorf("(%s) Error sending %s messagev5: %v", this.cid(), msg.Name(), err)
	}

	var onc OnCompleteFunc = func(msg, ack messagev5.Message, err error) error {
		onComplete := onComplete

		if err != nil {
			if onComplete != nil {
				return onComplete(msg, ack, err)
			}
			return err
		}

		unsub, ok := msg.(*messagev5.UnsubscribeMessage)
		if !ok {
			if onComplete != nil {
				return onComplete(msg, ack, fmt.Errorf("Invalid UnsubscribeMessage received"))
			}
			return nil
		}

		unsuback, ok := ack.(*messagev5.UnsubackMessage)
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
	msg := messagev5.NewPingreqMessage()

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
	}

	return false
}

func (this *service) cid() string {
	return this.ccid
}
