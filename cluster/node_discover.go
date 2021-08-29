package cluster

import (
	"errors"
	"sync"
)

type Node struct {
	NNA  string // 节点名称
	Addr string // 节点addr
}

var (
	NodeNoExist = errors.New("node no exist")
)

// NodeDiscover 集群节点发现
type NodeDiscover interface {
	GetNodes() ([]Node, error)
	GetNodeMap() map[string]Node
	GetNode(name string) (Node, error)
	RegisterMe(node Node) error
	RemoveNode(name string) error
}

type nodeDiscover struct {
	sync.RWMutex
	nodes map[string]Node
}

func NewStaticNodeDiscover(nodes map[string]Node) NodeDiscover {
	return &nodeDiscover{nodes: nodes}
}
func (n *nodeDiscover) GetNodes() ([]Node, error) {
	n.RLock()
	ret := make([]Node, len(n.nodes))
	i := 0
	for _, node := range n.nodes {
		n1 := node
		ret[i] = n1
		i++
	}
	n.RUnlock()
	return ret, nil
}
func (n *nodeDiscover) GetNodeMap() map[string]Node {
	n.RLock()
	ret := make(map[string]Node)
	for k, node := range n.nodes {
		ret[k] = node
	}
	n.RUnlock()
	return ret
}

func (n *nodeDiscover) GetNode(name string) (Node, error) {
	n.RLock()
	if node, ok := n.nodes[name]; ok {
		return node, nil
	}
	n.RUnlock()
	return Node{}, NodeNoExist
}
func (n *nodeDiscover) RemoveNode(name string) error {
	n.Lock()
	delete(n.nodes, name)
	n.Unlock()
	return nil
}
func (n *nodeDiscover) RegisterMe(node Node) error {
	n.Lock()
	n.nodes[node.NNA] = node
	n.Unlock()
	return nil
}
