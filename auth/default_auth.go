package auth

import (
	"SI-MQTT/comment"
	"SI-MQTT/config"
)

type noAuthenticator bool

var _ Authenticator = (*noAuthenticator)(nil)

var (
	memAuth noAuthenticator = true
)

// default auth的初始化
func defaultAuthInit() {
	consts := config.ConstConf.MyAuth
	if consts.Open {
		defaultName = consts.DefaultName
		defaultPwd = consts.DefaultPwd
	}
	openTestAuth = consts.Open
	DefaultConfig := config.ConstConf.DefaultConst
	if DefaultConfig.Authenticator == "" || DefaultConfig.Authenticator == comment.DefaultAuthenticator {
		// 默认不验证
		Register(comment.DefaultAuthenticator, NewDefaultAuth()) //开启默认验证
	}
}
func NewDefaultAuth() Authenticator {
	return &memAuth
}

//权限认证
func (this noAuthenticator) Authenticate(id string, cred interface{}) error {
	return nil
}
