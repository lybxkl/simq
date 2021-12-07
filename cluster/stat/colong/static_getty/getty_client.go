package static_getty

import (
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	msg "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	getty "github.com/apache/dubbo-getty"
	"sync"
)

import (
	"github.com/dubbogo/gost/sync"
)

type staticGettyClient struct {
	sync.RWMutex
	c map[string]*client
}

func (s *staticGettyClient) Close() error {
	s.Lock()
	defer s.Unlock()
	for _, clientFace := range s.c {
		clientFace.Close()
	}
	return nil
}

// RunStaticGettyNodeClients 静态节点客户端启动
func RunStaticGettyNodeClients(nodes map[string]cluster.Node, curName string, connectNum int, taskPoolMode bool, taskPoolSize int) colong.NodeClientFace {
	newStaticGettySend()
	static := &staticGettyClient{
		c: map[string]*client{},
	}
	for name, node := range nodes {
		static.c[name] = runClient(curName, name, node.Addr,
			connectNum, taskPoolMode, taskPoolSize)
	}
	return static
}

type client struct {
	serverName string
	getty.Client
	taskPool gxsync.GenericTaskPool
	*Pool
}

func (this *client) Close() error {
	colong.GetSender().(*staticGettySender).removeSession(this.serverName)
	this.Client.Close()
	this.Pool.Close()
	return nil
}
func runClient(curName, serverName string, serverAddr string, connectNum int, taskPoolMode bool, taskPoolSize int) *client {
	util.SetLimit()

	//util.Profiling(*pprofPort)

	cl := &client{
		serverName: serverName,
	}
	if taskPoolMode {
		cl.taskPool = gxsync.NewTaskPoolSimple(taskPoolSize)
	}
	cl.Pool = NewPool(connectNum*2, 20)
	c := getty.NewTCPClient(
		getty.WithServerAddress(serverAddr),
		getty.WithConnectionNumber(connectNum),
		getty.WithClientTaskPool(cl.taskPool),
		getty.WithReconnectInterval(30000),
	)

	c.RunEventLoop(cl.newHelloClientSession(curName, serverName))

	cl.Client = c

	colong.GetSender().(*staticGettySender).addSession(serverName, cl)
	//util.WaitCloseSignals(client)
	//colong.RemoveSession(serverName)
	//taskPool.Close()
	return cl
}

// NewHelloClientSession use for init client session
func (this *client) newHelloClientSession(curName, nodeName string) func(session getty.Session) error {
	return func(session getty.Session) error {
		SetSessionOnOpen(EventListener, func(name string, session getty.Session) {
			this.Put(session)
		})
		SetCurName(EventListener, curName)
		err := InitialSession(nodeName, session)
		if err != nil {
			return err
		}
		return nil
	}
}

// NewStaticSend 静态集群发送者
func newStaticGettySend() {
	colong.SetSender(&staticGettySender{
		sessionsSync: &sync.Map{},
	})
}

// 静态集群发送者
type staticGettySender struct {
	sessionsSync *sync.Map
}

// 非数据库采用的方式，每个连接一个NodeClientFace
func (s *staticGettySender) addSession(name string, session colong.NodeClientFace) {
	s.sessionsSync.Store(name, session)
}
func (s *staticGettySender) removeSession(name string) {
	_, _ = s.sessionsSync.LoadAndDelete(name)
}

// wrapperShare 发送共享主题消息
func wrapperShare(msg msg.Message, shareName string) ([]byte, error) {
	cmsg := NewWrapCMsgImpl(PubShareCMsg)
	cmsg.SetShare(shareName, msg)
	return EncodeCMsg(cmsg)
}

// wrapperPub 发送普通消息
func wrapperPub(msg msg.Message) ([]byte, error) {
	cmsg := NewWrapCMsgImpl(PubCMsg)
	cmsg.SetMsg(msg)
	return EncodeCMsg(cmsg)
}

// SendOneNode 单个发送，共享订阅
func (s *staticGettySender) SendOneNode(msg msg.Message, shareName, targetNode string,
	oneNodeSendSucFunc func(name string, message msg.Message),
	oneNodeSendFailFunc func(name string, message msg.Message)) {
	var (
		b   []byte
		err error
	)
	if serv, ok := s.sessionsSync.Load(targetNode); ok {
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
		if s, ok := serv.(*client).Pop(); !ok {
			log.Warnf("get client session error, node %v send share name : %v", targetNode, shareName)
		} else {
			if s.IsClosed() {
				log.Warnf("client session is closed , node %v send share name : %v", targetNode, shareName)
				return
			}
			_, er := s.WriteBytes(b)
			defer serv.(*client).Put(s)
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
		}
	} else if shareName != "" { // 没有这个节点，但是必须发送共享消息
		// TODO 重新选择节点发送该共享组名下的 共享消息
	}
}

// SendAllNode 发送所有的
func (s *staticGettySender) SendAllNode(msg msg.Message, allSuccess func(message msg.Message),
	oneNodeSendSucFunc func(name string, message msg.Message),
	oneNodeSendFailFunc func(name string, message msg.Message)) {
	var (
		b      []byte
		err    error
		sucTag = true
	)
	// 发送全部节点
	b, err = wrapperPub(msg)
	if err != nil {
		log.Warnf("wrapper pub msg error %v", err)
		if oneNodeSendFailFunc != nil {
			oneNodeSendFailFunc(colong.AllNodeName, msg)
		}
		return
	}
	s.sessionsSync.Range(func(k, value interface{}) bool {
		defer func() {
			if e := recover(); e != nil {
				println(e)
			}
		}()
		// FIXME 会因为一个client获取阻塞，导致后面其它节点全部都在阻塞
		if s, ok := value.(*client).Pop(); !ok {
			log.Warnf("client close, node %v send fail.", k)
		} else {
			if s.IsClosed() {
				log.Warnf("client session is closed , node %v send fail.", k)
				return true
			}
			_, er := s.WriteBytes(b)
			defer value.(*client).Put(s)
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
