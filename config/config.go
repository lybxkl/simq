package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/config/env"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	"github.com/BurntSushi/toml"
	"github.com/go-playground/locales/zh"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
	"os"
	"reflect"
	"strconv"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

var (
	cfgIn    *SIConfig
	Validate = validator.New()
	trans    ut.Translator
)

func init() {
	uni := ut.New(zh.New())
	trans, _ = uni.GetTranslator("zh")

	//注册一个函数，获取struct tag里自定义的label作为字段名
	Validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		label := fld.Tag.Get("label")
		if label == "" {
			return fld.Name
		}
		return label
	})

	utils.MustPanic(Validate.RegisterValidation("default", func(fl validator.FieldLevel) bool {
		switch fl.Field().Kind() {
		case reflect.String:
			if fl.Field().String() == "" {
				if strings.Contains(fl.Param(), "*") {
					fl.Field().Set(reflect.ValueOf(strings.Replace(fl.Param(), "*", utils.Generate(), 1)))
				} else {
					fl.Field().Set(reflect.ValueOf(fl.Param()))
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if fl.Field().Int() == 0 {
				setIntOrUint(fl)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if fl.Field().Uint() == 0 {
				setIntOrUint(fl)
			}
		case reflect.Float32, reflect.Float64:
			if fl.Field().Float() == 0 {
				setFloat(fl)
			}
		}
		return true
	}))

	//验证器注册翻译器
	utils.MustPanic(zh_translations.RegisterDefaultTranslations(Validate, trans))
}

func init() {
	if initCfg, exist := os.LookupEnv("CFG_INIT"); !exist || initCfg != "true" {
		return
	}
	name := "config.toml"
	if n := os.Getenv("CFG_NAME"); n != "" {
		name = n
	}

	cfg, err := Init(true, utils.GetConfigPath(utils.GetCurrentDirectory(), name))
	if err != nil {
		panic(err)
	}
	logger.LogInit(cfg.Log.Level) // 日志初始化
	cfgIn = cfg
}

func Init(print bool, path ...string) (*SIConfig, error) {
	cfgPath := ""
	if len(path) > 0 {
		cfgPath = path[0]
	}
	if siCfgPath, exist := env.GetEnv(env.SICfgPath); exist {
		cfgPath = siCfgPath
	}

	cfg := &SIConfig{}
	cfg.Broker.RetainAvailable = true

	if _, err := toml.DecodeFile(cfgPath, &cfg); err != nil {
		return nil, err
	}

	if cfg.Broker.MaxQos > 2 || cfg.Broker.MaxQos < 1 {
		cfg.Broker.MaxQos = 2
	}

	if print {
		fmt.Println(cfg.String())
	}
	return cfg, nil
}

type SIConfig struct {
	ServerVersion string `toml:"serverVersion" validate:"default=5.0"`
	Log           `toml:"log"`
	Broker        `toml:"broker"`
	Cluster       `toml:"cluster"`
	DefaultConfig `toml:"defaultConfig"`
	Store         `toml:"store"`
	PProf         `toml:"pprof"`
}

func (cfg *SIConfig) OpenShare() bool {
	return !cfg.Broker.CloseShareSub
}

func (cfg *SIConfig) String() string {
	b, err := json.Marshal(*cfg)
	if err != nil {
		return fmt.Sprintf("%+v", *cfg)
	}
	var out bytes.Buffer
	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		return fmt.Sprintf("%+v", *cfg)
	}
	return out.String()
}

func Configure(args []string) error {
	fs := flag.NewFlagSet("si_mqtt", flag.ExitOnError)

	fs.StringVar(&cfgIn.Log.Level, "log-level", cfgIn.Log.Level, "log level.")

	fs.StringVar(&cfgIn.Broker.TcpAddr, "broker-addr", cfgIn.Broker.TcpAddr, "broker tcp addr to listen on. eg. 'tcp://:1883'")
	fs.BoolVar(&cfgIn.Broker.TcpTLSOpen, "broker-tls", cfgIn.Broker.TcpTLSOpen, "whether broker tcp use tls.")
	fs.StringVar(&cfgIn.Broker.WsAddr, "ws-addr", cfgIn.Broker.WsAddr, "websocket broker addr, eg. ':8082'")
	fs.StringVar(&cfgIn.Broker.WsPath, "ws-path", cfgIn.Broker.WsPath, "websocket broker path. e.g., \"/mqtt\"")
	fs.StringVar(&cfgIn.Broker.Ca, "ca", cfgIn.Broker.Ca, "path of tls root ca file.")
	fs.StringVar(&cfgIn.Broker.WssAddr, "wss-addr", cfgIn.Broker.WssAddr, "HTTPS websocket broker addr, eg. ':8081'")
	fs.StringVar(&cfgIn.Broker.WssCertPath, "wss-certpath", cfgIn.Broker.WssCertPath, "HTTPS websocket broker public key file")
	fs.StringVar(&cfgIn.Broker.WssKeyPath, "wss-keypath", cfgIn.Broker.WssKeyPath, "HTTPS websocket broker private key file")

	fs.BoolVar(&cfgIn.Cluster.Enabled, "cluster-open", cfgIn.Cluster.Enabled, "open cluster.")
	fs.StringVar(&cfgIn.Cluster.Model, "cluster-model", cfgIn.Cluster.Model, "cluster startup mode.")
	fs.StringVar(&cfgIn.Cluster.ClusterName, "node-name", cfgIn.Cluster.ClusterName, "current node name of the cluster.")

	fs.StringVar(&cfgIn.Cluster.MongoUrl, "node-mongo-url", cfgIn.Cluster.MongoUrl, "node Mongo Url.")
	fs.Uint64Var(&cfgIn.Cluster.MongoMinPool, "node-mongo-min-pool", cfgIn.Cluster.MongoMinPool, "node Mongo Min Pool.")
	fs.Uint64Var(&cfgIn.Cluster.MongoMaxPool, "node-mongo-max-pool", cfgIn.Cluster.MongoMaxPool, "node Mongo Max Pool.")
	fs.Uint64Var(&cfgIn.Cluster.MongoMaxConnIdleTime, "node-mongo-max-con-idle", cfgIn.Cluster.MongoMaxConnIdleTime, "node Mongo Max ConnIdleTime.")
	fs.StringVar(&cfgIn.Cluster.MysqlUrl, "node-mysql-url", cfgIn.Cluster.MysqlUrl, "node Mysql Url.")
	fs.Uint64Var(&cfgIn.Cluster.MysqlMaxPool, "node-mysql-max-pool", cfgIn.Cluster.MysqlMaxPool, "node Mysql Max Pool.")
	fs.Uint64Var(&cfgIn.Cluster.Period, "node-db-period", cfgIn.Cluster.Period, "node DB period.")
	fs.Uint64Var(&cfgIn.Cluster.BatchSize, "node-db-batch-size", cfgIn.Cluster.BatchSize, "node DB Batch Size.")

	fs.StringVar(&cfgIn.Cluster.ClusterHost, "cluster-host", cfgIn.Cluster.ClusterHost, "cluster tcp host to listen on.")
	fs.IntVar(&cfgIn.Cluster.ClusterPort, "cluster-port", cfgIn.Cluster.ClusterPort, "cluster tcp port to listen on.")
	fs.BoolVar(&cfgIn.Cluster.ClusterTLS, "cluster-tls", cfgIn.Cluster.ClusterTLS, "whether cluster tcp use tls")
	fs.StringVar(&cfgIn.Cluster.ServerCertFile, "server-certfile", cfgIn.Cluster.ServerCertFile, "path of tls server cert file.")
	fs.StringVar(&cfgIn.Cluster.ServerKeyFile, "server-keyfile", cfgIn.Cluster.ServerKeyFile, "path of tls server key file.")
	fs.StringVar(&cfgIn.Cluster.ClientCertFile, "client-certfile", cfgIn.Cluster.ClientCertFile, "path of tls client cert file.")
	fs.StringVar(&cfgIn.Cluster.ClientKeyFile, "client-keyfile", cfgIn.Cluster.ClientKeyFile, "path of tls client key file.")

	fs.IntVar(&cfgIn.DefaultConfig.Connect.Keepalive, "keepalive", cfgIn.DefaultConfig.Connect.Keepalive, "Keepalive (sec)")
	fs.IntVar(&cfgIn.DefaultConfig.Connect.ConnectTimeout, "connect-timeout", cfgIn.DefaultConfig.Connect.ConnectTimeout, "Connect Timeout (sec)")
	fs.IntVar(&cfgIn.DefaultConfig.Connect.AckTimeout, "ack-timeout", cfgIn.DefaultConfig.Connect.AckTimeout, "Ack Timeout (sec)")
	fs.IntVar(&cfgIn.DefaultConfig.Connect.TimeoutRetries, "timeout-retries", cfgIn.DefaultConfig.Connect.TimeoutRetries, "Timeout Retries")
	fs.StringVar(&cfgIn.DefaultConfig.Provider.Authenticator, "auth", cfgIn.DefaultConfig.Provider.Authenticator, "Authenticator Type")
	//下面两个的value要改都要改
	fs.StringVar(&cfgIn.DefaultConfig.Provider.SessionsProvider, "sessions", cfgIn.DefaultConfig.Provider.SessionsProvider, "Session Provider Type")
	fs.StringVar(&cfgIn.DefaultConfig.Provider.TopicsProvider, "topics", cfgIn.DefaultConfig.Provider.TopicsProvider, "Topics Provider Type")

	fs.StringVar(&cfgIn.Store.Redis.Source, "redis-source", cfgIn.Store.Redis.Source, "Redis connect source")
	fs.IntVar(&cfgIn.Store.Redis.PoolSize, "redis-pool", cfgIn.Store.Redis.PoolSize, "Redis connect pool size")
	fs.IntVar(&cfgIn.Store.Redis.Db, "redis-db", cfgIn.Store.Redis.Db, "Redis db")

	fs.StringVar(&cfgIn.Store.Mysql.Source, "mysql-source", cfgIn.Store.Mysql.Source, "Mysql connect source")
	fs.IntVar(&cfgIn.Store.Mysql.PoolSize, "mysql-pool", cfgIn.Store.Mysql.PoolSize, "Mysql connect pool size")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return nil
}

func GetConfig() *SIConfig {
	return cfgIn
}

func setIntOrUint(fl validator.FieldLevel) bool {
	va, err := strconv.ParseInt(fl.Param(), 10, 64)
	if err != nil {
		return false
	}

	switch fl.Field().Kind() {
	case reflect.Int:
		fl.Field().Set(reflect.ValueOf(int(va)))
	case reflect.Uint:
		fl.Field().Set(reflect.ValueOf(uint(va)))
	case reflect.Int8:
		fl.Field().Set(reflect.ValueOf(int8(va)))
	case reflect.Uint8:
		fl.Field().Set(reflect.ValueOf(uint8(va)))
	case reflect.Int16:
		fl.Field().Set(reflect.ValueOf(int16(va)))
	case reflect.Uint16:
		fl.Field().Set(reflect.ValueOf(uint16(va)))
	case reflect.Int32:
		fl.Field().Set(reflect.ValueOf(int32(va)))
	case reflect.Uint32:
		fl.Field().Set(reflect.ValueOf(uint32(va)))
	case reflect.Int64:
		fl.Field().Set(reflect.ValueOf(int64(va)))
	case reflect.Uint64:
		fl.Field().Set(reflect.ValueOf(uint64(va)))
	}
	return true
}

func setFloat(fl validator.FieldLevel) bool {
	va, err := strconv.ParseFloat(fl.Param(), 64)
	if err != nil {
		return false
	}

	switch fl.Field().Kind() {
	case reflect.Float32:
		fl.Field().Set(reflect.ValueOf(float32(va)))
	case reflect.Float64:
		fl.Field().Set(reflect.ValueOf(float64(va)))
	}
	return true
}

func Translate(errs error) error {
	if errs == nil {
		return errs
	}
	if err, ok := errs.(validator.ValidationErrors); ok {
		var errList []string
		for _, e := range err {
			// can translate each error one at a time.
			errList = append(errList, e.Translate(trans))
		}
		return errors.New(strings.Join(errList, "|"))
	} else {
		return errs
	}
}
