package servicev5

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/cluster/store/repo"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/authv5"
	"gitee.com/Ljolan/si-mqtt/corev5/authv5/authplus"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	"io"
	"net"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
)

var (
	ErrInvalidConnectionType  error = errors.New("service: Invalid connection type")
	ErrInvalidSubscriber      error = errors.New("service: Invalid subscriber")
	ErrBufferNotReady         error = errors.New("service: buffer is not ready")
	ErrBufferInsufficientData error = errors.New("service: buffer has insufficient data.") //缓冲区数据不足。
)
var SVC *service
var serverName string

func GetServerName() string {
	return serverName
}

// Server is a library implementation of the MQTT server that, as best it can, complies
// with the MQTT 3.1 and 3.1.1 specs.
// Server是MQTT服务器的一个库实现，它尽其所能遵守
//使用MQTT 3.1和3.1.1规范。
type Server struct {
	Version string // 服务版本

	ConFig *config.SIConfig // 偷懒了

	// The number of seconds to keep the connection live if there's no data.
	// If not set then default to 5 mins.
	//如果没有数据，保持连接的秒数。
	//如果没有设置，则默认为5分钟。connect中的该值的1.5倍作为读超时
	KeepAlive int // uint16

	// 写超时，默认5分钟 1.5
	WriteTimeout int

	// The number of seconds to wait for the CONNECT messagev5 before disconnecting.
	// If not set then default to 5 seconds.
	//在断开连接之前等待连接消息的秒数。
	//如果没有设置，则默认为5秒。
	ConnectTimeout int

	// The number of seconds to wait for any ACK messages before failing.
	// If not set then default to 20 seconds.
	//失败前等待ACK消息的秒数。
	//如果没有设置，则默认为20秒。
	AckTimeout int

	// The number of times to retry sending a packet if ACK is not received.
	// If no set then default to 3 retries.
	//如果没有收到ACK，重试发送数据包的次数。
	//如果没有设置，则默认为3次重试。
	TimeoutRetries int

	// Authenticator is the authenticator used to check username and password sent
	// in the CONNECT messagev5. If not set then default to "mockSuccess".
	// Authenticator是验证器，用于检查发送的用户名和密码
	//在连接消息中。如果不设置，则默认为"mockSuccess"
	Authenticator string

	// SessionsProvider is the session store that keeps all the Session objects.
	// This is the store to check if CleanSession is set to 0 in the CONNECT messagev5.
	// If not set then default to "mem".
	// SessionsProvider是保存所有会话对象的会话存储。
	//这是用于检查在连接消息中是否将清洗设置为0的存储。
	//如果没有设置，则默认为"mem"。
	SessionsProvider string

	// TopicsProvider is the topic store that keeps all the subscription topicsv5.
	// If not set then default to "mem".
	// TopicsProvider是保存所有订阅主题的主题存储。
	//如果没有设置，则默认为"mem"。
	TopicsProvider string

	// 增强认证管理器
	AuthPlusProvider []string

	// authMgr is the authentication manager that we are going to use for authenticating
	// incoming connections
	// authMgr是我们将用于身份验证的认证管理器
	// 传入的连接
	authMgr *authv5.Manager

	// 增强认证允许的方法
	authPlusAllows map[string]*authplus.Manager

	// sessMgr is the sessionsv5 manager for keeping track of the sessionsv5
	// sessMgr是用于跟踪会话的会话管理器
	sessMgr *sessionsv5.Manager

	// topicsMgr is the topicsv5 manager for keeping track of subscriptions
	// topicsMgr是跟踪订阅的主题管理器
	topicsMgr *topicsv5.Manager

	// The quit channel for the server. If the server detects that this channel
	// is closed, then it's a signal for it to shutdown as well.
	//服务器的退出通道。如果服务器检测到该通道
	//是关闭的，那么它也是一个关闭的信号。
	quit chan struct{}

	ln net.Listener

	// A list of services created by the server. We keep track of them so we can
	// gracefully shut them down if they are still alive when the server goes down.
	//服务器创建的服务列表。我们跟踪他们，这样我们就可以
	//当服务器宕机时，如果它们仍然存在，那么可以优雅地关闭它们。
	// TODO 选择切片还是map
	svcs []*service

	// Mutex for updating svcs
	//用于更新svc的互斥锁
	mu sync.Mutex

	// A indicator on whether this server is running
	//指示服务器是否运行的指示灯
	running int32

	// A indicator on whether this server has already checked configuration
	//指示此服务器是否已检查配置
	configOnce sync.Once

	ClusterDiscover cluster.NodeDiscover
	ClusterServer   colong.NodeServerFace
	ClusterClient   colong.NodeClientFace

	ShareTopicMapNode cluster.ShareTopicMapNode

	SessionStore store.SessionStore
	MessageStore store.MessageStore
	EventStore   store.EventStore

	subs []interface{}
	qoss []byte

	close []io.Closer
}

