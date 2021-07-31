package config

//配置文件中字母要小写，结构体属性首字母要大写

//配置
type MyConst struct {
	DefaultConst  DefaultConstConfig
	ServerVersion string
	MyBuff        BuffConfig
	MyAuth        AuthConfig
	Cluster       Cluster
	Logger        Logger
	BrokerUrl     string
	WsBrokerUrl   string
}
type Logger struct {
	InfoOpen  bool
	DebugOpen bool
	Level     uint
	LogPath   string
}
type Cluster struct {
	Enabled   bool
	ZkUrl     string
	Name      string
	HostIp    string
	ZkTimeOut int
	ZkMqRoot  string
	ShareJoin string
}

type Version struct {
	ServerVersion string
}
type DefaultConstConfig struct {
	KeepAlive                 uint
	ConnectTimeout            uint
	AckTimeout                uint
	TimeoutRetries            uint
	SessionsProvider          string
	TopicsProvider            string
	Authenticator             string
	Redis                     RedisConfig
	Mysql                     MysqlConfig
	RedisSessionsProvider     string
	RedisTopicsProvider       string
	RedisAuthenticator        string
	RedisFailureAuthenticator string
}
type RedisConfig struct {
	RedisUrl string
	PassWord string
	RSize    uint
	DB       uint
}
type MysqlConfig struct {
	MysqlUrl      string
	Account       string
	PassWord      string
	DataBase      string
	MysqlPoolSize uint
}
type AuthConfig struct {
	Open        bool
	DefaultName string
	DefaultPwd  string
}
type BuffConfig struct {
	BufferSize     uint64
	ReadBlockSize  uint64
	WriteBlockSize uint64
}
