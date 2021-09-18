package servicev5

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5"
	"gitee.com/Ljolan/si-mqtt/logger"

	"io"
	"reflect"
)

var (
	errDisconnect = errors.New("Disconnect")
)

// processor() reads messages from the incoming buffer and processes them
// processor()从传入缓冲区读取消息并处理它们
func (this *service) processor() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Logger.Errorf("(%s) Recovering from panic: %v", this.cid(), r)
		}

		this.wgStopped.Done()
		this.stop()
		logger.Logger.Debugf("(%s) Stopping processor", this.cid())
	}()

	logger.Logger.Debugf("(%s) Starting processor", this.cid())

	this.wgStarted.Done()

	for {
		// 1. Find out what messagev5 is next and the size of the messagev5
		//了解接下来是什么消息以及消息的大小
		mtype, total, err := this.peekMessageSize()
		if err != nil {
			if err != io.EOF {
				logger.Logger.Errorf("(%s) Error peeking next messagev5 size", this.cid())
			}
			return
		}

		msg, n, err := this.peekMessage(mtype, total)
		if err != nil {
			if err != io.EOF {
				logger.Logger.Errorf("(%s) Error peeking next messagev5: %v", this.cid(), err)
			}
			return
		}

		logger.Logger.Debugf("(%s) Received: %s", this.cid(), msg)

		this.inStat.increment(int64(n))

		// 5. Process the read messagev5
		//处理读消息
		err = this.processIncoming(msg)
		if err != nil {
			if err != errDisconnect {
				logger.Logger.Errorf("(%s) Error processing %s: %v", this.cid(), msg, err)
			} else {
				return
			}
		}

		// 7. We should commit the bytes in the buffer so we can move on
		// 我们应该提交缓冲区中的字节，这样我们才能继续
		_, err = this.in.ReadCommit(total)
		if err != nil {
			if err != io.EOF {
				logger.Logger.Errorf("(%s) Error committing %d read bytes: %v", this.cid(), total, err)
			}
			return
		}

		// 7. Check to see if done is closed, if so, exit
		// 检查done是否关闭，如果关闭，退出
		if this.isDone() && this.in.Len() == 0 {
			return
		}

		if this.inStat.msgs%100000 == 0 {
			logger.Logger.Warn(fmt.Sprintf("(%s) Going to process messagev5 %d", this.cid(), this.inStat.msgs))
		}
	}
}

