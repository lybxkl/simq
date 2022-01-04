package config

type (
	StoreModel = string
)

const (
	MongoStore StoreModel = "mongo"
	MysqlStore StoreModel = "mysql"
)

type Mysql struct {
	Source   string `toml:"source"`
	PoolSize int    `toml:"poolSize"`
}

type Mongo struct {
	Source          string `toml:"source"`
	MinPoolSize     uint64 `toml:"minPool"`
	MaxPoolSize     uint64 `toml:"maxPool"`
	MaxConnIdleTime uint64 `toml:"maxConnIdleTime"`
}

type Redis struct {
	Source   string `toml:"source"`
	Db       int    `toml:"db"`
	PoolSize int    `toml:"poolSize"`
}

type Store struct {
	StoreModel `toml:"model"`
	Mysql      `toml:"mysql"`
	Mongo      `toml:"mongo"`
	Redis      `toml:"redis"`
}
