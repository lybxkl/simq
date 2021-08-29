package cliv5

import (
	"gitee.com/Ljolan/si-mqtt/cluster"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/tcp/client"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong/tcp/server"
	config2 "gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/authv5"
	"gitee.com/Ljolan/si-mqtt/corev5/authv5/authplus"
	"gitee.com/Ljolan/si-mqtt/corev5/servicev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5"
	"gitee.com/Ljolan/si-mqtt/logger"
	utils2 "gitee.com/Ljolan/si-mqtt/utils"
	"golang.org/x/net/websocket"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
)

func init() {
	utils2.MustPanic(config2.Configure(nil))
	cfg := config2.GetConfig()
	authv5.AuthInit(cfg.DefaultConfig.Provider.Authenticator)
	authplus.InitAuthPlus(cfg.DefaultConfig.Auth.Allows)
	sessionsv5.SessionInit(cfg.DefaultConfig.Provider.SessionsProvider)
	topicsv5.TopicInit(cfg.DefaultConfig.Provider.TopicsProvider)
}
func Run() {
	cfg := config2.GetConfig()
	conCif := cfg.DefaultConfig.Connect
	proCif := cfg.DefaultConfig.Provider
	authPlusCif := cfg.DefaultConfig.Auth

	svr := &servicev5.Server{
		ConFig:           &cfg,
		KeepAlive:        conCif.Keepalive,
		WriteTimeout:     conCif.WriteTimeout,
		ConnectTimeout:   conCif.ConnectTimeout,
		AckTimeout:       conCif.AckTimeout,
		TimeoutRetries:   conCif.TimeoutRetries,
		SessionsProvider: proCif.SessionsProvider,
		TopicsProvider:   proCif.TopicsProvider,
		Authenticator:    proCif.Authenticator,
		AuthPlusProvider: authPlusCif.Allows,
		Version:          cfg.ServerVersion,
	}

	if cfg.Cluster.Enabled {
		staticDisc := make(map[string]cluster.Node)
		for _, v := range cfg.Cluster.StaticNodeList {
			staticDisc[v.Name] = cluster.Node{
				NNA:  v.Name,
				Addr: v.Addr,
			}
		}
		svr.ClusterDiscover = cluster.NewStaticNodeDiscover(staticDisc)
		svr.ShareTopicMapNode = cluster.NewShareMap()
		svc := svr.NewService()
		svr.ClusterServer = server.RunClusterServer(cfg.Cluster.ClusterName,
			cfg.Cluster.ClusterHost+":"+strconv.Itoa(cfg.Cluster.ClusterPort),
			svc.ClusterInToPub, svc.ClusterInToPubShare, svc.ClusterInToPubSys, svr.ShareTopicMapNode)
		svr.ClusterClient = &sync.Map{}
		for name, node := range svr.ClusterDiscover.GetNodeMap() {
			go svr.ClusterClient.Store(name, client.RunClient(cfg.Cluster.ClusterName, name, node.Addr,
				1, true, 1000))
		}
	}

	var err error

	sigchan := make(chan os.Signal, 1)
	//signal.Notify(sigchan, os.Interrupt, os.Kill)
	signal.Notify(sigchan)
	// 性能分析
	if cfg.PProf.Open {
		go func() {
			// https://pdf.us/2019/02/18/2772.html
			// go tool pprof -http=:8000 http://localhost:8080/debug/pprof/heap    查看内存使用
			// go tool pprof -http=:8000 http://localhost:8080/debug/pprof/profile 查看cpu占用
			// 注意，需要提前安装 Graphviz 用于画图
			logger.Logger.Info(http.ListenAndServe(":"+strconv.Itoa(int(cfg.PProf.Port)), nil).Error())
		}()
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				panic(err)
			}
		}()
		sig := <-sigchan
		logger.Logger.Infof("服务停止：Existing due to trapped signal; %v", sig)

		err := svr.Close()
		if err != nil {
			logger.Logger.Errorf("server close err: %v", err)
		}
		os.Exit(0)
	}()
	mqttaddr := "tcp://:1883"
	if strings.TrimSpace(cfg.Broker.TcpAddr) != "" {
		mqttaddr = cfg.Broker.TcpAddr
	}
	wsAddr := cfg.Broker.WsAddr
	wssAddr := cfg.Broker.WssAddr
	BuffConfigInit()
	if len(cfg.Broker.WsPath) > 0 && (len(wsAddr) > 0 || len(wssAddr) > 0) {
		AddWebsocketHandler(cfg.Broker.WsPath, mqttaddr) // 将wsAddr的ws连接数据发到mqttaddr上

		/* start a plain websocket listener */
		if len(wsAddr) > 0 {
			go ListenAndServeWebsocket(wsAddr)
		}
		/* start a secure websocket listener */
		if len(wssAddr) > 0 && len(cfg.Broker.WssCertPath) > 0 && len(cfg.Broker.WssKeyPath) > 0 {
			go ListenAndServeWebsocketSecure(wssAddr, cfg.Broker.WssCertPath, cfg.Broker.WssKeyPath)
		}
	}
	/* create plain MQTT listener */
	err = svr.ListenAndServe(mqttaddr)
	if err != nil {
		logger.Logger.Errorf("MQTT 启动异常错误 simq/main: %v", err)
	}

}

