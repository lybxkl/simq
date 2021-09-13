package common

import "time"

type EventType int

const (
	_ EventType = iota
	Connect
	Sub
	UnSub
	DisConnect
)

func NewEvent(node, client string, tp EventType) Event {
	return Event{
		EventType: tp,
		Node:      node,
		Client:    client,
		Time:      time.Now().UnixNano(),
	}
}
func (e *Event) SubUnSub(topic string, qos uint8) {
	e.Topic = topic
	e.Qos = qos
}

type Event struct {
	EventType `bson:"event"`
	Node      string `bson:"node"`
	Client    string `bson:"client"`
	Topic     string `bson:"topic,omitempty"`
	Qos       uint8  `bson:"qos,omitempty"`
	Time      int64  `bson:"time"`
}
