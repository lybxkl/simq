package cluster

import (
	"errors"
	topicmapnode "gitee.com/Ljolan/si-mqtt/corev5/topicsv5/topic_map_node"
)

type ShareTopicMapNode interface {
	GetShareNames(topic []byte) (map[string]string, error)
	AddTopicMapNode(topic []byte, shareName, nodeName string) error
	RemoveTopicMapNode(topic []byte, shareName, nodeName string) error
}
type shareMapImpl struct {
	topicmapnode.TopicsMapNodeProvider
}

func NewShareMap() ShareTopicMapNode {
	return &shareMapImpl{
		topicmapnode.NewTopicMapNodeProvider(),
	}
}

func (s *shareMapImpl) GetShareNames(topic []byte) (map[string]string, error) {
	sharename, nodes := make([]string, 0), make([]string, 0)
	ret := make(map[string]string)
	err := s.Subscribers(topic, &sharename, &nodes)
	if err != nil {
		return nil, err
	}
	if len(sharename) != len(nodes) {
		return nil, errors.New("get share topic map nodes error : sharename lens != nodes lens")
	}
	for i := 0; i < len(sharename); i++ {
		ret[sharename[i]] = nodes[i]
	}
	return ret, nil
}

func (s *shareMapImpl) AddTopicMapNode(topic []byte, shareName, nodeName string) error {
	return s.Subscribe(topic, shareName, nodeName)
}

func (s *shareMapImpl) RemoveTopicMapNode(topic []byte, shareName, nodeName string) error {
	return s.Unsubscribe(topic, shareName, nodeName)
}
