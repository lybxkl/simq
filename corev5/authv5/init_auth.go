package authv5

func AuthInit(auth string) {
	switch auth {
	case "mysql":
		mysqlAuthInit()
	case "redis":
		redisAuthInit()
	default:
		defaultAuthInit()
	}
}
