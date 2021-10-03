package repo

import (
	"context"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	orm2 "gitee.com/Ljolan/si-mqtt/cluster/store/orm"
	mongoorm2 "gitee.com/Ljolan/si-mqtt/cluster/store/orm/mongo"
	po2 "gitee.com/Ljolan/si-mqtt/cluster/store/po"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5/impl"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	"time"
)

type SessionRepo struct {
	c orm2.SiOrm
}

func NewSessionStore() store2.SessionStore {
	return &SessionRepo{}
}
func (s *SessionRepo) Start(ctx context.Context, config config.SIConfig) error {
	var err error
	storeCfg := config.Store.Mongo
	s.c, err = mongoorm2.NewMongoOrm(storeCfg.Source, storeCfg.MinPoolSize, storeCfg.MaxPoolSize, storeCfg.MaxConnIdleTime)
	return err
}

func (s *SessionRepo) Stop(ctx context.Context) error {
	return nil
}

func (s *SessionRepo) GetSession(ctx context.Context, clientId string) (sessionsv5.Session, error) {
	defer func() {
		logger.Logger.Debugf("【GetSession <==】%s", clientId)
	}()
	data := make([]po2.Session, 0)
	err := s.c.Get(ctx, "si_session", orm2.Select{"_id": clientId}, &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	return poToVoSession(data[0]), err
}

func (s *SessionRepo) StoreSession(ctx context.Context, clientId string, session sessionsv5.Session) error {
	defer func() {
		logger.Logger.Debugf("【StoreSession ==>】%s", clientId)
	}()
	err := s.c.Save(ctx, "si_session", clientId, voToPoSession(clientId, session))
	if err != nil {
		return err
	}
	if will := session.Will(); will != nil {
		return s.c.Save(ctx, "si_will", clientId, voToPo(clientId, will))
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
	err = s.c.Delete(ctx, "si_session", orm2.Select{"_id": clientId})
	if err != nil {
		return err
	}
	return s.c.Delete(ctx, "si_will", orm2.Select{"_id": clientId})
}

func (s *SessionRepo) StoreSubscription(ctx context.Context, clientId string, subscription *messagev5.SubscribeMessage) error {
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
	return s.c.Delete(ctx, "si_sub", orm2.Select{"client_id": clientId, "topic": topic})
}

func (s *SessionRepo) ClearSubscriptions(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ClearSubscriptions ==X】%s", clientId)
	}()
	return s.c.Delete(ctx, "si_sub", orm2.Select{"client_id": clientId})
}

func (s *SessionRepo) GetSubscriptions(ctx context.Context, clientId string) ([]*messagev5.SubscribeMessage, error) {
	defer func() {
		logger.Logger.Debugf("【GetSubscriptions <==】%s", clientId)
	}()
	data := make([]po2.Subscription, 0)
	err := s.c.Get(ctx, "si_sub", orm2.Select{"client_id": clientId}, &data)
	if err != nil {
		return nil, err
	}
	ret := make([]*messagev5.SubscribeMessage, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVoSub(data[i])
	}
	return ret, err
}

func (s *SessionRepo) CacheInflowMsg(ctx context.Context, clientId string, message messagev5.Message) error {
	defer func() {
		logger.Logger.Debugf("【CacheInflowMsg ==>】%s", clientId)
	}()
	return s.c.Save(ctx, "si_inflow", "", voToPo(clientId, message.(*messagev5.PublishMessage)))
}
func (s *SessionRepo) ReleaseAllInflowMsg(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseAllInflowMsg ==X】%s ", clientId)
	}()
	filter := orm2.Select{"client_id": clientId}
	return s.c.Delete(ctx, "si_inflow", filter)
}
func (s *SessionRepo) ReleaseInflowMsgs(ctx context.Context, clientId string, pkId []uint16) error {
	defer func() {
		logger.Logger.Infof("【ReleaseInflowMsgs ==X】%s, pk_id: %v", clientId, pkId)
	}()
	err := s.c.Delete(ctx, "si_inflow", orm2.Select{"client_id": clientId, "pk_id": orm2.Select{"$in": pkId}})
	if err != nil {
		return err
	}
	return nil
}
func (s *SessionRepo) ReleaseInflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev5.Message, error) {
	defer func() {
		logger.Logger.Debugf("【ReleaseInflowMsg ==X】%s, pk_id: %v", clientId, pkId)
	}()
	ms := &po2.Message{}
	filter := orm2.Select{"client_id": clientId, "pk_id": pkId}
	err := s.c.GetAndDelete(ctx, "si_inflow", filter, ms)
	if err != nil {
		return nil, err
	}
	return poToVo(*ms), nil
}

