package gomw

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"artemis_agent/pkg/gomw/protocol"
	"artemis_agent/pkg/gomw/transport"

	"github.com/sirupsen/logrus"
)

// generateToken 生成一个随机注册 token
func generateToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// MasterConfig Master 配置
type MasterConfig struct {
	SocketPath        string
	HeartbeatInterval time.Duration
	Logger            *logrus.Logger
	LogDir            string
}

// WorkerProcess 代表一个运行中的 Worker 进程
type WorkerProcess struct {
	Name        string
	BinaryPath  string
	Args        []string
	Cmd         *exec.Cmd
	Process     *os.Process
	Status      string
	LastContact time.Time
	Token       string

	trans    *transport.Transport
	mu       sync.RWMutex
	stopChan chan struct{}
}

// EventHandler 事件处理函数
type EventHandler func(event *protocol.Event)

// ProcessStartHandler 进程启动回调函数
type ProcessStartHandler func(name string, pid int)

// Master 管理者
type Master struct {
	config              MasterConfig
	listener            net.Listener
	workers             map[string]*WorkerProcess
	workersMu           sync.RWMutex
	stopChan            chan struct{}
	stopOnce            sync.Once // 保护 stopChan
	logger              *logrus.Logger
	eventHandler        EventHandler
	processStartHandler ProcessStartHandler
}

// NewMaster 创建 Master
func NewMaster(config MasterConfig) *Master {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	return &Master{
		config:   config,
		workers:  make(map[string]*WorkerProcess),
		stopChan: make(chan struct{}),
		logger:   config.Logger,
	}
}

// SetEventHandler 设置事件处理回调
func (m *Master) SetEventHandler(handler EventHandler) {
	m.eventHandler = handler
}

// SetProcessStartHandler 设置进程启动回调
func (m *Master) SetProcessStartHandler(handler ProcessStartHandler) {
	m.processStartHandler = handler
}

// Start 启动 Master
func (m *Master) Start() error {
	m.logger.Info("Starting Master...")

	// 清理旧 socket
	if err := os.RemoveAll(m.config.SocketPath); err != nil {
		return fmt.Errorf("failed to remove old socket: %v", err)
	}

	// 创建目录
	if err := os.MkdirAll(filepath.Dir(m.config.SocketPath), 0755); err != nil {
		return fmt.Errorf("failed to create socket dir: %v", err)
	}

	// 监听 UDS
	lis, err := net.Listen("unix", m.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %v", err)
	}
	// 权限设置：Master 与 worker 同属一个服务账号，无需对其他本地用户开放
	os.Chmod(m.config.SocketPath, 0700)

	m.listener = lis
	m.logger.Infof("Master listening on %s", m.config.SocketPath)

	// 启动接收循环
	go m.acceptLoop()

	return nil
}

func (m *Master) acceptLoop() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			select {
			case <-m.stopChan:
				return // 正常退出
			default:
				// 检查 listener 是否已关闭
				if m.listener == nil {
					return
				}
				// 忽略 use of closed network connection 错误
				return
			}
		}

		go m.handleConnection(conn)
	}
}

func (m *Master) handleConnection(conn net.Conn) {
	trans := transport.NewTransport(conn)
	defer trans.Close()

	// 握手阶段：必须首先发送 RegisterRequest
	msgType, payload, err := trans.ReadMessage()
	if err != nil {
		m.logger.Errorf("Handshake read error: %v", err)
		return
	}

	if msgType != protocol.MsgRegisterRequest {
		m.logger.Errorf("Expected RegisterRequest, got %d", msgType)
		return
	}

	req, ok := payload.(*protocol.RegisterRequest)
	if !ok {
		m.logger.Errorf("Invalid payload type for RegisterRequest")
		return
	}

	m.logger.Infof("Worker registering: %s (PID: %d)", req.Name, req.Pid)

	// 查找 Worker 记录
	m.workersMu.RLock()
	wp, exists := m.workers[req.Name]
	m.workersMu.RUnlock()

	if !exists {
		m.logger.Warnf("Unknown worker attempting to register: %s", req.Name)
		// 可以选择拒绝，或者接受（如果是动态 Worker）
		// 这里我们拒绝
		trans.WriteMessage(protocol.MsgRegisterResponse, protocol.RegisterResponse{
			Success: false,
			Message: "Unknown worker name",
		})
		return
	}

	wp.mu.RLock()
	expectedToken := wp.Token
	expectedPid := 0
	if wp.Process != nil {
		expectedPid = wp.Process.Pid
	}
	wp.mu.RUnlock()

	if expectedToken == "" || req.Token != expectedToken {
		m.logger.Errorf("Worker %s registered with invalid token", req.Name)
		trans.WriteMessage(protocol.MsgRegisterResponse, protocol.RegisterResponse{
			Success: false,
			Message: "invalid token",
		})
		return
	}

	if pid, ok := peerPid(conn); ok && expectedPid != 0 && pid != expectedPid {
		m.logger.Errorf("Worker %s PID mismatch: peer=%d expected=%d", req.Name, pid, expectedPid)
		trans.WriteMessage(protocol.MsgRegisterResponse, protocol.RegisterResponse{
			Success: false,
			Message: "pid mismatch",
		})
		return
	}

	// 注册成功
	wp.mu.Lock()
	wp.trans = trans // 绑定 Transport
	wp.Status = "active"
	wp.LastContact = time.Now()
	wp.mu.Unlock()

	// 发送响应
	err = trans.WriteMessage(protocol.MsgRegisterResponse, protocol.RegisterResponse{
		Success:           true,
		Message:           "OK",
		HeartbeatInterval: m.config.HeartbeatInterval,
	})
	if err != nil {
		m.logger.Errorf("Failed to send register response: %v", err)
		return
	}

	// 进入消息循环
	for {
		msgType, payload, err := trans.ReadMessage()
		if err != nil {
			if err != io.EOF {
				m.logger.Errorf("Connection error from %s: %v", req.Name, err)
			}
			break
		}

		wp.mu.Lock()
		wp.LastContact = time.Now()
		wp.mu.Unlock()

		switch msgType {
		case protocol.MsgHeartbeat:
			hb := payload.(*protocol.Heartbeat)
			// m.logger.Debugf("Heartbeat from %s: cpu=%.1f%%", hb.WorkerName, hb.CpuUsage)
			// 可以更新更详细的状态
			_ = hb

		case protocol.MsgEvent:
			evt := payload.(*protocol.Event)
			// m.logger.Infof("Event from %s: %s", evt.WorkerName, evt.Type)
			if m.eventHandler != nil {
				m.eventHandler(evt)
			}

		default:
			m.logger.Warnf("Unexpected message type from %s: %d", req.Name, msgType)
		}
	}

	m.logger.Infof("Connection closed for worker %s", req.Name)
	wp.mu.Lock()
	if wp.trans == trans {
		wp.trans = nil // 清理连接引用
		wp.Status = "disconnected"
	}
	wp.mu.Unlock()
}

