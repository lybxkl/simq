package cli

import (
	"SI-MQTT/comment"
	"SI-MQTT/config"
	"SI-MQTT/core/logger"
	"SI-MQTT/core/service"
	"SI-MQTT/core/utils"
	"flag"
	"fmt"
	"golang.org/x/net/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
)

var (
	keepAlive        int
	connectTimeout   int
	ackTimeout       int
	timeoutRetries   int
	authenticator    string
	sessionsProvider string
	topicsProvider   string
	cpuprofile       string
	wsAddr           string // HTTPS websocket address eg. :8080
	wssAddr          string // HTTPS websocket address, eg. :8081
	wssCertPath      string // path to HTTPS public key
	wssKeyPath       string // path to HTTPS private key
)

func init() {
	consts := config.ConstConf
	authenticator = consts.DefaultConst.Authenticator
	sessionsProvider = consts.DefaultConst.SessionsProvider
	topicsProvider = consts.DefaultConst.TopicsProvider
	cpuprofile = utils.GetCurrentDirectory() + "/pprof_file/cpu.txt"
	flag.IntVar(&keepAlive, "keepalive", comment.DefaultKeepAlive, "Keepalive (sec)")
	flag.IntVar(&connectTimeout, "connecttimeout", comment.DefaultConnectTimeout, "Connect Timeout (sec)")
	flag.IntVar(&ackTimeout, "acktimeout", comment.DefaultAckTimeout, "Ack Timeout (sec)")
	flag.IntVar(&timeoutRetries, "retries", comment.DefaultTimeoutRetries, "Timeout Retries")
	//权限认证的
	flag.StringVar(&authenticator, "auth", authenticator, "Authenticator Type")
	//下面两个的value要改都要改
	flag.StringVar(&sessionsProvider, "sessions", sessionsProvider, "Session Provider Type")
	flag.StringVar(&topicsProvider, "topics", topicsProvider, "Topics Provider Type")

	flag.StringVar(&cpuprofile, "cpuprofile", "", "CPU Profile Filename")
	flag.StringVar(&wsAddr, "wsaddr", "", "HTTP websocket address, eg. ':8080'")
	flag.StringVar(&wssAddr, "wssaddr", "", "HTTPS websocket address, eg. ':8081'")
	flag.StringVar(&wssCertPath, "wsscertpath", "", "HTTPS server public key file")
	flag.StringVar(&wssKeyPath, "wsskeypath", "", "HTTPS server private key file")
	flag.Parse()
}

func Run() {
	svr := &service.Server{
		KeepAlive:        keepAlive,
		ConnectTimeout:   connectTimeout,
		AckTimeout:       ackTimeout,
		TimeoutRetries:   timeoutRetries,
		SessionsProvider: sessionsProvider,
		TopicsProvider:   topicsProvider,
		Authenticator:    authenticator,
	}

	var f *os.File
	var err error

	if cpuprofile != "" {
		f, err = os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
	}
	sigchan := make(chan os.Signal, 1)
	//signal.Notify(sigchan, os.Interrupt, os.Kill)
	signal.Notify(sigchan)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				panic(err)
			}
		}()
		sig := <-sigchan
		logger.Logger.Info(fmt.Sprintf("服务停止：Existing due to trapped signal; %v", sig))

		if f != nil {
			logger.Logger.Info("Stopping profile")
			pprof.StopCPUProfile()
			f.Close()
		}

		err := svr.Close()
		if err != nil {
			logger.Logger.Error(fmt.Sprintf("server close err: %v", err))
		}
		os.Exit(0)
	}()
	mqttaddr := "tcp://:1883"
	if strings.TrimSpace(config.ConstConf.BrokerUrl) != "" {
		mqttaddr = strings.TrimSpace(config.ConstConf.BrokerUrl)
	}
	wsAddr := ""
	if strings.TrimSpace(config.ConstConf.WsBrokerUrl) != "" {
		wsAddr = strings.TrimSpace(config.ConstConf.WsBrokerUrl)
	}
	BuffConfigInit()
	if len(wsAddr) > 0 || len(wssAddr) > 0 {
		AddWebsocketHandler("/mqtt", mqttaddr) // 将wsAddr的ws连接数据发到mqttaddr上

		/* start a plain websocket listener */
		if len(wsAddr) > 0 {
			go ListenAndServeWebsocket(wsAddr)
		}
		/* start a secure websocket listener */
		if len(wssAddr) > 0 && len(wssCertPath) > 0 && len(wssKeyPath) > 0 {
			go ListenAndServeWebsocketSecure(wssAddr, wssCertPath, wssKeyPath)
		}
	}

	/* create plain MQTT listener */
	err = svr.ListenAndServe(mqttaddr)
	if err != nil {
		logger.Logger.Error(fmt.Sprintf("MQTT 启动异常错误 surgemq/main: %v", err))
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
	//service.BuffConfigInit(defaultBufferSize, defaultReadBlockSize, defaultWriteBlockSize)
}

func AddWebsocketHandler(urlPattern string, uri string) error {
	logger.Logger.Info(fmt.Sprintf("AddWebsocketHandler urlPattern=%s, uri=%s", urlPattern, uri))
	u, err := url.Parse(uri)
	if err != nil {
		logger.Logger.Error(fmt.Sprintf("go-mqtt-v2/main: %v", err))
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
