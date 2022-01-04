package mysqlrepo

import (
	"context"
	"encoding/hex"
	"fmt"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/orm"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/orm/mysql"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/po"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions/impl"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	"strings"
	"time"
)

type SessionRepo struct {
	c orm.SiOrm
}

func NewSessionStore() store2.SessionStore {
	return &SessionRepo{}
}
func (s *SessionRepo) Start(ctx context.Context, config *config.SIConfig) error {
	var err error
	storeCfg := config.Store.Mysql
	s.c, err = mysql.NewMysqlOrm(storeCfg.Source, storeCfg.PoolSize)
	utils.MustPanic(s.c.AutoMigrate(&po.Will{}))
	utils.MustPanic(s.c.AutoMigrate(&po.Retain{}))
	utils.MustPanic(s.c.AutoMigrate(&po.Inflow{}))
	utils.MustPanic(s.c.AutoMigrate(&po.Outflow{}))
	utils.MustPanic(s.c.AutoMigrate(&po.MessagePk{}))
	utils.MustPanic(s.c.AutoMigrate(&po.Offline{}))
	utils.MustPanic(s.c.AutoMigrate(&po.Session{}))
	utils.MustPanic(s.c.AutoMigrate(&po.Subscription{}))
	return err
}

func (s *SessionRepo) Stop(ctx context.Context) error {
	return nil
}

func (s *SessionRepo) GetSession(ctx context.Context, clientId string) (sessions.Session, error) {
	defer func() {
		logger.Logger.Debugf("【GetSession <==】%s", clientId)
	}()
	data := make([]po.Session, 0)
	err := s.c.Get(ctx, "si_session", wrapperOne("client_id", clientId), &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	return poToVoSession(data[0]), err
}

func (s *SessionRepo) StoreSession(ctx context.Context, clientId string, session sessions.Session) error {
	defer func() {
		logger.Logger.Debugf("【StoreSession ==>】%s", clientId)
	}()
	err := s.c.Save(ctx, "si_session", "client_id", "", voToPoSession(clientId, session))
	if err != nil {
		return err
	}
	if will := session.Will(); will != nil {
		return s.c.Save(ctx, "si_will", "client_id", "", voToPoWill(clientId, will))
	}
	return nil
}

// ClearSession todo 事务
func (s *SessionRepo) ClearSession(ctx context.Context, clientId string, clearOfflineMsg bool) error {
	defer func() {
		logger.Logger.Debugf("【ClearSession ==X】%s, clearOfflineMsg: %v", clientId, clearOfflineMsg)
	}()
	if clearOfflineMsg {
		err := s.ClearOfflineMsgs(ctx, clientId)
		if err != nil {
			return err
		}
		// 清理过程消息
		err = s.ReleaseAllOutflowMsg(ctx, clientId)
		if err != nil {
			return err
		}
		err = s.ReleaseAllInflowMsg(ctx, clientId)
		if err != nil {
			return err
		}
		err = s.ReleaseAllOutflowSecMsg(ctx, clientId)
		if err != nil {
			return err
		}
	}
	err := s.ClearSubscriptions(ctx, clientId)
	if err != nil {
		return err
	}
	err = s.c.Delete(ctx, "si_session", wrapperOne("client_id", clientId), &po.Session{})
	if err != nil {
		return err
	}
	return s.c.Delete(ctx, "si_will", wrapperOne("client_id", clientId), po.Will{})
}

func (s *SessionRepo) StoreSubscription(ctx context.Context, clientId string, subscription *messagev2.SubscribeMessage) error {
	defer func() {
		logger.Logger.Debugf("【StoreSubscription ==>】%s", clientId)
	}()
	sc, sub := voToPoSub(clientId, subscription)
	return s.c.SaveMany(ctx, "si_sub", sc, sub)
}

func (s *SessionRepo) DelSubscription(ctx context.Context, clientId, topic string) error {
	defer func() {
		logger.Logger.Debugf("【DelSubscription ==X】%s, topic: %v", clientId, topic)
	}()
	return s.c.Delete(ctx, "si_sub", fmt.Sprintf("client_id = '%s' and topic = '%s'", clientId, topic), &po.Subscription{})
}

func (s *SessionRepo) ClearSubscriptions(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ClearSubscriptions ==X】%s", clientId)
	}()
	return s.c.Delete(ctx, "si_sub", wrapperOne("client_id", clientId), &po.Subscription{})
}

func (s *SessionRepo) GetSubscriptions(ctx context.Context, clientId string) ([]*messagev2.SubscribeMessage, error) {
	defer func() {
		logger.Logger.Debugf("【GetSubscriptions <==】%s", clientId)
	}()
	data := make([]po.Subscription, 0)
	err := s.c.Get(ctx, "si_sub", wrapperOne("client_id", clientId), &data)
	if err != nil {
		return nil, err
	}
	ret := make([]*messagev2.SubscribeMessage, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVoSub(data[i])
	}
	return ret, err
}

