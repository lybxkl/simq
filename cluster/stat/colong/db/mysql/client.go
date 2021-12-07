package mysql

import (
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	autocompress "gitee.com/Ljolan/si-mqtt/cluster/stat/colong/auto_compress_sub"
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
)

type dbSender struct {
	curNodeName string
	c           *mysqlOrm
}

func NewMysqlClusterClient(curNodeName, mysqlUrl string, maxConn int, compressCfg autocompress.CompressCfg) colong.NodeClientFace {
	db := newMysqlOrm(curNodeName, mysqlUrl, maxConn, compressCfg)
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

func (this *dbSender) SendOneNode(msg messagev52.Message,
	shareName, targetNode string,
	oneNodeSendSucFunc func(name string, message messagev52.Message),
	oneNodeSendFailFunc func(name string, message messagev52.Message)) {
	var e error
	switch msg := msg.(type) {
	case *messagev52.PublishMessage:
		if targetNode != "" && shareName != "" {
			e = this.c.SaveSharePub(targetNode, shareName, msg)
		} else {
			e = this.c.SavePub(msg)
		}
	case *messagev52.SubscribeMessage:
		e = this.c.SaveSub(msg)
	case *messagev52.UnsubscribeMessage:
		e = this.c.SaveUnSub(msg)
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

func (this *dbSender) SendAllNode(msg messagev52.Message,
	allSuccess func(message messagev52.Message),
	oneNodeSendSucFunc func(name string, message messagev52.Message),
	oneNodeSendFailFunc func(name string, message messagev52.Message)) {
	var e error
	switch msg := msg.(type) {
	case *messagev52.PublishMessage:
		e = this.c.SavePub(msg)
	case *messagev52.SubscribeMessage:
		e = this.c.SaveSub(msg)
	case *messagev52.UnsubscribeMessage:
		e = this.c.SaveUnSub(msg)
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
