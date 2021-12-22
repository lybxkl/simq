package impl

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/logger"
	"time"
)

type batchOption interface {
	GetBatchNum() int
	GetNum() int
	GetLastTime() int64
	BatchReleaseOutflowMsg() error
	BatchReleaseOutflowSecMsgId() error
}
type dbAckqueue struct {
	memAQ          *ackqueue // 内存的
	clientId       string
	isIn           bool        // 入方向，false表示出方向
	cachePkId      chan uint16 // 缓存的待删除消息id，供批量删除使用
	lastDeleteTime int64       // 上一次删除的时间
	batchNum       int
	sessionStore   store.SessionStore
}

func newDbAckQueue(sessionStore store.SessionStore,
	size int, clientId string, isIn bool, isBatchOp bool) sessions.Ackqueue {
	batchNum := 100
	dack := &dbAckqueue{
		memAQ:        newMemAckQueue(size),
		sessionStore: sessionStore,
		clientId:     clientId,
		isIn:         isIn,
		batchNum:     batchNum,
	}
	if isBatchOp {
		dack.cachePkId = make(chan uint16, batchNum*2)
	}
	return dack
}
func (d *dbAckqueue) GetBatchNum() int {
	return d.batchNum
}
func (d *dbAckqueue) GetLastTime() int64 {
	return d.lastDeleteTime
}
func (d *dbAckqueue) GetNum() int {
	return len(d.cachePkId)
}
func (d *dbAckqueue) BatchReleaseOutflowMsg() error {
	pks := make([]uint16, 0)
	for i := 0; i < len(d.cachePkId); i++ {
		pks = append(pks, <-d.cachePkId)
	}
	err1 := d.sessionStore.ReleaseOutflowMsgs(context.Background(), d.clientId, pks)
	if err1 != nil {
		return err1
	}
	d.lastDeleteTime = time.Now().UnixNano()
	return nil
}
func (d *dbAckqueue) BatchReleaseOutflowSecMsgId() error {
	pks := make([]uint16, 0)
	for i := 0; i < len(d.cachePkId); i++ {
		pks = append(pks, <-d.cachePkId)
	}
	err1 := d.sessionStore.ReleaseOutflowSecMsgIds(context.Background(), d.clientId, pks)
	if err1 != nil {
		return err1
	}
	d.lastDeleteTime = time.Now().UnixNano()
	return nil
}
func (d *dbAckqueue) Size() int64 {
	return d.memAQ.size
}
func (d *dbAckqueue) Len() int {
	return d.memAQ.Len()
}
func (d *dbAckqueue) Wait(msg messagev2.Message, onComplete interface{}) (err error) {
	switch msg := msg.(type) {
	case *messagev2.PublishMessage:
		if msg.QoS() == messagev2.QosAtMostOnce {
			//return fmt.Errorf("QoS 0 messages don't require ack")
			return errWaitMessage
		} else if msg.QoS() == messagev2.QosExactlyOnce && d.isIn {
			err = d.sessionStore.CacheInflowMsg(context.Background(), d.clientId, msg)
		} else if msg.QoS() == messagev2.QosExactlyOnce && !d.isIn {
			err = d.sessionStore.CacheOutflowMsg(context.Background(), d.clientId, msg)
		} else if msg.QoS() == messagev2.QosAtLeastOnce && !d.isIn {
			err = d.sessionStore.CacheOutflowMsg(context.Background(), d.clientId, msg)
		}
	case *messagev2.SubscribeMessage:
		err = d.sessionStore.StoreSubscription(context.Background(), d.clientId, msg)
	case *messagev2.UnsubscribeMessage:
	case *messagev2.PingreqMessage:
	default:
		return errWaitMessage
	}
	if err != nil {
		return err
	}
	return d.memAQ.Wait(msg, onComplete)
}

func (d *dbAckqueue) Ack(msg messagev2.Message) (err error) {
	switch msg.Type() {
	case messagev2.PUBACK:
		// 服务端下发qos=1的消息的回应
		// 需要删除db中的数据 // TODO 批量删除
		_, err = d.sessionStore.ReleaseOutflowMsg(context.Background(), d.clientId, msg.PacketId())
		//d.cachePkId <- msg.PacketId()
	case messagev2.PUBREC:
		// 服务端下发的qos=2的第一次回应
		// 删除消息，插入过程消息 // TODO 批量删除
		_, err = d.sessionStore.ReleaseOutflowMsg(context.Background(), d.clientId, msg.PacketId())
		if err != nil {
			logger.Logger.Error(err)
			return err
		}
		err = d.sessionStore.CacheOutflowSecMsgId(context.Background(), d.clientId, msg.PacketId())
	case messagev2.PUBREL:
		// 服务端收到的qos=2的第二次回应
		// 删除过程消息 // TODO 批量删除
		_, err = d.sessionStore.ReleaseInflowMsg(context.Background(), d.clientId, msg.PacketId())
	case messagev2.PUBCOMP:
		// 服务端下发的qos=2的第二次回应
		// 删除过程消息 // TODO 批量删除
		err = d.sessionStore.ReleaseOutflowSecMsgId(context.Background(), d.clientId, msg.PacketId())
		//d.cachePkId <- msg.PacketId()
	case messagev2.PINGRESP:
	default:
		return errAckMessage
	}
	if err != nil {
		logger.Logger.Error(err)
		return err
	}
	return d.memAQ.Ack(msg)
}

func (d *dbAckqueue) Acked() []sessions.Ackmsg {
	return d.memAQ.Acked()
}
