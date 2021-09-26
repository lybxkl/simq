package colong

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"sync"
)

var (
	sessionsSync = &sync.Map{}
)

func AddSession(name string, session *Client) {
	sessionsSync.Store(name, session)
}
func RemoveSession(name string) {
	_, _ = sessionsSync.LoadAndDelete(name)
}

// wrapperShare 发送共享主题消息
func wrapperShare(msg messagev5.Message, shareName string) ([]byte, error) {
	cmsg := NewWrapCMsgImpl(PubShareCMsg)
	cmsg.SetShare(shareName, msg)
	return EncodeCMsg(cmsg)
}

// wrapperPub 发送普通消息
func wrapperPub(msg messagev5.Message) ([]byte, error) {
	cmsg := NewWrapCMsgImpl(PubCMsg)
	cmsg.SetMsg(msg)
	return EncodeCMsg(cmsg)
}

// SendMsgToCluster 发送消息到集群
// shareName 共享主题组
// targetNode 目标节点
// 这两个参数用于集群共享主题消息发送到特定的节点，TODO share msg 节点发送确认
func SendMsgToCluster(msg messagev5.Message, shareName, targetNode string, allSuccess func(message messagev5.Message),
	oneNodeSendSucFunc func(name string, message messagev5.Message),
	oneNodeSendFailFunc func(name string, message messagev5.Message)) {
	var (
		b      []byte
		err    error
		sucTag = true
	)
	if targetNode != "" { // 单个发送，可能是共享消息
		if serv, ok := sessionsSync.Load(targetNode); ok {
			if shareName == "" { // 普通消息发送单个节点
				b, err = wrapperPub(msg)
				if err != nil {
					log.Warnf("wrapper pub msg error %v", err)
					if oneNodeSendFailFunc != nil {
						oneNodeSendFailFunc(targetNode, msg)
					}
					return
				}
			} else { // 共享主题消息，发送到单个节点
				b, err = wrapperShare(msg, shareName)
				if err != nil {
					log.Warnf("wrapper share msg error %v", err)
					if oneNodeSendFailFunc != nil {
						oneNodeSendFailFunc(targetNode, msg)
					}
					return
				}
			}
			if s, ok := serv.(*Client).Pop(); !ok {
				log.Warnf("get client session error, node %v send share name : %v", targetNode, shareName)
			} else {
				_, er := s.WriteBytes(b)
				defer serv.(*Client).Put(s)
				if er != nil {
					log.Warnf("send msg to cluster node: %s/%s: encode msg error : msg: %+v,err:%v",
						targetNode, s.RemoteAddr(), msg, er)
					if oneNodeSendFailFunc != nil {
						oneNodeSendFailFunc(targetNode, msg)
					}
					return
				}
				if oneNodeSendSucFunc != nil {
					oneNodeSendSucFunc(targetNode, msg)
				}
				if allSuccess != nil {
					allSuccess(msg)
				}
			}
		} else if shareName != "" { // 没有这个节点，但是必须发送共享消息
			// TODO 重新选择节点发送该共享组名下的 共享消息
		}
		return
	}
	// 发送全部节点
	b, err = wrapperPub(msg)
	if err != nil {
		log.Warnf("wrapper pub msg error %v", err)
		if oneNodeSendFailFunc != nil {
			oneNodeSendFailFunc(targetNode, msg)
		}
		return
	}
	sessionsSync.Range(func(k, value interface{}) bool {
		defer func() {
			if e := recover(); e != nil {
				println(e)
			}
		}()
		if s, ok := value.(*Client).Pop(); !ok {
			log.Warnf("client close, node %v send share name : %v", targetNode, shareName)
		} else {
			_, er := s.WriteBytes(b)
			defer value.(*Client).Put(s)
			if er != nil {
				log.Warnf("send msg to cluster node: %s/%s: encode msg error : msg: %+v,err:%v",
					k, s.RemoteAddr(), msg, er)
				if oneNodeSendFailFunc != nil {
					oneNodeSendFailFunc(k.(string), msg)
				}
				sucTag = false
				return true // 不能返回false,不然这次range会停止
			}
			if oneNodeSendSucFunc != nil {
				oneNodeSendSucFunc(k.(string), msg)
			}
		}
		return true
	})
	if sucTag && allSuccess != nil {
		allSuccess(msg)
	}
}
