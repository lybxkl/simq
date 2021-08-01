package auth

func init() {
	defaultAuthInit()
	mysqlAuthInit()
	redisAuthInit()
}
