package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/util/bufpool"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/util/cron"
	"gitee.com/Ljolan/si-mqtt/logger"
	"io"
	"net"
	"reflect"
	"time"
)

var (
	errDisconnect = errors.New("disconnect") // 直接close的错误信号
)

// processor() reads messages from the incoming buffer and processes them
// processor()从传入缓冲区读取消息并处理它们
func (svc *service) processor() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Logger.Errorf("(%s) Recovering from panic: %v", svc.cid(), r)
		}

		svc.wgStopped.Done()
		svc.stop()
		logger.Logger.Debugf("(%s) Stopping processor", svc.cid())
	}()

	logger.Logger.Debugf("(%s) Starting processor", svc.cid())

	svc.wgStarted.Done()

	for {
		// 检查done是否关闭，如果关闭，退出
		if svc.isDone() {
			return
		}
		// 流控处理
		if e := svc.streamController(); e != nil {
			logger.Logger.Warn(e)
			return
		}

		//了解接下来是什么消息以及消息的大小
		mtype, total, err := svc.peekMessageSize()
		if err != nil {
			if err != io.EOF {
				logger.Logger.Errorf("(%s) Error peeking next messagev5 size", svc.cid())
			}
			return
		}
		if total > int(svc.conFig.Broker.MaxPacketSize) {
			writeMessage(svc.conn, messagev2.NewDiscMessageWithCodeInfo(messagev2.MessageTooLong, nil))
			logger.Logger.Errorf("(%s): message sent is too large", svc.cid())
			return
		}

		msg, n, err := svc.peekMessage(mtype, total)
		if err != nil {
			if err != io.EOF {
				logger.Logger.Errorf("(%s) Error peeking next messagev5: %v", svc.cid(), err)
			}
			return
		}

		logger.Logger.Debugf("(%s) Received: %s", svc.cid(), msg)

		svc.inStat.increment(int64(n))

		//处理读消息
		err = svc.processIncoming(msg)
		if err != nil {
			if err != errDisconnect {
				logger.Logger.Errorf("(%s) Error processing %s: %v", svc.cid(), msg, err)

				var dis = messagev2.NewDiscMessageWithCodeInfo(messagev2.UnspecifiedError, []byte(err.Error()))
				if reasonErr, ok := err.(*messagev2.Code); ok {
					dis = messagev2.NewDiscMessageWithCodeInfo(reasonErr.ReasonCode, []byte(reasonErr.Error()))
				}

				writeMessage(svc.conn, dis)
			}
			return
		}

		// 我们应该提交缓冲区中的字节，这样我们才能继续
		_, err = svc.in.ReadCommit(total)
		if err != nil {
			if err != io.EOF {
				logger.Logger.Errorf("(%s) Error committing %d read bytes: %v", svc.cid(), total, err)
			}
			return
		}

		// 中间件处理消息，可桥接
		for _, opt := range svc.middleware {
			canSkipErr, middleWareErr := opt.Apply(msg)
			if middleWareErr != nil {
				if canSkipErr {
					logger.Logger.Errorf("(%s) middlewrae deal msg %s error [skip]: %v", svc.cid(), msg, middleWareErr)
				} else {
					logger.Logger.Errorf("(%s) middlewrae deal msg %s error [no_skip]: %v", svc.cid(), msg, middleWareErr)
					break
				}
			}
		}
	}
}

// 流控
func (svc *service) streamController() error {
	// 监控流量
	if svc.inStat.msgs%100000 == 0 {
		logger.Logger.Warn(fmt.Sprintf("(%s) Going to process messagev5 %d", svc.cid(), svc.inStat.msgs))
	}
	if svc.sign.BeyondQuota() {
		_, _ = svc.sendBeyondQuota()
		return fmt.Errorf("(%s) Beyond quota", svc.cid())
	}
	if svc.sign.Limit() {
		_, _ = svc.sendTooManyMessages()
		return fmt.Errorf("(%s) limit req", svc.cid())
	}
	return nil
}

func (svc *service) sendBeyondQuota() (int, error) {
	dis := messagev2.NewDisconnectMessage()
	dis.SetReasonCode(messagev2.BeyondQuota)
	return svc.sendByConn(dis)
}

