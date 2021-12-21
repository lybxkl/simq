package service

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/cluster/store/memImpl"
	mongorepo "gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/repo"
	mysqlpo "gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/repo"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/auth"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/auth/authplus"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/consts"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	sessions_impl "gitee.com/Ljolan/si-mqtt/corev5/v2/sessions/impl"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	topics_impl "gitee.com/Ljolan/si-mqtt/corev5/v2/topics/impl"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/util/cron"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/util/middleware"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/util/runtimex"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	"io"
	"net"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidConnectionType  error = errors.New("service: Invalid connection type")
	ErrInvalidSubscriber      error = errors.New("service: Invalid subscriber")
	ErrBufferNotReady         error = errors.New("service: buffer is not ready")
	ErrBufferInsufficientData error = errors.New("service: buffer has insufficient data.") //缓冲区数据不足。

	serverName string
)

func GetServerName() string {
	return serverName
}

type Server struct {
	Version string // 服务版本

	ConFig *config.SIConfig

	//如果没有数据，保持连接的秒数。
	//如果没有设置，则默认为5分钟。connect中的该值的1.5倍作为读超时
	KeepAlive int // uint16
	// 写超时，默认5分钟 1.5
	WriteTimeout int
	//在断开连接之前等待连接消息的秒数。
	//如果没有设置，则默认为5秒。
	ConnectTimeout int
	//失败前等待ACK消息的秒数。
	//如果没有设置，则默认为20秒。
	AckTimeout int
	//如果没有收到ACK，重试发送数据包的次数。
	//如果没有设置，则默认为3次重试。
	TimeoutRetries int

	// 增强认证管理器
	AuthPlusProvider []string
	// authMgr是我们将用于普通身份验证的认证管理器
	authMgr auth.Authenticator
	// 增强认证允许的方法
	authPlusAllows *sync.Map // map[string]authplus.AuthPlus

	// sessMgr是用于跟踪会话的会话管理器
	sessMgr sessions.Provider

	// topicsMgr是跟踪订阅的主题管理器
	topicsMgr topics.Manager

	//服务器的退出通道。如果服务器检测到该通道 是关闭的，那么它也是一个关闭的信号。
	quit chan struct{}

	ln net.Listener

	//用于更新svc的互斥锁
	mu sync.Mutex
	//服务器创建的服务列表。我们跟踪他们，这样我们就可以
	//当服务器宕机时，如果它们仍然存在，那么可以优雅地关闭它们。
	svcs []*service

	//指示服务器是否运行的指示灯
	running int32

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

	middleware middleware.Options
}

//func (s *Server) TopicProvider() topics.Manager {
//	return s.topicsMgr
//}

// ListenAndServe 监听请求的URI上的连接，并处理任何连接
//传入的MQTT客户机会话。 在调用Close()之前，它不应该返回
//或者有一些关键的错误导致服务器停止运行。
//URI 提供的格式应该是“protocol://host:port”，可以通过它进行解析 url.Parse ()。
//例如，URI可以是“tcp://0.0.0.0:1883”。
func (server *Server) ListenAndServe(uri string) error {
	defer atomic.CompareAndSwapInt32(&server.running, 1, 0)

	// 防止重复启动
	if !atomic.CompareAndSwapInt32(&server.running, 0, 1) {
		return fmt.Errorf("server/ListenAndServe: Server is already running")
	}
	serverName = server.ConFig.Cluster.ClusterName

	server.quit = make(chan struct{})

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	//这个是配置各种钩子，比如账号认证钩子
	err = server.checkAndInitConfiguration()
	if err != nil {
		panic(err)
	}

	var tempDelay time.Duration // 接受失败要睡多久，默认5ms，最大1s

	server.ln, err = net.Listen(u.Scheme, u.Host) // 监听连接
	if err != nil {
		return err
	}
	defer server.ln.Close()

	logger.Logger.Infof("AddMQTTHandler uri=%v", uri)
	for {
		conn, err := server.ln.Accept()

		if err != nil {
			// http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-server.quit: //关闭服务器
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
			defer runtimex.Recover()
			_, handleErr := server.handleConnection(conn)
			if handleErr != nil {
				logger.Logger.Error(handleErr.Error())
			}
		}()
	}
}

