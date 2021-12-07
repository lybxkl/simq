package store

import (
	"context"
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
)

type MessageStore interface {
	BaseStore

	StoreWillMessage(ctx context.Context, clientId string, message *messagev52.PublishMessage) error
	ClearWillMessage(ctx context.Context, clientId string) error
	GetWillMessage(ctx context.Context, clientId string) (*messagev52.PublishMessage, error)

	StoreRetainMessage(ctx context.Context, topic string, message *messagev52.PublishMessage) error
	ClearRetainMessage(ctx context.Context, topic string) error
	GetRetainMessage(ctx context.Context, topic string) (*messagev52.PublishMessage, error)

	GetAllRetainMsg(ctx context.Context) ([]*messagev52.PublishMessage, error)
}
