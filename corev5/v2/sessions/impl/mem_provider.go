package impl

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/logger"
	"io"
	"strconv"
	"sync"
)

var _ sessions.Provider = (*memProvider)(nil)

type memProvider struct {
	st map[string]sessions.Session
	mu sync.RWMutex
}

func NewMemProvider() sessions.Provider {
	return &memProvider{
		st: make(map[string]sessions.Session),
	}
}

func (this *memProvider) New(id string, cleanStart bool, expiryTime uint32) (sessions.Session, error) {
	if id == "" {
		id = sessionId()
	}
	this.mu.Lock()
	defer this.mu.Unlock()
	if expiryTime == 0 {
		this.st[id] = NewMemSession(id)
	} else {
		this.st[id] = NewDBSession(id) // 新开始，需要删除旧的
	}
	return this.st[id], nil
}

func (this *memProvider) Get(id string, cleanStart bool, expiryTime uint32) (sessions.Session, error) {
	this.mu.RLock()
	defer this.mu.RUnlock()

	sess, ok := this.st[id]
	if !ok {
		return nil, fmt.Errorf("store/Get: No session found for key %s", id)
	}
	logger.Logger.Info(strconv.Itoa(len(this.st)))
	return sess, nil
}

func (this *memProvider) Del(id string) {
	this.mu.Lock()
	defer this.mu.Unlock()
	delete(this.st, id)
}

func (this *memProvider) Save(id string) error {
	return nil
}

func (this *memProvider) Count() int {
	return len(this.st)
}

func (this *memProvider) Close() error {
	this.st = make(map[string]sessions.Session)
	return nil
}
func sessionId() string {
	b := make([]byte, 15)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}