func (s *Server) TopicProvider() *topicsv5.Manager {
	return s.topicsMgr
}

// ListenAndServe listents to connections on the URI requested, and handles any
// incoming MQTT client sessionsv5. It should not return until Close() is called
// or if there's some critical error that stops the server from running. The URI
// supplied should be of the form "protocol://host:port" that can be parsed by
// url.Parse(). For example, an URI could be "tcp://0.0.0.0:1883".
// ListenAndServe监听请求的URI上的连接，并处理任何连接
//传入的MQTT客户机会话。 在调用Close()之前，它不应该返回
//或者有一些关键的错误导致服务器停止运行。 URI
//提供的格式应该是“protocol://host:port”，可以通过它进行解析
// url.Parse ()。
//例如，URI可以是“tcp://0.0.0.0:1883”。
func (this *Server) ListenAndServe(uri string) error {
	defer atomic.CompareAndSwapInt32(&this.running, 1, 0)

	// 防止重复启动
	if !atomic.CompareAndSwapInt32(&this.running, 0, 1) {
		return fmt.Errorf("server/ListenAndServe: Server is already running")
	}
	serverName = this.ConFig.Cluster.ClusterName

	this.quit = make(chan struct{})

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	//这个是配置各种钩子，比如账号认证钩子
	err = this.checkConfiguration()
	if err != nil {
		panic(err)
	}
	// store
	this.InitStore()
	// cluster
	this.RunClusterComp()

	printBanner(this.Version)
	var tempDelay time.Duration // how long to sleep on accept failure 接受失败要睡多久，默认5ms，最大1s

	this.ln, err = net.Listen(u.Scheme, u.Host) // 监听连接
	if err != nil {
		return err
	}
	defer this.ln.Close()
	logger.Logger.Infof("AddMQTTHandler uri=%v", uri)
	for {
		conn, err := this.ln.Accept()

		if err != nil {
			// http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-this.quit: //关闭服务器
				return nil

			default:
			}

			// Borrowed from go1.3.3/src/pkg/net/http/server.go:1699
			// 暂时的错误处理
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				logger.Logger.Errorf("Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}

		go func() {
			svc, err := this.handleConnection(conn)
			if err != nil {
				logger.Logger.Error(err.Error())
			} else {
				SVC = svc // 这一步是不是多余的
			}
		}()
	}
}

func (this *Server) InitStore() {
	this.SessionStore = repo.NewSessionStore()
	this.MessageStore = repo.NewMessageStore()
	this.EventStore = repo.NewEventStore()
	ctx := context.Background()
	utils.MustPanic(this.SessionStore.Start(ctx, *this.ConFig))
	utils.MustPanic(this.MessageStore.Start(ctx, *this.ConFig))
	utils.MustPanic(this.EventStore.Start(ctx, *this.ConFig))
}
func (this *Server) RunClusterComp() {
	cfg := this.ConFig
	if !cfg.Cluster.Enabled { // 集群服务启动
		return
	}
	switch cfg.Cluster.Model {
	case config.Getty:
		GettyClusterRun(this, cfg)
	case config.MongoEm:
		DBMongoClusterRun(this, cfg)
	case config.MysqlEm:
		DBMysqlClusterRun(this, cfg)
	default:
		logger.Logger.Errorf("the cluster startup mode: %v is not supported", cfg.Cluster.Model)
		return
	}
	logger.Logger.Infof("cluster startup mode: \"%v\", run success", cfg.Cluster.Model)
}

// 打印启动banner
func printBanner(serverVersion string) {
	logger.Logger.Info("\n" +
		"\\***\n" +
		"*  _ooOoo_\n" +
		"* o8888888o\n" +
		"*'88' . '88' \n" +
		"* (| -_- |)\n" +
		"*  O\\ = /O\n" +
		"* ___/`---'\\____\n" +
		"* .   ' \\| |// `.\n" +
		"* / \\\\||| : |||// \\\n*" +
		" / _||||| -:- |||||- \\\n*" +
		" | | \\\\\\ - /// | |\n" +
		"* | \\_| ''\\---/'' | |\n" +
		"* \\ .-\\__ `-` ___/-. /\n" +
		"* ___`. .' /--.--\\ `. . __\n" +
		"* .'' '< `.___\\_<|>_/___.' >''''.\n" +
		"* | | : `- \\`.;`\\ _ /`;.`/ - ` : | |\n" +
		"* \\ \\ `-. \\_ __\\ /__ _/ .-` / /\n" +
		"* ======`-.____`-.___\\_____/___.-`____.-'======\n" +
		"* `=---='\n" +
		"*          .............................................\n" +
		"*           佛曰：bug泛滥，我已瘫痪！\n" +
		"*/\n" + "/****\n" +
		"* ░▒█████▒█    ██  ▄████▄ ██ ▄█▀       ██████╗   ██╗   ██╗ ██████╗\n" +
		"* ▓█     ▒ ██    ▓█ ▒▒██▀ ▀█  ██▄█▒        ██╔══██╗ ██║   ██║ ██╔════╝\n" +
		"* ▒████ ░▓█   ▒██░▒▓█    ▄   ██▄░         ██████╔╝ ██║   ██║ ██║  ███╗\n" +
		"* ░▓█▒    ░▓▓   ░██░▒▓▓▄ ▄█  ▒▓██▄       ██╔══██╗ ██║   ██║ ██║   ██║\n" +
		"* ░▒█░    ▒▒█████▓ ▒ ▓███▀  ░▒██▒ █▄     ██████╔╝╚██████╔╝╚██████╔╝\n" +
		"*  ▒ ░    ░▒▓▒ ▒ ▒ ░ ░▒ ▒  ░▒ ▒▒ ▓▒       ╚═════╝      ╚═════╝    ╚═════╝\n" +
		"*  ░      ░░▒░ ░ ░   ░  ▒   ░ ░▒ ▒░\n" +
		"*  ░ ░     ░░░ ░ ░ ░        ░ ░░ ░\n" +
		"*             ░     ░ ░      ░  ░\n" +
		"*/" +
		"服务器准备就绪: server is ready... version: " + serverVersion)
}

func (this *Server) AddCloser(close io.Closer) {
	this.close = append(this.close, close)
}

// Close terminates the server by shutting down all the client connections and closing
// the listener. It will, as best it can, clean up after itself.
func (this *Server) Close() error {
	// By closing the quit channel, we are telling the server to stop accepting new
	// connection.
	close(this.quit)

	if this.sessMgr != nil {
		err := this.sessMgr.Close()
		if err != nil {
			logger.Logger.Errorf("关闭session管理器错误:%v", err)
		}
	}

	if this.topicsMgr != nil {
		err := this.topicsMgr.Close()
		if err != nil {
			logger.Logger.Errorf("关闭topic管理器错误:%v", err)
		}
	}
	// We then close the net.Listener, which will force Accept() to return if it's
	// blocked waiting for new connections.
	err := this.ln.Close()
	if err != nil {
		logger.Logger.Errorf("关闭网络Listener错误:%v", err)
	}
	// 后面不会执行到，不知道为啥
	// TODO 将当前节点上的客户端数据保存持久化到mysql或者redis都行，待这些客户端重连集群时，可以搜索到旧session，也要考虑是否和客户端连接时的cleanSession有绑定
	for i := 0; i < len(this.close); i++ {
		this.close[i].Close()
	}
	return nil
}
func (this *Server) NewService() *service {
	return &service{
		sessMgr:       this.sessMgr,
		topicsMgr:     this.topicsMgr,
		clusterBelong: true,
	}
}

// HandleConnection is for the broker to handle an incoming connection from a client
// HandleConnection用于代理处理来自客户机的传入连接
func (this *Server) handleConnection(c io.Closer) (svc *service, err error) {
	if c == nil {
		return nil, ErrInvalidConnectionType
	}

	defer func() {
		if err != nil {
			_ = c.Close()
		}
	}()

	conn, ok := c.(net.Conn)
	if !ok {
		return nil, ErrInvalidConnectionType
	}

	//要建立联系，我们必须
	// 1.阅读并解码信息从连接线上的ConnectMessage
	// 2.如果没有解码错误，则使用用户名和密码进行身份验证。
	//否则，发送ConnackMessage与合适的原因码。
	// 3.如果身份验证成功，则创建一个新会话或 检索现有会话
	// 4.给客户端发送成功的ConnackMessage消息
	// 从连线中读取连接消息，如果错误，则检查它是否正确一个连接错误。
	// 如果是连接错误，请发回正确的连接错误
	conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(this.ConnectTimeout)))

	resp := messagev5.NewConnackMessage()
	// 从本次连接中获取到connectMessage
	req, err := getConnectMessage(conn)
	if err != nil {
		if cerr, ok := err.(messagev5.ReasonCode); ok {
			logger.Logger.Debugf("request messagev5: %s\nresponse messagev5: %s\nerror : %v", nil, resp, err)
			resp.SetReasonCode(cerr)
			resp.SetSessionPresent(false)
			writeMessage(conn, resp)
		}
		return nil, err
	}
	svconf := this.ConFig.DefaultConfig.Server
	if svconf.RedirectOpen { // 重定向
		dis := messagev5.NewDisconnectMessage()
		if svconf.RedirectIsForEver {
			dis.SetReasonCode(messagev5.ServerHasMoved)
		} else {
			dis.SetReasonCode(messagev5.UseOtherServers)
		}
		dis.SetServerReference([]byte(svconf.Redirects[0]))
		err = writeMessage(conn, dis)
		return nil, nil
	}
	// 版本
	logger.Logger.Debugf("client mqtt version :%v", req.Version())

	// 认证
	if e := this.auth(conn, resp, req); e != nil {
		return nil, e
	}

	if req.KeepAlive() == 0 {
		//设置默认的keepalive数，五分钟
		req.SetKeepAlive(uint16(this.KeepAlive))
	}
	svc = &service{
		id:          atomic.AddUint64(&gsvcid, 1),
		client:      false,
		clusterOpen: this.ConFig.Cluster.Enabled,

		keepAlive:      int(req.KeepAlive()),
		writeTimeout:   this.WriteTimeout,
		connectTimeout: this.ConnectTimeout,
		ackTimeout:     this.AckTimeout,
		timeoutRetries: this.TimeoutRetries,

		conn:              conn,
		sessMgr:           this.sessMgr,
		topicsMgr:         this.topicsMgr,
		shareTopicMapNode: this.ShareTopicMapNode,
	}
	err = this.getSession(svc, req, resp)
	if err != nil {
		return nil, err
	}

	resp.SetReasonCode(messagev5.Success)

	svc.inStat.increment(int64(req.Len()))
	// 设置配额和limiter
	// TODO 简单设置测试
	svc.quota = 0 // 此配额需要持久化
	svc.limit = 0 // 限速，每秒100次请求

	if err := svc.start(resp); err != nil {
		svc.stop()
		return nil, err
	}

	//this.mu.Lock()
	//this.svcs = append(this.svcs, svc)
	//this.mu.Unlock()

	logger.Logger.Debugf("(%s) server/handleConnection: Connection established.", svc.cid())

	return svc, nil
}
func (this *Server) auth(conn net.Conn, resp *messagev5.ConnackMessage, req *messagev5.ConnectMessage) error {
	// 增强认证
	authMethod := req.AuthMethod() // 第一次的增强认证方法
	if len(authMethod) > 0 {
		authData := req.AuthData()
		auVerify, ok := this.authPlusAllows[string(authMethod)]
		if !ok {
			dis := messagev5.NewDisconnectMessage()
			dis.SetReasonCode(messagev5.InvalidAuthenticationMethod)
			return writeMessage(conn, dis)
		}
	AC:
		authContinueData, continueAuth, err := auVerify.Verify(authData)
		if err != nil {
			dis := messagev5.NewDisconnectMessage()
			dis.SetReasonCode(messagev5.UnAuthorized)
			dis.SetReasonStr([]byte(err.Error()))
			return writeMessage(conn, dis)
		}
		if continueAuth {
			au := messagev5.NewAuthMessage()
			au.SetReasonCode(messagev5.ContinueAuthentication)
			au.SetAuthMethod(authMethod)
			au.SetAuthData(authContinueData)
			err = writeMessage(conn, au)
			if err != nil {
				return err
			}
			msg, err := getAuthMessageOrOther(conn) // 后续的auth
			if err != nil {
				return err
			}
			switch msg.Type() {
			case messagev5.DISCONNECT: // 增强认证过程中断开连接
				return errors.New("disconnect in auth")
			case messagev5.AUTH:
				auMsg := msg.(*messagev5.AuthMessage)
				if !reflect.DeepEqual(auMsg.AuthMethod(), authMethod) {
					ds := messagev5.NewDisconnectMessage()
					ds.SetReasonCode(messagev5.InvalidAuthenticationMethod)
					ds.SetReasonStr([]byte("auth method is different from last time"))
					err = writeMessage(conn, ds)
					if err != nil {
						return err
					}
					return errors.New("authplus: the authentication method is different from last time.")
				}
				authData = auMsg.AuthData()
				goto AC // 需要继续认证
			default:
				return errors.New(fmt.Sprintf("unSupport deal msg %s", msg))
			}
		} else {
			// 成功
			resp.SetReasonCode(messagev5.Success)
			resp.SetAuthMethod(authMethod)
			logger.Logger.Infof("增强认证成功：%s", req.ClientId())
		}
	} else {
		if err := this.authMgr.Authenticate(string(req.Username()), string(req.Password())); err != nil {
			//登陆失败日志，断开连接
			resp.SetReasonCode(messagev5.UserNameOrPasswordIsIncorrect)
			resp.SetSessionPresent(false)
			return writeMessage(conn, resp)
		}
		logger.Logger.Infof("普通认证成功：%s", req.ClientId())
	}
	return nil
}
func (this *Server) checkConfiguration() error {
	var err error

	this.configOnce.Do(func() {
		if this.KeepAlive == 0 {
			this.KeepAlive = 300
		}

		if this.ConnectTimeout == 0 {
			this.ConnectTimeout = 5
		}

		if this.AckTimeout == 0 {
			this.AckTimeout = 20
		}

		if this.TimeoutRetries == 0 {
			this.TimeoutRetries = 3
		}

		if this.Authenticator == "" {
			logger.Logger.Info("缺少权限认证，采用默认方式")
		}

		this.authMgr, err = authv5.NewManager(this.Authenticator)
		if err != nil {
			panic(err)
		}

		auPlus := make(map[string]*authplus.Manager)
		for _, s := range this.AuthPlusProvider {
			ma, err := authplus.NewManager(s)
			if err != nil {
				panic(err)
			}
			if s == "" {
				s = authplus.Default
			}
			auPlus[s] = ma
		}
		this.authPlusAllows = auPlus

		if this.SessionsProvider == "" {
			logger.Logger.Info("缺少Session管理器，采用默认方式")
		}

		this.sessMgr, err = sessionsv5.NewManager(this.SessionsProvider)
		if err != nil {
			panic(err)
		}

		if this.TopicsProvider == "" {
			logger.Logger.Info("缺少Topic管理器，采用默认方式")
		}

		this.topicsMgr, err = topicsv5.NewManager(this.TopicsProvider)

		return
	})

	return err
}

