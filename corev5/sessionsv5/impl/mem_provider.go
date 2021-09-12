package impl

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
	"gitee.com/Ljolan/si-mqtt/logger"
	"strconv"
	"sync"
)

var _ sessionsv5.SessionsProvider = (*memProvider)(nil)

func init() {
	memProviderInit()
}
func memProviderInit() {
	sessionsv5.Register("", NewMemProvider())
	logger.Logger.Info("开启mem进行session管理")
}

type memProvider struct {
	st map[string]sessionsv5.Session
	mu sync.RWMutex
}

func NewMemProvider() *memProvider {
	return &memProvider{
		st: make(map[string]sessionsv5.Session),
	}
}

func (this *memProvider) New(id string, cleanStart bool) (sessionsv5.Session, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if cleanStart {
		this.st[id] = NewMemSession(id)
	} else {
		this.st[id] = NewDBSession(id) // 新开始，需要删除旧的
	}
	return this.st[id], nil
}

func (this *memProvider) Get(id string, cleanStart bool) (sessionsv5.Session, error) {
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
	this.st = make(map[string]sessionsv5.Session)
	return nil
}
