package memImpl

import (
	"container/list"
	"context"
	"errors"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"strconv"
	"sync"
)

var obj struct{}

type memSessionStore struct {
	rwm sync.RWMutex
	db  map[string]sessions.Session

	subRwm   sync.RWMutex
	subCache map[string]map[string]*messagev2.SubscribeMessage

	offRwm       sync.RWMutex
	offlineTable map[string]*list.List
	msgMaxNum    int

	recRwm   sync.RWMutex
	recCache map[string]map[uint16]messagev2.Message

	sendRwm   sync.RWMutex
	sendCache map[string]map[uint16]messagev2.Message

	secTwoRwm   sync.RWMutex
	secTwoCache map[string]map[uint16]struct{}
}

func NewMemSessionStore() store2.SessionStore {
	return &memSessionStore{
		db:           make(map[string]sessions.Session),
		subCache:     make(map[string]map[string]*messagev2.SubscribeMessage),
		offlineTable: make(map[string]*list.List),
		msgMaxNum:    1000,
		recCache:     make(map[string]map[uint16]messagev2.Message),
		sendCache:    make(map[string]map[uint16]messagev2.Message),
		secTwoCache:  make(map[string]map[uint16]struct{}),
	}
}
func (m *memSessionStore) Start(ctx context.Context, config *config.SIConfig) error {
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
	if clearOfflineMsg {
		return m.ClearOfflineMsgs(ctx, clientId)
	}
	return nil
}

func (m *memSessionStore) StoreSubscription(ctx context.Context, clientId string, subscription *messagev2.SubscribeMessage) error {
	m.subRwm.Lock()
	defer m.subRwm.Unlock()
	v, ok := m.subCache[clientId]
	if !ok {
		v = make(map[string]*messagev2.SubscribeMessage)
		m.subCache[clientId] = v
	}
	topicS := subscription.Topics()
	subs := subscription.Clone()
	for i := 0; i < len(topicS); i++ {
		v[string(topicS[i])] = subs[i]
	}
	return nil
}

func (m *memSessionStore) DelSubscription(ctx context.Context, client, topic string) error {
	m.subRwm.Lock()
	defer m.subRwm.Unlock()
	v, ok := m.subCache[client]
	if !ok {
		return errors.New("no subscription")
	}
	delete(v, topic)
	return nil
}

func (m *memSessionStore) ClearSubscriptions(ctx context.Context, clientId string) error {
	m.subRwm.Lock()
	defer m.subRwm.Unlock()
	delete(m.subCache, clientId)
	return nil
}

func (m *memSessionStore) GetSubscriptions(ctx context.Context, clientId string) ([]*messagev2.SubscribeMessage, error) {
	m.subRwm.RLock()
	defer m.subRwm.RUnlock()
	v, ok := m.subCache[clientId]
	if !ok {
		return nil, nil
	}
	ret := make([]*messagev2.SubscribeMessage, 0, len(v))
	for _, subscribeMessage := range v {
		sub := subscribeMessage
		ret = append(ret, sub)
	}
	return ret, nil
}

func (m *memSessionStore) CacheInflowMsg(ctx context.Context, clientId string, msg messagev2.Message) error {
	m.recRwm.Lock()
	defer m.recRwm.Unlock()
	v, ok := m.recCache[clientId]
	if !ok {
		v = make(map[uint16]messagev2.Message)
		m.recCache[clientId] = v
	}
	v[msg.PacketId()] = msg
	return nil
}

func (m *memSessionStore) ReleaseInflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev2.Message, error) {
	m.recRwm.Lock()
	defer m.recRwm.Unlock()
	v, ok := m.recCache[clientId]
	if !ok {
		return nil, errors.New("no inflow msg")
	}
	msg, ok := v[pkId]
	if !ok {
		return nil, errors.New("no inflow msg")
	}
	delete(v, pkId)
	return msg, nil
}

func (m *memSessionStore) ReleaseInflowMsgs(ctx context.Context, clientId string, pkIds []uint16) error {
	m.recRwm.Lock()
	defer m.recRwm.Unlock()
	v, ok := m.recCache[clientId]
	if !ok {
		return nil
	}
	for i := 0; i < len(pkIds); i++ {
		delete(v, pkIds[i])
	}
	return nil
}

func (m *memSessionStore) ReleaseAllInflowMsg(ctx context.Context, clientId string) error {
	m.recRwm.Lock()
	defer m.recRwm.Unlock()
	delete(m.recCache, clientId)
	return nil
}

func (m *memSessionStore) GetAllInflowMsg(ctx context.Context, clientId string) ([]messagev2.Message, error) {
	m.recRwm.RLock()
	defer m.recRwm.RUnlock()
	v, ok := m.recCache[clientId]
	if !ok {
		return nil, nil
	}
	ret := make([]messagev2.Message, 0, len(v))
	for _, message := range v {
		ret = append(ret, message)
	}
	return ret, nil
}

func (m *memSessionStore) CacheOutflowMsg(ctx context.Context, clientId string, msg messagev2.Message) error {
	m.sendRwm.Lock()
	defer m.sendRwm.Unlock()
	v, ok := m.sendCache[clientId]
	if !ok {
		v = make(map[uint16]messagev2.Message)
		m.sendCache[clientId] = v
	}
	v[msg.PacketId()] = msg
	return nil
}

