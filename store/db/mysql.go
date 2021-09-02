package mySql

import (
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/model"
	"gitee.com/Ljolan/si-mqtt/store/po"
	"gorm.io/driver/mysql"
	_ "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 获取大于当前msgID的消息
type mysqlSessionStore struct {
	client *gorm.DB
}

func (m *mysqlSessionStore) Start(config config.SIConfig) error {
	var err error
	m.client, err = gorm.Open(mysql.New(mysql.Config{
		DriverName: "mysql",
		DSN:        "gorm:gorm@tcp(localhost:3306)/sq?charset=utf8mb4&parseTime=True&loc=Local",
	}))
	return err
}

func (m *mysqlSessionStore) Stop() error {
	return nil
}

func (m *mysqlSessionStore) GetSession(clientId string) (model.Session, error) {
	session := po.Session{ClientId: clientId}
	if err := m.client.First(&session).Error; err != nil {
		return model.Session{}, err
	}
	return model.NewSession(clientId, model.Status(session.Status), session.OfflineTime), nil
}

func (m *mysqlSessionStore) StoreSession(clientId string, session model.Session) error {
	if err := m.client.Save(&po.Session{
		ClientId:    clientId,
		Status:      uint8(session.Status()),
		OfflineTime: session.OfflineTime(),
	}).Error; err != nil {
		return err
	}
	return nil
}

func (m *mysqlSessionStore) ClearSession(clientId string, clearOfflineMsg bool) (e error) {
	c, err := m.client.DB()
	if err != nil {
		return err
	}
	tx, err := c.Begin()
	if err != nil {
		return err
	}
	defer func() {
		// TODO 错误处理
		if e != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	_, err = tx.Query("delete from session where client_id=?", clientId)
	if err != nil {
		return err
	}
	if clearOfflineMsg {
		_, err = tx.Query("delete from session where client_id=?", clientId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *mysqlSessionStore) StoreSubscription(clientId string, subscription model.Subscription) error {
	panic("implement me")
}

func (m *mysqlSessionStore) DelSubscription(client, topic string) error {
	panic("implement me")
}

func (m *mysqlSessionStore) ClearSubscription(clientId string) error {
	panic("implement me")
}

func (m *mysqlSessionStore) GetSubscriptions(clientId string) ([]model.Subscription, error) {
	panic("implement me")
}

func (m *mysqlSessionStore) CacheInflowMsg(clientId string, message messagev5.Message) error {
	panic("implement me")
}

func (m *mysqlSessionStore) ReleaseInflowMsg(clientId string, msgId int64) (messagev5.Message, error) {
	panic("implement me")
}

func (m *mysqlSessionStore) GetAllInflowMsg(clientId string) ([]messagev5.Message, error) {
	panic("implement me")
}

func (m *mysqlSessionStore) CacheOutflowMsg(client string, message messagev5.Message) error {
	panic("implement me")
}

func (m *mysqlSessionStore) GetAllOutflowMsg(clientId string) (messagev5.Message, error) {
	panic("implement me")
}

func (m *mysqlSessionStore) ReleaseOutflowMsg(clientId string, msgId int64) (messagev5.Message, error) {
	panic("implement me")
}

func (m *mysqlSessionStore) CacheOutflowSecMsgId(clientId string, msgId int64) error {
	panic("implement me")
}

func (m *mysqlSessionStore) GetAllOutflowSecMsg(clientId string) ([]int64, error) {
	panic("implement me")
}

func (m *mysqlSessionStore) ReleaseOutflowSecMsgId(clientId string, msgId int64) error {
	panic("implement me")
}

func (m *mysqlSessionStore) StoreOfflineMsg(clientId string, message messagev5.Message) error {
	panic("implement me")
}

func (m *mysqlSessionStore) GetAllOfflineMsg(clientId string) ([]messagev5.Message, error) {
	panic("implement me")
}

func (m *mysqlSessionStore) ClearOfflineMsgs(clientId string) error {
	panic("implement me")
}

func (m *mysqlSessionStore) ClearOfflineMsgById(clientId string, msgIds []int64) error {
	panic("implement me")
}
