package autocompress

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sync"
	"time"
)

var (
	log  *zap.Logger
	once sync.Once
)

type Sub struct {
	Id         int64  `gorm:"primary_key"`
	SubOrUnSub int    `gorm:"column:sub_or_unsub"` // 1: sub 2: unSub
	Sender     string `gorm:"column:sender;type:varchar(30)"`
	Topic      string `gorm:"column:topic;type:varchar(80)"` // 通过转为hex字符串拼接,号存储
	Num        uint32 `gorm:"column:num"`                    // 合并的数据，默认0表示一个，其它表示 值+1 如， 2 表示 2+1=3
	Stamp      int64  `gorm:"column:stamp;index"`
}

func (s Sub) TableName() string {
	return "sub"
}

type AutoCompress interface {
	Lock()                                                    // 阻塞时抢锁，可睡眠再抢，谁抢到谁负责合并sub
	LockAndLive() bool                                        // 给锁续命，失败false,则会重新开始抢锁
	GetSubs(min int) ([]*Sub, error)                          // 获取订阅和取消订阅事件，数据量低于min则不处理
	DelSome(id int64, num int32, del []int64) error           // del中的全部删除 和 更新一条id的num数据
	DelPatALl(sender string, topic string, del []int64) error // del中的全部删除
}

// SubAutoCompress 自动合并压缩订阅、取消订阅事件数据
// 方便获取最全集群共享订阅数据
// min 最小保持数
// period 周期，单位秒
// auto 根据不同存储实现的插件
func SubAutoCompress(url string, min, period int, auto AutoCompress) {
	once.Do(func() {
		initLog()
	})
	auto.Lock()
	go func() {
		for {
			select {
			case <-time.After(time.Duration(period) * time.Second):
				if !auto.LockAndLive() {
					log.Warn("重新抢锁")
					SubAutoCompress(url, min, period, auto)
					return
				}
			}
			subs, err := auto.GetSubs(min)
			if err != nil {
				log.Error(err.Error())
				continue
			}
			if len(subs) == 0 {
				continue
			}
			// 处理合并
			for i := 0; i < len(subs); i++ {
				if subs[i] == nil {
					continue
				}
				var (
					del    = make([]int64, 0)
					sender = subs[i].Sender
					topic  = subs[i].Topic
					num    = int32(0) // 最终数量，最后需要减一存储
					tag    = false    // 操作标志
					id     = subs[i].Id
				)
				if subs[i].SubOrUnSub == 1 {
					num = int32(subs[i].Num) + 1
				} else { // 正常不会出现这个
					subs[i] = nil // todo 是否需要删除数据库
					continue
				}

				for j := i + 1; j < len(subs); j++ {
					sb := subs[j]
					if sb == nil || sb.Sender != sender || sb.Topic != topic {
						continue
					}
					if sb.SubOrUnSub == 1 {
						num += 1
					} else if sb.SubOrUnSub == 2 {
						num -= 1
					} else {
						continue // todo 是否需要删除数据库
					}
					tag = true
					del = append(del, sb.Id)
					subs[j] = nil
				}
				// 修改 subs[i].Id对应的nums ， 同时删除 del中的记录
				if !tag {
					continue
				}
				if num == 0 { // 处理合并后，等于0 表示直接删除该sender和topic的当前获取到的所有 即可
					del = append(del, id) // 删除del的即可
					if e := auto.DelPatALl(sender, topic, del); e != nil {
						log.Error(e.Error())
					}
				} else if num > 0 && len(del) > 0 {
					num--
					// 更新 subs[i].Id 数据，删除del的记录
					if e := auto.DelSome(id, num, del); e != nil {
						log.Error(e.Error())
					}
				} else if len(del) == 0 {
					continue
				} else {
					// 正常情况不会出现这个
				}
			}
		}
	}()
}

func initLog() {
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
	atom := zap.NewAtomicLevelAt(zap.WarnLevel)

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
	lg, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("log 初始化失败: %v", err))
	}
	log = lg
}
