package topicmapnode

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/consts"
	"github.com/valyala/fastrand"
	"sync"
)

type memTopicMapNode struct {
	// Sub/unsub mutex
	smu sync.RWMutex
	// Subscription tree
	sroot *rSnode
}

func NewTopicMapNodeProvider() TopicsMapNodeProvider {
	return &memTopicMapNode{
		sroot: newRSNode(),
	}
}
func (m *memTopicMapNode) Subscribe(topic []byte, shareName, node string, num uint32) error {
	m.smu.Lock()
	err := m.sroot.sinsert(topic, shareName, node, num)
	m.smu.Unlock()
	return err
}

func (m *memTopicMapNode) Unsubscribe(topic []byte, shareName, node string) error {
	m.smu.Lock()
	err := m.sroot.sremove(topic, shareName, node)
	m.smu.Unlock()
	return err
}

func (m *memTopicMapNode) Subscribers(topic []byte, shareNames, nodes *[]string) error {
	m.smu.RLock()
	err := m.sroot.smatch(topic, shareNames, nodes)
	m.smu.RUnlock()
	return err
}

func (m *memTopicMapNode) Close() error {
	m.sroot = nil
	return nil
}

type nodeWeight struct {
	node   string
	wright uint32
}

func newNodeWeight(node string, wright uint32) *nodeWeight {
	return &nodeWeight{
		node:   node,
		wright: wright,
	}
}

// subscrition nodes
type rSnode struct {
	// If this is the end of the topic string, then add subscribers here
	tmn map[string][]*nodeWeight // shareName map nodes
	// Otherwise add the next topic level here
	rsnodes map[string]*rSnode
}

func newRSNode() *rSnode {
	return &rSnode{
		rsnodes: make(map[string]*rSnode),
		tmn:     make(map[string][]*nodeWeight),
	}
}

func (this *rSnode) sinsert(topic []byte, shareName, node string, num uint32) error {

	if len(topic) == 0 {
		var v []*nodeWeight
		var ok bool
		if v, ok = this.tmn[shareName]; ok {
			for i := range v {
				if v[i].node == node {
					v[i].wright += num
					return nil
				}
			}
		}
		v = append(v, newNodeWeight(node, num))
		this.tmn[shareName] = v
		return nil
	}

	// Not the last level, so let's find or create the next level snode, and
	// recursively call it's insert().

	// ntl = next topic level
	ntl, rem, err := nextRTopicLevel(topic)
	if err != nil {
		return err
	}

	level := string(ntl)

	// Add snode if it doesn't already exist
	n, ok := this.rsnodes[level]
	if !ok {
		n = newRSNode()
		this.rsnodes[level] = n
	}

	return n.sinsert(rem, shareName, node, num)
}

// This remove implementation ignores the QoS, as long as the subscriber
// matches then it's removed
func (this *rSnode) sremove(topic []byte, shareName, node string) error {
	// If the topic is empty, it means we are at the final matching snode. If so,
	// let's find the matching subscribers and remove them.
	if len(topic) == 0 {
		// If shareName == nil, then it's signal to remove ALL node
		// TODO ???shareName????????????????????????????????????
		if shareName == "" {
			this.tmn = make(map[string][]*nodeWeight)
			return nil
		}
		var v []*nodeWeight
		var ok bool
		if v, ok = this.tmn[shareName]; ok {
			for i := range v {
				if v[i].node == node {
					v[i].wright--         // ????????????
					if v[i].wright == 0 { // ???node?????????????????????????????????
						v = append(v[:i], v[i+1:]...)
						this.tmn[shareName] = v
					}
					return nil
				}
			}
		}
		return fmt.Errorf("memtopics/remove: No topic found for sharename: %s", shareName)
	}

	// Not the last level, so let's find the next level snode, and recursively
	// call it's remove().

	// ntl = next topic level
	ntl, rem, err := nextTopicLevel(topic)
	if err != nil {
		return err
	}

	level := string(ntl)

	// Find the snode that matches the topic level
	n, ok := this.rsnodes[level]
	if !ok {
		return fmt.Errorf("memtopics/remove: No topic found")
	}

	// Remove the subscriber from the next level snode
	if err := n.sremove(rem, shareName, node); err != nil {
		return err
	}

	// If there are no more subscribers and snodes to the next level we just visited
	// let's remove it
	if len(n.tmn) == 0 && len(n.rsnodes) == 0 {
		delete(this.rsnodes, level)
	}

	return nil
}

