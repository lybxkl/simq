package impl

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
	"gitee.com/Ljolan/si-mqtt/logger"
)

type dbAckqueue struct {
	memAQ        *ackqueue // 内存的
	clientId     string
	isIn         bool // 入方向，false表示出方向
	sessionStore store.SessionStore
}

func newDbAckQueue(sessionStore store.SessionStore,
	size int, clientId string, isIn bool) sessionsv5.Ackqueue {
	return &dbAckqueue{
		memAQ:        newMemAckQueue(size),
		sessionStore: sessionStore,
		clientId:     clientId,
		isIn:         isIn,
	}
}
func (d *dbAckqueue) Wait(msg messagev5.Message, onComplete interface{}) (err error) {
	switch msg := msg.(type) {
	case *messagev5.PublishMessage:
		if msg.QoS() == messagev5.QosAtMostOnce {
			//return fmt.Errorf("QoS 0 messages don't require ack")
			return errWaitMessage
		} else if msg.QoS() == messagev5.QosExactlyOnce && d.isIn {
			err = d.sessionStore.CacheInflowMsg(context.Background(), d.clientId, msg)
		} else if msg.QoS() == messagev5.QosExactlyOnce && !d.isIn {
			err = d.sessionStore.CacheOutflowMsg(context.Background(), d.clientId, msg)
		} else if msg.QoS() == messagev5.QosAtLeastOnce && !d.isIn {
			err = d.sessionStore.CacheOutflowMsg(context.Background(), d.clientId, msg)
		}
	case *messagev5.SubscribeMessage:
		err = d.sessionStore.StoreSubscription(context.Background(), d.clientId, msg)
	case *messagev5.UnsubscribeMessage:
	case *messagev5.PingreqMessage:
	default:
		return errWaitMessage
	}
	if err != nil {
		return err
	}
	return d.memAQ.Wait(msg, onComplete)
}

func (d *dbAckqueue) Ack(msg messagev5.Message) (err error) {
	switch msg.Type() {
	case messagev5.PUBACK:
		// 服务端下发qos=1的消息的回应
		// 需要删除db中的数据 // TODO 批量删除
		_, err = d.sessionStore.ReleaseOutflowMsg(context.Background(), d.clientId, msg.PacketId())
	case messagev5.PUBREC:
		// 服务端下发的qos=2的第一次回应
		// 删除消息，插入过程消息 // TODO 批量删除
		_, err = d.sessionStore.ReleaseOutflowMsg(context.Background(), d.clientId, msg.PacketId())
		if err != nil {
			logger.Logger.Error(err)
			return err
		}
		err = d.sessionStore.CacheOutflowSecMsgId(context.Background(), d.clientId, msg.PacketId())
	case messagev5.PUBREL:
		// 服务端收到的qos=2的第二次回应
		// 删除过程消息 // TODO 批量删除
		_, err = d.sessionStore.ReleaseInflowMsg(context.Background(), d.clientId, msg.PacketId())
	case messagev5.PUBCOMP:
		// 服务端下发的qos=2的第二次回应
		// 删除过程消息 // TODO 批量删除
		err = d.sessionStore.ReleaseOutflowSecMsgId(context.Background(), d.clientId, msg.PacketId())
	case messagev5.PINGRESP:
	default:
		return errAckMessage
	}
	if err != nil {
		logger.Logger.Error(err)
		return err
	}
	return d.memAQ.Ack(msg)
}

func (d *dbAckqueue) Acked() []sessionsv5.Ackmsg {
	return d.memAQ.Acked()
}
