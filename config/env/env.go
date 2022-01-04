package env

const (
	SICfgInit = "CFG_INIT"
	SICfgName = "CFG_NAME"
	SICfgPath = "SI_CFG_PATH"
)

var env = map[string]string{
	SICfgInit: "false",
	SICfgName: "config.toml",
	SICfgPath: "F:\\Go_pro\\src\\si-mqtt\\config\\config.toml",
}

func GetEnv(envKey string) (string, bool) {
	v, exist := env[envKey]
	return v, exist
}
