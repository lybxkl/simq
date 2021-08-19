package authv5

import (
	"gitee.com/Ljolan/si-mqtt/core/logger"
)

type authenticator bool

var _ Authenticator = (*authenticator)(nil)

var (
	mysqlAuthenticator authenticator = true
)

// mysql auth的初始化
func mysqlAuthInit() {
	Register("mysql", NewMysqlAuth())
	logger.Logger.Info("开启mysql账号认证")
}
func NewMysqlAuth() Authenticator {
	return &mysqlAuthenticator
}

//权限认证
func (this authenticator) Authenticate(id string, cred interface{}) error {
	if this {
		if clientID, ok := checkAuth(id, cred); ok {
			logger.Logger.Infof("mysql : 账号：%s，密码：%v，登陆成功，clientID==%s", id, cred, clientID)
			return nil
		} else {
			logger.Logger.Infof("mysql : 账号：%s，密码：%v，登陆失败", id, cred)
			return ErrAuthFailure
		}
	}
	logger.Logger.Info("当前未开启账号验证，取消一切连接。。。")
	//取消客户端登陆连接
	return ErrAuthFailure
}

/**
	验证身份，返回clientID
**/
func checkAuth(id string, cred interface{}) (string, bool) {
	if clientID, ok := mysqlAuth(id, cred); ok {
		return clientID, true
	}
	return "", false
}

/**
	mysql账号认证，返回clientID
**/
func mysqlAuth(id string, cred interface{}) (string, bool) {
	return "", false
}
