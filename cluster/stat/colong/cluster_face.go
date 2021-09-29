package colong

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
)

var (
	sender Sender // 消息发送者，提供全发和单发
)

// wrapperShare 发送共享主题消息
func wrapperShare(msg messagev5.Message, shareName string) ([]byte, error) {
	cmsg := NewWrapCMsgImpl(PubShareCMsg)
	cmsg.SetShare(shareName, msg)
	return EncodeCMsg(cmsg)
}

// wrapperPub 发送普通消息
func wrapperPub(msg messagev5.Message) ([]byte, error) {
	cmsg := NewWrapCMsgImpl(PubCMsg)
	cmsg.SetMsg(msg)
	return EncodeCMsg(cmsg)
}

// SendMsgToCluster 发送消息到集群
// shareName 共享主题组
// targetNode 目标节点
// 这两个参数用于集群共享主题消息发送到特定的节点，TODO 静态Getty启动 需要有 msg 节点发送确认
func SendMsgToCluster(msg messagev5.Message, shareName, targetNode string, allSuccess func(message messagev5.Message),
	oneNodeSendSucFunc func(name string, message messagev5.Message),
	oneNodeSendFailFunc func(name string, message messagev5.Message)) {
	if sender == nil {
		log.Warnf("sender is nil")
		return
	}
	if targetNode != "" { // 单个发送，可能是共享消息
		sender.SendOneNode(msg, shareName, targetNode, allSuccess, oneNodeSendSucFunc, oneNodeSendFailFunc)
		return
	}
	// 发送全部节点
	sender.SendAllNode(msg, shareName, allSuccess, oneNodeSendSucFunc, oneNodeSendFailFunc)
}

type Sender interface {
	SendOneNode(msg messagev5.Message, shareName, targetNode string, allSuccess func(message messagev5.Message),
		oneNodeSendSucFunc func(name string, message messagev5.Message),
		oneNodeSendFailFunc func(name string, message messagev5.Message))
	SendAllNode(msg messagev5.Message, shareName string, allSuccess func(message messagev5.Message),
		oneNodeSendSucFunc func(name string, message messagev5.Message),
		oneNodeSendFailFunc func(name string, message messagev5.Message))
}