func (this *service) processIncoming(msg messagev5.Message) error {
	var err error = nil

	switch msg := msg.(type) {
	case *messagev5.PublishMessage:
		// For PUBLISH messagev5, we should figure out what QoS it is and process accordingly
		// If QoS == 0, we should just take the next step, no ack required
		// If QoS == 1, we should send back PUBACK, then take the next step
		// If QoS == 2, we need to put it in the ack queue, send back PUBREC
		err = this.processPublish(msg)

	case *messagev5.PubackMessage:
		// For PUBACK messagev5, it means QoS 1, we should send to ack queue
		this.sess.Pub1ack().Ack(msg)
		this.processAcked(this.sess.Pub1ack())

	case *messagev5.PubrecMessage:
		// For PUBREC messagev5, it means QoS 2, we should send to ack queue, and send back PUBREL
		if err = this.sess.Pub2out().Ack(msg); err != nil {
			break
		}

		resp := messagev5.NewPubrelMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = this.writeMessage(resp)

	case *messagev5.PubrelMessage:
		// For PUBREL messagev5, it means QoS 2, we should send to ack queue, and send back PUBCOMP
		if err = this.sess.Pub2in().Ack(msg); err != nil {
			break
		}

		this.processAcked(this.sess.Pub2in())

		resp := messagev5.NewPubcompMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = this.writeMessage(resp)

	case *messagev5.PubcompMessage:
		// For PUBCOMP messagev5, it means QoS 2, we should send to ack queue
		if err = this.sess.Pub2out().Ack(msg); err != nil {
			break
		}

		this.processAcked(this.sess.Pub2out())

	case *messagev5.SubscribeMessage:
		// For SUBSCRIBE messagev5, we should add subscriber, then send back SUBACK
		//对于订阅消息，我们应该添加订阅者，然后发送回SUBACK
		return this.processSubscribe(msg)

	case *messagev5.SubackMessage:
		// For SUBACK messagev5, we should send to ack queue
		this.sess.Suback().Ack(msg)
		this.processAcked(this.sess.Suback())

	case *messagev5.UnsubscribeMessage:
		// For UNSUBSCRIBE messagev5, we should remove subscriber, then send back UNSUBACK
		return this.processUnsubscribe(msg)

	case *messagev5.UnsubackMessage:
		// For UNSUBACK messagev5, we should send to ack queue
		this.sess.Unsuback().Ack(msg)
		this.processAcked(this.sess.Unsuback())

	case *messagev5.PingreqMessage:
		// For PINGREQ messagev5, we should send back PINGRESP
		resp := messagev5.NewPingrespMessage()
		_, err = this.writeMessage(resp)

	case *messagev5.PingrespMessage:
		this.sess.Pingack().Ack(msg)
		this.processAcked(this.sess.Pingack())

	case *messagev5.DisconnectMessage:
		logger.Logger.Debugf("client %v disconnect reason code: %s", this.cid(), msg.ReasonCode())
		// 0x04 包含遗嘱消息的断开  客户端希望断开但也需要服务端发布它的遗嘱消息。
		if msg.ReasonCode() != messagev5.DisconnectionIncludesWill {
			this.sess.Cmsg().SetWillFlag(false)
		}
		// 判断是否需要重新设置session过期时间
		if msg.SessionExpiryInterval() > 0 {
			if this.sess.Cmsg().SessionExpiryInterval() == 0 {
				// 如果CONNECT报文中的会话过期间隔为0，则客户端在DISCONNECT报文中设置非0会话过期间隔将造成协议错误（Protocol Error）
				// 如果服务端收到这种非0会话过期间隔，则不会将其视为有效的DISCONNECT报文。
				// TODO  服务端使用包含原因码为0x82（协议错误）的DISCONNECT报文
				return errDisconnect // 这里暂时还是简单处理
			}
			// TODO 需要更新过期间隔
			this.sess.Cmsg().SetSessionExpiryInterval(msg.SessionExpiryInterval())
			err := this.SessionStore.StoreSession(context.Background(), this.cid(), this.sess)
			if err != nil {
				return err
			}
		}
		return errDisconnect

	default:
		return fmt.Errorf("(%s) invalid messagev5 type %s.", this.cid(), msg.Name())
	}

	if err != nil {
		logger.Logger.Debugf("(%s) Error processing acked messagev5: %v", this.cid(), err)
	}

	return err
}

