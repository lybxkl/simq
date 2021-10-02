package colong

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/logger"
)

var (
	sender      Sender   // 消息发送者，提供全发和单发
	AllNodeName = "@all" // TODO 表示所有节点都发送失败使用的名称，所以节点名称不要使用此
)

type (
	ClusterInToPub      func(msg1 *messagev5.PublishMessage) error
	ClusterInToPubShare func(msg1 *messagev5.PublishMessage, shareName string) error
	ClusterInToPubSys   func(msg1 *messagev5.PublishMessage) error
)

func SetSender(sd Sender) {
	sender = sd
}
func GetSender() Sender {
	return sender
}

// SendMsgToCluster 发送消息到集群
// shareName 共享主题组
// targetNode 目标节点
// 这两个参数用于集群共享主题消息发送到特定的节点，TODO 静态Getty启动 需要有 msg 节点发送确认
func SendMsgToCluster(msg messagev5.Message, shareName, targetNode string, allSuccess func(message messagev5.Message),
	oneNodeSendSucFunc func(name string, message messagev5.Message),
	oneNodeSendFailFunc func(name string, message messagev5.Message)) {
	if sender == nil {
		logger.Logger.Warnf("sender is nil")
		return
	}
	if targetNode != "" { // 单个发送，可能是共享消息
		sender.SendOneNode(msg, shareName, targetNode, oneNodeSendSucFunc, oneNodeSendFailFunc)
		return
	}
	// 发送全部节点
	sender.SendAllNode(msg, allSuccess, oneNodeSendSucFunc, oneNodeSendFailFunc)
}

// Sender 只要实现此接口就可以通过SendMsgToCluster(...)方法发送集群消息
type Sender interface {
	// SendOneNode 主要是用来发送共享主题消息
	SendOneNode(msg messagev5.Message, shareName, targetNode string,
		oneNodeSendSucFunc func(name string, message messagev5.Message),
		oneNodeSendFailFunc func(name string, message messagev5.Message))
	// SendAllNode 发送除共享主题消息外的消息
	SendAllNode(msg messagev5.Message, allSuccess func(message messagev5.Message),
		oneNodeSendSucFunc func(name string, message messagev5.Message),
		oneNodeSendFailFunc func(name string, message messagev5.Message))
}