// AddWorker 添加并启动 Worker
func (m *Master) AddWorker(name, binaryPath string, args []string) error {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	if _, exists := m.workers[name]; exists {
		return fmt.Errorf("worker %s already exists", name)
	}

	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("failed to generate worker token: %v", err)
	}

	wp := &WorkerProcess{
		Name:       name,
		BinaryPath: binaryPath,
		Args:       args,
		Status:     "starting",
		Token:      token,
		stopChan:   make(chan struct{}),
	}
	m.workers[name] = wp

	go m.runWorker(wp)
	return nil
}

// runWorker 运行并监控 Worker 进程
func (m *Master) runWorker(wp *WorkerProcess) {
	for {
		select {
		case <-m.stopChan:
			return
		case <-wp.stopChan:
			return
		default:
			cmd := exec.Command(wp.BinaryPath, wp.Args...)
			cmd.Env = append(os.Environ(), fmt.Sprintf("GOMW_SOCKET=%s", m.config.SocketPath))
			cmd.Env = append(cmd.Env, fmt.Sprintf("GOMW_WORKER_NAME=%s", wp.Name))
			cmd.Env = append(cmd.Env, fmt.Sprintf("GOMW_TOKEN=%s", wp.Token))
			var logFile *os.File
			if m.config.LogDir != "" {
				logPath := filepath.Join(m.config.LogDir, fmt.Sprintf("worker-%s.log", wp.Name))
				f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err == nil {
					logFile = f
					cmd.Stdout = f
					cmd.Stderr = f
				} else {
					m.logger.Errorf("Failed to open log file: %v", err)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
				}
			} else {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			}

			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			m.logger.Infof("Starting worker %s...", wp.Name)
			if err := cmd.Start(); err != nil {
				m.logger.Errorf("Failed to start worker %s: %v", wp.Name, err)
				if logFile != nil {
					logFile.Close()
				}
				time.Sleep(5 * time.Second)
				continue
			}

			// 进程启动成功，触发回调
			if m.processStartHandler != nil {
				// 异步调用，避免阻塞
				go m.processStartHandler(wp.Name, cmd.Process.Pid)
			}

			wp.mu.Lock()
			wp.Process = cmd.Process
			wp.mu.Unlock()

			cmd.Wait()
			m.logger.Warnf("Worker %s exited", wp.Name)

			if logFile != nil {
				logFile.Close()
			}

			wp.mu.Lock()
			wp.Process = nil
			wp.trans = nil
			wp.Status = "stopped"
			wp.mu.Unlock()

			time.Sleep(3 * time.Second)
		}
	}
}

// SendCommand 发送指令给 Worker
func (m *Master) SendCommand(workerName string, cmdType string, payload interface{}) error {
	m.workersMu.RLock()
	wp, exists := m.workers[workerName]
	m.workersMu.RUnlock()

	if !exists {
		return fmt.Errorf("worker %s not found", workerName)
	}

	wp.mu.RLock()
	trans := wp.trans
	wp.mu.RUnlock()

	if trans == nil {
		return fmt.Errorf("worker %s not connected", workerName)
	}

	return trans.WriteMessage(protocol.MsgCommand, protocol.Command{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:    cmdType,
		Payload: payload,
	})
}

// StopWorker 停止指定 Worker
func (m *Master) StopWorker(name string) error {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	wp, ok := m.workers[name]
	if !ok {
		return fmt.Errorf("worker not found")
	}

	// 触发退出
	close(wp.stopChan)

	// 如果进程还在，发送信号
	wp.mu.Lock()
	if wp.Process != nil {
		wp.Process.Signal(syscall.SIGTERM)
	}
	wp.mu.Unlock()

	delete(m.workers, name)
	return nil
}

// Stop 停止
func (m *Master) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
		if m.listener != nil {
			m.listener.Close()
		}

		m.workersMu.RLock()
		defer m.workersMu.RUnlock()

		for _, wp := range m.workers {
			// 关闭 stopChan 可能导致重复关闭（因为 StopWorker 也会关闭它）
			// 但我们这里是 Master 级停止，应该直接杀进程
			// wp.stopChan 的关闭应该由 runWorker 检测 m.stopChan 来处理

			// 这里我们只需要发送信号
			wp.mu.Lock()
			if wp.Process != nil {
				wp.Process.Signal(syscall.SIGTERM)
			}
			wp.mu.Unlock()
		}
	})
}