func (this *service) processAcked(ackq sessionsv5.Ackqueue) {
	for _, ackmsg := range ackq.Acked() {
		// Let's get the messages from the saved messagev5 byte slices.
		//让我们从保存的消息字节片获取消息。
		msg, err := ackmsg.Mtype.New()
		if err != nil {
			logger.Logger.Errorf("process/processAcked: Unable to creating new %s messagev5: %v", ackmsg.Mtype, err)
			continue
		}

		if msg.Type() != messagev5.PINGREQ {
			if _, err := msg.Decode(ackmsg.Msgbuf); err != nil {
				logger.Logger.Errorf("process/processAcked: Unable to decode %s messagev5: %v", ackmsg.Mtype, err)
				continue
			}
		}

		ack, err := ackmsg.State.New()
		if err != nil {
			logger.Logger.Errorf("process/processAcked: Unable to creating new %s messagev5: %v", ackmsg.State, err)
			continue
		}

		if ack.Type() == messagev5.PINGRESP {
			logger.Logger.Debug("process/processAcked: PINGRESP")
			continue
		}

		if _, err := ack.Decode(ackmsg.Ackbuf); err != nil {
			logger.Logger.Errorf("process/processAcked: Unable to decode %s messagev5: %v", ackmsg.State, err)
			continue
		}

		//glog.Debugf("(%s) Processing acked messagev5: %v", this.cid(), ack)

		// - PUBACK if it's QoS 1 messagev5. This is on the client side.
		// - PUBREL if it's QoS 2 messagev5. This is on the server side.
		// - PUBCOMP if it's QoS 2 messagev5. This is on the client side.
		// - SUBACK if it's a subscribe messagev5. This is on the client side.
		// - UNSUBACK if it's a unsubscribe messagev5. This is on the client side.
		//如果是QoS 1消息，则返回。这是在客户端。
		//- PUBREL，如果它是QoS 2消息。这是在服务器端。
		//如果是QoS 2消息，则为PUBCOMP。这是在客户端。
		//- SUBACK如果它是一个订阅消息。这是在客户端。
		//- UNSUBACK如果它是一个取消订阅的消息。这是在客户端。
		switch ackmsg.State {
		case messagev5.PUBREL:
			// If ack is PUBREL, that means the QoS 2 messagev5 sent by a remote client is
			// releassed, so let's publish it to other subscribers.
			//如果ack为PUBREL，则表示远程客户端发送的QoS 2消息为
			//发布了，让我们把它发布给其他订阅者吧。
			if err = this.onPublish(msg.(*messagev5.PublishMessage)); err != nil {
				logger.Logger.Errorf("(%s) Error processing ack'ed %s messagev5: %v", this.cid(), ackmsg.Mtype, err)
			}

		case messagev5.PUBACK, messagev5.PUBCOMP, messagev5.SUBACK, messagev5.UNSUBACK:
			logger.Logger.Debugf("process/processAcked: %s", ack)
			// If ack is PUBACK, that means the QoS 1 messagev5 sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是PUBACK，则表示此服务发送的QoS 1消息已获取
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PUBCOMP, that means the QoS 2 messagev5 sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack为PUBCOMP，则表示此服务发送的QoS 2消息已获得
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is SUBACK, that means the SUBSCRIBE messagev5 sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是SUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is UNSUBACK, that means the SUBSCRIBE messagev5 sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是UNSUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PINGRESP, that means the PINGREQ messagev5 sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			// PINGRESP 直接跳过了，因为发送ping时，我们选择了不保存ping数据，所以这里无法验证
			err = nil

		default:
			logger.Logger.Errorf("(%s) Invalid ack messagev5 type %s.", this.cid(), ackmsg.State)
			continue
		}

		// Call the registered onComplete function
		if ackmsg.OnComplete != nil {
			onComplete, ok := ackmsg.OnComplete.(OnCompleteFunc)
			if !ok {
				logger.Logger.Errorf("process/processAcked: Error type asserting onComplete function: %v", reflect.TypeOf(ackmsg.OnComplete))
			} else if onComplete != nil {
				if err := onComplete(msg, ack, nil); err != nil {
					logger.Logger.Errorf("process/processAcked: Error running onComplete(): %v", err)
				}
			}
		}
	}
}

// For PUBLISH messagev5, we should figure out what QoS it is and process accordingly
// If QoS == 0, we should just take the next step, no ack required
// If QoS == 1, we should send back PUBACK, then take the next step
// If QoS == 2, we need to put it in the ack queue, send back PUBREC
//对于发布消息，我们应该弄清楚它的QoS是什么，并相应地进行处理
//如果QoS == 0，我们应该采取下一步，不需要ack
//如果QoS == 1，我们应该返回PUBACK，然后进行下一步
//如果QoS == 2，我们需要将其放入ack队列中，发送回PUBREC
func (this *service) processPublish(msg *messagev5.PublishMessage) error {

	switch msg.QoS() {
	case messagev5.QosExactlyOnce:
		this.sess.Pub2in().Wait(msg, nil)

		resp := messagev5.NewPubrecMessage()
		resp.SetPacketId(msg.PacketId())

		_, err := this.writeMessage(resp)
		return err

	case messagev5.QosAtLeastOnce:
		resp := messagev5.NewPubackMessage()
		resp.SetPacketId(msg.PacketId())

		if _, err := this.writeMessage(resp); err != nil {
			return err
		}

		return this.onPublish(msg)

	case messagev5.QosAtMostOnce:
		return this.onPublish(msg)
	}

	return fmt.Errorf("(%s) invalid messagev5 QoS %d.", this.cid(), msg.QoS())
}

var sharePrefix = []byte("$share/")

