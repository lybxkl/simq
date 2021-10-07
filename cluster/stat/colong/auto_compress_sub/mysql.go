package autocompress

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
	"strings"
	"time"
)

type Lock struct {
	Id     int    `gorm:"primaryKey"`
	Source string `gorm:"column:source;uniqueIndex:source;not null;type:varchar(50)"`
	Belong string `gorm:"column:belong;not null;type:varchar(50)"`
	Desc   string `gorm:"column:desc;type:varchar(100)"`
	Time   int64  `gorm:"column:time"`
}

func (receiver Lock) TableName() string {
	return "si_lock"
}

type mysqlCompress struct {
	db       *gorm.DB
	nodeName string
	min      int
	period   int
	source   string
	dupErr   string
}

func NewAutoCompress(nodeName, url string) AutoCompress {
	cfg := &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 glog.Default.LogMode(glog.Silent),
		PrepareStmt:            true,
		DisableAutomaticPing:   true,
	}
	db, err := gorm.Open(mysql.Open(url), cfg)
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(&Lock{})
	sql, _ := db.DB()
	sql.SetMaxOpenConns(10)
	return &mysqlCompress{
		db:       db,
		nodeName: nodeName,
		source:   "auto_compress_sub", //这个改了下面的错误频断也需要改动的
		dupErr:   "Error 1062: Duplicate entry 'auto_compress_sub' for key 'source'",
	}
}
func (m *mysqlCompress) Lock() {
	// 加分布式锁
LOCK:
	// 删除超时锁
	sql1 := "delete from si_lock where `source`=? and ? - `time` > ?"
	db1 := m.db.Exec(sql1, m.source, time.Now().Unix(), m.period*2)
	if e := db1.Error; e != nil {
		log.Error(fmt.Sprintf("超时删除错误： %v", e.Error()))
		time.Sleep(time.Duration(m.period*2) * time.Second)
		goto LOCK
	}
	if db1.RowsAffected == 1 {
		log.Debug(fmt.Sprintf("超时删除影响行数正常： %v\n", db1.RowsAffected))
	} else if db1.RowsAffected > 1 {
		log.Warn(fmt.Sprintf("删除影响行数异常： %v\n", db1.RowsAffected))
	}
	// 如果上面锁没有超时，则此会插入失败的
	sql2 := "insert into si_lock(`source`,`belong`,`desc`,`time`) values(?,?,'集群共享订阅事件合并排他锁',?)"
	if e := m.db.Exec(sql2, m.source, m.nodeName, time.Now().Unix()).Error; e != nil {
		if !strings.Contains(e.Error(), m.dupErr) {
			log.Error(fmt.Sprintf("抢锁异常失败： %v", e.Error()))
		} else {
			log.Debug(fmt.Sprintf("抢锁失败： %v", e.Error()))
		}
		time.Sleep(time.Duration(m.period*2) * time.Second)
		goto LOCK
	}
}

func (m *mysqlCompress) LockAndLive() bool {
	// 锁续命
	sql3 := "update si_lock set `time` = ? where `source` = ? and `belong`= ?"
	db3 := m.db.Exec(sql3, time.Now().Unix(), m.source, m.nodeName)
	if e := db3.Error; e != nil {
		log.Error(fmt.Sprintf("给锁续命失败，丢失锁，等待重新抢锁：%v", e.Error()))
		time.Sleep(time.Duration(m.period*2) * time.Second)
		return false
	}
	if db3.RowsAffected == 1 {
		log.Debug(fmt.Sprintf("续命成功，影响行数正常： %v", db3.RowsAffected))
		return true
	} else if db3.RowsAffected > 1 {
		log.Warn(fmt.Sprintf("续命异常，影响行数异常： %v", db3.RowsAffected))
	} else {
		log.Warn(fmt.Sprintf("续命失败，影响行数异常： %v", db3.RowsAffected))
	}
	time.Sleep(time.Duration(m.period*2) * time.Second)
	return false
}
func (m *mysqlCompress) buildSql(del []int64) (strings.Builder, []interface{}) {
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

func (m *mysqlCompress) getCount() int64 {
	c := int64(0)
	b := m.db.Raw("select count(*) from sub")
	if r, er := b.Rows(); er != nil {
		log.Warn(er.Error())
	} else {
		for r.Next() {
			e := b.ScanRows(r, &c)
			if e != nil {
				log.Warn(e.Error())
			}
			break
		}
	}
	return c
}
func (m *mysqlCompress) GetSubs(min int) ([]*Sub, error) {
	count := m.getCount()
	if count < int64(min) { // 过小就不需要处理
		return nil, nil
	}
	subs := make([]*Sub, 0)
	b2 := m.db.Raw("select * from sub order by id asc limit ?", count*2/3)
	if r, er := b2.Rows(); er != nil {
		return nil, er
	} else {
		for r.Next() {
			sub := &Sub{}
			e := b2.ScanRows(r, sub)
			if e != nil {
				return nil, e
			}
			subs = append(subs, sub)
		}
	}
	return subs, nil
}

func (m *mysqlCompress) DelSome(id int64, num int32, del []int64) error {
	sb, dels := m.buildSql(del)
	tx := m.db.Begin()
	if e := tx.Exec("update sub set num = ? where id = ?", uint32(num), id).Error; e != nil {
		tx.Rollback()
		return e
	} else {
		if e = tx.Exec(sb.String(), dels...).Error; e != nil {
			tx.Rollback()
			return e
		} else {
			tx.Commit()
			log.Debug("auto compress success")
		}
	}
	return nil
}

func (m *mysqlCompress) DelPatALl(sender string, topic string, del []int64) error {
	sb, dels := m.buildSql(del)
	tx := m.db.Exec(sb.String(), dels...)
	if e := tx.Error; e != nil {
		return e
	} else {
		log.Debug(fmt.Sprintf("auto compress success, this %s topic %s all delete %v", sender, topic, dels))
	}
	return nil
}
