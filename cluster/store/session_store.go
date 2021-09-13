package store

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
)

type SessionStore interface {
	BaseStore

	GetSession(ctx context.Context, clientId string) (sessionsv5.Session, error)
	StoreSession(ctx context.Context, clientId string, session sessionsv5.Session) error
	ClearSession(ctx context.Context, clientId string, clearOfflineMsg bool) error
	StoreSubscription(ctx context.Context, clientId string, subscription *messagev5.SubscribeMessage) error
	DelSubscription(ctx context.Context, client, topic string) error
	ClearSubscriptions(ctx context.Context, clientId string) error
	GetSubscriptions(ctx context.Context, clientId string) ([]*messagev5.SubscribeMessage, error)
	/**
	 * 缓存qos2 publish报文消息-入栈消息
	 * @return true:缓存成功   false:缓存失败
	 */
	CacheInflowMsg(ctx context.Context, clientId string, message messagev5.Message) error
	ReleaseInflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev5.Message, error)
	GetAllInflowMsg(ctx context.Context, clientId string) ([]messagev5.Message, error)

	/**
	 * 缓存出栈消息-分发给客户端的qos1,qos2消息
	 */
	CacheOutflowMsg(ctx context.Context, client string, message messagev5.Message) error
	GetAllOutflowMsg(ctx context.Context, clientId string) ([]messagev5.Message, error)
	ReleaseOutflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev5.Message, error)

	/**
	 * 出栈qos2第二阶段，缓存msgId
	 */
	CacheOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error
	GetAllOutflowSecMsg(ctx context.Context, clientId string) ([]uint16, error)
	ReleaseOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error

	StoreOfflineMsg(ctx context.Context, clientId string, message messagev5.Message) error
	GetAllOfflineMsg(ctx context.Context, clientId string) ([]messagev5.Message, []string, error)
	ClearOfflineMsgs(ctx context.Context, clientId string) error
	ClearOfflineMsgById(ctx context.Context, clientId string, msgIds []string) error
}