// For SUBSCRIBE messagev5, we should add subscriber, then send back SUBACK
func (this *service) processSubscribe(msg *messagev5.SubscribeMessage) error {
	resp := messagev5.NewSubackMessage()
	resp.SetPacketId(msg.PacketId())

	// Subscribe to the different topics
	//订阅不同的主题
	var retcodes []byte

	topics := msg.Topics()
	qos := msg.Qos()

	this.rmsgs = this.rmsgs[0:0]

	for i, t := range topics {
		noLocal := msg.TopicNoLocal(t)
		retainAsPublished := msg.TopicRetainAsPublished(t)
		retainHandling := msg.TopicRetainHandling(t)
		tp := t
		rqos, err := this.topicsMgr.Subscribe(topicsv5.Sub{
			Topic:             tp,
			Qos:               qos[i],
			NoLocal:           noLocal,
			RetainAsPublished: retainAsPublished,
			RetainHandling:    retainHandling,
		}, &this.onpub)
		if err != nil {
			return err
		}
		tp2 := t
		err = this.sess.AddTopic(topicsv5.Sub{
			Topic:             tp2,
			Qos:               qos[i],
			NoLocal:           noLocal,
			RetainAsPublished: retainAsPublished,
			RetainHandling:    retainHandling,
		})

		retcodes = append(retcodes, rqos)

		// yeah I am not checking errors here. If there's an error we don't want the
		// subscription to stop, just let it go.
		//是的，我没有检查错误。如果有错误，我们不想
		//订阅要停止，就放手吧。
		if retainHandling == messagev5.NoSendRetain {
			continue
		} else if retainHandling == messagev5.CanSendRetain {
			_ = this.topicsMgr.Retained(t, &this.rmsgs)
			logger.Logger.Debugf("(%s) topic = %s, retained count = %d", this.cid(), t, len(this.rmsgs))
		} else if retainHandling == messagev5.NoExistSubSendRetain {
			// 已存在订阅的情况下不发送保留消息是很有用的，比如重连完成时客户端不确定订阅是否在之前的会话连接中被创建。
			oldTp, er := this.sess.Topics()
			if er != nil {
				return er
			}
			existThisTopic := false
			for jk := 0; jk < len(oldTp); jk++ {
				if len(oldTp[jk].Topic) != len(tp) {
					continue
				}
				otp := oldTp[jk].Topic
				for jj := 0; jj < len(otp); jj++ {
					if otp[jj] != tp[jj] {
						goto END
					}
				}
				// 存在就不发送了
				existThisTopic = true
				break
			END:
			}
			if !existThisTopic {
				_ = this.topicsMgr.Retained(t, &this.rmsgs)
				logger.Logger.Debugf("(%s) topic = %s, retained count = %d", this.cid(), t, len(this.rmsgs))
			}
		}

		/**
		* 可以在这里向其它集群节点发送添加主题消息
		**/

	}
	logger.Logger.Infof("客户端：%s，订阅主题：%s，qos：%d，retained count = %d", this.cid(), topics, qos, len(this.rmsgs))
	if err := resp.AddReasonCodes(retcodes); err != nil {
		return err
	}

	if _, err := this.writeMessage(resp); err != nil {
		return err
	}

	this.processToCluster(topics, msg)
	for _, rm := range this.rmsgs {
		// TODO
		old := rm.QoS()
		// 这里也做了调整
		if rm.QoS() > qos[0] {
			rm.SetQoS(qos[0])
		}
		if err := this.publish(rm, nil); err != nil {
			logger.Logger.Errorf("service/processSubscribe: Error publishing retained messagev5: %v", err)
			rm.SetQoS(old)
			return err
		}
		rm.SetQoS(old)
	}

	return nil
}

// For UNSUBSCRIBE messagev5, we should remove the subscriber, and send back UNSUBACK
func (this *service) processUnsubscribe(msg *messagev5.UnsubscribeMessage) error {
	topics := msg.Topics()

	for _, t := range topics {
		this.topicsMgr.Unsubscribe(t, &this.onpub)
		this.sess.RemoveTopic(string(t))
	}

	resp := messagev5.NewUnsubackMessage()
	resp.SetPacketId(msg.PacketId())
	resp.AddReasonCode(messagev5.Success.Value())

	_, err := this.writeMessage(resp)
	if err != nil {
		return err
	}

	this.processToCluster(topics, msg)
	logger.Logger.Infof("客户端：%s 取消订阅主题：%s", this.cid(), topics)
	return nil
}

