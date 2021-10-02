package static_getty

import (
	"errors"
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
)

type serverMessageHandler struct {
	SessionOnOpen func(session getty.Session)
	inner
	clusterInToPub      colong.ClusterInToPub
	clusterInToPubShare colong.ClusterInToPubShare
	clusterInToPubSys   colong.ClusterInToPubSys
	shareTopicMapNode   cluster.ShareTopicMapNode
}

func SetPubFunc(el getty.EventListener, clusterInToPub colong.ClusterInToPub,
	clusterInToPubShare colong.ClusterInToPubShare, clusterInToPubSys colong.ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode) {
	if listener, ok := el.(*serverMessageHandler); ok {
		listener.clusterInToPubShare = clusterInToPubShare
		listener.clusterInToPub = clusterInToPub
		listener.clusterInToPubSys = clusterInToPubSys
		listener.shareTopicMapNode = shareTopicMapNode
	}
}
func NewServerEventListener() getty.EventListener {
	return &serverMessageHandler{inner: newInner()}
}
func (h *serverMessageHandler) OnOpen(session getty.Session) error {
	log.Infof("OnOpen session{%s} open", session.Stat())
	if h.SessionOnOpen != nil {
		h.SessionOnOpen(session)
	}
	return nil
}

func (h *serverMessageHandler) OnError(session getty.Session, err error) {
	log.Infof("OnError session{%s} got error{%v}, will be closed.", session.Stat(), err)
	rip := session.GetAttribute(CremoteIp)
	ss, ok := cs.Load(session.GetAttribute(Cname))
	if ok {
		ss.(*sync.Map).Delete(rip.(string))
	}
	// todo 需要清理为空的
}

func (h *serverMessageHandler) OnClose(session getty.Session) {
	log.Infof("OnClose session{%s} is closing......", session.Stat())
	rip := session.GetAttribute(CremoteIp)
	ss, ok := cs.Load(session.GetAttribute(Cname))
	if ok {
		ss.(*sync.Map).Delete(rip.(string))
	}
}

func (h *serverMessageHandler) OnMessage(session getty.Session, m interface{}) {
	pkg1, ok := m.(WrapCMsg)
	if !ok {
		return
	}
	switch pkg1.Type() {
	case StatusCMsg:
		// TODO 状态处理
		return
	case CloseSession:
		// TODO 断开客户端连接，清理远端session
		return
	}
	msg := pkg1.Msg()
	if msg == nil {
		return
	}
	cname := session.GetAttribute(Cname)
	if cname == nil {
		switch pkg := msg.(type) {
		case *messagev5.ConnectMessage: // 直接使用connec报文中的用户属性传递节点连接数据，简单方便
			h.connectAuth(session, pkg)
		default:
			session.Close()
			return
		}
	}
	// TODO 消息是否需要确认？
	switch pkg := msg.(type) {
	case *messagev5.ConnectMessage: // 直接使用connec报文中的用户属性传递节点连接数据，简单方便
		//h.connectAuth(session, pkg)
	case *messagev5.PingreqMessage:
		submit(func() {
			_, err := session.WriteBytes(pingresp)
			if err != nil {
				log.Errorf("OnMessage PingreqMessage: session{%s} write bytes err: {%v}", session.Stat(), err)
			}
		})
	case *messagev5.PublishMessage:
		// 本地发送
		submit(func() {
			if pkg1.Share() {
				err := h.clusterInToPubShare(pkg, pkg1.Tag()[0])
				if err != nil {
					log.Errorf("clusterInToPubShare: err %v", err)
				} else {
					log.Debugf("收到节点：%s 发来的 共享消息：%s", cname.(string), pkg)
				}
			} else {
				err := h.clusterInToPub(pkg)
				if err != nil {
					log.Errorf("clusterInToPub: err %v", err)
				} else {
					log.Debugf("收到节点：%s 发来的 普通消息：%s", cname.(string), pkg)
				}
			}
		})
	case *messagev5.SubscribeMessage:
		// 更新【本地订阅树】  与 【主题与节点的映射】
		submit(func() {
			tpk := pkg.Topics()
			node := cname.(string)
			for i := 0; i < len(tpk); i++ {
				// 解析share name
				shareName, top := shareTopic(tpk[i])
				if shareName != "" {
					err := h.shareTopicMapNode.AddTopicMapNode(top, shareName, node)
					if err != nil {
						log.Errorf("%s,共享订阅节点新增失败 : %v", node, shareName, err)
					} else {
						log.Debugf("收到节点：%s 发来的 共享订阅：topic-%s, shareName-%s", node, top, shareName)
					}
				} else {
					log.Warnf("收到非法订阅：%s", string(tpk[i]))
				}
			}
		})
	case *messagev5.UnsubscribeMessage:
		submit(func() {
			tpk := pkg.Topics()
			node := cname.(string)
			for i := 0; i < len(tpk); i++ {
				// 解析share name
				shareName, top := shareTopic(tpk[i])
				if shareName != "" {
					err := h.shareTopicMapNode.RemoveTopicMapNode(top, shareName, node)
					if err != nil {
						log.Errorf("%s,共享订阅节点减少失败 : %v", node, shareName, err)
					} else {
						log.Debugf("收到节点：%s 发来的 取消共享订阅：topic-%s, shareName-%s", node, top, shareName)
					}
				} else {
					log.Warnf("收到非法取消订阅：%s", string(tpk[i]))
				}
			}
		})
	default:
		log.Infof("OnMessage: %s", pkg)
	}
}

