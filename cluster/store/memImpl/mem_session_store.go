package memImpl

import (
	"context"
	"errors"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"sync"
)

type memSessionStore struct {
	rwm *sync.RWMutex
	db  map[string]sessions.Session
}

func NewMemSessionStore() store2.SessionStore {
	return &memSessionStore{
		rwm: &sync.RWMutex{},
		db:  make(map[string]sessions.Session),
	}
}
func (m *memSessionStore) Start(ctx context.Context, config config.SIConfig) error {
	return nil
}

func (m *memSessionStore) Stop(ctx context.Context) error {
	return nil
}

func (m *memSessionStore) GetSession(ctx context.Context, clientId string) (sessions.Session, error) {
	m.rwm.RLock()
	defer m.rwm.RUnlock()
	s, ok := m.db[clientId]
	if !ok {
		return nil, errors.New("session no exist")
	}
	return s, nil
}

func (m *memSessionStore) StoreSession(ctx context.Context, clientId string, session sessions.Session) error {
	m.rwm.Lock()
	defer m.rwm.Unlock()
	m.db[clientId] = session
	return nil
}

func (m *memSessionStore) ClearSession(ctx context.Context, clientId string, clearOfflineMsg bool) error {
	m.rwm.Lock()
	defer m.rwm.Unlock()
	delete(m.db, clientId)
	return nil
}

func (m *memSessionStore) StoreSubscription(ctx context.Context, clientId string, subscription *messagev52.SubscribeMessage) error {
	panic("implement me")
}

func (m *memSessionStore) DelSubscription(ctx context.Context, client, topic string) error {
	panic("implement me")
}

func (m *memSessionStore) ClearSubscriptions(ctx context.Context, clientId string) error {
	panic("implement me")
}

func (m *memSessionStore) GetSubscriptions(ctx context.Context, clientId string) ([]*messagev52.SubscribeMessage, error) {
	panic("implement me")
}

func (m *memSessionStore) CacheInflowMsg(ctx context.Context, clientId string, msg messagev52.Message) error {
	panic("implement me")
}

func (m *memSessionStore) ReleaseInflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev52.Message, error) {
	panic("implement me")
}

func (m *memSessionStore) ReleaseInflowMsgs(ctx context.Context, clientId string, pkId []uint16) error {
	panic("implement me")
}

func (m *memSessionStore) ReleaseAllInflowMsg(ctx context.Context, clientId string) error {
	panic("implement me")
}

func (m *memSessionStore) GetAllInflowMsg(ctx context.Context, clientId string) ([]messagev52.Message, error) {
	panic("implement me")
}

func (m *memSessionStore) CacheOutflowMsg(ctx context.Context, client string, msg messagev52.Message) error {
	panic("implement me")
}

func (m *memSessionStore) GetAllOutflowMsg(ctx context.Context, clientId string) ([]messagev52.Message, error) {
	panic("implement me")
}

func (m *memSessionStore) ReleaseOutflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev52.Message, error) {
	panic("implement me")
}

func (m *memSessionStore) ReleaseOutflowMsgs(ctx context.Context, clientId string, pkId []uint16) error {
	panic("implement me")
}

func (m *memSessionStore) ReleaseAllOutflowMsg(ctx context.Context, clientId string) error {
	panic("implement me")
}

func (m *memSessionStore) CacheOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	panic("implement me")
}

func (m *memSessionStore) GetAllOutflowSecMsg(ctx context.Context, clientId string) ([]uint16, error) {
	panic("implement me")
}

func (m *memSessionStore) ReleaseOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	panic("implement me")
}

func (m *memSessionStore) ReleaseOutflowSecMsgIds(ctx context.Context, clientId string, pkId []uint16) error {
	panic("implement me")
}

func (m *memSessionStore) ReleaseAllOutflowSecMsg(ctx context.Context, clientId string) error {
	panic("implement me")
}

func (m *memSessionStore) StoreOfflineMsg(ctx context.Context, clientId string, msg messagev52.Message) error {
	panic("implement me")
}

func (m *memSessionStore) GetAllOfflineMsg(ctx context.Context, clientId string) ([]messagev52.Message, []string, error) {
	panic("implement me")
}

func (m *memSessionStore) ClearOfflineMsgs(ctx context.Context, clientId string) error {
	panic("implement me")
}

func (m *memSessionStore) ClearOfflineMsgById(ctx context.Context, clientId string, msgIds []string) error {
	panic("implement me")
}