func (server *Server) AddCloser(close io.Closer) {
	server.close = append(server.close, close)
}

// Close terminates the server by shutting down all the client connections and closing
// the listener. It will, as best it can, clean up after itself.
func (server *Server) Close() error {
	// By closing the quit channel, we are telling the server to stop accepting new
	// connection.
	close(server.quit)

	if server.sessMgr != nil {
		err := server.sessMgr.Close()
		if err != nil {
			logger.Logger.Errorf("关闭session管理器错误:%v", err)
		}
	}

	if server.topicsMgr != nil {
		err := server.topicsMgr.Close()
		if err != nil {
			logger.Logger.Errorf("关闭topic管理器错误:%v", err)
		}
	}
	// We then close the net.Listener, which will force Accept() to return if it's
	// blocked waiting for new connections.
	err := server.ln.Close()
	if err != nil {
		logger.Logger.Errorf("关闭网络Listener错误:%v", err)
	}
	// 后面不会执行到，不知道为啥
	// TODO 将当前节点上的客户端数据保存持久化到mysql或者redis都行，待这些客户端重连集群时，可以搜索到旧session，也要考虑是否和客户端连接时的cleanSession有绑定
	for i := 0; i < len(server.close); i++ {
		server.close[i].Close()
	}
	return nil
}
func (server *Server) NewService() *service {
	return &service{
		sessMgr:       server.sessMgr,
		topicsMgr:     server.topicsMgr,
		clusterBelong: true,
	}
}

// handleConnection 用于代理处理来自客户机的传入连接
func (server *Server) handleConnection(c io.Closer) (svc *service, err error) {
	if c == nil {
		return nil, ErrInvalidConnectionType
	}

	defer func() {
		if err != nil && c != nil {
			time.Sleep(10 * time.Millisecond)
			_ = c.Close()
		}
	}()

	conn, ok := c.(net.Conn)
	if !ok {
		return nil, ErrInvalidConnectionType
	}

	//要建立联系，我们必须
	// 1.阅读并解码信息从连接线上的ConnectMessage
	// 2.如果没有解码错误，则使用用户名和密码 或者增强认证 进行身份验证。 否则，发送ConnackMessage与合适的原因码。
	// 3.如果身份验证成功，则创建一个新会话 或 检索现有会话
	// 4.给客户端发送成功的ConnackMessage消息
	// 从连线中读取连接消息，如果错误，则检查它是否正确一个连接错误。
	// 如果是连接错误，请返回正确的连接错误
	utils.MustPanic(conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(server.ConnectTimeout))))

	// 等待连接认证
	req, resp, err := server.conAuth(conn)
	if err != nil || resp == nil {
		return nil, err
	}

	svc = &service{
		id:          atomic.AddUint64(&gsvcid, 1),
		client:      false,
		clusterOpen: server.ConFig.Cluster.Enabled,

		keepAlive:      int(req.KeepAlive()),
		writeTimeout:   server.WriteTimeout,
		connectTimeout: server.ConnectTimeout,
		ackTimeout:     server.AckTimeout,
		timeoutRetries: server.TimeoutRetries,

		conn:              conn,
		sessMgr:           server.sessMgr,
		topicsMgr:         server.topicsMgr,
		shareTopicMapNode: server.ShareTopicMapNode,
		conFig:            server.ConFig,

		middleware: server.middleware,
	}
	err = server.getSession(svc, req, resp)
	if err != nil {
		return nil, err
	}

	resp.SetReasonCode(messagev2.Success)

	svc.inStat.increment(int64(req.Len()))
	// 设置配额和limiter
	// TODO 简单设置测试
	svc.quota = 0 // 此配额需要持久化
	svc.limit = 0 // 限速，每秒100次请求

	if err = svc.start(resp); err != nil {
		svc.stop()
		return nil, err
	}

	server.mu.Lock()
	server.svcs = append(server.svcs, svc)
	server.mu.Unlock()

	logger.Logger.Debugf("(%s) server/handleConnection: Connection established.", svc.cid())

	return svc, nil
}