func (s *SessionRepo) CacheInflowMsg(ctx context.Context, clientId string, message messagev2.Message) error {
	defer func() {
		logger.Logger.Debugf("【CacheInflowMsg ==>】%s", clientId)
	}()
	return s.c.Save(ctx, "si_inflow", "", "", voToPoInflow(clientId, message.(*messagev2.PublishMessage)))
}
func (s *SessionRepo) ReleaseAllInflowMsg(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseAllInflowMsg ==X】%s ", clientId)
	}()
	return s.c.Delete(ctx, "si_inflow", wrapperOne("client_id", clientId), &po.Inflow{})
}
func (s *SessionRepo) ReleaseInflowMsgs(ctx context.Context, clientId string, pkId []uint16) error {
	defer func() {
		logger.Logger.Infof("【ReleaseInflowMsgs ==X】%s, pk_id: %v", clientId, pkId)
	}()
	f := inPks(clientId, pkId)
	err := s.c.Delete(ctx, "si_inflow", f, &po.Inflow{})
	if err != nil {
		return err
	}
	return nil
}

func (s *SessionRepo) ReleaseInflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev2.Message, error) {
	defer func() {
		logger.Logger.Debugf("【ReleaseInflowMsg ==X】%s, pk_id: %v", clientId, pkId)
	}()
	ms := &po.Inflow{}
	err := s.c.GetAndDelete(ctx, "si_inflow", fmt.Sprintf("client_id = '%s' and pk_id = %v", clientId, pkId), ms, &po.Inflow{})
	if err != nil {
		return nil, err
	}
	return poToVoInflow(*ms), nil
}

func (s *SessionRepo) GetAllInflowMsg(ctx context.Context, clientId string) (t []messagev2.Message, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllInflowMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po.Inflow, 0)
	err := s.c.Get(ctx, "si_inflow", wrapperOne("client_id", clientId), &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]messagev2.Message, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVoInflow(data[i])
	}
	return ret, nil
}

func (s *SessionRepo) CacheOutflowMsg(ctx context.Context, clientId string, message messagev2.Message) error {
	defer func() {
		logger.Logger.Debugf("【CacheOutflowMsg ==>】%s", clientId)
	}()
	return s.c.Save(ctx, "si_outflow", "", "", voToPoOutflow(clientId, message.(*messagev2.PublishMessage)))
}

func (s *SessionRepo) GetAllOutflowMsg(ctx context.Context, clientId string) (t []messagev2.Message, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllOutflowMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po.Outflow, 0)
	err := s.c.Get(ctx, "si_outflow", wrapperOne("client_id", clientId), &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]messagev2.Message, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVoOutflow(data[i])
	}
	return ret, nil
}
func (s *SessionRepo) ReleaseAllOutflowMsg(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseAllOutflowMsg ==X】%s", clientId)
	}()
	err := s.c.Delete(ctx, "si_outflow", wrapperOne("client_id", clientId), &po.Outflow{})
	if err != nil {
		return err
	}
	return nil
}
func (s *SessionRepo) ReleaseOutflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev2.Message, error) {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowMsg ==X】%s, pk_id: %v", clientId, pkId)
	}()
	ms := &po.Outflow{}
	err := s.c.GetAndDelete(ctx, "si_outflow", fmt.Sprintf("client_id = '%s' and pk_id = %v", clientId, pkId), ms, &po.Outflow{})
	if err != nil {
		return nil, err
	}
	return poToVoOutflow(*ms), nil
}
func (s *SessionRepo) ReleaseOutflowMsgs(ctx context.Context, clientId string, pkId []uint16) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowMsgs ==X】%s, pk_id: %v", clientId, pkId)
	}()
	f := inPks(clientId, pkId)
	err := s.c.Delete(ctx, "si_outflow", f, &po.Outflow{})
	if err != nil {
		return err
	}
	return nil
}
func (s *SessionRepo) CacheOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	defer func() {
		logger.Logger.Debugf("【CacheOutflowSecMsgId ==>】%s, pk_id: %v", clientId, pkId)
	}()
	return s.c.Save(ctx, "si_outflowsec", "", "", po.MessagePk{
		ClientId: clientId,
		PkId:     pkId,
		OpTime:   time.Now().UnixNano(),
	})
}

func (s *SessionRepo) GetAllOutflowSecMsg(ctx context.Context, clientId string) (t []uint16, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllOutflowSecMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po.MessagePk, 0)
	err := s.c.Get(ctx, "si_outflowsec", wrapperOne("client_id", clientId), &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]uint16, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = data[i].PkId
	}
	return ret, nil
}
func (s *SessionRepo) ReleaseAllOutflowSecMsg(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseAllOutflowSecMsg ==X】%s", clientId)
	}()
	return s.c.Delete(ctx, "si_outflowsec", wrapperOne("client_id", clientId), &po.MessagePk{})
}
func (s *SessionRepo) ReleaseOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowSecMsgId ==X】%s, pk_id: %v", clientId, pkId)
	}()

	return s.c.Delete(ctx, "si_outflowsec", fmt.Sprintf("client_id = '%s' and pk_id = %v", clientId, pkId), &po.MessagePk{})
}
func (s *SessionRepo) ReleaseOutflowSecMsgIds(ctx context.Context, clientId string, pkId []uint16) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowSecMsgIds ==X】%s, pk_id: %v", clientId, pkId)
	}()
	f := inPks(clientId, pkId)
	return s.c.Delete(ctx, "si_outflowsec", f, &po.MessagePk{})
}

