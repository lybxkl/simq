package config

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/consts"
	logger2 "gitee.com/Ljolan/si-mqtt/logger"
	utils2 "gitee.com/Ljolan/si-mqtt/utils"
	"github.com/BurntSushi/toml"
	"os"
)

var cfg SIConfig

func init() {
	name := "config.toml"
	if n := os.Getenv("CFG_NAME"); n != "" {
		name = n
	}

	cfg.Broker.RetainAvailable = true

	if _, err := toml.DecodeFile(utils2.GetConfigPath(utils2.GetCurrentDirectory(), name), &cfg); err != nil {
		panic(err)
	}
	fmt.Println(cfg.String())
	logger2.LogInit(cfg.Log.Level) // 日志必须提前初始化

	if cfg.Broker.AutoIdPrefix == "" {
		cfg.Broker.AutoIdPrefix = consts.AutoIdPrefix
	}
	if cfg.Broker.MaxQueueMessages == 0 {
		cfg.Broker.MaxQueueMessages = consts.MaxQueueMessages
	}
	if cfg.Broker.SysInterval == 0 {
		cfg.Broker.SysInterval = consts.SysInterval
	}
	if cfg.Broker.MaxKeepalive == 0 {
		cfg.Broker.MaxKeepalive = consts.KeepAlive
	}
}

type SIConfig struct {
	ServerVersion string        `toml:"serverVersion"`
	Log           Log           `toml:"log"`
	Broker        Broker        `toml:"broker"`
	Cluster       Cluster       `toml:"cluster"`
	DefaultConfig DefaultConfig `toml:"defaultConfig"`
	Store         Store         `toml:"store"`
	PProf         PProf         `toml:"pprof"`
}

func (cfg *SIConfig) OpenShare() bool {
	return !cfg.Broker.CloseShareSub
}

type Log struct {
	Level string `toml:"level"`
}
type PProf struct {
	Open bool  `toml:"open"`
	Port int64 `toml:"port"`
}
type Broker struct {
	TcpAddr       string `toml:"tcpAddr"`
	TcpTLSOpen    bool   `toml:"tcpTlsOpen"`
	WsAddr        string `toml:"wsAddr"`
	WsPath        string `toml:"wsPath"`
	WssAddr       string `toml:"wssAddr"`
	WssCertPath   string `toml:"wssCertPath"`
	WssKeyPath    string `toml:"wssKeyPath"`
	Ca            string `toml:"ca"`
	CloseShareSub bool   `toml:"closeShareSub"`

	AllowZeroLengthClientId    bool   `toml:"allow_zero_length_clientId"`
	AutoIdPrefix               string `toml:"auto_id_prefix"`               // 设置客户端id前缀， 默认auto-
	PerListenerSettings        bool   `toml:"per_listener_settings"`        // 每个listener都需要配置参数
	CheckRetainSource          bool   `toml:"check_retain_source"`          // 检查发送者是否有发送保留消息的权限，有则可以发送
	MaxInflightBytes           int32  `toml:"max_inflight_bytes"`           // 对于Qos1/2, 表示最大正在处理的字节数，默认为0（无最大值）
	MaxInflightMessages        int32  `toml:"max_inflight_messages"`        // 同时传输的消息最大数， qos>0
	MaxKeepalive               uint16 `toml:"max_keepalive"`                // 允许最大值65535 单位为s, 默认5分钟
	MaxPacketSize              int32  `toml:"max_packet_size"`              // 最大数据包大小，单位字节，超过会主动断开连接
	MaxQueueBytes              int64  `toml:"max_queue_bytes"`              // 最大消息队列的数量，超过会被丢弃，默认0-无最大值
	MaxQueueMessages           int32  `toml:"max_queue_messages"`           // 超过max_inflight_messages的消息会被缓存到队列中，默认100
	MessageSizeLimit           int32  `toml:"message_size_limit"`           // 允许发送的最大有效负载大小，默认0-无限制
	PersistentClientExpiration int64  `toml:"persistent_client_expiration"` // 客户端断开超过该时间，broker会删除该会话信息，就收不到断开后的离线消息了，非标准选项，默认为0-永不
	QueueQos0Messages          bool   `toml:"queue_qos0_messages"`          // 设置为true，在持久化session的离线消息中，会有qos=0的消息也在排队，同时也会受max_queue_messages限制，默认false
	RetainAvailable            bool   `toml:"retain_available"`             // 设置为true表示，支持retain 消息，设置为false，不支持，当发送了保留位设置为1的消息，会断开连接
	SysInterval                int64  `toml:"sys_interval"`                 // 订阅$sys level topic树的更新时间，默认10s，设为0，表示禁用$sys level
	UpgradeOutgoingQos         bool   `toml:"upgrade_outgoing_qos"`         // 设置为true，改变qos等级，与订阅者的消息等级匹配，默认false
}
type (
	ClusterModelEm = string
	StoreModel     = string
)

const (
	MongoStore StoreModel = "mongo"
	MysqlStore StoreModel = "mysql"
)
const (
	Getty   ClusterModelEm = "getty"
	MongoEm ClusterModelEm = "mongo"
	MysqlEm ClusterModelEm = "mysql"
)

