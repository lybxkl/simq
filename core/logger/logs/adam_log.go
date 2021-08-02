package logs

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strings"
	"sync"
	"time"
)

var adamLogger *AdamLog // adam 启动时初始化，和adam中的logger是一个

func GetLogger() *AdamLog {
	return adamLogger
}

var once sync.Once

type AdamLog struct {
	zap *zap.Logger
	*zap.SugaredLogger
}
type Field = zap.Field

func LogInit(level string) {
	old := level
	level = strings.ToLower(level)
	logLevel := zap.InfoLevel
	switch level {
	case "debug":
		logLevel = zap.DebugLevel
	case "info":
	case "panic":
		logLevel = zap.PanicLevel
	case "error":
		logLevel = zap.ErrorLevel
	case "warn":
		logLevel = zap.WarnLevel
	case "dpanic":
		logLevel = zap.DPanicLevel
	case "fatal":
		logLevel = zap.FatalLevel
	default:
		panic(errors.New(fmt.Sprintf("unSupport log level [%v]." + old)))
	}
	NewAdamLog(logLevel)
}

// 自行配置日志
func NewAdamLogSelf(encoderConfig zapcore.EncoderConfig, zapConfig zap.Config) *AdamLog {
	// 构建日志
	logger, err := zapConfig.Build()
	if err != nil {
		panic(fmt.Sprintf("log 初始化失败: %v", err))
	}
	logger.Info("log 初始化成功", Time("runTime", time.Now()))
	adamLogger = &AdamLog{
		zap: logger,
	}
	return adamLogger
}

// 系统自动配置
func NewAdamLog(level zapcore.Level) *AdamLog {
	once.Do(func() {
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder, // 小写编码器
			EncodeTime:     zapcore.ISO8601TimeEncoder,    // ISO8601 UTC 时间格式
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder, // 短路径编码器
		}

		// 设置日志级别
		atom := zap.NewAtomicLevelAt(level)

		config := zap.Config{
			Level:         atom,          // 日志级别
			Development:   false,         // 开发模式，堆栈跟踪
			Encoding:      "console",     // 输出格式 console 或 json
			EncoderConfig: encoderConfig, // 编码器配置
			//InitialFields:    map[string]interface{}{"SaltIceMQTT": "SaltIce"}, // 初始化字段，如：添加一个服务器名称
			OutputPaths:      []string{"stdout"}, // 输出到指定文件 stdout（标准输出，正常颜色） stderr（错误输出，红色）
			ErrorOutputPaths: []string{"stderr"},
		}

		// 构建日志
		logger, err := config.Build()
		if err != nil {
			panic(fmt.Sprintf("log 初始化失败: %v", err))
		}
		logger.Info("log 初始化成功", Time("runTime", time.Now()))
		sugar := logger.Sugar()
		adamLogger = &AdamLog{
			zap:           logger,
			SugaredLogger: sugar,
		}
	})
	return adamLogger
}

func (a *AdamLog) Close() error {
	a.zap.Sync()
	a.SugaredLogger.Sync()
	return nil
}
