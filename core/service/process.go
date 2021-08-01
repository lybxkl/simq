package service

import (
	"SI-MQTT/core/logger"
	"SI-MQTT/core/message"
	"SI-MQTT/core/sessions"
	"errors"
	"fmt"

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
			logger.Logger.Error(fmt.Sprintf("(%s) Recovering from panic: %v", this.cid(), r))
		}

		this.wgStopped.Done()
		this.stop()
		logger.Logger.Debug(fmt.Sprintf("(%s) Stopping processor", this.cid()))
	}()

	logger.Logger.Debug(fmt.Sprintf("(%s) Starting processor", this.cid()))

	this.wgStarted.Done()

	for {
		// 1. Find out what message is next and the size of the message
		//了解接下来是什么消息以及消息的大小
		mtype, total, err := this.peekMessageSize()
		if err != nil {
			if err != io.EOF {
				logger.Logger.Error(fmt.Sprintf("(%s) Error peeking next message size", this.cid()))
			}
			return
		}

		msg, n, err := this.peekMessage(mtype, total)
		if err != nil {
			if err != io.EOF {
				logger.Logger.Error(fmt.Sprintf("(%s) Error peeking next message: %v", this.cid(), err))
			}
			return
		}

		logger.Logger.Debug(fmt.Sprintf("(%s) Received: %s", this.cid(), msg))

		this.inStat.increment(int64(n))

		// 5. Process the read message
		//处理读消息
		err = this.processIncoming(msg)
		if err != nil {
			if err != errDisconnect {
				logger.Logger.Error(fmt.Sprintf("(%s) Error processing %s: %v", this.cid(), msg.Name(), err))
			} else {
				return
			}
		}

		// 7. We should commit the bytes in the buffer so we can move on
		// 我们应该提交缓冲区中的字节，这样我们才能继续
		_, err = this.in.ReadCommit(total)
		if err != nil {
			if err != io.EOF {
				logger.Logger.Error(fmt.Sprintf("(%s) Error committing %d read bytes: %v", this.cid(), total, err))
			}
			return
		}

		// 7. Check to see if done is closed, if so, exit
		// 检查done是否关闭，如果关闭，退出
		if this.isDone() && this.in.Len() == 0 {
			return
		}

		if this.inStat.msgs%100000 == 0 {
			logger.Logger.Warn(fmt.Sprintf("(%s) Going to process message %d", this.cid(), this.inStat.msgs))
		}
	}
}

func (this *service) processIncoming(msg message.Message) error {
	var err error = nil

	switch msg := msg.(type) {
	case *message.PublishMessage:
		// For PUBLISH message, we should figure out what QoS it is and process accordingly
		// If QoS == 0, we should just take the next step, no ack required
		// If QoS == 1, we should send back PUBACK, then take the next step
		// If QoS == 2, we need to put it in the ack queue, send back PUBREC
		err = this.processPublish(msg)

	case *message.PubackMessage:
		// For PUBACK message, it means QoS 1, we should send to ack queue
		this.sess.Pub1ack.Ack(msg)
		this.processAcked(this.sess.Pub1ack)

	case *message.PubrecMessage:
		// For PUBREC message, it means QoS 2, we should send to ack queue, and send back PUBREL
		if err = this.sess.Pub2out.Ack(msg); err != nil {
			break
		}

		resp := message.NewPubrelMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = this.writeMessage(resp)

	case *message.PubrelMessage:
		// For PUBREL message, it means QoS 2, we should send to ack queue, and send back PUBCOMP
		if err = this.sess.Pub2in.Ack(msg); err != nil {
			break
		}

		this.processAcked(this.sess.Pub2in)

		resp := message.NewPubcompMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = this.writeMessage(resp)

	case *message.PubcompMessage:
		// For PUBCOMP message, it means QoS 2, we should send to ack queue
		if err = this.sess.Pub2out.Ack(msg); err != nil {
			break
		}

		this.processAcked(this.sess.Pub2out)

	case *message.SubscribeMessage:
		// For SUBSCRIBE message, we should add subscriber, then send back SUBACK
		//对于订阅消息，我们应该添加订阅者，然后发送回SUBACK
		return this.processSubscribe(msg)

	case *message.SubackMessage:
		// For SUBACK message, we should send to ack queue
		this.sess.Suback.Ack(msg)
		this.processAcked(this.sess.Suback)

	case *message.UnsubscribeMessage:
		// For UNSUBSCRIBE message, we should remove subscriber, then send back UNSUBACK
		return this.processUnsubscribe(msg)

	case *message.UnsubackMessage:
		// For UNSUBACK message, we should send to ack queue
		this.sess.Unsuback.Ack(msg)
		this.processAcked(this.sess.Unsuback)

	case *message.PingreqMessage:
		// For PINGREQ message, we should send back PINGRESP
		resp := message.NewPingrespMessage()
		_, err = this.writeMessage(resp)

	case *message.PingrespMessage:
		this.sess.Pingack.Ack(msg)
		this.processAcked(this.sess.Pingack)

	case *message.DisconnectMessage:
		// For DISCONNECT message, we should quit
		this.sess.Cmsg.SetWillFlag(false)
		return errDisconnect

	default:
		return fmt.Errorf("(%s) invalid message type %s.", this.cid(), msg.Name())
	}

	if err != nil {
		logger.Logger.Debug(fmt.Sprintf("(%s) Error processing acked message: %v", this.cid(), err))
	}

	return err
}