func (s *SessionRepo) GetAllInflowMsg(ctx context.Context, clientId string) (t []messagev5.Message, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllInflowMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po2.Message, 0)
	err := s.c.Get(ctx, "si_inflow", orm2.Select{"client_id": clientId}, &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]messagev5.Message, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVo(data[i])
	}
	return ret, nil
}

func (s *SessionRepo) CacheOutflowMsg(ctx context.Context, clientId string, message messagev5.Message) error {
	defer func() {
		logger.Logger.Debugf("【CacheOutflowMsg ==>】%s", clientId)
	}()
	return s.c.Save(ctx, "si_outflow", "", voToPo(clientId, message.(*messagev5.PublishMessage)))
}

func (s *SessionRepo) GetAllOutflowMsg(ctx context.Context, clientId string) (t []messagev5.Message, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllOutflowMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po2.Message, 0)
	err := s.c.Get(ctx, "si_outflow", orm2.Select{"client_id": clientId}, &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]messagev5.Message, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVo(data[i])
	}
	return ret, nil
}
func (s *SessionRepo) ReleaseAllOutflowMsg(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseAllOutflowMsg ==X】%s", clientId)
	}()
	filter := orm2.Select{"client_id": clientId}
	err := s.c.Delete(ctx, "si_outflow", filter)
	if err != nil {
		return err
	}
	return nil
}
func (s *SessionRepo) ReleaseOutflowMsg(ctx context.Context, clientId string, pkId uint16) (messagev5.Message, error) {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowMsg ==X】%s, pk_id: %v", clientId, pkId)
	}()
	ms := &po2.Message{}
	filter := orm2.Select{"client_id": clientId, "pk_id": pkId}
	err := s.c.GetAndDelete(ctx, "si_outflow", filter, ms)
	if err != nil {
		return nil, err
	}
	return poToVo(*ms), nil
}
func (s *SessionRepo) ReleaseOutflowMsgs(ctx context.Context, clientId string, pkId []uint16) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowMsgs ==X】%s, pk_id: %v", clientId, pkId)
	}()
	err := s.c.Delete(ctx, "si_outflow", orm2.Select{"client_id": clientId, "pk_id": orm2.Select{"$in": pkId}})
	if err != nil {
		return err
	}
	return nil
}
func (s *SessionRepo) CacheOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	defer func() {
		logger.Logger.Debugf("【CacheOutflowSecMsgId ==>】%s, pk_id: %v", clientId, pkId)
	}()
	return s.c.Save(ctx, "si_outflowsec", "", po2.MessagePk{
		ClientId: clientId,
		PkId:     pkId,
		OpTime:   time.Now().UnixNano(),
	})
}

func (s *SessionRepo) GetAllOutflowSecMsg(ctx context.Context, clientId string) (t []uint16, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllOutflowSecMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po2.MessagePk, 0)
	err := s.c.Get(ctx, "si_outflowsec", orm2.Select{"client_id": clientId}, &data)
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
	filter := orm2.Select{"client_id": clientId}
	return s.c.Delete(ctx, "si_outflowsec", filter)
}
func (s *SessionRepo) ReleaseOutflowSecMsgId(ctx context.Context, clientId string, pkId uint16) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowSecMsgId ==X】%s, pk_id: %v", clientId, pkId)
	}()
	filter := orm2.Select{"client_id": clientId, "pk_id": pkId}
	return s.c.Delete(ctx, "si_outflowsec", filter)
}
func (s *SessionRepo) ReleaseOutflowSecMsgIds(ctx context.Context, clientId string, pkId []uint16) error {
	defer func() {
		logger.Logger.Debugf("【ReleaseOutflowSecMsgIds ==X】%s, pk_id: %v", clientId, pkId)
	}()
	return s.c.Delete(ctx, "si_outflowsec", orm2.Select{"client_id": clientId, "pk_id": orm2.Select{"$in": pkId}})
}

func (s *SessionRepo) StoreOfflineMsg(ctx context.Context, clientId string, message messagev5.Message) error {
	defer func() {
		logger.Logger.Debugf("【StoreOfflineMsg ==>】%s", clientId)
	}()
	msg := voToPo(clientId, message.(*messagev5.PublishMessage))
	msg.MsgId = utils.Generate()
	return s.c.Save(ctx, "si_offline", "", msg)
}

