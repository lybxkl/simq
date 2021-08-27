package servicev5

import (
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
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
		// For DISCONNECT messagev5, we should quit
		// 主动断开连接，不需要发送will消息，这里直接设置为false，外面处理就不会发送will了
		this.sess.Cmsg().SetWillFlag(false)
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

		if _, err := msg.Decode(ackmsg.Msgbuf); err != nil {
			logger.Logger.Errorf("process/processAcked: Unable to decode %s messagev5: %v", ackmsg.Mtype, err)
			continue
		}

		ack, err := ackmsg.State.New()
		if err != nil {
			logger.Logger.Errorf("process/processAcked: Unable to creating new %s messagev5: %v", ackmsg.State, err)
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

		case messagev5.PUBACK, messagev5.PUBCOMP, messagev5.SUBACK, messagev5.UNSUBACK, messagev5.PINGRESP:
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
			//如果ack是PINGRESP，则表示此服务发送的PINGREQ消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。
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
		rqos, err := this.topicsMgr.Subscribe(t, qos[i], &this.onpub)
		if err != nil {
			return err
		}
		this.sess.AddTopic(string(t), qos[i])

		retcodes = append(retcodes, rqos)

		// yeah I am not checking errors here. If there's an error we don't want the
		// subscription to stop, just let it go.
		//是的，我没有检查错误。如果有错误，我们不想
		//订阅要停止，就放手吧。
		this.topicsMgr.Retained(t, &this.rmsgs)
		logger.Logger.Debugf("(%s) topic = %s, retained count = %d", this.cid(), t, len(this.rmsgs))

		/**
		* 可以在这里向其它集群节点发送添加主题消息
		* 我选择带缓存的channel
		**/

	}
	logger.Logger.Infof("客户端：%s，订阅主题：%s，qos：%d，retained count = %d", this.cid(), topics, qos, len(this.rmsgs))
	if err := resp.AddReasonCodes(retcodes); err != nil {
		return err
	}

	if _, err := this.writeMessage(resp); err != nil {
		return err
	}

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
		/**
		* 可以在这里向其它节点发送移除主题消息
		**/
	}

	resp := messagev5.NewUnsubackMessage()
	resp.SetPacketId(msg.PacketId())
	resp.AddReasonCode(messagev5.Success.Value())

	_, err := this.writeMessage(resp)

	logger.Logger.Infof("客户端：%s 取消订阅主题：%s", this.cid(), topics)
	return err
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
	// todo 集群模式，需要另外设计
	// 发送当前主题的所有共享组
	err := this.pubFnPlus(msg1)
	if err != nil {
		logger.Logger.Errorf("%v 发送共享：%v 主题错误：%+v", this.id, msg1.Topic(), *msg1)
	}
	// 发送非共享订阅主题
	return this.pubFn(msg1, "", false)
}
func (this *service) pubFn(msg *messagev5.PublishMessage, shareName string, onlyShare bool) error {
	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &this.subs, &this.qoss, false, shareName, onlyShare)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}
	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) Publishing to topic %s and %d subscribers：%v", this.cid(), msg.Topic(), len(this.subs), shareName)
	for i, s := range this.subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				return fmt.Errorf("Invalid onPublish Function")
			} else {
				_ = msg.SetQoS(this.qoss[i]) // 设置为该发的qos级别
				err = (*fn)(msg)
				if err == io.EOF {
					// TODO 断线了，是否对于qos=1和2的保存至离线消息
				}
			}
		}
	}
	return nil
}
func (this *service) pubFnPlus(msg *messagev5.PublishMessage) error {
	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &this.subs, &this.qoss, false, "", true)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}
	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) Publishing to all shareName topic in %s and %d subscribers：%v", this.cid(), msg.Topic(), len(this.subs))
	for i, s := range this.subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				return fmt.Errorf("Invalid onPublish Function")
			} else {
				_ = msg.SetQoS(this.qoss[i]) // 设置为该发的qos级别
				err = (*fn)(msg)
				if err == io.EOF {
					// TODO 断线了，是否对于qos=1和2的保存至离线消息
				}
			}
		}
	}
	return nil
}
