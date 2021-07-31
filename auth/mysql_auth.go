package auth

import (
	"SI-MQTT/config"
	"SI-MQTT/logger"
	. "SI-MQTT/mysql"
	"fmt"
)

type authenticator bool

var _ Authenticator = (*authenticator)(nil)

var (
	mysqlAuthenticator authenticator = true
	defaultName                      = ""
	defaultPwd                       = ""
	openTestAuth                     = false
	mysqlPoolSize      int
	mysqlUrl           string
	mysqlAccount       string
	mysqlPassword      string
	mysqlDatabase      string
)

//初始化数据库连接池
var dbChanPool chan DB //创建10连接的数据库连接池
// mysql auth的初始化
func mysqlAuthInit() {
	consts := config.ConstConf.MyAuth
	if consts.Open {
		defaultName = consts.DefaultName
		defaultPwd = consts.DefaultPwd
	}
	openTestAuth = consts.Open
	mysqlConfig := config.ConstConf.DefaultConst
	if mysqlConfig.Authenticator != "mysql" {
		return
	}
	mysqlUrl = mysqlConfig.Mysql.MysqlUrl
	mysqlAccount = mysqlConfig.Mysql.Account
	mysqlPassword = mysqlConfig.Mysql.PassWord
	mysqlDatabase = mysqlConfig.Mysql.DataBase
	mysqlPoolSize = int(mysqlConfig.Mysql.MysqlPoolSize)
	if mysqlPoolSize <= 0 {
		mysqlPoolSize = 10 //要是格式不对，设置默认的
	}
	dbChanPool = make(chan DB, mysqlPoolSize)
	for i := 0; i < 10; i++ {
		dd := DB{}
		db := dd.OpenLink("mysql", mysqlAccount+":"+mysqlPassword+"@tcp("+mysqlUrl+")/"+mysqlDatabase) //注意严格区分大小写 mq-mysql
		dd.Prepare(db, "select clientID,password from dev_user where account = ?")
		dbChanPool <- dd
	}
	Register(mysqlConfig.Authenticator, NewMysqlAuth()) //开启mem验证
	logger.Logger.Info("开启mysql账号认证")
}
func NewMysqlAuth() Authenticator {
	return &mysqlAuthenticator
}

//权限认证
func (this authenticator) Authenticate(id string, cred interface{}) error {
	if this {
		//判断是否开启测试账号功能
		if openTestAuth {
			if id == defaultName && cred == defaultPwd {
				logger.Logger.Info("默认账号登陆成功")
				//放行
				return nil
			}
		}
		if clientID, ok := checkAuth(id, cred); ok {
			logger.Logger.Info(fmt.Sprintf("mysql : 账号：%s，密码：%v，登陆成功，clientID==%s", id, cred, clientID))
			return nil
		} else {
			logger.Logger.Info(fmt.Sprintf("mysql : 账号：%s，密码：%v，登陆失败", id, cred))
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
	db := <-dbChanPool
	defer func() { dbChanPool <- db }()
	clientID, password, err := db.SelectClient(id)
	if err != nil {
		return "", false
	}
	//这里可以添加MD5验证
	if password == cred.(string) {
		return clientID, true
	} else {
		return "", false
	}
}