// 共享组和topic
func shareTopic(b []byte) (string, []byte) {
	if len(b) < len(sharePrefix) {
		return "", b
	}
	for i := 0; i < len(sharePrefix); i++ {
		if b[i] != sharePrefix[i] {
			return "", b
		}
	}
	for i := len(sharePrefix); i < len(b); i++ {
		if b[i] == '/' {
			return string(b[len(sharePrefix):i]), b[i+1:]
		}
	}
	return "", b
}
func (h *serverMessageHandler) OnCron(session getty.Session) {
	active := session.GetActive()
	if CronPeriod < time.Since(active).Nanoseconds() {
		//log.Infof("OnCorn session{%s} timeout{%s}", session.Stat(), time.Since(active).String())
		session.Close()
	}
}

// 简单校验地址
func parseAddr(addr string) error {
	url := strings.Split(addr, ":")
	if len(url) != 2 {
		return errors.New("client node ip addr error")
	}
	ip := net.ParseIP(url[0])
	if ip == nil {
		return errors.New("client node ip addr error")
	}
	_, err := strconv.ParseUint(url[1], 10, 16)
	if err != nil {
		return errors.New("client node ip addr error")
	}
	return nil
}
func (this *serverMessageHandler) connectAuth(session getty.Session, pkg *messagev5.ConnectMessage) {
	clientNodeId := string(pkg.ClientId())
	if clientNodeId == "" {
		log.Errorf("client node connect info error, %v", pkg)
		session.Close()
	}
	up := pkg.UserProperty()
	var addr string
	for i := 0; i < len(up); i++ {
		add := strings.SplitN(string(up[i]), ":", 2)
		if len(add) == 2 && add[0] == Caddr && len(add[1]) > 0 {
			addr = add[1]
		}
	}
	if err := parseAddr(addr); err != nil {
		log.Error(err)
		session.Close()
		return
	}
	var s getty.Session
	if sessMap, ok := cs.Load(clientNodeId); ok { // 重复建立连接认证 how to handle
		smap := sessMap.(*sync.Map)
		if sess, ok := smap.Load(session.RemoteAddr()); ok {
			s = sess.(getty.Session)
		} else {
			smap.Store(session.RemoteAddr(), session)
		}
	} else {
		smap := &sync.Map{}
		smap.Store(session.RemoteAddr(), session)
		cs.Store(clientNodeId, smap)
	}
	if s != nil {
		log.Warnf("remote ip session exist: %v", s.RemoteAddr())
		s.Close()
		return
	}
	_, err := session.WriteBytes(ack)
	if err != nil {
		log.Errorf("write connect ack error: %v", err)
		return
	}
	session.SetAttribute(Caddr, addr)
	session.SetAttribute(Cname, clientNodeId)
	session.SetAttribute(CremoteIp, session.RemoteAddr())
	log.Infof("%v/%v/%v", session.GetAttribute(Cname), session.GetAttribute(Caddr), session.GetAttribute(CremoteIp))
	this.SetAuthOk(session, true)
}
