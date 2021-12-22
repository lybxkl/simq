package store

import (
	"context"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
)

type MessageStore interface {
	BaseStore

	StoreWillMessage(ctx context.Context, clientId string, message *messagev2.PublishMessage) error
	ClearWillMessage(ctx context.Context, clientId string) error
	GetWillMessage(ctx context.Context, clientId string) (*messagev2.PublishMessage, error)

	StoreRetainMessage(ctx context.Context, topic string, message *messagev2.PublishMessage) error
	ClearRetainMessage(ctx context.Context, topic string) error
	GetRetainMessage(ctx context.Context, topic string) (*messagev2.PublishMessage, error)

	GetAllRetainMsg(ctx context.Context) ([]*messagev2.PublishMessage, error)
}
