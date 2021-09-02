package model

type Subscription struct {
	clientId string
	qos int
	topic string
}
func NewSubscription(clientId string, qos int, topic string) *Subscription {
	return &Subscription{clientId: clientId, qos: qos, topic: topic}
}
func (s *Subscription) ClientId() string {
	return s.clientId
}

func (s *Subscription) SetClientId(clientId string) {
	s.clientId = clientId
}

func (s *Subscription) Qos() int {
	return s.qos
}

func (s *Subscription) SetQos(qos int) {
	s.qos = qos
}

func (s *Subscription) Topic() string {
	return s.topic
}

func (s *Subscription) SetTopic(topic string) {
	s.topic = topic
}



