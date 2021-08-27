package store

import (
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/model"
)

type SessionStore interface {
	Start(config config.SIConfig) error
	Stop() error

	GetSession(clientId string) (model.Session, error)
	StoreSession(clientId string, session model.Session) error
	ClearSession(clientId string, clearOfflineMsg bool) error
	StoreSubscription(clientId string, subscription model.Subscription) error
	DelSubscription(client, topic string) error
	ClearSubscription(clientId string) error
	GetSubscriptions(clientId string) ([]model.Subscription, error)
	/**
	 * 缓存qos2 publish报文消息-入栈消息
	 * @return true:缓存成功   false:缓存失败
	 */
	CacheInflowMsg(clientId string, message messagev5.Message) error
	ReleaseInflowMsg(clientId string, msgId int64) (messagev5.Message, error)
	GetAllInflowMsg(clientId string) ([]messagev5.Message, error)

	/**
	 * 缓存出栈消息-分发给客户端的qos1,qos2消息
	 */
	CacheOutflowMsg(client string, message messagev5.Message) error
	GetAllOutflowMsg(clientId string) (messagev5.Message, error)
	ReleaseOutflowMsg(clientId string, msgId int64) (messagev5.Message, error)

	/**
	 * 出栈qos2第二阶段，缓存msgId
	 */
	CacheOutflowSecMsgId(clientId string, msgId int64) error
	GetAllOutflowSecMsg(clientId string) ([]int64, error)
	ReleaseOutflowSecMsgId(clientId string, msgId int64) error

	StoreOfflineMsg(clientId string, message messagev5.Message) error
	GetAllOfflineMsg(clientId string) ([]messagev5.Message, error)
	ClearOfflineMsgs(clientId string) error
	ClearOfflineMsgById(clientId string, msgIds []int64) error
}