// smatch()
func (this *rSnode) smatch(topic []byte, shareName, nodes *[]string) error {
	// If the topic is empty, it means we are at the final matching snode. If so,
	// let's find the subscribers that match the qos and append them to the list.
	if len(topic) == 0 {
		this.matchNode(shareName, nodes)
		return nil
	}

	// ntl = next topic level
	ntl, rem, err := nextTopicLevel(topic)
	if err != nil {
		return err
	}

	level := string(ntl)

	for k, n := range this.rsnodes {
		// If the key is "#", then these subscribers are added to the result set
		if k == consts.MWC {
			n.matchNode(shareName, nodes)
		} else if k == consts.SWC || k == level {
			if err := n.smatch(rem, shareName, nodes); err != nil {
				return err
			}
		}
	}

	return nil
}

// Returns topic level, remaining topic levels and any errors
func nextRTopicLevel(topic []byte) ([]byte, []byte, error) {
	s := consts.StateCHR

	for i, c := range topic {
		switch c {
		case '/':
			if s == consts.StateMWC {
				return nil, nil, fmt.Errorf("redistopics/nextTopicLevel: Multi-level wildcard found in topic and it's not at the last level")
			}

			if i == 0 {
				return []byte(consts.SWC), topic[i+1:], nil
			}

			return topic[:i], topic[i+1:], nil

		case '#':
			if i != 0 {
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Wildcard character '#' must occupy entire topic level")
			}

			s = consts.StateMWC

		case '+':
			if i != 0 {
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Wildcard character '+' must occupy entire topic level")
			}

			s = consts.StateSWC

		case '$':
			if i == 0 {
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Cannot publish to $ topics")
			}

			s = consts.StateSYS

		default:
			if s == consts.StateMWC || s == consts.StateSWC {
				return nil, nil, fmt.Errorf("redistopics/nextTopicLevel: Wildcard characters '#' and '+' must occupy entire topic level")
			}

			s = consts.StateCHR
		}
	}

	// If we got here that means we didn't hit the separator along the way, so the
	// topic is either empty, or does not contain a separator. Either way, we return
	// the full topic
	return topic, nil, nil
}

// matchNode TODO ???????????????????????????????????????????????????????????????
func (this *rSnode) matchNode(shareName, nodes *[]string) {
	for sn, nd := range this.tmn {
		w := uint32(0)
		for i := 0; i < len(nd); i++ {
			w += nd[i].wright
		}
		// FIXME ??????????????????????????????????????????crypto/rand?????????
		rn := int(fastrand.Uint32n(w)) + 1 // ?????????0???????????????????????????1?????????????????????
		//result, _ := rand.Int(rand.Reader, big.NewInt(int64(w)))
		//rn := int(result.Int64())
		for _, v := range nd {
			rn -= int(v.wright)
			if rn > 0 {
				continue
			}
			*shareName = append(*shareName, sn)
			*nodes = append(*nodes, v.node)
			break
		}
	}
	if v, ok := this.rsnodes["#"]; ok {
		v.matchNode(shareName, nodes)
	}
}

// Returns topic level, remaining topic levels and any errors
//?????????????????????????????????????????????????????????
func nextTopicLevel(topic []byte) ([]byte, []byte, error) {
	s := consts.StateCHR

	//??????topic?????????????????????????????????
	for i, c := range topic {
		switch c {
		case '/':
			if s == consts.StateMWC {
				//????????????????????????????????????????????????????????????
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Multi-level wildcard found in topic and it's not at the last level")
			}

			if i == 0 {
				return []byte(consts.SWC), topic[i+1:], nil
			}

			return topic[:i], topic[i+1:], nil

		case '#':
			if i != 0 {
				//????????????#?????????????????????????????????
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Wildcard character '#' must occupy entire topic level")
			}

			s = consts.StateMWC

		case '+':
			if i != 0 {
				//????????????+?????????????????????????????????
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Wildcard character '+' must occupy entire topic level")
			}

			s = consts.StateSWC

		case '$':
			if i == 0 {
				//???????????????$ topics
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Cannot publish to $ topics")
			}

			s = consts.StateSYS

		default:
			if s == consts.StateMWC || s == consts.StateSWC {
				//????????????#?????????+?????????????????????????????????
				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Wildcard characters '#' and '+' must occupy entire topic level")
			}

			s = consts.StateCHR
		}
	}

	// If we got here that means we didn't hit the separator along the way, so the
	// topic is either empty, or does not contain a separator. Either way, we return
	// the full topic
	//????????????????????????????????????????????????????????????????????????????????????
	//?????????????????????????????????????????????????????????????????????????????????
	//???????????????
	return topic, nil, nil
}