// 返回离线消息，和对应的消息id
func (s *SessionRepo) GetAllOfflineMsg(ctx context.Context, clientId string) (t []messagev5.Message, mi []string, e error) {
	defer func() {
		logger.Logger.Debugf("【GetAllOfflineMsg <==】%s, size: %v", clientId, len(t))
	}()
	data := make([]po2.Message, 0)
	err := s.c.Get(ctx, "si_offline", orm2.Select{"client_id": clientId}, &data)
	if err != nil || len(data) == 0 {
		return nil, nil, err
	}
	ret := make([]messagev5.Message, len(data))
	msgId := make([]string, len(data))
	for i := 0; i < len(data); i++ {
		ret[i] = poToVo(data[i])
		msgId[i] = data[i].MsgId
	}
	return ret, msgId, nil
}

func (s *SessionRepo) ClearOfflineMsgs(ctx context.Context, clientId string) error {
	defer func() {
		logger.Logger.Debugf("【ClearOfflineMsgs ==X】%s", clientId)
	}()
	return s.c.Delete(ctx, "si_offline", orm2.Select{"client_id": clientId})
}

func (s *SessionRepo) ClearOfflineMsgById(ctx context.Context, clientId string, msgIds []string) error {
	defer func() {
		logger.Logger.Debugf("【ClearOfflineMsgById ==X】%s, msgIds：%v", clientId, msgIds)
	}()
	return s.c.Delete(ctx, "si_offline", orm2.Select{"client_id": clientId, "msg_id": orm2.Select{"$in": msgIds}})
}

func voToPoSession(clientId string, session sessionsv5.Session) po2.Session {
	return po2.Session{
		ClientId:           clientId,
		Status:             session.Status(),
		ExpiryInterval:     session.ExpiryInterval(),
		ReceiveMaximum:     session.ReceiveMaximum(),
		MaxPacketSize:      session.MaxPacketSize(),
		TopicAliasMax:      session.TopicAliasMax(),
		RequestRespInfo:    session.RequestRespInfo(),
		RequestProblemInfo: session.RequestProblemInfo(),
		UserProperty:       session.UserProperty(),
		OfflineTime:        session.OfflineTime(),
	}
}
func voToPoSub(clientId string, sub *messagev5.SubscribeMessage) ([]map[string]interface{}, []interface{}) {
	ret := make([]interface{}, 0)
	qos := sub.Qos()
	top := sub.Topics()
	sc := make([]map[string]interface{}, len(qos))
	for i := 0; i < len(qos); i++ {
		tp := string(top[i])
		ret = append(ret, po2.Subscription{
			ClientId:        clientId,
			Qos:             qos[i],
			Topic:           tp,
			SubId:           sub.SubscriptionIdentifier(),
			NoLocal:         sub.TopicNoLocal(top[i]),
			RetainAsPublish: sub.TopicRetainAsPublished(top[i]),
			RetainHandling:  uint8(sub.TopicRetainHandling(top[i])),
		})
		p := make(map[string]interface{})
		p["client_id"] = clientId
		p["topic"] = tp
		sc[i] = p
	}
	return sc, ret
}
func poToVoSession(session po2.Session) sessionsv5.Session {
	sessionRet := impl.NewMemSession(session.ClientId)
	sessionRet.SetClientId(session.ClientId)
	sessionRet.SetStatus(session.Status)
	sessionRet.SetExpiryInterval(session.ExpiryInterval)
	sessionRet.SetReceiveMaximum(session.ReceiveMaximum)
	sessionRet.SetMaxPacketSize(session.MaxPacketSize)
	sessionRet.SetTopicAliasMax(session.TopicAliasMax)
	sessionRet.SetRequestRespInfo(session.RequestRespInfo)
	sessionRet.SetRequestProblemInfo(session.RequestProblemInfo)
	sessionRet.SetUserProperty(session.UserProperty)
	sessionRet.SetOfflineTime(session.OfflineTime)
	return sessionRet
}

// 这里可以批量转为一条sub消息
func poToVoSub(subscription po2.Subscription) *messagev5.SubscribeMessage {
	sub := messagev5.NewSubscribeMessage()
	_ = sub.AddTopicAll([]byte(subscription.Topic), subscription.Qos, subscription.NoLocal, subscription.RetainAsPublish, subscription.RetainHandling)
	return sub
}