func (svc *service) sendTooManyMessages() (int, error) {
	dis := messagev2.NewDisconnectMessage()
	dis.SetReasonCode(messagev2.TooManyMessages)
	return svc.sendByConn(dis)
}

// sendByConn 直接从conn连接发送数据
func (svc *service) sendByConn(msg messagev2.Message) (int, error) {
	b := make([]byte, msg.Len())
	_, err := msg.Encode(b)
	if err != nil {
		return 0, err
	}
	return svc.conn.(net.Conn).Write(b)
}

func (svc *service) processIncoming(msg messagev2.Message) error {
	var err error = nil

	switch msg := msg.(type) {
	case *messagev2.PublishMessage:
		// For PUBLISH messagev5, we should figure out what QoS it is and process accordingly
		// If QoS == 0, we should just take the next step, no ack required
		// If QoS == 1, we should send back PUBACK, then take the next step
		// If QoS == 2, we need to put it in the ack queue, send back PUBREC
		err = svc.processPublish(msg)

	case *messagev2.PubackMessage:
		svc.sign.AddQuota() // 增加配额
		// For PUBACK messagev5, it means QoS 1, we should send to ack queue
		if err = svc.sess.Pub1ack().Ack(msg); err != nil {
			err = messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
			break
		}
		svc.processAcked(svc.sess.Pub1ack())
	case *messagev2.PubrecMessage:
		if msg.ReasonCode() > messagev2.QosFailure {
			svc.sign.AddQuota()
		}

		// For PUBREC messagev5, it means QoS 2, we should send to ack queue, and send back PUBREL
		if err = svc.sess.Pub2out().Ack(msg); err != nil {
			err = messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
			break
		}

		resp := messagev2.NewPubrelMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = svc.writeMessage(resp)
		if err != nil {
			err = messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
		}
	case *messagev2.PubrelMessage:
		// For PUBREL messagev5, it means QoS 2, we should send to ack queue, and send back PUBCOMP
		if err = svc.sess.Pub2in().Ack(msg); err != nil {
			break
		}

		svc.processAcked(svc.sess.Pub2in())

		resp := messagev2.NewPubcompMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = svc.writeMessage(resp)
		if err != nil {
			err = messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
		}
	case *messagev2.PubcompMessage:
		svc.sign.AddQuota() // 增加配额

		// For PUBCOMP messagev5, it means QoS 2, we should send to ack queue
		if err = svc.sess.Pub2out().Ack(msg); err != nil {
			err = messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
			break
		}

		svc.processAcked(svc.sess.Pub2out())
	case *messagev2.SubscribeMessage:
		// For SUBSCRIBE messagev5, we should add subscriber, then send back SUBACK
		//对于订阅消息，我们应该添加订阅者，然后发送回SUBACK
		return svc.processSubscribe(msg)

	case *messagev2.SubackMessage:
		// For SUBACK messagev5, we should send to ack queue
		svc.sess.Suback().Ack(msg)
		svc.processAcked(svc.sess.Suback())

	case *messagev2.UnsubscribeMessage:
		// For UNSUBSCRIBE messagev5, we should remove subscriber, then send back UNSUBACK
		return svc.processUnsubscribe(msg)

	case *messagev2.UnsubackMessage:
		// For UNSUBACK messagev5, we should send to ack queue
		svc.sess.Unsuback().Ack(msg)
		svc.processAcked(svc.sess.Unsuback())

	case *messagev2.PingreqMessage:
		// For PINGREQ messagev5, we should send back PINGRESP
		resp := messagev2.NewPingrespMessage()
		_, err = svc.writeMessage(resp)
		if err != nil {
			err = messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
		}
	case *messagev2.PingrespMessage:
		svc.sess.Pingack().Ack(msg)
		svc.processAcked(svc.sess.Pingack())

	case *messagev2.DisconnectMessage:
		logger.Logger.Debugf("client %v disconnect reason code: %s", svc.cid(), msg.ReasonCode())
		// 0x04 包含遗嘱消息的断开  客户端希望断开但也需要服务端发布它的遗嘱消息。
		switch msg.ReasonCode() {
		case messagev2.DisconnectionIncludesWill:
			// 发布遗嘱消息。或者延迟发布
			svc.sendWillMsg()
		case messagev2.Success:
			// 服务端收到包含原因码为0x00（正常关闭） 的DISCONNECT报文之后删除了遗嘱消息
			svc.sess.Cmsg().SetWillMessage(nil)
		default:
			svc.sess.Cmsg().SetWillMessage(nil)
		}

		// 判断是否需要重新设置session过期时间
		if msg.SessionExpiryInterval() > 0 {
			if svc.sess.Cmsg().SessionExpiryInterval() == 0 {
				// 如果CONNECT报文中的会话过期间隔为0，则客户端在DISCONNECT报文中设置非0会话过期间隔将造成协议错误（Protocol Error）
				// 如果服务端收到这种非0会话过期间隔，则不会将其视为有效的DISCONNECT报文。
				// TODO  服务端使用包含原因码为0x82（协议错误）的DISCONNECT报文
				return messagev2.NewCodeErr(messagev2.ProtocolError, fmt.Sprintf("(%s) message protocol error: session_ %d", svc.cid(), msg.SessionExpiryInterval()))
			}
			// TODO 需要更新过期间隔
			svc.sess.Cmsg().SetSessionExpiryInterval(msg.SessionExpiryInterval())
			err = svc.SessionStore.StoreSession(context.Background(), svc.cid(), svc.sess)
			if err != nil {
				return messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
			}
		}
		return errDisconnect

	default:
		return messagev2.NewCodeErr(messagev2.ProtocolError, fmt.Sprintf("(%s) invalid messagev5 type %s", svc.cid(), msg.Name()))
	}

	if err != nil {
		logger.Logger.Debugf("(%s) Error processing acked messagev5: %v", svc.cid(), err)
	}

	return err
}

