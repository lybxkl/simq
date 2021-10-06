package auto_compress_sub

import (
	"fmt"
	sql "gitee.com/Ljolan/si-mqtt/cluster/stat/colong/db/mysql"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
	"strings"
	"time"
)

var log *zap.Logger
var mysqlDb *gorm.DB

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
	atom := zap.NewAtomicLevelAt(zap.InfoLevel)

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
	lg.Info("log 初始化成功")
	log = lg
}

// MysqlAutoCompressSub 自动合并压缩订阅、取消订阅事件数据
// 方便获取最全集群共享订阅数据
func MysqlAutoCompressSub(url string, min, period int) {
	initLog()

	cfg := &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 glog.Default.LogMode(glog.Silent),
		PrepareStmt:            true,
		DisableAutomaticPing:   true,
	}
	var err error
	mysqlDb, err = gorm.Open(mysql.Open(url), cfg)
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			select {
			case <-time.After(time.Duration(period) * time.Second):
			}

			subs := getSubs(min)
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
					delPatALl(sender, topic, del)
				} else if num > 0 && len(del) > 0 {
					num--
					// 更新 subs[i].Id 数据，删除del的记录
					delSome(num, id, del)
				} else if len(del) == 0 {
					continue
				} else {
					// 正常情况不会出现这个
				}
			}
		}
	}()
}

func delSome(num int32, id int64, del []int64) {
	sb, dels := buildSql(del)
	tx := mysqlDb.Begin()
	if e := tx.Exec("update sub set num = ? where id = ?", uint32(num), id).Error; e != nil {
		tx.Rollback()
		log.Error(e.Error())
	} else {
		if e = tx.Exec(sb.String(), dels...).Error; e != nil {
			tx.Rollback()
			log.Error(e.Error())
		} else {
			tx.Commit()
			log.Debug("auto compress success")
		}
	}
}

func delPatALl(sender string, topic string, del []int64) {
	sb, dels := buildSql(del)
	tx := mysqlDb.Exec(sb.String(), dels...)
	if e := tx.Error; e != nil {
		log.Error(e.Error())
	} else {
		log.Debug(fmt.Sprintf("auto compress success, this %s topic %s all delete %v", sender, topic, dels))
	}
}

func buildSql(del []int64) (strings.Builder, []interface{}) {
	sb := strings.Builder{}
	sb.WriteString("delete from sub where id in (")
	dels := make([]interface{}, len(del))
	for j := 0; j < len(del); j++ {
		dels[j] = del[j]
		if j == len(del)-1 {
			sb.WriteString("?")
		} else {
			sb.WriteString("?,")
		}
	}
	sb.WriteString(")")
	return sb, dels
}

func getSubs(min int) []*sql.Sub {
	count := getCount()
	if count < int64(min) { // 过小就不需要处理
		return nil
	}
	subs := make([]*sql.Sub, 0)
	b2 := mysqlDb.Raw("select * from sub order by id asc limit ?", count*2/3)
	if r, er := b2.Rows(); er != nil {
		log.Warn(er.Error())
	} else {
		for r.Next() {
			sub := &sql.Sub{}
			e := b2.ScanRows(r, sub)
			if e != nil {
				log.Warn(e.Error())
				break
			}
			subs = append(subs, sub)
		}
	}
	return subs
}

func getCount() int64 {
	m := int64(0)
	b := mysqlDb.Raw("select count(*) from sub")
	if r, er := b.Rows(); er != nil {
		log.Warn(er.Error())
	} else {
		for r.Next() {
			e := b.ScanRows(r, &m)
			if e != nil {
				log.Warn(e.Error())
			}
			break
		}
	}
	return m
}
