package authplus

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/logger"
)

var (
	providers = make(map[string]AuthPlus)
)

type AuthPlus interface {
	//Verify  校验
	// param: authData 认证数据
	// return:
	//        d: 继续校验的认证数据
	//        continueAuth：true：成功，false：继续校验
	//        err != nil: 校验失败
	Verify(authData []byte) (d []byte, continueAuth bool, err error) // 客户端自己验证时，忽略continueAuth
}

var Default = "default"

// 服务启动前注册有的鉴权插件
func Register(name string, provider AuthPlus) {
	if provider == nil {
		panic("authplus: Register provide is nil")
	}
	if name == "" {
		name = Default
	}
	if _, dup := providers[name]; dup {
		panic("authplus: Register called twice for provider " + name)
	}

	providers[name] = provider
	logger.Logger.Infof("Register AuthPlusProvide：'%s' success，%T", name, provider)
}

func Unregister(name string) {
	if name == "" {
		name = Default
	}
	delete(providers, name)
}

// 鉴权管理器，无需枷锁，保持无状态就行
type Manager struct {
	p AuthPlus
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

func (this *Manager) Verify(authData []byte) (d []byte, continueAuth bool, err error) {
	return this.p.Verify(authData)
}