// 连接认证
func (server *Server) conAuth(conn net.Conn) (*messagev2.ConnectMessage, *messagev2.ConnackMessage, error) {
	resp := messagev2.NewConnackMessage()
	if !server.ConFig.Broker.CloseShareSub { // 简单处理，热修改需要考虑的东西有点复杂
		resp.SetSharedSubscriptionAvailable(1)
	}
	// 从本次连接中获取到connectMessage
	req, err := getConnectMessage(conn)
	if err != nil {
		if code, ok := err.(messagev2.ReasonCode); ok {
			logger.Logger.Debugf("request messagev5: %s\nresponse messagev5: %s\nerror : %v", nil, resp, err)
			resp.SetReasonCode(code)
			resp.SetSessionPresent(false)
			utils.MustPanic(writeMessage(conn, resp))
		}
		return nil, nil, err
	}

	// 判断是否允许空client id
	if len(req.ClientId()) == 0 && !server.ConFig.Broker.AllowZeroLengthClientId {
		dis := messagev2.NewDisconnectMessage()
		dis.SetReasonCode(messagev2.CustomerIdentifierInvalid)
		dis.SetReasonStr([]byte("the length of the client ID cannot be zero"))
		writeMessage(conn, dis)
		return nil, nil, errors.New("the length of the client ID cannot be zero")
	}
	if server.ConFig.Broker.MaxKeepalive > 0 && req.KeepAlive() > server.ConFig.Broker.MaxKeepalive {
		dis := messagev2.NewDisconnectMessage()
		dis.SetReasonCode(messagev2.UnspecifiedError)
		dis.SetReasonStr([]byte("the keepalive value exceeds the maximum value"))
		writeMessage(conn, dis)
		return nil, nil, errors.New("the keepalive value exceeds the maximum value")
	}
	if server.ConFig.Broker.MaxPacketSize > 0 && req.Len() > int(server.ConFig.Broker.MaxPacketSize) {
		dis := messagev2.NewDisconnectMessage()
		dis.SetReasonCode(messagev2.UnspecifiedError)
		dis.SetReasonStr([]byte("exceeds the maximum package size"))
		writeMessage(conn, dis)
		return nil, nil, errors.New("exceeds the maximum package size")
	}

	svcConf := server.ConFig.DefaultConfig.Server
	if svcConf.RedirectOpen { // 重定向
		dis := messagev2.NewDisconnectMessage()
		if svcConf.RedirectIsForEver {
			dis.SetReasonCode(messagev2.ServerHasMoved)
		} else {
			dis.SetReasonCode(messagev2.UseOtherServers)
		}
		dis.SetServerReference([]byte(svcConf.Redirects[0]))
		writeMessage(conn, dis)
		return nil, nil, nil
	}
	// 版本
	logger.Logger.Debugf("client %s --> mqtt version :%v", req.ClientId(), req.Version())

	// 认证
	if err = server.auth(conn, resp, req); err != nil {
		return nil, nil, err
	}

	// broker 的默认值
	if req.KeepAlive() == 0 {
		//设置默认的keepalive数，五分钟
		req.SetKeepAlive(uint16(server.KeepAlive))
	}
	return req, resp, nil
}

