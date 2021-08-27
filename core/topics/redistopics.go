package topics

//
//import (
//	"Go-MQTT/mqtt_v5/config"
//	"gitee.com/Ljolan/si-mqtt/message"
//	"fmt"
//	"reflect"
//	"sync"
//)
//
//var (
//	// MaxQosAllowed is the maximum QOS supported by this server
//	MaxQosAllowed_redis = message.QosExactlyOnce
//)
//
//var _ TopicsProvider = (*redisTopics)(nil)
//
//type redisTopics struct {
//	// Sub/unsub mutex
//	smu sync.RWMutex
//	// Subscription tree
//	sroot *rSnode
//
//	// Retained message mutex
//	rmu sync.RWMutex
//	// Retained messages topic tree
//	rroot *rRnode
//}
//
//var (
//	redisTopicsProvider = "redis"
//)
//
//func redisTopicInit() {
//	consts := config.ConstConf
//	if redisTopicsProvider == consts.DefaultConst.TopicsProvider {
//		Register(redisTopicsProvider, NewRedisProvider())
//	}
//}
//
//var _ TopicsProvider = (*redisTopics)(nil)
//
//// NewMemProvider returns an new instance of the memTopics, which is implements the
//// TopicsProvider interface. memProvider is a hidden struct that stores the topic
//// subscriptions and retained messages in memory. The content is not persistend so
//// when the server goes, everything will be gone. Use with care.
//func NewRedisProvider() *redisTopics {
//	return &redisTopics{
//		sroot: newRSNode(),
//		rroot: newRRNode(),
//	}
//}
//func (this *redisTopics) AllSubInfo() (map[string][]string, error) {
//	return nil, nil
//}
//func (this *redisTopics) Subscribe(topic []byte, qos byte, sub interface{}) (byte, error) {
//	if !message.ValidQos(qos) {
//		return message.QosFailure, fmt.Errorf("Invalid QoS %d", qos)
//	}
//
//	if sub == nil {
//		return message.QosFailure, fmt.Errorf("Subscriber cannot be nil")
//	}
//
//	this.smu.Lock()
//	defer this.smu.Unlock()
//
//	if qos > MaxQosAllowed_redis {
//		qos = MaxQosAllowed_redis
//	}
//
//	if err := this.sroot.sinsert(topic, qos, sub); err != nil {
//		return message.QosFailure, err
//	}
//
//	return qos, nil
//}
//
//func (this *redisTopics) Unsubscribe(topic []byte, sub interface{}) error {
//	this.smu.Lock()
//	defer this.smu.Unlock()
//
//	return this.sroot.sremove(topic, sub)
//}
//
//// Returned values will be invalidated by the next Subscribers call
//func (this *redisTopics) Subscribers(topic []byte, qos byte, subs *[]interface{}, qoss *[]byte, svc bool, shareName string, onlyShare bool) error {
//	if !message.ValidQos(qos) {
//		return fmt.Errorf("Invalid QoS %d", qos)
//	}
//
//	this.smu.RLock()
//	defer this.smu.RUnlock()
//
//	*subs = (*subs)[0:0]
//	*qoss = (*qoss)[0:0]
//
//	return this.sroot.smatch(topic, qos, subs, qoss)
//}
//
//func (this *redisTopics) Retain(msg *message.PublishMessage) error {
//	this.rmu.Lock()
//	defer this.rmu.Unlock()
//
//	// So apparently, at least according to the MQTT Conformance/Interoperability
//	// Testing, that a payload of 0 means delete the retain message.
//	// https://eclipse.org/paho/clients/testing/
//	if len(msg.Payload()) == 0 {
//		return this.rroot.rremove(msg.Topic())
//	}
//
//	return this.rroot.rinsert(msg.Topic(), msg)
//}
//
//func (this *redisTopics) Retained(topic []byte, msgs *[]*message.PublishMessage) error {
//	this.rmu.RLock()
//	defer this.rmu.RUnlock()
//
//	return this.rroot.rmatch(topic, msgs)
//}
//
//func (this *redisTopics) Close() error {
//	this.sroot = nil
//	this.rroot = nil
//	return nil
//}
//
//// subscrition nodes
//type rSnode struct {
//	// If this is the end of the topic string, then add subscribers here
//	subs []interface{}
//	qos  []byte
//
//	// Otherwise add the next topic level here
//	rsnodes map[string]*rSnode
//}
//
//func newRSNode() *rSnode {
//	return &rSnode{
//		rsnodes: make(map[string]*rSnode),
//	}
//}
//
//func (this *rSnode) sinsert(topic []byte, qos byte, sub interface{}) error {
//	// If there's no more topic levels, that means we are at the matching snode
//	// to insert the subscriber. So let's see if there's such subscriber,
//	// if so, update it. Otherwise insert it.
//	if len(topic) == 0 {
//		// Let's see if the subscriber is already on the list. If yes, update
//		// QoS and then return.
//		for i := range this.subs {
//			if equal(this.subs[i], sub) {
//				this.qos[i] = qos
//				return nil
//			}
//		}
//
//		// Otherwise add.
//		this.subs = append(this.subs, sub)
//		this.qos = append(this.qos, qos)
//
//		return nil
//	}
//
//	// Not the last level, so let's find or create the next level snode, and
//	// recursively call it's insert().
//
//	// ntl = next topic level
//	ntl, rem, err := nextRTopicLevel(topic)
//	if err != nil {
//		return err
//	}
//
//	level := string(ntl)
//
//	// Add snode if it doesn't already exist
//	n, ok := this.rsnodes[level]
//	if !ok {
//		n = newRSNode()
//		this.rsnodes[level] = n
//	}
//
//	return n.sinsert(rem, qos, sub)
//}
//
//// This remove implementation ignores the QoS, as long as the subscriber
//// matches then it's removed
//func (this *rSnode) sremove(topic []byte, sub interface{}) error {
//	// If the topic is empty, it means we are at the final matching snode. If so,
//	// let's find the matching subscribers and remove them.
//	if len(topic) == 0 {
//		// If subscriber == nil, then it's signal to remove ALL subscribers
//		if sub == nil {
//			this.subs = this.subs[0:0]
//			this.qos = this.qos[0:0]
//			return nil
//		}
//
//		// If we find the subscriber then remove it from the list. Technically
//		// we just overwrite the slot by shifting all other items up by one.
//		for i := range this.subs {
//			if equal(this.subs[i], sub) {
//				this.subs = append(this.subs[:i], this.subs[i+1:]...)
//				this.qos = append(this.qos[:i], this.qos[i+1:]...)
//				return nil
//			}
//		}
//
//		return fmt.Errorf("memtopics/remove: No topic found for subscriber")
//	}
//
//	// Not the last level, so let's find the next level snode, and recursively
//	// call it's remove().
//
//	// ntl = next topic level
//	ntl, rem, err := nextTopicLevel(topic)
//	if err != nil {
//		return err
//	}
//
//	level := string(ntl)
//
//	// Find the snode that matches the topic level
//	n, ok := this.rsnodes[level]
//	if !ok {
//		return fmt.Errorf("memtopics/remove: No topic found")
//	}
//
//	// Remove the subscriber from the next level snode
//	if err := n.sremove(rem, sub); err != nil {
//		return err
//	}
//
//	// If there are no more subscribers and snodes to the next level we just visited
//	// let's remove it
//	if len(n.subs) == 0 && len(n.rsnodes) == 0 {
//		delete(this.rsnodes, level)
//	}
//
//	return nil
//}
//
//// smatch() returns all the subscribers that are subscribed to the topic. Given a topic
//// with no wildcards (publish topic), it returns a list of subscribers that subscribes
//// to the topic. For each of the level names, it's a match
//// - if there are subscribers to '#', then all the subscribers are added to result set
//func (this *rSnode) smatch(topic []byte, qos byte, subs *[]interface{}, qoss *[]byte) error {
//	// If the topic is empty, it means we are at the final matching snode. If so,
//	// let's find the subscribers that match the qos and append them to the list.
//	if len(topic) == 0 {
//		this.matchQos(qos, subs, qoss)
//		return nil
//	}
//
//	// ntl = next topic level
//	ntl, rem, err := nextTopicLevel(topic)
//	if err != nil {
//		return err
//	}
//
//	level := string(ntl)
//
//	for k, n := range this.rsnodes {
//		// If the key is "#", then these subscribers are added to the result set
//		if k == MWC {
//			n.matchQos(qos, subs, qoss)
//		} else if k == SWC || k == level {
//			if err := n.smatch(rem, qos, subs, qoss); err != nil {
//				return err
//			}
//		}
//	}
//
//	return nil
//}
//
//// retained message nodes
//type rRnode struct {
//	// If this is the end of the topic string, then add retained messages here
//	msg *message.PublishMessage
//	buf []byte
//
//	// Otherwise add the next topic level here
//	rrnodes map[string]*rRnode
//}
//
//func newRRNode() *rRnode {
//	return &rRnode{
//		rrnodes: make(map[string]*rRnode),
//	}
//}
//
//func (this *rRnode) rinsert(topic []byte, msg *message.PublishMessage) error {
//	// If there's no more topic levels, that means we are at the matching rnode.
//	if len(topic) == 0 {
//		l := msg.Len()
//
//		// Let's reuse the buffer if there's enough space
//		if l > cap(this.buf) {
//			this.buf = make([]byte, l)
//		} else {
//			this.buf = this.buf[0:l]
//		}
//
//		if _, err := msg.Encode(this.buf); err != nil {
//			return err
//		}
//
//		// Reuse the message if possible
//		if this.msg == nil {
//			this.msg = message.NewPublishMessage()
//		}
//
//		if _, err := this.msg.Decode(this.buf); err != nil {
//			return err
//		}
//
//		return nil
//	}
//
//	// Not the last level, so let's find or create the next level snode, and
//	// recursively call it's insert().
//
//	// ntl = next topic level
//	ntl, rem, err := nextTopicLevel(topic)
//	if err != nil {
//		return err
//	}
//
//	level := string(ntl)
//
//	// Add snode if it doesn't already exist
//	n, ok := this.rrnodes[level]
//	if !ok {
//		n = newRRNode()
//		this.rrnodes[level] = n
//	}
//
//	return n.rinsert(rem, msg)
//}
//
//// Remove the retained message for the supplied topic
//func (this *rRnode) rremove(topic []byte) error {
//	// If the topic is empty, it means we are at the final matching rnode. If so,
//	// let's remove the buffer and message.
//	if len(topic) == 0 {
//		this.buf = nil
//		this.msg = nil
//		return nil
//	}
//
//	// Not the last level, so let's find the next level rnode, and recursively
//	// call it's remove().
//
//	// ntl = next topic level
//	ntl, rem, err := nextTopicLevel(topic)
//	if err != nil {
//		return err
//	}
//
//	level := string(ntl)
//
//	// Find the rnode that matches the topic level
//	n, ok := this.rrnodes[level]
//	if !ok {
//		return fmt.Errorf("memtopics/rremove: No topic found")
//	}
//
//	// Remove the subscriber from the next level rnode
//	if err := n.rremove(rem); err != nil {
//		return err
//	}
//
//	// If there are no more rnodes to the next level we just visited let's remove it
//	if len(n.rrnodes) == 0 {
//		delete(this.rrnodes, level)
//	}
//
//	return nil
//}
//
//// rmatch() finds the retained messages for the topic and qos provided. It's somewhat
//// of a reverse match compare to match() since the supplied topic can contain
//// wildcards, whereas the retained message topic is a full (no wildcard) topic.
//func (this *rRnode) rmatch(topic []byte, msgs *[]*message.PublishMessage) error {
//	// If the topic is empty, it means we are at the final matching rnode. If so,
//	// add the retained msg to the list.
//	if len(topic) == 0 {
//		if this.msg != nil {
//			*msgs = append(*msgs, this.msg)
//		}
//		return nil
//	}
//
//	// ntl = next topic level
//	ntl, rem, err := nextTopicLevel(topic)
//	if err != nil {
//		return err
//	}
//
//	level := string(ntl)
//
//	if level == MWC {
//		// If '#', add all retained messages starting this node
//		this.allRetained(msgs)
//	} else if level == SWC {
//		// If '+', check all nodes at this level. Next levels must be matched.
//		for _, n := range this.rrnodes {
//			if err := n.rmatch(rem, msgs); err != nil {
//				return err
//			}
//		}
//	} else {
//		// Otherwise, find the matching node, go to the next level
//		if n, ok := this.rrnodes[level]; ok {
//			if err := n.rmatch(rem, msgs); err != nil {
//				return err
//			}
//		}
//	}
//
//	return nil
//}
//
//func (this *rRnode) allRetained(msgs *[]*message.PublishMessage) {
//	if this.msg != nil {
//		*msgs = append(*msgs, this.msg)
//	}
//
//	for _, n := range this.rrnodes {
//		n.allRetained(msgs)
//	}
//}
//
//// Returns topic level, remaining topic levels and any errors
//func nextRTopicLevel(topic []byte) ([]byte, []byte, error) {
//	s := stateCHR
//
//	for i, c := range topic {
//		switch c {
//		case '/':
//			if s == stateMWC {
//				return nil, nil, fmt.Errorf("redistopics/nextTopicLevel: Multi-level wildcard found in topic and it's not at the last level")
//			}
//
//			if i == 0 {
//				return []byte(SWC), topic[i+1:], nil
//			}
//
//			return topic[:i], topic[i+1:], nil
//
//		case '#':
//			if i != 0 {
//				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Wildcard character '#' must occupy entire topic level")
//			}
//
//			s = stateMWC
//
//		case '+':
//			if i != 0 {
//				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Wildcard character '+' must occupy entire topic level")
//			}
//
//			s = stateSWC
//
//		case '$':
//			if i == 0 {
//				return nil, nil, fmt.Errorf("memtopics/nextTopicLevel: Cannot publish to $ topics")
//			}
//
//			s = stateSYS
//
//		default:
//			if s == stateMWC || s == stateSWC {
//				return nil, nil, fmt.Errorf("redistopics/nextTopicLevel: Wildcard characters '#' and '+' must occupy entire topic level")
//			}
//
//			s = stateCHR
//		}
//	}
//
//	// If we got here that means we didn't hit the separator along the way, so the
//	// topic is either empty, or does not contain a separator. Either way, we return
//	// the full topic
//	return topic, nil, nil
//}
//
//// The QoS of the payload messages sent in response to a subscription must be the
//// minimum of the QoS of the originally published message (in this case, it's the
//// qos parameter) and the maximum QoS granted by the server (in this case, it's
//// the QoS in the topic tree).
////
//// It's also possible that even if the topic matches, the subscriber is not included
//// due to the QoS granted is lower than the published message QoS. For example,
//// if the client is granted only QoS 0, and the publish message is QoS 1, then this
//// client is not to be send the published message.
//func (this *rSnode) matchQos(qos byte, subs *[]interface{}, qoss *[]byte) {
//	for i, sub := range this.subs {
//		// If the published QoS is higher than the subscriber QoS, then we skip the
//		// subscriber. Otherwise, add to the list.
//		if qos <= this.qos[i] {
//			*subs = append(*subs, sub)
//			*qoss = append(*qoss, qos)
//		}
//	}
//}
//
//func requal(k1, k2 interface{}) bool {
//	if reflect.TypeOf(k1) != reflect.TypeOf(k2) {
//		return false
//	}
//
//	if reflect.ValueOf(k1).Kind() == reflect.Func {
//		return &k1 == &k2
//	}
//
//	if k1 == k2 {
//		return true
//	}
//
//	switch k1 := k1.(type) {
//	case string:
//		return k1 == k2.(string)
//
//	case int64:
//		return k1 == k2.(int64)
//
//	case int32:
//		return k1 == k2.(int32)
//
//	case int16:
//		return k1 == k2.(int16)
//
//	case int8:
//		return k1 == k2.(int8)
//
//	case int:
//		return k1 == k2.(int)
//
//	case float32:
//		return k1 == k2.(float32)
//
//	case float64:
//		return k1 == k2.(float64)
//
//	case uint:
//		return k1 == k2.(uint)
//
//	case uint8:
//		return k1 == k2.(uint8)
//
//	case uint16:
//		return k1 == k2.(uint16)
//
//	case uint32:
//		return k1 == k2.(uint32)
//
//	case uint64:
//		return k1 == k2.(uint64)
//
//	case uintptr:
//		return k1 == k2.(uintptr)
//	}
//
//	return false
//}