func (this *service) processAcked(ackq *sessions.Ackqueue) {
	for _, ackmsg := range ackq.Acked() {
		// Let's get the messages from the saved message byte slices.
		//让我们从保存的消息字节片获取消息。
		msg, err := ackmsg.Mtype.New()
		if err != nil {
			logger.Logger.Error(fmt.Sprintf("process/processAcked: Unable to creating new %s message: %v", ackmsg.Mtype, err))
			continue
		}

		if _, err := msg.Decode(ackmsg.Msgbuf); err != nil {
			logger.Logger.Error(fmt.Sprintf("process/processAcked: Unable to decode %s message: %v", ackmsg.Mtype, err))
			continue
		}

		ack, err := ackmsg.State.New()
		if err != nil {
			logger.Logger.Error(fmt.Sprintf("process/processAcked: Unable to creating new %s message: %v", ackmsg.State, err))
			continue
		}

		if _, err := ack.Decode(ackmsg.Ackbuf); err != nil {
			logger.Logger.Error(fmt.Sprintf("process/processAcked: Unable to decode %s message: %v", ackmsg.State, err))
			continue
		}

		//glog.Debugf("(%s) Processing acked message: %v", this.cid(), ack)

		// - PUBACK if it's QoS 1 message. This is on the client side.
		// - PUBREL if it's QoS 2 message. This is on the server side.
		// - PUBCOMP if it's QoS 2 message. This is on the client side.
		// - SUBACK if it's a subscribe message. This is on the client side.
		// - UNSUBACK if it's a unsubscribe message. This is on the client side.
		//如果是QoS 1消息，则返回。这是在客户端。
		//- PUBREL，如果它是QoS 2消息。这是在服务器端。
		//如果是QoS 2消息，则为PUBCOMP。这是在客户端。
		//- SUBACK如果它是一个订阅消息。这是在客户端。
		//- UNSUBACK如果它是一个取消订阅的消息。这是在客户端。
		switch ackmsg.State {
		case message.PUBREL:
			// If ack is PUBREL, that means the QoS 2 message sent by a remote client is
			// releassed, so let's publish it to other subscribers.
			//如果ack为PUBREL，则表示远程客户端发送的QoS 2消息为
			//发布了，让我们把它发布给其他订阅者吧。
			if err = this.onPublish(msg.(*message.PublishMessage)); err != nil {
				logger.Logger.Error(fmt.Sprintf("(%s) Error processing ack'ed %s message: %v", this.cid(), ackmsg.Mtype, err))
			}

		case message.PUBACK, message.PUBCOMP, message.SUBACK, message.UNSUBACK, message.PINGRESP:
			logger.Logger.Debug(fmt.Sprintf("process/processAcked: %s", ack))
			// If ack is PUBACK, that means the QoS 1 message sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是PUBACK，则表示此服务发送的QoS 1消息已获取
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PUBCOMP, that means the QoS 2 message sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack为PUBCOMP，则表示此服务发送的QoS 2消息已获得
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is SUBACK, that means the SUBSCRIBE message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是SUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is UNSUBACK, that means the SUBSCRIBE message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是UNSUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PINGRESP, that means the PINGREQ message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是PINGRESP，则表示此服务发送的PINGREQ消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。
			err = nil

		default:
			logger.Logger.Error(fmt.Sprintf("(%s) Invalid ack message type %s.", this.cid(), ackmsg.State))
			continue
		}

		// Call the registered onComplete function
		if ackmsg.OnComplete != nil {
			onComplete, ok := ackmsg.OnComplete.(OnCompleteFunc)
			if !ok {
				logger.Logger.Error(fmt.Sprintf("process/processAcked: Error type asserting onComplete function: %v", reflect.TypeOf(ackmsg.OnComplete)))
			} else if onComplete != nil {
				if err := onComplete(msg, ack, nil); err != nil {
					logger.Logger.Error(fmt.Sprintf("process/processAcked: Error running onComplete(): %v", err))
				}
			}
		}
	}
}