func (server *Server) auth(conn net.Conn, resp *messagev2.ConnackMessage, req *messagev2.ConnectMessage) error {
	// 增强认证
	authMethod := req.AuthMethod() // 第一次的增强认证方法
	if len(authMethod) > 0 {
		authData := req.AuthData()
		auVerify, ok := server.authPlusAllows.Load(string(authMethod))
		if !ok {
			dis := messagev2.NewDisconnectMessage()
			dis.SetReasonCode(messagev2.InvalidAuthenticationMethod)
			return writeMessage(conn, dis)
		}
	AC:
		authContinueData, continueAuth, err := auVerify.(authplus.AuthPlus).Verify(authData)
		if err != nil {
			dis := messagev2.NewDisconnectMessage()
			dis.SetReasonCode(messagev2.UnAuthorized)
			dis.SetReasonStr([]byte(err.Error()))
			return writeMessage(conn, dis)
		}
		if continueAuth {
			au := messagev2.NewAuthMessage()
			au.SetReasonCode(messagev2.ContinueAuthentication)
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
			case messagev2.DISCONNECT: // 增强认证过程中断开连接
				return errors.New("disconnect in auth")
			case messagev2.AUTH:
				auMsg := msg.(*messagev2.AuthMessage)
				if !reflect.DeepEqual(auMsg.AuthMethod(), authMethod) {
					ds := messagev2.NewDisconnectMessage()
					ds.SetReasonCode(messagev2.InvalidAuthenticationMethod)
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
			resp.SetReasonCode(messagev2.Success)
			resp.SetAuthMethod(authMethod)
			logger.Logger.Infof("增强认证成功：%s", req.ClientId())
		}
	} else {
		if err := server.authMgr.Authenticate(string(req.Username()), string(req.Password())); err != nil {
			//登陆失败日志，断开连接
			resp.SetReasonCode(messagev2.UserNameOrPasswordIsIncorrect)
			resp.SetSessionPresent(false)
			return writeMessage(conn, resp)
		}
		logger.Logger.Infof("普通认证成功：%s", req.ClientId())
	}
	return nil
}

func (server *Server) checkAndInitConfiguration() error {
	var err error

	server.configOnce.Do(func() {
		if server.KeepAlive == 0 {
			server.KeepAlive = consts.KeepAlive
		}

		if server.ConnectTimeout == 0 {
			server.ConnectTimeout = consts.ConnectTimeout
		}

		if server.AckTimeout == 0 {
			server.AckTimeout = consts.AckTimeout
		}

		if server.TimeoutRetries == 0 {
			server.TimeoutRetries = consts.TimeoutRetries
		}

		// store
		server.initStore()

		auPlus := &sync.Map{} // make(map[string]authplus.AuthPlus)
		for _, s := range server.AuthPlusProvider {
			auPlus.Store(s, authplus.NewDefaultAuth()) // TODO
		}
		server.authPlusAllows = auPlus
		server.authMgr = auth.NewDefaultAuth()
		server.sessMgr = sessions_impl.NewMemProvider()
		server.topicsMgr = topics_impl.NewMemProvider()

		(server.sessMgr).(store.Store).SetStore(server.SessionStore, server.MessageStore)
		(server.topicsMgr).(store.Store).SetStore(server.SessionStore, server.MessageStore)

		// cluster
		server.runClusterComp()

		// init middleware
		server.initMiddleware(middleware.WithConsole())

		// 打印启动banner
		printBanner(server.Version)
		return
	})

	return err
}

func (server *Server) initMiddleware(option ...middleware.Option) {
	if server.middleware == nil {
		server.middleware = make(middleware.Options, 0)
	}
	for i := 0; i < len(option); i++ {
		server.middleware.Apply(option[i])
	}
}

// 初始化存储
func (server *Server) initStore() {
	switch server.ConFig.Store.Model {
	case config.MongoStore:
		server.SessionStore = mongorepo.NewSessionStore()
		server.MessageStore = mongorepo.NewMessageStore()
	case config.MysqlStore:
		server.SessionStore = mysqlpo.NewSessionStore()
		server.MessageStore = mysqlpo.NewMessageStore()
	default:
		server.SessionStore = memImpl.NewMemSessionStore()
		server.MessageStore = memImpl.NewMemMessageStore()
	}
	ctx := context.Background()
	utils.MustPanic(server.SessionStore.Start(ctx, *server.ConFig))
	utils.MustPanic(server.MessageStore.Start(ctx, *server.ConFig))
}

// 运行集群
func (server *Server) runClusterComp() {
	cfg := server.ConFig
	if !cfg.Cluster.Enabled { // 集群服务启动
		return
	}
	switch cfg.Cluster.Model {
	case config.Getty:
		GettyClusterRun(server, cfg)
	case config.MongoEm:
		DBMongoClusterRun(server, cfg)
	case config.MysqlEm:
		DBMysqlClusterRun(server, cfg)
	default:
		logger.Logger.Errorf("the cluster startup mode: %v is not supported", cfg.Cluster.Model)
		return
	}
	logger.Logger.Infof("cluster startup mode: \"%v\", run success", cfg.Cluster.Model)
}

func (server *Server) getSession(svc *service, req *messagev2.ConnectMessage, resp *messagev2.ConnackMessage) error {
	//如果cleanession设置为0，服务器必须恢复与客户端基于当前会话的状态，由客户端识别标识符。
	//如果没有会话与客户端标识符相关联, 服务器必须创建一个新的会话。
	//
	//如果cleanession设置为1，客户端和服务器必须丢弃任何先前的
	//创建一个新的session。这个会话持续的时间与网络connection。与此会话关联的状态数据绝不能在任何会话中重用后续会话。

	var err error

	//检查客户端是否提供了ID，如果没有，生成一个并设置 清理会话。
	if len(req.ClientId()) == 0 {
		req.SetClientId([]byte(fmt.Sprintf("internalclient%d", svc.id)))
		req.SetCleanSession(true)
	}

	cid := string(req.ClientId())

	//如果没有设置清除会话，请检查会话存储是否存在会话。
	//如果找到，返回。
	// TODO 通知其它节点断开那边的该客户端ID的连接，如果有的话
	// ......FIXME 这里断开后，会丢失断开到重新连接其间的离线消息
	// TODO 会话过期间隔 > 0 , 需要存储session，==0 则在连接断开时清除session
	if !req.CleanStart() { // 使用旧session
		// FIXME 当断线前是cleanStare=true，我们使用的是mem，但是重新连接时，使用了false，how to deal?
		if svc.sess, err = server.sessMgr.Get(cid, req.CleanStart(), req.SessionExpiryInterval()); err == nil {
			// TODO 这里是懒删除，最好再加个定时删除
			if svc.sess.Cmsg().SessionExpiryInterval() == 0 ||
				(svc.sess.OfflineTime()+int64(svc.sess.Cmsg().SessionExpiryInterval()) <= time.Now().UnixNano()) {
				// 删除session ， 因为已经过期了
				svc.sess = nil
				err = server.SessionStore.ClearSession(context.Background(), cid, true)
				if err != nil {
					return err
				}
				server.sessMgr.Del(cid)
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
		err = server.SessionStore.ClearSession(context.Background(), cid, true)
		if err != nil {
			return err
		}
		server.sessMgr.Del(cid)
	}
	//如果没有session则创建一个
	if svc.sess == nil {
		// 这里因为前面通知其它节点断开旧连接，所以这里可以直接New
		if svc.sess, err = server.sessMgr.New(cid, req.CleanStart(), req.SessionExpiryInterval()); err != nil {
			return err
		}
		svc.sess.(store.Store).SetStore(server.SessionStore, server.MessageStore)

		// 新建的present设置为false
		resp.SetSessionPresent(false)
		// 从当前connectMessage初始化该session
		if err = svc.sess.Init(req); err != nil {
			return err
		}
	}

	// 取消任务
	cron.DelayTaskManager.Cancel(string(req.ClientId()))

	return nil
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