func (s *SessionRepo) StoreOfflineMsg(ctx context.Context, clientId string, message messagev2.Message) error {
	defer func() {
		logger.Logger.Debugf("【StoreOfflineMsg ==>】%s", clientId)
	}()
	msg := voToPoOffline(clientId, message.(*messagev2.PublishMessage))
	msg.MsgId = utils.Generate()
	return s.c.Save(ctx, "si_offline", "", "", msg)
}

// 返回离线消息，和对应的消息id
func (s *SessionRepo) GetAllOfflineMsg(ctx context.Context, clientId string) (t []messagev2.Message, mi []string, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllOfflineMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po.Offline, 0)
	err := s.c.Get(ctx, "si_offline", wrapperOne("client_id", clientId), &data)
	if err != nil || len(data) == 0 {
		return nil, nil, err
	}
	ret := make([]messagev2.Message, len(data))
	msgId := make([]string, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVoOffline(data[i])
		msgId[i] = data[i].MsgId
	}
	return ret, msgId, nil
}

func (s *SessionRepo) ClearOfflineMsgs(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ClearOfflineMsgs ==X】%s", clientId)
	}()
	return s.c.Delete(ctx, "si_offline", wrapperOne("client_id", clientId), &po.Offline{})
}

func (s *SessionRepo) ClearOfflineMsgById(ctx context.Context, clientId string, msgIds []string) error {
	defer func() {
		logger.Logger.Debugf("【ClearOfflineMsgById ==X】%s, msgIds：%v", clientId, msgIds)
	}()
	f := inMsgs(clientId, msgIds)
	return s.c.Delete(ctx, "si_offline", f, &po.Offline{})
}

func voToPoSession(clientId string, session sessions.Session) po.Session {
	up := session.UserProperty()
	var ups string
	if up != nil && len(up) >= 0 {
		for i := 0; i < len(up); i++ {
			ups += hex.EncodeToString([]byte(up[i]))
			ups += ","
		}
	}
	if len(ups) > 0 {
		ups = ups[:len(ups)-1]
	}
	return po.Session{
		ClientId:           clientId,
		Status:             session.Status(),
		ExpiryInterval:     session.ExpiryInterval(),
		ReceiveMaximum:     session.ReceiveMaximum(),
		MaxPacketSize:      session.MaxPacketSize(),
		TopicAliasMax:      session.TopicAliasMax(),
		RequestRespInfo:    session.RequestRespInfo(),
		RequestProblemInfo: session.RequestProblemInfo(),
		UserProperty:       ups,
		OfflineTime:        session.OfflineTime(),
	}
}
func voToPoSub(clientId string, sub *messagev2.SubscribeMessage) ([]string, []interface{}) {
	ret := make([]interface{}, 0)
	qos := sub.Qos()
	top := sub.Topics()
	sc := make([]string, len(qos))
	for i := 0; i < len(qos); i++ {
		tp := string(top[i])
		ret = append(ret, po.Subscription{
			ClientId:        clientId,
			Qos:             qos[i],
			Topic:           tp,
			SubId:           sub.SubscriptionIdentifier(),
			NoLocal:         sub.TopicNoLocal(top[i]),
			RetainAsPublish: sub.TopicRetainAsPublished(top[i]),
			RetainHandling:  uint8(sub.TopicRetainHandling(top[i])),
		})
		sc[i] = fmt.Sprintf("client_id = '%s' and topic = '%s'", clientId, tp)
	}
	return sc, ret
}
func poToVoSession(session po.Session) sessions.Session {
	sessionRet := impl.NewMemSession(session.ClientId)
	sessionRet.SetClientId(session.ClientId)
	sessionRet.SetStatus(session.Status)
	sessionRet.SetExpiryInterval(session.ExpiryInterval)
	sessionRet.SetReceiveMaximum(session.ReceiveMaximum)
	sessionRet.SetMaxPacketSize(session.MaxPacketSize)
	sessionRet.SetTopicAliasMax(session.TopicAliasMax)
	sessionRet.SetRequestRespInfo(session.RequestRespInfo)
	sessionRet.SetRequestProblemInfo(session.RequestProblemInfo)
	up := session.UserProperty
	if len(up) > 0 {
		upsp := strings.Split(up, ",")
		ups := make([]string, len(upsp))
		for i := 0; i < len(upsp); i++ {
			b, _ := hex.DecodeString(upsp[i])
			ups[i] = string(b)
		}
		sessionRet.SetUserProperty(ups)
	}
	sessionRet.SetOfflineTime(session.OfflineTime)
	return sessionRet
}

// 这里可以批量转为一条sub消息
func poToVoSub(subscription po.Subscription) *messagev2.SubscribeMessage {
	sub := messagev2.NewSubscribeMessage()
	_ = sub.AddTopicAll([]byte(subscription.Topic), subscription.Qos, subscription.NoLocal, subscription.RetainAsPublish, subscription.RetainHandling)
	return sub
}
