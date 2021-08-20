package auth

import (
	logger2 "gitee.com/Ljolan/si-mqtt/logger"
)

type redisAuthenticator bool

var (
	redisAuth redisAuthenticator = true
)

func redisAuthInit() {
	Register("redis", NewRedisAuth()) //开启验证
	logger2.Logger.Info("开启redis进行账号认证")
}
func NewRedisAuth() Authenticator {
	return &redisAuth
}
func (this redisAuthenticator) Authenticate(id string, cred interface{}) error {
	if this {
		if cid, ok := redisCheck(id, cred); ok {
			logger2.Logger.Infof("redis : 账号：%v，密码：%v，登陆-成功，clientID==%s", id, cred, cid)
			return nil
		}
		logger2.Logger.Infof("redis : 账号：%v，密码：%v，登陆-失败", id, cred)
		return ErrAuthFailure
	}
	logger2.Logger.Info("当前未开启账号验证，取消一切连接。。。")
	return ErrAuthFailure
}
func redisCheck(id string, cred interface{}) (string, bool) {
	return "", false
}