// processToCluster 分主题发送到其它节点发送
func (this *service) processToCluster(topics [][]byte, msg messagev5.Message) {
	if !this.clusterOpen {
		return
	}
	tag := len(topics)
	for i := 0; i < len(topics); i++ {
		if len(topics[i]) < 10 { // $share/{shareName}/{topic} 至少长度为9，即其中shareName不能为空，topic也不能为空
			tag--
			continue
		}
		for j := 0; j < len(sharePrefix); j++ {
			if topics[i][j] != sharePrefix[j] {
				tag--
				break
			}
		}
		// 共享名称组
		k := 0
		for j := len(sharePrefix); j < len(topics[i]); j++ {
			if topics[i][j] == '/' {
				k = j
				break
			}
		}
		if k == len(sharePrefix) || k == len(topics[i])-1 {
			tag--
			continue
		}
		switch msg.Type() {
		case messagev5.SUBSCRIBE:
			// 添加本地集群共享订阅
			this.shareTopicMapNode.AddTopicMapNode(topics[i][k+1:], string(topics[i][len(sharePrefix):k]), GetServerName())
			logger.Logger.Debugf("添加本地集群共享订阅：%s,%s,%s", topics[i][k+1:], string(topics[i][len(sharePrefix):k]), GetServerName())
		case messagev5.UNSUBSCRIBE:
			// 删除本地集群共享订阅
			this.shareTopicMapNode.RemoveTopicMapNode(topics[i][k+1:], string(topics[i][len(sharePrefix):k]), GetServerName())
			logger.Logger.Debugf("删除本地集群共享订阅：%s,%s,%s", topics[i][k+1:], string(topics[i][len(sharePrefix):k]), GetServerName())
		}
	}
	if tag > 0 { // 确实是有共享主题，才发送到集群其它节点
		this.sendCluster(msg) // 发送到其它节点，不需要qos处理的，TODO 但是需要考虑session相关问题
	}
}

// onPublish() is called when the server receives a PUBLISH messagev5 AND have completed
// the ack cycle. This method will get the list of subscribers based on the publish
// topic, and publishes the messagev5 to the list of subscribers.
// onPublish()在服务器接收到发布消息并完成时调用
// ack循环。此方法将根据发布获取订阅服务器列表
//主题，并将消息发布到订阅方列表。
func (this *service) onPublish(msg1 *messagev5.PublishMessage) error {
	if msg1.Retain() {
		if err := this.topicsMgr.Retain(msg1); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", this.cid(), err)
		}
	}
	b := make([]byte, msg1.Len())
	_, _ = msg1.Encode(b)                   // 这里没有选择直接拿msg内的buf
	tmpMsg := messagev5.NewPublishMessage() // 必须重新弄一个，防止被下面改动qos引起bug
	_, _ = tmpMsg.Decode(b)
	this.sendShareToCluster(tmpMsg) // todo 共享订阅时把非本地选项设为1将造成协议错误
	this.sendCluster(tmpMsg)

	// 发送非共享订阅主题, 这里面会改动qos
	return this.pubFn(msg1, "", false)
}

// 发送共享主题到集群其它节点去，以共享组的方式发送
func (this *service) sendShareToCluster(msg1 *messagev5.PublishMessage) {
	if !this.clusterOpen {
		// 没开集群，就只要发送到当前节点下就行
		// 发送当前主题的所有共享组
		err := this.pubFnPlus(msg1)
		if err != nil {
			logger.Logger.Errorf("%v 发送共享：%v 主题错误：%+v", this.id, msg1.Topic(), *msg1)
		}
		return
	}
	submit(func() {
		// 发送共享主题消息
		sn, err := this.shareTopicMapNode.GetShareNames(msg1.Topic())
		if err != nil {
			logger.Logger.Errorf("%v 获取主题%s共享组错误:%v", this.id, msg1.Topic(), err)
			return
		}
		for shareName, node := range sn {
			if node == GetServerName() {
				err = this.pubFn(msg1, shareName, true)
				if err != nil {
					// FIXME 错误处理
				}
			} else {
				colong.SendMsgToCluster(msg1, shareName, node, nil, nil, nil)
			}
		}
	})
}

