package model

type Status uint8

const (
	_       Status = iota
	NULL           // 从未连接过（之前 cleanStart为1 的也为NULL）
	ONLINE         // 在线
	OFFLINE        // cleanStart为0，且连接过mqtt集群，已离线，会返回offlineTime（离线时间）
)

type Session struct {
	clientId    string
	status      Status
	offlineTime int64
}

func (s *Session) ClientId() string {
	return s.clientId
}

func (s *Session) SetClientId(clientId string) {
	s.clientId = clientId
}

func (s *Session) Status() Status {
	return s.status
}

func (s *Session) SetStatus(status Status) {
	s.status = status
}

func (s *Session) OfflineTime() int64 {
	return s.offlineTime
}

func (s *Session) SetOfflineTime(offlineTime int64) {
	s.offlineTime = offlineTime
}

func NewSession(clientId string, status Status, offlineTime int64) Session {
	return Session{clientId: clientId, status: status, offlineTime: offlineTime}
}
