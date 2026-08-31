package gomw

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"

	"artemis_agent/pkg/gomw/protocol"
	"artemis_agent/pkg/gomw/transport"

	"github.com/sirupsen/logrus"
)

// WorkerHandler 指令处理器接口
type WorkerHandler interface {
	HandleCommand(cmd *protocol.Command) error
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
	Name       string
	Version    string
	SocketPath string
	Token      string
	Logger     *logrus.Logger
}

// Worker 客户端
type Worker struct {
	config     WorkerConfig
	trans      *transport.Transport
	logger     *logrus.Logger
	stopChan   chan struct{}
	hbInterval time.Duration
}

// NewWorker 创建 Worker
func NewWorker(config WorkerConfig) (*Worker, error) {
	if config.SocketPath == "" {
		config.SocketPath = os.Getenv("GOMW_SOCKET")
	}
	if config.Name == "" {
		config.Name = os.Getenv("GOMW_WORKER_NAME")
	}
	if config.Token == "" {
		config.Token = os.Getenv("GOMW_TOKEN")
	}

	if config.SocketPath == "" {
		return nil, fmt.Errorf("socket path required")
	}

	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	return &Worker{
		config:   config,
		logger:   config.Logger,
		stopChan: make(chan struct{}),
	}, nil
}

// Start 启动 Worker
func (w *Worker) Start(handler WorkerHandler) error {
	w.logger.Infof("Worker %s starting...", w.config.Name)

	// 连接 Master
	conn, err := net.Dial("unix", w.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to master: %v", err)
	}

	w.trans = transport.NewTransport(conn)
	defer w.trans.Close()

	// 注册
	if err := w.register(); err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}

	// 启动心跳
	go w.heartbeatLoop()

	// 接收指令循环
	for {
		msgType, payload, err := w.trans.ReadMessage()
		if err != nil {
			if err == io.EOF {
				w.logger.Info("Master closed connection")
				return nil
			}
			w.logger.Errorf("Read error: %v", err)
			return err
		}

		switch msgType {
		case protocol.MsgCommand:
			cmd := payload.(*protocol.Command)
			if handler != nil {
				if err := handler.HandleCommand(cmd); err != nil {
					w.logger.Errorf("Handle command error: %v", err)
				}
			}
		default:
			w.logger.Warnf("Unexpected message type: %d", msgType)
		}
	}
}

func (w *Worker) register() error {
	req := protocol.RegisterRequest{
		Name:    w.config.Name,
		Pid:     os.Getpid(),
		Version: w.config.Version,
		Token:   w.config.Token,
	}

	if err := w.trans.WriteMessage(protocol.MsgRegisterRequest, req); err != nil {
		return err
	}

	// 等待响应
	msgType, payload, err := w.trans.ReadMessage()
	if err != nil {
		return err
	}

	if msgType != protocol.MsgRegisterResponse {
		return fmt.Errorf("expected register response, got %d", msgType)
	}

	resp := payload.(*protocol.RegisterResponse)
	if !resp.Success {
		return fmt.Errorf("register refused: %s", resp.Message)
	}

	w.hbInterval = resp.HeartbeatInterval
	if w.hbInterval == 0 {
		w.hbInterval = 5 * time.Second
	}

	w.logger.Infof("Registered successfully, heartbeat: %v", w.hbInterval)
	return nil
}

func (w *Worker) heartbeatLoop() {
	ticker := time.NewTicker(w.hbInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			hb := protocol.Heartbeat{
				WorkerName: w.config.Name,
				Timestamp:  time.Now().Unix(),
				Status:     "running",
				MemUsage:   memStats.Alloc,
			}

			if err := w.trans.WriteMessage(protocol.MsgHeartbeat, hb); err != nil {
				w.logger.Errorf("Heartbeat failed: %v", err)
				return // 外部循环会捕获到连接错误
			}
		}
	}
}

// ReportEvent 上报事件
func (w *Worker) ReportEvent(evtType string, data interface{}) error {
	return w.ReportEventWithName(w.config.Name, evtType, data)
}

// ReportEventWithName 上报事件（指定 WorkerName）
func (w *Worker) ReportEventWithName(workerName, evtType string, data interface{}) error {
	if w.trans == nil {
		return fmt.Errorf("not connected")
	}

	evt := protocol.Event{
		WorkerName: workerName,
		Type:       evtType,
		Timestamp:  time.Now().Unix(),
		Data:       data,
	}

	return w.trans.WriteMessage(protocol.MsgEvent, evt)
}
