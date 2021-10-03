package mongo

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
)

type dbSender struct {
	curNodeName string
	c           *mongoOrm
}

func NewDBClusterClient(curNodeName, url string, minPool, maxPool, maxConnIdle uint64) colong.NodeClientFace {
	db, e := newMongoOrm(curNodeName, url, minPool, maxPool, maxConnIdle)
	if e != nil {
		panic(e)
	}
	dbSend := &dbSender{
		curNodeName: curNodeName,
		c:           db,
	}
	colong.SetSender(dbSend)
	return dbSend
}

func (this *dbSender) Close() error {
	return nil
}

func (this *dbSender) SendOneNode(msg messagev5.Message,
	shareName, targetNode string,
	oneNodeSendSucFunc func(name string, message messagev5.Message),
	oneNodeSendFailFunc func(name string, message messagev5.Message)) {
	var e error
	switch msg := msg.(type) {
	case *messagev5.PublishMessage:
		if targetNode != "" && shareName != "" {
			e = this.c.SaveSharePub(context.TODO(), "cluster_msg", targetNode, shareName, msg)
		} else {
			e = this.c.SavePub(context.TODO(), "cluster_msg", msg)
		}
	case *messagev5.SubscribeMessage:
		e = this.c.SaveSub(context.TODO(), "cluster_msg", msg)
	case *messagev5.UnsubscribeMessage:
		e = this.c.SaveUnSub(context.TODO(), "cluster_msg", msg)
	}
	if e != nil {
		if oneNodeSendSucFunc != nil {
			go oneNodeSendSucFunc(targetNode, msg)
		}
	} else {
		if oneNodeSendFailFunc != nil {
			go oneNodeSendFailFunc(targetNode, msg)
		}
	}
}

func (this *dbSender) SendAllNode(msg messagev5.Message,
	allSuccess func(message messagev5.Message),
	oneNodeSendSucFunc func(name string, message messagev5.Message),
	oneNodeSendFailFunc func(name string, message messagev5.Message)) {
	var e error
	switch msg := msg.(type) {
	case *messagev5.PublishMessage:
		e = this.c.SavePub(context.TODO(), "cluster_msg", msg)
	case *messagev5.SubscribeMessage:
		e = this.c.SaveSub(context.TODO(), "cluster_msg", msg)
	case *messagev5.UnsubscribeMessage:
		e = this.c.SaveUnSub(context.TODO(), "cluster_msg", msg)
	}
	if e != nil {
		if allSuccess != nil {
			go allSuccess(msg)
		}
		if oneNodeSendSucFunc != nil {
			go oneNodeSendSucFunc(colong.AllNodeName, msg)
		}
	} else {
		if oneNodeSendFailFunc != nil {
			go oneNodeSendFailFunc(colong.AllNodeName, msg)
		}
	}
}
