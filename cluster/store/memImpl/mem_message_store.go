package memImpl

import (
	"context"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
)

type memMessageStore struct {
}

func NewMemMessageStore() store2.MessageStore {
	return &memMessageStore{}
}
func (m *memMessageStore) Start(ctx context.Context, config config.SIConfig) error {
	return nil
}

func (m *memMessageStore) Stop(ctx context.Context) error {
	return nil
}

func (m *memMessageStore) StoreWillMessage(ctx context.Context, clientId string, message *messagev52.PublishMessage) error {
	panic("implement me")
}

func (m *memMessageStore) ClearWillMessage(ctx context.Context, clientId string) error {
	panic("implement me")
}

func (m *memMessageStore) GetWillMessage(ctx context.Context, clientId string) (*messagev52.PublishMessage, error) {
	panic("implement me")
}

func (m *memMessageStore) StoreRetainMessage(ctx context.Context, topic string, message *messagev52.PublishMessage) error {
	panic("implement me")
}

func (m *memMessageStore) ClearRetainMessage(ctx context.Context, topic string) error {
	panic("implement me")
}

func (m *memMessageStore) GetRetainMessage(ctx context.Context, topic string) (*messagev52.PublishMessage, error) {
	panic("implement me")
}

func (m *memMessageStore) GetAllRetainMsg(ctx context.Context) ([]*messagev52.PublishMessage, error) {
	panic("implement me")
}