type Cluster struct {
	Enabled             bool           `toml:"enabled"`
	Model               ClusterModelEm `toml:"model"`
	TaskClusterPoolSize uint32         `toml:"taskClusterPoolSize"`
	TaskServicePoolSize uint32         `toml:"taskServicePoolSize"`
	ClusterName         string         `toml:"clusterName"`
	SubMinNum           uint32         `toml:"subMinNum"`
	AutoPeriod          uint32         `toml:"autoPeriod"`
	LockTimeOut         uint32         `toml:"lockTimeOut"`
	LockAddLive         uint32         `toml:"lockAddLive"`
	CompressProportion  float32        `toml:"compressProportion"`

	// mongo配置
	MongoUrl             string `toml:"mongoUrl"`
	MongoMinPool         uint64 `toml:"mongoMinPool"`
	MongoMaxPool         uint64 `toml:"mongoMaxPool"`
	MongoMaxConnIdleTime uint64 `toml:"mongoMaxConnIdleTime"`
	MysqlUrl             string `toml:"mysqlUrl"`
	MysqlMaxPool         uint64 `toml:"mysqlMaxPool"`
	Period               uint64 `toml:"period"`
	BatchSize            uint64 `toml:"batchSize"`

	// getty配置
	ClusterHost    string     `toml:"clusterHost"`
	ClusterPort    int        `toml:"clusterPort"`
	ClientConNum   uint64     `toml:"clientConNum"`
	ClusterTLS     bool       `toml:"clusterTls"`
	ServerCertFile string     `toml:"serverCertFile"`
	ServerKeyFile  string     `toml:"serverKeyFile"`
	ClientCertFile string     `toml:"clientCertFile"`
	ClientKeyFile  string     `toml:"clientKeyFile"`
	StaticNodeList []NodeInfo `toml:"staticNodeList"`
}
type NodeInfo struct {
	Name string `toml:"name"`
	Addr string `toml:"addr"`
}
type Connect struct {
	Keepalive      int `toml:"keepalive"`
	WriteTimeout   int `toml:"writeTimeout"`
	ConnectTimeout int `toml:"connectTimeout"`
	AckTimeout     int `toml:"ackTimeout"`
	TimeoutRetries int `toml:"timeOutRetries"`
}
type Provider struct {
	SessionsProvider string `toml:"sessionsProvider"`
	TopicsProvider   string `toml:"topicsProvider"`
	Authenticator    string `toml:"authenticator"`
}
type DefaultConfig struct {
	Connect  Connect  `toml:"connect"`
	Provider Provider `toml:"provider"`
	Auth     Auth     `toml:"auth"`
	Server   Server   `toml:"server"`
}
type Auth struct {
	Allows []string `toml:"allows"`
}
type Server struct {
	Redirects         []string `tome:"redirects"`
	RedirectOpen      bool     `tome:"redirectOpen"`
	RedirectIsForEver bool     `tome:"redirectIsForEver"`
}
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
	Model StoreModel `toml:"model"`
	Mysql Mysql      `toml:"mysql"`
	Mongo Mongo      `toml:"mongo"`
	Redis Redis      `toml:"redis"`
}

func (conf *SIConfig) String() string {
	b, err := json.Marshal(*conf)
	if err != nil {
		return fmt.Sprintf("%+v", *conf)
	}
	var out bytes.Buffer
	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		return fmt.Sprintf("%+v", *conf)
	}
	return out.String()
}