func (this *Server) getSession(svc *service, req *messagev5.ConnectMessage, resp *messagev5.ConnackMessage) error {
	// If CleanSession is set to 0, the server MUST resume communications with the
	// client based on state from the current session, as identified by the client
	// identifier. If there is no session associated with the client identifier the
	// server must create a new session.
	//
	// If CleanSession is set to 1, the client and server must discard any previous
	// session and start a new one. This session lasts as long as the network c
	// onnection. State data associated with this session must not be reused in any
	// subsequent session.
	//如果cleanession设置为0，服务器必须恢复与
	//客户端基于当前会话的状态，由客户端识别
	//标识符。如果没有会话与客户端标识符相关联
	//服务器必须创建一个新的会话。
	//
	//如果cleanession设置为1，客户端和服务器必须丢弃任何先前的
	//创建一个新的session。这个会话持续的时间与网络connection。与此会话关联的状态数据绝不能在任何会话中重用
	//后续会话。

	var err error

	// Check to see if the client supplied an ID, if not, generate one and set
	// clean session.
	//检查客户端是否提供了ID，如果没有，生成一个并设置
	//清洁会话。
	if len(req.ClientId()) == 0 {
		req.SetClientId([]byte(fmt.Sprintf("internalclient%d", svc.id)))
		req.SetCleanSession(true)
	}

	cid := string(req.ClientId())

	// If CleanSession is NOT set, check the session store for existing session.
	// If found, return it.
	//如果没有设置清除会话，请检查会话存储是否存在会话。
	//如果找到，返回。
	// TODO 通知其它节点断开那边的该客户端ID的连接，如果有的话
	// ......FIXME 这里断开后，会丢失断开到重新连接其间的离线消息
	// TODO 会话过期间隔 > 0 , 需要存储session，==0 则在连接断开时清除session
	if !req.CleanStart() { // 使用旧session
		// FIXME 当断线前是cleanStare=true，我们使用的是mem，但是重新连接时，使用了false，how to deal?
		if svc.sess, err = this.sessMgr.Get(cid, req.CleanStart(), req.SessionExpiryInterval()); err == nil {
			// TODO 这里是懒删除，最好再加个定时删除
			if svc.sess.Cmsg().SessionExpiryInterval() == 0 ||
				(svc.sess.OfflineTime()+int64(svc.sess.Cmsg().SessionExpiryInterval()) <= time.Now().UnixNano()) {
				// 删除session ， 因为已经过期了
				svc.sess = nil
				err = this.SessionStore.ClearSession(context.Background(), cid, true)
				if err != nil {
					return err
				}
				this.sessMgr.Del(cid)
			} else {
				// 如果在该节点，则可以直接使用该session，前提是集群节点通知断开、清理session是成功的
				resp.SetSessionPresent(true)

				if err = svc.sess.Update(req); err != nil {
					return err
				}
			}
		} else {
			// log
		}
	} else {
		// 删除旧数据，清空旧连接的离线消息和未完成的过程消息，会话数据
		err = this.SessionStore.ClearSession(context.Background(), cid, true)
		if err != nil {
			return err
		}
		this.sessMgr.Del(cid)
	}
	//如果没有session则创建一个
	// If CleanSession, or no existing session found, then create a new one
	if svc.sess == nil {
		// 这里因为前面通知其它节点断开旧连接，所以这里可以直接New
		if svc.sess, err = this.sessMgr.New(cid, req.CleanStart(), req.SessionExpiryInterval()); err != nil {
			return err
		}
		svc.sess.(store.Store).SetStore(this.SessionStore, this.MessageStore)

		// 新建的present设置为false
		resp.SetSessionPresent(false)
		// 从当前connectMessage初始化该session
		if err = svc.sess.Init(req); err != nil {
			return err
		}
	}

	return nil
}