// 发送遗嘱消息
func (svc *service) sendWillMsg() {
	willMsg := svc.sess.Will()
	if willMsg != nil && !svc.hasSendWill {
		willMsgExpiry := svc.sess.Cmsg().WillMsgExpiryInterval()
		if willMsgExpiry > 0 {
			willMsg.SetMessageExpiryInterval(willMsgExpiry)
		}
		willMsgDelaySendTime := svc.sess.Cmsg().WillDelayInterval()
		if willMsgDelaySendTime == 0 {
			// 直接发送
		} else {
			// 延迟发送，在延迟发送前重新登录，需要取消发送
		}
		cron.DelayTaskManager.DelaySendWill(&cron.DelayWillTask{
			ID:       cron.ID(svc.sess.ClientId()),
			DealTime: time.Duration(willMsgDelaySendTime), // 为0还是由DelayTaskManager处理
			Data:     willMsg,
			Fn: func(data *messagev2.PublishMessage) {
				logger.Logger.Debugf("%s send will message", svc.cid())
				// 发送，只需要发送本地，集群的化，内部实现处理获取消息
				svc.pubFn(data, "", false) // TODO 发送遗嘱嘱消息时是否需要处理共享订阅
			},
		})
		svc.hasSendWill = true
	}
}

func (svc *service) processAcked(ackq sessions.Ackqueue) {
	for _, ackmsg := range ackq.Acked() {
		// Let's get the messages from the saved messagev5 byte slices.
		//让我们从保存的消息字节片获取消息。
		msg, err := ackmsg.Mtype.New()
		if err != nil {
			logger.Logger.Errorf("process/processAcked: Unable to creating new %s messagev5: %v", ackmsg.Mtype, err)
			continue
		}

		if msg.Type() != messagev2.PINGREQ {
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

		if ack.Type() == messagev2.PINGRESP {
			logger.Logger.Debug("process/processAcked: PINGRESP")
			continue
		}

		if _, err := ack.Decode(ackmsg.Ackbuf); err != nil {
			logger.Logger.Errorf("process/processAcked: Unable to decode %s messagev5: %v", ackmsg.State, err)
			continue
		}

		//glog.Debugf("(%s) Processing acked messagev5: %v", svc.cid(), ack)

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
		case messagev2.PUBREL:
			// If ack is PUBREL, that means the QoS 2 messagev5 sent by a remote client is
			// releassed, so let's publish it to other subscribers.
			//如果ack为PUBREL，则表示远程客户端发送的QoS 2消息为
			//发布了，让我们把它发布给其他订阅者吧。
			if err = svc.onPublish(msg.(*messagev2.PublishMessage)); err != nil {
				logger.Logger.Errorf("(%s) Error processing ack'ed %s messagev5: %v", svc.cid(), ackmsg.Mtype, err)
			}

		case messagev2.PUBACK, messagev2.PUBCOMP, messagev2.SUBACK, messagev2.UNSUBACK:
			logger.Logger.Debugf("process/processAcked: %s", ack)
			// If ack is PUBACK, that means the QoS 1 messagev5 sent by svc service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是PUBACK，则表示此服务发送的QoS 1消息已获取
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PUBCOMP, that means the QoS 2 messagev5 sent by svc service got
			// ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack为PUBCOMP，则表示此服务发送的QoS 2消息已获得
			// ack。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is SUBACK, that means the SUBSCRIBE messagev5 sent by svc service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是SUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is UNSUBACK, that means the SUBSCRIBE messagev5 sent by svc service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			//如果ack是UNSUBACK，则表示此服务发送的订阅消息
			//得到“消”。除了调用下面的onComplete()之外，没有什么可以做的。

			// If ack is PINGRESP, that means the PINGREQ messagev5 sent by svc service
			// got ack'ed. There's nothing to do other than calling onComplete() below.
			// PINGRESP 直接跳过了，因为发送ping时，我们选择了不保存ping数据，所以这里无法验证
			err = nil

		default:
			logger.Logger.Errorf("(%s) Invalid ack messagev5 type %s.", svc.cid(), ackmsg.State)
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

// 入消息的主题别名处理 第一阶段 验证
func (svc *service) topicAliceIn(msg *messagev2.PublishMessage) error {
	if msg.TopicAlias() > 0 && len(msg.Topic()) == 0 {
		if tp, ok := svc.sess.GetAliceTopic(msg.TopicAlias()); ok {
			_ = msg.SetTopic(tp)
			logger.Logger.Debugf("%v set topic by alice ==> topic：%v alice：%v", svc.cid(), tp, msg.TopicAlias())
		} else {
			return messagev2.NewCodeErr(messagev2.ProtocolError)
		}
	} else if msg.TopicAlias() > svc.sess.TopicAliasMax() {
		return messagev2.NewCodeErr(messagev2.ProtocolError)
	} else if msg.TopicAlias() > 0 && len(msg.Topic()) > 0 {
		// 需要保存主题别名，只会与当前连接保存生命一致
		if svc.sess.TopicAliasMax() < msg.TopicAlias() {
			return messagev2.NewCodeErr(messagev2.InvalidTopicAlias, "topic alias is not allowed or too large")
		}
		svc.sess.AddTopicAlice(msg.Topic(), msg.TopicAlias())
		logger.Logger.Debugf("%v save topic alice ==> topic：%v alice：%v", svc.cid(), msg.Topic(), msg.TopicAlias())
		msg.SetTopicAlias(0)
	}
	return nil
}

// For PUBLISH messagev5, we should figure out what QoS it is and process accordingly
// If QoS == 0, we should just take the next step, no ack required
// If QoS == 1, we should send back PUBACK, then take the next step
// If QoS == 2, we need to put it in the ack queue, send back PUBREC
//对于发布消息，我们应该弄清楚它的QoS是什么，并相应地进行处理
//如果QoS == 0，我们应该采取下一步，不需要ack
//如果QoS == 1，我们应该返回PUBACK，然后进行下一步
//如果QoS == 2，我们需要将其放入ack队列中，发送回PUBREC
func (svc *service) processPublish(msg *messagev2.PublishMessage) error {
	if svc.conFig.Broker.MaxQos < int(msg.QoS()) { // 判断是否是支持的qos等级，只会验证本broker内收到的，集群转发来的不会验证，前提是因为集群来的已经是验证过的
		return messagev2.NewCodeErr(messagev2.UnsupportedQoSLevel, fmt.Sprintf("a maximum of qos %d is supported", svc.conFig.Broker.MaxQos))
	}
	if msg.Retain() && !svc.conFig.Broker.RetainAvailable {
		return messagev2.NewCodeErr(messagev2.UnsupportedRetention, "unSupport retain message")
	}

	if err := svc.topicAliceIn(msg); err != nil {
		return err
	}
	switch msg.QoS() {
	case messagev2.QosExactlyOnce:
		svc.sess.Pub2in().Wait(msg, nil)

		resp := messagev2.NewPubrecMessage()
		resp.SetPacketId(msg.PacketId())

		_, err := svc.writeMessage(resp)
		if err != nil {
			return messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
		}
		return nil

	case messagev2.QosAtLeastOnce:
		resp := messagev2.NewPubackMessage()
		resp.SetPacketId(msg.PacketId())

		if _, err := svc.writeMessage(resp); err != nil {
			return messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
		}

		return svc.onPublish(msg)

	case messagev2.QosAtMostOnce:
		return svc.onPublish(msg)
	}

	return messagev2.NewCodeErr(messagev2.UnsupportedQoSLevel, fmt.Sprintf("(%s) invalid message QoS %d.", svc.cid(), msg.QoS()))
}

// For SUBSCRIBE messagev5, we should add subscriber, then send back SUBACK
func (svc *service) processSubscribe(msg *messagev2.SubscribeMessage) error {
	resp := messagev2.NewSubackMessage()
	resp.SetPacketId(msg.PacketId())

	// Subscribe to the different topics
	//订阅不同的主题
	var retcodes []byte

	tps := msg.Topics()
	qos := msg.Qos()

	svc.rmsgs = svc.rmsgs[0:0]
	tmpResCode := make([]byte, 0)
	unSupport := false
	for _, t := range tps {
		// 简单处理，直接断开连接，返回原因码
		if svc.conFig.Broker.CloseShareSub && len(t) > 6 && reflect.DeepEqual(t[:6], []byte{'$', 's', 'h', 'a', 'r', 'e'}) {
			//dis := messagev5.NewDisconnectMessage()
			//dis.SetReasonCode(messagev5.UnsupportedSharedSubscriptions)
			//if _, err := svc.writeMessage(resp); err != nil {
			//	return err
			//}
			unSupport = true
			tmpResCode = append(tmpResCode, messagev2.UnsupportedSharedSubscriptions.Value())
		} else {
			tmpResCode = append(tmpResCode, messagev2.UnspecifiedError.Value())
		}
	}
	switch {
	case unSupport && len(tmpResCode) > 0:
		_ = resp.AddReasonCodes(tmpResCode)
		if _, err := svc.writeMessage(resp); err != nil {
			err = messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
			return err
		}
		return nil
	case len(tmpResCode) == 0:
		// 正常情况不会走到此处，在编解码处已经限制了至少有一个主题过滤器/订阅选项对
		return messagev2.NewCodeErr(messagev2.InvalidMessage, ErrInvalidSubscriber.Error())
	}

	for i, t := range tps {
		noLocal := msg.TopicNoLocal(t)
		retainAsPublished := msg.TopicRetainAsPublished(t)
		retainHandling := msg.TopicRetainHandling(t)
		tp := make([]byte, len(t))
		copy(tp, t) // 必须copy
		sub := topics.Sub{
			Topic:             tp,
			Qos:               qos[i],
			NoLocal:           noLocal,
			RetainAsPublished: retainAsPublished,
			RetainHandling:    retainHandling,
			SubIdentifier:     msg.SubscriptionIdentifier(),
		}
		rqos, err := svc.topicsMgr.Subscribe(sub, &svc.onpub)
		if err != nil {
			return messagev2.NewCodeErr(messagev2.ServiceBusy, err.Error())
		}
		err = svc.sess.AddTopic(sub)
		retcodes = append(retcodes, rqos)

		// yeah I am not checking errors here. If there's an error we don't want the
		// subscription to stop, just let it go.
		//是的，我没有检查错误。如果有错误，我们不想
		//订阅要停止，就放手吧。
		if retainHandling == messagev2.NoSendRetain {
			continue
		} else if retainHandling == messagev2.CanSendRetain {
			err = svc.topicsMgr.Retained(t, &svc.rmsgs)
			if err != nil {
				return messagev2.NewCodeErr(messagev2.ServiceBusy, err.Error())
			}
			logger.Logger.Debugf("(%s) topic = %s, retained count = %d", svc.cid(), t, len(svc.rmsgs))
		} else if retainHandling == messagev2.NoExistSubSendRetain {
			// 已存在订阅的情况下不发送保留消息是很有用的，比如重连完成时客户端不确定订阅是否在之前的会话连接中被创建。
			oldTp, _ := svc.sess.Topics()

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
				err = svc.topicsMgr.Retained(t, &svc.rmsgs)
				if err != nil {
					return messagev2.NewCodeErr(messagev2.ServiceBusy, err.Error())
				}
				logger.Logger.Debugf("(%s) topic = %s, retained count = %d", svc.cid(), t, len(svc.rmsgs))
			}
		}

		/**
		* 可以在这里向其它集群节点发送添加主题消息
		**/

	}
	logger.Logger.Infof("客户端：%s，订阅主题：%s，qos：%d，retained count = %d", svc.cid(), tps, qos, len(svc.rmsgs))
	if err := resp.AddReasonCodes(retcodes); err != nil {
		return messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
	}

	if _, err := svc.writeMessage(resp); err != nil {
		return messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
	}

	svc.processToCluster(tps, msg)
	for _, rm := range svc.rmsgs {
		// 下面不用担心因为又重新设置为old qos而有问题，因为在内部ackqueue都已经encode了
		old := rm.QoS()
		// 这里也做了调整
		if rm.QoS() > qos[0] {
			rm.SetQoS(qos[0])
		}
		if err := svc.publish(rm, nil); err != nil {
			logger.Logger.Errorf("service/processSubscribe: Error publishing retained messagev5: %v", err)
			//rm.SetQoS(old)
			//return err
		}
		rm.SetQoS(old)
	}

	return nil
}

// For UNSUBSCRIBE messagev5, we should remove the subscriber, and send back UNSUBACK
func (svc *service) processUnsubscribe(msg *messagev2.UnsubscribeMessage) error {
	tps := msg.Topics()

	for _, t := range tps {
		svc.topicsMgr.Unsubscribe(t, &svc.onpub)
		svc.sess.RemoveTopic(string(t))
	}

	resp := messagev2.NewUnsubackMessage()
	resp.SetPacketId(msg.PacketId())
	resp.AddReasonCode(messagev2.Success.Value())

	_, err := svc.writeMessage(resp)
	if err != nil {
		return messagev2.NewCodeErr(messagev2.UnspecifiedError, err.Error())
	}

	svc.processToCluster(tps, msg)
	logger.Logger.Infof("客户端：%s 取消订阅主题：%s", svc.cid(), tps)
	return nil
}

var (
	shareByte = []byte("$share/")
	// reflect.DeepEqual(topic[:len(shareByte)],shareByte)
	// 这样可以减少反射操作
	deepEqual = func(topic []byte) bool {
		for i, i2 := range shareByte {
			if topic[i] != i2 {
				return false
			}
		}
		return true
	}
)

// processToCluster 分主题发送到其它节点发送
func (svc *service) processToCluster(topics [][]byte, msg messagev2.Message) {
	if !svc.clusterOpen {
		return
	}
	tag := 0
	shareTp := make([][]byte, 0)
	for i := 0; i < len(topics); i++ {
		if len(topics[i]) > len(shareByte) && deepEqual(topics[i]) {
			var index = len(shareByte)
			// 找到共享主题名称
			for j, b := range topics[i][len(shareByte):] {
				if b == '/' {
					index += j
					break
				} else if b == '+' || b == '#' {
					// {ShareName} 是一个不包含 "/", "+" 以及 "#" 的字符串。
					continue
				}
			}
			if index == len(shareByte) {
				continue
			}
			if len(topics[i]) >= 2+index && topics[i][index] == '/' {
				shareName := topics[i][len(shareByte):index]
				tp := topics[i][index+1:]
				shareTp = append(shareTp, topics[i])
				tag++
				switch msg.Type() {
				case messagev2.SUBSCRIBE:
					// 添加本地集群共享订阅
					svc.shareTopicMapNode.AddTopicMapNode(tp, string(shareName), GetServerName(), 1)
					logger.Logger.Debugf("添加本地集群共享订阅：%s,%s,%s", tp, string(shareName), GetServerName())
				case messagev2.UNSUBSCRIBE:
					// 删除本地集群共享订阅
					svc.shareTopicMapNode.RemoveTopicMapNode(tp, string(shareName), GetServerName())
					logger.Logger.Debugf("删除本地集群共享订阅：%s,%s,%s", tp, string(shareName), GetServerName())
				}
			}
		}

	}
	if tag > 0 { // 确实是有共享主题，才发送到集群其它节点
		// 只需要发送共享订阅主题
		var msgTo messagev2.Message
		switch msg.Type() {
		case messagev2.SUBSCRIBE:
			sub := messagev2.NewSubscribeMessage()
			for i := 0; i < len(shareTp); i++ {
				sub.AddTopic(shareTp[i], 0)
			}
			msgTo = sub
		case messagev2.UNSUBSCRIBE:
			sub := messagev2.NewUnsubscribeMessage()
			for i := 0; i < len(shareTp); i++ {
				sub.AddTopic(shareTp[i])
			}
			msgTo = sub
		}
		svc.sendCluster(msgTo) // 发送到其它节点，不需要qos处理的，TODO 但是需要考虑session相关问题
	}
}

// onPublish()在服务器接收到发布消息并完成时调用 ack循环。
//此方法将根据发布获取订阅服务器列表主题，并将消息发布到订阅方列表。
func (svc *service) onPublish(msg *messagev2.PublishMessage) error {
	if msg.Retain() {
		if err := svc.topicsMgr.Retain(msg); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", svc.cid(), err)
		}
	}

	var buf *bytes.Buffer

	if svc.openCluster() || svc.openShare() {
		buf = bufpool.BufferPoolGet()
		defer bufpool.BufferPoolPut(buf)
		msg.EncodeToBuf(buf) // 这里没有选择直接拿msg内的buf
	}

	if svc.openCluster() && buf != nil {
		tmpMsg2 := messagev2.NewPublishMessage() // 必须重新弄一个，防止被下面改动qos引起bug
		tmpMsg2.Decode(buf.Bytes())
		svc.sendCluster(tmpMsg2)
	}
	if svc.openShare() && buf != nil {
		tmpMsg := messagev2.NewPublishMessage() // 必须重新弄一个，防止被下面改动qos引起bug
		tmpMsg.Decode(buf.Bytes())
		svc.sendShareToCluster(tmpMsg) // todo 共享订阅时把非本地选项设为1将造成协议错误
	}

	// 发送非共享订阅主题, 这里面会改动qos
	return svc.pubFn(msg, "", false)
}

// 发送共享主题到集群其它节点去，以共享组的方式发送
func (svc *service) sendShareToCluster(msg *messagev2.PublishMessage) {
	if !svc.openCluster() {
		// 没开集群，就只要发送到当前节点下就行
		// 发送当前主题的所有共享组
		err := svc.pubFnPlus(msg)
		if err != nil {
			logger.Logger.Errorf("%v 发送共享：%v 主题错误：%+v", svc.id, msg.Topic(), *msg)
		}
		return
	}
	submit(func() {
		// 发送共享主题消息
		sn, err := svc.shareTopicMapNode.GetShareNames(msg.Topic())
		if err != nil {
			logger.Logger.Errorf("%v 获取主题%s共享组错误:%v", svc.id, msg.Topic(), err)
			return
		}
		for shareName, node := range sn {
			if node == GetServerName() {
				err = svc.pubFn(msg, shareName, true)
				if err != nil {
					// FIXME 错误处理
				}
			} else {
				colong.SendMsgToCluster(msg, shareName, node, nil, nil, nil)
			}
		}
	})
}

// 发送到集群其它节点去
func (svc *service) sendCluster(message messagev2.Message) {
	if !svc.openCluster() {
		return
	}
	submit(func() {
		colong.SendMsgToCluster(message, "", "", func(message messagev2.Message) {

		}, func(name string, message messagev2.Message) {

		}, func(name string, message messagev2.Message) {

		})
	})
}

// ClusterInToPub 集群节点发来的普通消息
func (svc *service) ClusterInToPub(msg *messagev2.PublishMessage) error {
	if !svc.isCluster() {
		return nil
	}
	if msg.Retain() {
		if err := svc.topicsMgr.Retain(msg); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", svc.cid(), err)
		}
	}
	// 发送非共享订阅主题
	return svc.pubFn(msg, "", false)
}