func Configure(args []string) error {
	fs := flag.NewFlagSet("si_mqtt", flag.ExitOnError)

	fs.StringVar(&cfg.Log.Level, "log-level", cfg.Log.Level, "log level.")

	fs.StringVar(&cfg.Broker.TcpAddr, "broker-addr", cfg.Broker.TcpAddr, "broker tcp addr to listen on. eg. 'tcp://:1883'")
	fs.BoolVar(&cfg.Broker.TcpTLSOpen, "broker-tls", cfg.Broker.TcpTLSOpen, "whether broker tcp use tls.")
	fs.StringVar(&cfg.Broker.WsAddr, "ws-addr", cfg.Broker.WsAddr, "websocket broker addr, eg. ':8082'")
	fs.StringVar(&cfg.Broker.WsPath, "ws-path", cfg.Broker.WsPath, "websocket broker path. e.g., \"/mqtt\"")
	fs.StringVar(&cfg.Broker.Ca, "ca", cfg.Broker.Ca, "path of tls root ca file.")
	fs.StringVar(&cfg.Broker.WssAddr, "wss-addr", cfg.Broker.WssAddr, "HTTPS websocket broker addr, eg. ':8081'")
	fs.StringVar(&cfg.Broker.WssCertPath, "wss-certpath", cfg.Broker.WssCertPath, "HTTPS websocket broker public key file")
	fs.StringVar(&cfg.Broker.WssKeyPath, "wss-keypath", cfg.Broker.WssKeyPath, "HTTPS websocket broker private key file")

	fs.BoolVar(&cfg.Cluster.Enabled, "cluster-open", cfg.Cluster.Enabled, "open cluster.")
	fs.StringVar(&cfg.Cluster.Model, "cluster-model", cfg.Cluster.Model, "cluster startup mode.")
	fs.StringVar(&cfg.Cluster.ClusterName, "node-name", cfg.Cluster.ClusterName, "current node name of the cluster.")

	fs.StringVar(&cfg.Cluster.MongoUrl, "node-mongo-url", cfg.Cluster.MongoUrl, "node Mongo Url.")
	fs.Uint64Var(&cfg.Cluster.MongoMinPool, "node-mongo-min-pool", cfg.Cluster.MongoMinPool, "node Mongo Min Pool.")
	fs.Uint64Var(&cfg.Cluster.MongoMaxPool, "node-mongo-max-pool", cfg.Cluster.MongoMaxPool, "node Mongo Max Pool.")
	fs.Uint64Var(&cfg.Cluster.MongoMaxConnIdleTime, "node-mongo-max-con-idle", cfg.Cluster.MongoMaxConnIdleTime, "node Mongo Max ConnIdleTime.")
	fs.StringVar(&cfg.Cluster.MysqlUrl, "node-mysql-url", cfg.Cluster.MysqlUrl, "node Mysql Url.")
	fs.Uint64Var(&cfg.Cluster.MysqlMaxPool, "node-mysql-max-pool", cfg.Cluster.MysqlMaxPool, "node Mysql Max Pool.")
	fs.Uint64Var(&cfg.Cluster.Period, "node-db-period", cfg.Cluster.Period, "node DB period.")
	fs.Uint64Var(&cfg.Cluster.BatchSize, "node-db-batch-size", cfg.Cluster.BatchSize, "node DB Batch Size.")

	fs.StringVar(&cfg.Cluster.ClusterHost, "cluster-host", cfg.Cluster.ClusterHost, "cluster tcp host to listen on.")
	fs.IntVar(&cfg.Cluster.ClusterPort, "cluster-port", cfg.Cluster.ClusterPort, "cluster tcp port to listen on.")
	fs.BoolVar(&cfg.Cluster.ClusterTLS, "cluster-tls", cfg.Cluster.ClusterTLS, "whether cluster tcp use tls")
	fs.StringVar(&cfg.Cluster.ServerCertFile, "server-certfile", cfg.Cluster.ServerCertFile, "path of tls server cert file.")
	fs.StringVar(&cfg.Cluster.ServerKeyFile, "server-keyfile", cfg.Cluster.ServerKeyFile, "path of tls server key file.")
	fs.StringVar(&cfg.Cluster.ClientCertFile, "client-certfile", cfg.Cluster.ClientCertFile, "path of tls client cert file.")
	fs.StringVar(&cfg.Cluster.ClientKeyFile, "client-keyfile", cfg.Cluster.ClientKeyFile, "path of tls client key file.")

	fs.IntVar(&cfg.DefaultConfig.Connect.Keepalive, "keepalive", cfg.DefaultConfig.Connect.Keepalive, "Keepalive (sec)")
	fs.IntVar(&cfg.DefaultConfig.Connect.ConnectTimeout, "connect-timeout", cfg.DefaultConfig.Connect.ConnectTimeout, "Connect Timeout (sec)")
	fs.IntVar(&cfg.DefaultConfig.Connect.AckTimeout, "ack-timeout", cfg.DefaultConfig.Connect.AckTimeout, "Ack Timeout (sec)")
	fs.IntVar(&cfg.DefaultConfig.Connect.TimeoutRetries, "timeout-retries", cfg.DefaultConfig.Connect.TimeoutRetries, "Timeout Retries")
	fs.StringVar(&cfg.DefaultConfig.Provider.Authenticator, "auth", cfg.DefaultConfig.Provider.Authenticator, "Authenticator Type")
	//下面两个的value要改都要改
	fs.StringVar(&cfg.DefaultConfig.Provider.SessionsProvider, "sessions", cfg.DefaultConfig.Provider.SessionsProvider, "Session Provider Type")
	fs.StringVar(&cfg.DefaultConfig.Provider.TopicsProvider, "topics", cfg.DefaultConfig.Provider.TopicsProvider, "Topics Provider Type")

	fs.StringVar(&cfg.Store.Redis.Source, "redis-source", cfg.Store.Redis.Source, "Redis connect source")
	fs.IntVar(&cfg.Store.Redis.PoolSize, "redis-pool", cfg.Store.Redis.PoolSize, "Redis connect pool size")
	fs.IntVar(&cfg.Store.Redis.Db, "redis-db", cfg.Store.Redis.Db, "Redis db")

	fs.StringVar(&cfg.Store.Mysql.Source, "mysql-source", cfg.Store.Mysql.Source, "Mysql connect source")
	fs.IntVar(&cfg.Store.Mysql.PoolSize, "mysql-pool", cfg.Store.Mysql.PoolSize, "Mysql connect pool size")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return nil
}

// copy一份返回即可
func GetConfig() SIConfig {
	return cfg
}