func (m *memSessionStore) GetAllOutflowMsg(ctx context.Context, clientId string) ([]messagev2.Message, error) {
	m.sendRwm.RLock()
	defer m.sendRwm.RUnlock()
	v, ok := m.sendCache[clientId]
	if !ok {
		return nil, nil
	}
	ret := make([]messagev2.Message, 0, len(v))
	for _, message := range v {
		ret = append(ret, message)
	}
	return ret, nil
}

func (m *memSessionStore) ReleaseOutflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev2.Message, error) {
	m.sendRwm.Lock()
	defer m.sendRwm.Unlock()
	v, ok := m.sendCache[clientId]
	if !ok {
		return nil, errors.New("no out inflow msg")
	}
	msg, ok := v[pkId]
	if !ok {
		return nil, errors.New("no out inflow msg")
	}
	delete(v, pkId)
	return msg, nil
}

func (m *memSessionStore) ReleaseOutflowMsgs(ctx context.Context, clientId string, pkIds []uint16) error {
	m.sendRwm.Lock()
	defer m.sendRwm.Unlock()
	v, ok := m.sendCache[clientId]
	if !ok {
		return nil
	}
	for i := 0; i < len(pkIds); i++ {
		delete(v, pkIds[i])
	}
	return nil
}

func (m *memSessionStore) ReleaseAllOutflowMsg(ctx context.Context, clientId string) error {
	m.sendRwm.Lock()
	defer m.sendRwm.Unlock()
	delete(m.sendCache, clientId)
	return nil
}

func (m *memSessionStore) CacheOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	m.secTwoRwm.Lock()
	defer m.secTwoRwm.Unlock()
	v, ok := m.secTwoCache[clientId]
	if !ok {
		v = make(map[uint16]struct{})
		m.secTwoCache[clientId] = v
	}
	v[pkId] = obj
	return nil
}

func (m *memSessionStore) GetAllOutflowSecMsg(ctx context.Context, clientId string) ([]uint16, error) {
	m.secTwoRwm.RLock()
	defer m.secTwoRwm.RUnlock()
	v, ok := m.secTwoCache[clientId]
	if !ok {
		return nil, nil
	}
	ret := make([]uint16, 0, len(v))
	for id, _ := range v {
		ret = append(ret, id)
	}
	return ret, nil
}

func (m *memSessionStore) ReleaseOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	m.secTwoRwm.Lock()
	defer m.secTwoRwm.Unlock()
	v, ok := m.secTwoCache[clientId]
	if !ok {
		return errors.New("no out inflow msg")
	}
	delete(v, pkId)
	return nil
}

func (m *memSessionStore) ReleaseOutflowSecMsgIds(ctx context.Context, clientId string, pkIds []uint16) error {
	m.secTwoRwm.Lock()
	defer m.secTwoRwm.Unlock()
	v, ok := m.secTwoCache[clientId]
	if !ok {
		return nil
	}
	for i := 0; i < len(pkIds); i++ {
		delete(v, pkIds[i])
	}
	return nil
}

func (m *memSessionStore) ReleaseAllOutflowSecMsg(ctx context.Context, clientId string) error {
	m.secTwoRwm.Lock()
	defer m.secTwoRwm.Unlock()
	delete(m.secTwoCache, clientId)
	return nil
}

func (m *memSessionStore) StoreOfflineMsg(ctx context.Context, clientId string, msg messagev2.Message) error {
	m.offRwm.Lock()
	defer m.offRwm.Unlock()
	v, ok := m.offlineTable[clientId]
	if !ok {
		v = list.New()
	}
	if v.Len() >= m.msgMaxNum {
		return errors.New("too many msg")
	}
	v.PushBack(msg)
	return nil
}

func (m *memSessionStore) GetAllOfflineMsg(ctx context.Context, clientId string) ([]messagev2.Message, []string, error) {
	m.offRwm.RLock()
	defer m.offRwm.RUnlock()
	v, ok := m.offlineTable[clientId]
	if !ok || v.Len() == 0 {
		return []messagev2.Message{}, []string{}, nil
	}

	ret := make([]messagev2.Message, 0, v.Len())
	ids := make([]string, 0, v.Len())
	head := v.Front()

	for head != nil {
		msg := head.Value.(messagev2.Message)
		head = head.Next()

		ret = append(ret, msg)
		ids = append(ids, strconv.Itoa(int(msg.PacketId())))
	}
	return ret, ids, nil
}

func (m *memSessionStore) ClearOfflineMsgs(ctx context.Context, clientId string) error {
	m.offRwm.Lock()
	defer m.offRwm.Unlock()
	delete(m.offlineTable, clientId)
	return nil
}

func (m *memSessionStore) ClearOfflineMsgById(ctx context.Context, clientId string, msgIds []string) error {
	m.offRwm.Lock()
	defer m.offRwm.Unlock()
	v, ok := m.offlineTable[clientId]
	if !ok || v.Len() == 0 {
		return nil
	}

	head := v.Front()
	for head != nil {
		msg := head.Value.(messagev2.Message)
		del := head
		head = head.Next()
		for i := 0; i < len(msgIds); i++ {
			if msgIds[i] != "" && strconv.Itoa(int(msg.PacketId())) == msgIds[i] {
				msgIds[i] = ""
				v.Remove(del)
				break
			}
		}
	}
	return nil
}
