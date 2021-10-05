package mysql

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/orm"
	"gorm.io/gorm/logger"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type mysqlOrmImpl struct {
	db *gorm.DB
}

func NewMysqlOrm(url string, maxConn int) (orm.SiOrm, error) {
	cfg := &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Error),
		PrepareStmt:            true,
		DisableAutomaticPing:   true,
	}
	db, err := gorm.Open(mysql.Open(url), cfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(maxConn)
	return &mysqlOrmImpl{
		db: db,
	}, nil
}
func (m *mysqlOrmImpl) AutoMigrate(dest interface{}) error {
	return m.db.AutoMigrate(dest)
}
func (m *mysqlOrmImpl) Save(ctx context.Context, tab string, key string, value string, msg interface{}) error {
	if key == "" {
		return m.db.Table(tab).Create(msg).Error
	}
	if value == "" {
		return m.db.Table(tab).Save(msg).Error
	}
	return m.db.Table(tab).Where(key+"= ?", value).Save(msg).Error
}

func (m *mysqlOrmImpl) SaveMany(ctx context.Context, tab string, k []string, msg []interface{}) error {
	b := m.db.Table(tab).Begin()
	for i := 0; i < len(k); i++ {
		err := m.db.Table(tab).Where(k[i]).Save(msg[i]).Error
		if err != nil {
			b.Rollback()
			return err
		}
	}
	b.Commit()
	return nil
}

func (m *mysqlOrmImpl) Get(ctx context.Context, tab string, where string, decoder interface{}) error {
	return m.db.Table(tab).Where(where).Find(decoder).Error
}

func (m *mysqlOrmImpl) Delete(ctx context.Context, tab string, where string, value interface{}) error {
	return m.db.Table(tab).Where(where).Delete(value).Error
}

func (m *mysqlOrmImpl) GetAndDelete(ctx context.Context, tab string, where string, decoder interface{}, value interface{}) error {
	b := m.db.Table(tab).Where(where)
	if e := b.Find(decoder).Error; e != nil {
		return e
	}
	return b.Delete(value).Error
}