// buff 配置设置
func BuffConfigInit() {
	//buff := config.ConstConf.MyBuff
	//if buff.BufferSize > math.MaxInt64 {
	//	panic("config.ConstConf.MyBuff.BufferSize more than math.MaxInt64")
	//}
	//if buff.ReadBlockSize > math.MaxInt64 {
	//	panic("config.ConstConf.MyBuff.ReadBlockSize more than math.MaxInt64")
	//}
	//if buff.WriteBlockSize > math.MaxInt64 {
	//	panic("config.ConstConf.MyBuff.WriteBlockSize more than math.MaxInt64")
	//}
	//defaultBufferSize := buff.BufferSize
	//defaultReadBlockSize := buff.ReadBlockSize
	//defaultWriteBlockSize := buff.WriteBlockSize
	//servicev5.BuffConfigInit(defaultBufferSize, defaultReadBlockSize, defaultWriteBlockSize)
}

// 转发websocket的数据到tcp处理中去
func AddWebsocketHandler(urlPattern string, uri string) error {
	logger.Logger.Infof("AddWebsocketHandler urlPattern=%s, uri=%s", urlPattern, uri)
	u, err := url.Parse(uri)
	if err != nil {
		logger.Logger.Errorf("simq/main: %v", err)
		return err
	}

	h := func(ws *websocket.Conn) {
		WebsocketTcpProxy(ws, u.Scheme, u.Host)
	}
	http.Handle(urlPattern, websocket.Handler(h))
	return nil
}

/* handler that proxies websocket <-> unix domain socket */
func WebsocketTcpProxy(ws *websocket.Conn, nettype string, host string) error {
	client, err := net.Dial(nettype, host)
	if err != nil {
		return err
	}
	defer client.Close()
	defer ws.Close()
	chDone := make(chan bool)

	go func() {
		io_ws_copy(client, ws)
		chDone <- true
	}()
	go func() {
		io_copy_ws(ws, client)
		chDone <- true
	}()
	<-chDone
	return nil
}

/* start a listener that proxies websocket <-> tcp */
func ListenAndServeWebsocket(addr string) error {
	return http.ListenAndServe(addr, nil)
}

/* starts an HTTPS listener */
func ListenAndServeWebsocketSecure(addr string, cert string, key string) error {
	return http.ListenAndServeTLS(addr, cert, key, nil)
}

/* copy from websocket to writer, this copies the binary frames as is */
func io_copy_ws(src *websocket.Conn, dst io.Writer) (int, error) {
	var buffer []byte
	count := 0
	for {
		err := websocket.Message.Receive(src, &buffer)
		if err != nil {
			return count, err
		}
		n := len(buffer)
		count += n
		i, err := dst.Write(buffer)
		if err != nil || i < 1 {
			return count, err
		}
	}
}

/* copy from reader to websocket, this copies the binary frames as is */
func io_ws_copy(src io.Reader, dst *websocket.Conn) (int, error) {
	buffer := make([]byte, 2048)
	count := 0
	for {
		n, err := src.Read(buffer)
		if err != nil || n < 1 {
			return count, err
		}
		count += n
		err = websocket.Message.Send(dst, buffer[0:n])
		if err != nil {
			return count, err
		}
	}
}
