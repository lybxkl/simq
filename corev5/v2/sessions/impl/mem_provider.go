package impl

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/logger"
	"io"
	"strconv"
	"sync"
)

var _ sessions.Provider = (*memProvider)(nil)

type memProvider struct {
	sessStore store.SessionStore

	mu sync.RWMutex
	st map[string]sessions.Session
}

func (prv *memProvider) SetStore(store store.SessionStore, _ store.MessageStore) {
	prv.sessStore = store
}

func NewMemProvider() sessions.Provider {
	return &memProvider{
		st: make(map[string]sessions.Session),
	}
}

func (prv *memProvider) New(id string, cleanStart bool, expiryTime uint32) (sessions.Session, error) {
	if id == "" {
		id = sessionId()
	}
	prv.mu.Lock()
	defer prv.mu.Unlock()
	if expiryTime == 0 {
		prv.st[id] = NewMemSession(id)
	} else {
		s := NewDBSession(id) // 新开始，需要删除旧的
		prv.st[id] = s
		prv.sessStore.StoreSession(context.Background(), id, s)
	}
	return prv.st[id], nil
}

func (prv *memProvider) Get(id string, cleanStart bool, expiryTime uint32) (sessions.Session, error) {
	prv.mu.RLock()
	defer prv.mu.RUnlock()

	sess, ok := prv.st[id]
	if !ok {
		return nil, fmt.Errorf("store/Get: No session found for key %s", id)
	}
	logger.Logger.Info(strconv.Itoa(len(prv.st)))
	return sess, nil
}

func (prv *memProvider) Del(id string) {
	prv.mu.Lock()
	defer prv.mu.Unlock()
	prv.sessStore.ClearSession(context.Background(), id, true)
	delete(prv.st, id)
}

func (prv *memProvider) Save(id string) error {
	return nil
}

func (prv *memProvider) Count() int {
	return len(prv.st)
}

func (prv *memProvider) Close() error {
	prv.st = make(map[string]sessions.Session)
	return nil
}
func sessionId() string {
	b := make([]byte, 15)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}