// ClusterInToPubShare 集群节点发来的共享主题消息，需要发送到特定的共享组 onlyShare = true
// 和 普通节点 onlyShare = false
func (svc *service) ClusterInToPubShare(msg *messagev2.PublishMessage, shareName string, onlyShare bool) error {
	if !svc.isCluster() {
		return nil
	}
	if msg.Retain() {
		if err := svc.topicsMgr.Retain(msg); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", svc.cid(), err)
		}
	}
	// 根据onlyShare 确定是否只发送共享订阅主题
	return svc.pubFn(msg, shareName, onlyShare)
}

// ClusterInToPubSys 集群节点发来的系统主题消息
func (svc *service) ClusterInToPubSys(msg *messagev2.PublishMessage) error {
	if !svc.isCluster() {
		return nil
	}
	if msg.Retain() {
		if err := svc.topicsMgr.Retain(msg); err != nil { // 为这个主题保存最后一条保留消息
			logger.Logger.Errorf("(%s) Error retaining messagev5: %v", svc.cid(), err)
		}
	}
	// 发送
	return svc.pubFnSys(msg)
}

func (svc *service) pubFn(msg *messagev2.PublishMessage, shareName string, onlyShare bool) error {
	var (
		subs []interface{}
		qoss []topics.Sub
	)

	err := svc.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &subs, &qoss, false, shareName, onlyShare)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", svc.cid(), err)
		return messagev2.NewCodeErr(messagev2.ServiceBusy, err.Error())
	}

	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) publishing to topic %s and %s subscribers：%v", svc.cid(), msg.Topic(), shareName, len(subs))

	return svc.lookSend(msg, subs, qoss, onlyShare)
}

