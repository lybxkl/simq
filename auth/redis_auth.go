package auth

import (
	"SI-MQTT/config"
	"SI-MQTT/logger"
	"SI-MQTT/redis"
	"fmt"
	"strconv"
)

type redisAuthenticator bool

var (
	redisAuth     redisAuthenticator = true
	size          int
	redisUrl      string
	redisPassword string
	redisDB       int
)

//redis池
var redisChanPool chan redisNode

//连接节点结构体
type redisNode struct {
	id    string
	redis *redis.Redis
}

func redisAuthInit() {
	consts := config.ConstConf.MyAuth
	if consts.Open {
		defaultName = consts.DefaultName
		defaultPwd = consts.DefaultPwd
	}
	openTestAuth = consts.Open
	redisConfig := config.ConstConf.DefaultConst
	if !(redisConfig.Authenticator == "redis") {
		//判断是否采用redis,采用才初始化redis池
		return
	}
	if redisConfig.Redis.RSize <= 0 {
		size = 10 //要是没有设置，默认10个
	} else {
		size = int(redisConfig.Redis.RSize)
	}
	redisChanPool = make(chan redisNode, size)
	redisUrl = redisConfig.Redis.RedisUrl
	redisPassword = redisConfig.Redis.PassWord
	redisDB = int(redisConfig.Redis.DB)

	count := 0
	for i := 0; i < size; i++ {
		r := redis.Redis{}
		err := r.CreatCon("tcp", redisUrl, redisPassword, redisDB)
		if err != nil {
			count++
			logger.Logger.Error(fmt.Sprintf("Connect to redis-%d error: %v", i, err))
		}
		rd := redisNode{id: "redis-" + strconv.Itoa(i), redis: &r}
		redisChanPool <- rd
	}
	logger.Logger.Info(fmt.Sprintf("redis池化处理完成，size：%d", size-count))
	if openTestAuth {
		rd := <-redisChanPool
		defer func() { redisChanPool <- rd }()
		r := rd.redis
		//插入默认账号密码
		r.SetNX(defaultName, defaultPwd)
	}
	Register(redisConfig.Authenticator, NewRedisAuth()) //开启验证
	logger.Logger.Info("开启redis进行账号认证")

}
func NewRedisAuth() Authenticator {
	return &redisAuth
}
func (this redisAuthenticator) Authenticate(id string, cred interface{}) error {
	if this {
		if cid, ok := redisCheck(id, cred); ok {
			logger.Logger.Info(fmt.Sprintf("redis : 账号：%v，密码：%v，登陆-成功，clientID==%s", id, cred, cid))
			return nil
		}
		logger.Logger.Info(fmt.Sprintf("redis : 账号：%v，密码：%v，登陆-失败", id, cred))
		return ErrAuthFailure
	}
	logger.Logger.Info("当前未开启账号验证，取消一切连接。。。")
	return ErrAuthFailure
}
func redisCheck(id string, cred interface{}) (string, bool) {
	rd := <-redisChanPool
	defer func() { redisChanPool <- rd }()
	r := rd.redis
	b, err := r.GetV(id)
	if err != nil {
		logger.Logger.Info(fmt.Sprintf("redis get %s failed:%v\n", id, err.Error()))
		return "", false
	}
	if string(b) == cred.(string) {
		return "id", true
	} else {
		return "", false
	}
}
