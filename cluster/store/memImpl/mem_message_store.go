package memImpl

import (
	"context"
	"errors"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"sync"
)

type memMessageStore struct {
	wrw       sync.RWMutex
	willTable map[string]*messagev2.PublishMessage

	rrw         sync.RWMutex
	retainTable map[string]*messagev2.PublishMessage
}

func NewMemMessageStore() store2.MessageStore {
	return &memMessageStore{
		willTable:   map[string]*messagev2.PublishMessage{},
		retainTable: map[string]*messagev2.PublishMessage{},
	}
}
func (m *memMessageStore) Start(ctx context.Context, config config.SIConfig) error {
	return nil
}

func (m *memMessageStore) Stop(ctx context.Context) error {
	return nil
}

func (m *memMessageStore) StoreWillMessage(ctx context.Context, clientId string, message *messagev2.PublishMessage) error {
	m.wrw.Lock()
	defer m.wrw.Unlock()
	m.willTable[clientId] = message
	return nil
}

func (m *memMessageStore) ClearWillMessage(ctx context.Context, clientId string) error {
	m.wrw.Lock()
	defer m.wrw.Unlock()
	delete(m.willTable, clientId)
	return nil
}

func (m *memMessageStore) GetWillMessage(ctx context.Context, clientId string) (*messagev2.PublishMessage, error) {
	m.wrw.RLock()
	defer m.wrw.RUnlock()
	msg, ok := m.willTable[clientId]
	if ok {
		return msg, nil
	}
	return nil, errors.New("no will msg")
}

func (m *memMessageStore) StoreRetainMessage(ctx context.Context, topic string, message *messagev2.PublishMessage) error {
	m.rrw.Lock()
	defer m.rrw.Unlock()
	m.retainTable[topic] = message
	return nil
}

func (m *memMessageStore) ClearRetainMessage(ctx context.Context, topic string) error {
	m.rrw.Lock()
	defer m.rrw.Unlock()
	delete(m.retainTable, topic)
	return nil
}

func (m *memMessageStore) GetRetainMessage(ctx context.Context, topic string) (*messagev2.PublishMessage, error) {
	m.rrw.RLock()
	defer m.rrw.RUnlock()
	msg, ok := m.retainTable[topic]
	if ok {
		return msg, nil
	}
	return nil, errors.New("no retain msg")
}

func (m *memMessageStore) GetAllRetainMsg(ctx context.Context) ([]*messagev2.PublishMessage, error) {
	m.rrw.RLock()
	defer m.rrw.RUnlock()
	ret := make([]*messagev2.PublishMessage, 0, len(m.retainTable))
	for _, message := range m.retainTable {
		ret = append(ret, message)
	}
	return ret, nil
}
