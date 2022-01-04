package config

type (
	ClusterModelEm = string
)

const (
	Getty   ClusterModelEm = "getty"
	MongoEm ClusterModelEm = "mongo"
	MysqlEm ClusterModelEm = "mysql"
)

type Cluster struct {
	Enabled             bool           `toml:"enabled"`
	Model               ClusterModelEm `toml:"model"  validate:"default=getty"`
	TaskClusterPoolSize uint32         `toml:"taskClusterPoolSize" validate:"default=1000"`
	TaskServicePoolSize uint32         `toml:"taskServicePoolSize" validate:"default=1000"`
	ClusterName         string         `toml:"clusterName" validate:"default=cluster-*"`
	SubMinNum           uint32         `toml:"subMinNum" validate:"default=50"`
	AutoPeriod          uint32         `toml:"autoPeriod" validate:"default=100"`
	LockTimeOut         uint32         `toml:"lockTimeOut" validate:"default=50"`
	LockAddLive         uint32         `toml:"lockAddLive" validate:"default=50"`
	CompressProportion  float32        `toml:"compressProportion" validate:"default=0.7"`

	// mongo配置
	MongoUrl             string `toml:"mongoUrl"`
	MongoMinPool         uint64 `toml:"mongoMinPool"  validate:"default=50"`
	MongoMaxPool         uint64 `toml:"mongoMaxPool" validate:"default=100"`
	MongoMaxConnIdleTime uint64 `toml:"mongoMaxConnIdleTime" validate:"default=100"`
	MysqlUrl             string `toml:"mysqlUrl"`
	MysqlMaxPool         uint64 `toml:"mysqlMaxPool" validate:"default=50"`
	Period               uint64 `toml:"period" validate:"default=50"`
	BatchSize            uint64 `toml:"batchSize" validate:"default=50"`

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
