package auth

import (
	logger2 "gitee.com/Ljolan/si-mqtt/logger"
)

type noAuthenticator bool

var _ Authenticator = (*noAuthenticator)(nil)

var (
	noAuth noAuthenticator = true
)

// default auth的初始化
func defaultAuthInit() {
	Register("", NewDefaultAuth()) //开启默认验证
	logger2.Logger.Info("开启default进行账号认证")
}
func NewDefaultAuth() Authenticator {
	return &noAuth
}

//权限认证
func (this noAuthenticator) Authenticate(id string, cred interface{}) error {
	return nil
}
