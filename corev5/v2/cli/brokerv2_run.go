package cli

import (
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/service"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
)

var once sync.Once

func Start() {
	once.Do(func() {
		// 配置初始化
		cfg, err := config.Init(true, "F:\\Go_pro\\src\\si-mqtt\\config\\config.toml")
		utils.MustPanic(err)

		// 日志初始化
		logger.LogInit(cfg.Log.Level)

		svr, err := brokerInitAndRun(cfg)
		utils.MustPanic(err)

		svr.Wait()
	})
}

// broker 初始化
func brokerInitAndRun(cfg *config.SIConfig) (*service.Server, error) {

	svr := service.NewServer(cfg)

	exitSignal(svr)
	pprof(cfg.PProf.Open, int(cfg.PProf.Port))

	var (
		mqttAddr    = cfg.Broker.TcpAddr
		wsAddr      = cfg.Broker.WsAddr
		wsPath      = cfg.Broker.WsPath
		wssAddr     = cfg.Broker.WssAddr
		wssKeyPath  = cfg.Broker.WssKeyPath
		wssCertPath = cfg.Broker.WssCertPath

		serverTaskPoolSize = cfg.Broker.ServerTaskPoolSize
	)
	// 启动 ws
	if err := wsRun(wsPath, wsAddr, wssAddr, mqttAddr, wssCertPath, wssKeyPath); err != nil {
		return nil, err
	}

	return svr, svr.ListenAndServeByGetty(mqttAddr, serverTaskPoolSize)
}

func wsRun(wsPath string, wsAddr string, wssAddr string, mqttAddr string, wssCertPath string, wssKeyPath string) error {
	if len(wsPath) > 0 && (len(wsAddr) > 0 || len(wssAddr) > 0) {
		err := AddWebsocketHandler(wsPath, mqttAddr) // 将wsAddr的ws连接数据发到 mqttAddr 上
		if err != nil {
			return err
		}
		/* start a plain websocket listener */
		if len(wsAddr) > 0 {
			go ListenAndServeWebsocket(wsAddr)
		}
		/* start a secure websocket listener */
		if len(wssAddr) > 0 && len(wssCertPath) > 0 && len(wssKeyPath) > 0 {
			go ListenAndServeWebsocketSecure(wssAddr, wssCertPath, wssKeyPath)
		}
	}
	return nil
}

func exitSignal(server *service.Server) {
	signChan := make(chan os.Signal, 1)
	//signal.Notify(sigchan, os.Interrupt, os.Kill)
	signal.Notify(signChan)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				panic(err)
			}
		}()
		sig := <-signChan
		logger.Logger.Infof("服务停止：Existing due to trapped signal; %v", sig)

		err := server.Close()
		if err != nil {
			logger.Logger.Errorf("server close err: %v", err)
		}
		os.Exit(0)
	}()
}

func pprof(open bool, port int) {
	// 性能分析
	if open {
		go func() {
			// https://pdf.us/2019/02/18/2772.html
			// go tool pprof -http=:8000 http://localhost:8080/debug/pprof/heap    查看内存使用
			// go tool pprof -http=:8000 http://localhost:8080/debug/pprof/profile 查看cpu占用
			// 注意，需要提前安装 Graphviz 用于画图
			logger.Logger.Info(http.ListenAndServe(":"+strconv.Itoa(port), nil).Error())
		}()
	}
}