// For PUBLISH message, we should figure out what QoS it is and process accordingly
// If QoS == 0, we should just take the next step, no ack required
// If QoS == 1, we should send back PUBACK, then take the next step
// If QoS == 2, we need to put it in the ack queue, send back PUBREC
//对于发布消息，我们应该弄清楚它的QoS是什么，并相应地进行处理
//如果QoS == 0，我们应该采取下一步，不需要ack
//如果QoS == 1，我们应该返回PUBACK，然后进行下一步
//如果QoS == 2，我们需要将其放入ack队列中，发送回PUBREC
func (this *service) processPublish(msg *message.PublishMessage) error {

	switch msg.QoS() {
	case message.QosExactlyOnce:
		this.sess.Pub2in.Wait(msg, nil)

		resp := message.NewPubrecMessage()
		resp.SetPacketId(msg.PacketId())

		_, err := this.writeMessage(resp)
		return err

	case message.QosAtLeastOnce:
		resp := message.NewPubackMessage()
		resp.SetPacketId(msg.PacketId())

		if _, err := this.writeMessage(resp); err != nil {
			return err
		}

		return this.onPublish(msg)

	case message.QosAtMostOnce:
		return this.onPublish(msg)
	}

	return fmt.Errorf("(%s) invalid message QoS %d.", this.cid(), msg.QoS())
}

// For SUBSCRIBE message, we should add subscriber, then send back SUBACK
func (this *service) processSubscribe(msg *message.SubscribeMessage) error {
	resp := message.NewSubackMessage()
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
		logger.Logger.Debug(fmt.Sprintf("(%s) topic = %s, retained count = %d", this.cid(), string(t), len(this.rmsgs)))

		/**
		* 可以在这里向其它集群节点发送添加主题消息
		* 我选择带缓存的channel
		**/

	}
	logger.Logger.Info(fmt.Sprintf("客户端：%s，订阅主题：%s，qos：%d，retained count = %d", this.cid(), topics, qos, len(this.rmsgs)))
	if err := resp.AddReturnCodes(retcodes); err != nil {
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
			logger.Logger.Error(fmt.Sprintf("service/processSubscribe: Error publishing retained message: %v", err))
			rm.SetQoS(old)
			return err
		}
		rm.SetQoS(old)
	}

	return nil
}

// For UNSUBSCRIBE message, we should remove the subscriber, and send back UNSUBACK
func (this *service) processUnsubscribe(msg *message.UnsubscribeMessage) error {
	topics := msg.Topics()

	for _, t := range topics {
		this.topicsMgr.Unsubscribe(t, &this.onpub)
		this.sess.RemoveTopic(string(t))
		/**
		* 可以在这里向其它节点发送移除主题消息
		* 我选择带缓存的channel
		**/
	}

	resp := message.NewUnsubackMessage()
	resp.SetPacketId(msg.PacketId())

	_, err := this.writeMessage(resp)

	logger.Logger.Info(fmt.Sprintf("客户端：%s 取消订阅主题：%s", this.cid(), topics))
	return err
}

// onPublish() is called when the server receives a PUBLISH message AND have completed
// the ack cycle. This method will get the list of subscribers based on the publish
// topic, and publishes the message to the list of subscribers.
// onPublish()在服务器接收到发布消息并完成时调用
// ack循环。此方法将根据发布获取订阅服务器列表
//主题，并将消息发布到订阅方列表。
func (this *service) onPublish(msg1 *message.PublishMessage) error {
	if msg1.Retain() {
		if err := this.topicsMgr.Retain(msg1); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Error(fmt.Sprintf("(%s) Error retaining message: %v", this.cid(), err))
		}
	}
	// todo 集群模式，需要另外设计
	// 发送当前主题的所有共享组
	err := this.pubFnPlus(msg1)
	if err != nil {
		logger.Logger.Error(fmt.Sprintf("%v 发送共享：%v 主题错误：%+v", this.id, msg1.Topic(), *msg1))
	}
	// 发送非共享订阅主题
	return this.pubFn(msg1, "", false)
}
func (this *service) pubFn(msg *message.PublishMessage, shareName string, onlyShare bool) error {
	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &this.subs, &this.qoss, false, shareName, onlyShare)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}
	msg.SetRetain(false)
	logger.Logger.Debug(fmt.Sprintf("(%s) Publishing to topic %q and %d subscribers：%v", this.cid(), string(msg.Topic()), len(this.subs), shareName))
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
func (this *service) pubFnPlus(msg *message.PublishMessage) error {
	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &this.subs, &this.qoss, false, "", true)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}
	msg.SetRetain(false)
	logger.Logger.Debug(fmt.Sprintf("(%s) Publishing to all shareName topic in %q and %d subscribers：%v", this.cid(), string(msg.Topic()), len(this.subs)))
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