// 发送到集群其它节点去
func (this *service) sendCluster(message messagev5.Message) {
	if !this.clusterOpen {
		return
	}
	submit(func() {
		colong.SendMsgToCluster(message, "", "", func(message messagev5.Message) {

		}, func(name string, message messagev5.Message) {

		}, func(name string, message messagev5.Message) {

		})
	})
}

// 集群节点发来的普通消息
func (this *service) ClusterInToPub(msg1 *messagev5.PublishMessage) error {
	if !this.clusterBelong {
		return nil
	}
	if msg1.Retain() {
		if err := this.topicsMgr.Retain(msg1); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", this.cid(), err)
		}
	}
	// 发送非共享订阅主题
	return this.pubFn(msg1, "", false)
}

// 集群节点发来的共享主题消息，需要发送到特定的共享组 和 普通节点
func (this *service) ClusterInToPubShare(msg1 *messagev5.PublishMessage, shareName string) error {
	if !this.clusterBelong {
		return nil
	}
	if msg1.Retain() {
		if err := this.topicsMgr.Retain(msg1); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", this.cid(), err)
		}
	}
	// 发送非共享订阅主题
	return this.pubFn(msg1, shareName, false)
}

// 集群节点发来的系统主题消息
func (this *service) ClusterInToPubSys(msg1 *messagev5.PublishMessage) error {
	if !this.clusterBelong {
		return nil
	}
	if msg1.Retain() {
		if err := this.topicsMgr.Retain(msg1); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", this.cid(), err)
		}
	}
	// 发送
	return this.pubFnSys(msg1)
}

func (this *service) pubFn(msg *messagev5.PublishMessage, shareName string, onlyShare bool) error {
	var (
		subs []interface{}
		qoss []topicsv5.Sub
	)

	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &subs, &qoss, false, shareName, onlyShare)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}
	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) Publishing to topic %s and %s subscribers：%v", this.cid(), msg.Topic(), shareName, len(subs))
	for i, s := range subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				return fmt.Errorf("Invalid onPublish Function")
			} else {
				_ = msg.SetQoS(qoss[i].Qos) // 设置为该发的qos级别
				err = (*fn)(msg, this.cid(), onlyShare)
				if err == io.EOF {
					// TODO 断线了，是否对于qos=1和2的保存至离线消息
				}
			}
		}
	}
	return nil
}

// 发送当前主题所有共享组
func (this *service) pubFnPlus(msg *messagev5.PublishMessage) error {
	var (
		subs []interface{}
		qoss []topicsv5.Sub
	)
	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &subs, &qoss, false, "", true)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}
	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) Publishing to all shareName topic in %s to subscribers：%v", this.cid(), msg.Topic(), subs)
	for i, s := range subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				return fmt.Errorf("Invalid onPublish Function")
			} else {
				_ = msg.SetQoS(qoss[i].Qos) // 设置为该发的qos级别
				err = (*fn)(msg, this.cid(), true)
				if err == io.EOF {
					// TODO 断线了，是否对于qos=1和2的保存至离线消息
				}
			}
		}
	}
	return nil
}

// 发送系统消息
func (this *service) pubFnSys(msg *messagev5.PublishMessage) error {
	var (
		subs []interface{}
		qoss []topicsv5.Sub
	)
	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &subs, &qoss, true, "", false)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}
	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) Publishing sys topic %s to subscribers：%v", this.cid(), msg.Topic(), subs)
	for i, s := range subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				return fmt.Errorf("Invalid onPublish Function")
			} else {
				_ = msg.SetQoS(qoss[i].Qos) // 设置为该发的qos级别
				err = (*fn)(msg, this.cid(), false)
				if err == io.EOF {
					// TODO 断线了，是否对于qos=1和2的保存至离线消息
				}
			}
		}
	}
	return nil
}
