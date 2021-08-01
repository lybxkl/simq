package sessions

import (
	"SI-MQTT/config"
	"SI-MQTT/core/logger"
	"fmt"
	"strconv"
	"sync"
)

var (
	sessionsProvider = "mem"
)
var _ SessionsProvider = (*memProvider)(nil)

func memProviderInit() {
	consts := config.ConstConf
	provider := consts.DefaultConst.SessionsProvider
	if provider == sessionsProvider {
		Register(sessionsProvider, NewMemProvider())
	}
}

type memProvider struct {
	st map[string]*Session
	mu sync.RWMutex
}

func NewMemProvider() *memProvider {
	return &memProvider{
		st: make(map[string]*Session),
	}
}

func (this *memProvider) New(id string) (*Session, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.st[id] = &Session{id: id}
	//logger.Info(strconv.Itoa(len(this.st)))
	return this.st[id], nil
}

func (this *memProvider) Get(id string) (*Session, error) {
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
	this.st = make(map[string]*Session)
	return nil
}