// 发送当前主题所有共享组
func (svc *service) pubFnPlus(msg *messagev2.PublishMessage) error {
	var (
		subs []interface{}
		qoss []topics.Sub
	)
	err := svc.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &subs, &qoss, false, "", true)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", svc.cid(), err)
		return messagev2.NewCodeErr(messagev2.ServiceBusy, err.Error())
	}

	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) Publishing to all shareName topic in %s to subscribers：%v", svc.cid(), msg.Topic(), subs)

	return svc.lookSend(msg, subs, qoss, true)
}

// 发送系统消息
func (svc *service) pubFnSys(msg *messagev2.PublishMessage) error {
	var (
		subs []interface{}
		qoss []topics.Sub
	)
	err := svc.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &subs, &qoss, true, "", false)
	if err != nil {
		//logger.Logger.Error(err, "(%s) Error retrieving subscribers list: %v", svc.cid(), err)
		return messagev2.NewCodeErr(messagev2.ServiceBusy, err.Error())
	}

	msg.SetRetain(false)
	logger.Logger.Debugf("(%s) publishing sys topic %s to subscribers：%v", svc.cid(), msg.Topic(), subs)

	return svc.lookSend(msg, subs, qoss, false)
}

// 循环发送， todo 可丢到协程池处理
func (svc *service) lookSend(msg *messagev2.PublishMessage, subs []interface{}, qoss []topics.Sub, onlyShare bool) error {
	for i, s := range subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				return fmt.Errorf("invalid onPublish Function")
			} else {
				err := (*fn)(copyMsg(msg, qoss[i].Qos), qoss[i], svc.cid(), onlyShare)
				if err == io.EOF {
					// TODO 断线了，是否对于qos=1和2的保存至离线消息
				}
			}
		}
	}
	return nil
}

// 是否是集群server
func (svc *service) isCluster() bool {
	return svc.clusterBelong
}

// 是否开启集群
func (svc *service) openCluster() bool {
	return svc.clusterOpen
}

// 是否开启共享
func (svc *service) openShare() bool {
	return svc.conFig.OpenShare()
}

func copyMsg(msg *messagev2.PublishMessage, newQos byte) *messagev2.PublishMessage {
	buf := bufpool.BufferPoolGet()
	msg.EncodeToBuf(buf)

	tmpMsg := messagev2.NewPublishMessage() // 必须重新弄一个，防止被下面改动qos引起bug
	tmpMsg.Decode(buf.Bytes())
	tmpMsg.SetQoS(newQos) // 设置为该发的qos级别

	bufpool.BufferPoolPut(buf) // 归还
	return tmpMsg
}
