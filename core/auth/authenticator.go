package auth

import (
	"gitee.com/Ljolan/si-mqtt/core/logger"
	"errors"
	"fmt"
)

var (
	ErrAuthFailure          = errors.New("auth: Authentication failure")
	ErrAuthProviderNotFound = errors.New("auth: Authentication provider not found")

	providers = make(map[string]Authenticator)
)

type Authenticator interface {
	Authenticate(id string, cred interface{}) error
}

var Default = "default"

// 服务启动前注册有的鉴权插件
func Register(name string, provider Authenticator) {
	if provider == nil {
		panic("auth: Register provide is nil")
	}
	if name == "" {
		name = Default
	}
	if _, dup := providers[name]; dup {
		panic("auth: Register called twice for provider " + name)
	}

	providers[name] = provider
	logger.Logger.Infof("Register AuthProvide：'%s' success，%T", name, provider)
}

func Unregister(name string) {
	if name == "" {
		name = Default
	}
	delete(providers, name)
}

// 鉴权管理器，无需枷锁，保持无状态就行
type Manager struct {
	p Authenticator
}

func NewManager(providerName string) (*Manager, error) {
	if providerName == "" {
		providerName = Default
	}
	p, ok := providers[providerName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provider %q", providerName)
	}

	return &Manager{p: p}, nil
}

func (this *Manager) Authenticate(id string, cred interface{}) error {
	return this.p.Authenticate(id, cred)
}
