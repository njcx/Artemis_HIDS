package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"artemis_agent/pkg/gomw"
	"artemis_agent/pkg/gomw/protocol"

	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/structpb"

	pb "artemis_agent/pb"
)

// AgentID 配置
const (
	// UniqueIDLength 唯一标识符长度（不含连字符的字符数）
	// 可选值：
	// 8  - 短格式（如：A3F5-D9C2）
	// 12 - 三组格式（如：A3F5-D9C2-B1E4）
	// 16 - 四组格式（推荐，类似 Windows 激活码：A3F5-D9C2-B1E4-7F6D）
	// 20 - 五组格式（如：A3F5-D9C2-B1E4-7F6D-8C9A）
	UniqueIDLength = 16
	LogDir         = "/var/log/artemis-agent"
)

type ResourceLimit struct {
	MemoryMB   int `json:"memory_mb"`   // 内存限制(MB)
	CPUPercent int `json:"cpu_percent"` // CPU百分比
}

type WorkerConfig struct {
	Name          string        `json:"name"`
	BinaryPath    string        `json:"binary_path"`
	ResourceLimit ResourceLimit `json:"resource_limit"`
}

type WorkerInstance struct {
	config WorkerConfig
	// client       *plugin.Client
	// worker       shared.MonitorWorker
	pid          int
	process      *os.Process
	cgroupName   string
	lastSeen     time.Time
	restartCount int
	mu           sync.Mutex
}

// ============================================
// Agent Master
// ============================================

type AgentMaster struct {
	config       *Config
	configPath   string // 启动时加载配置文件的真实路径，setConfig 保存配置时复用
	workers      map[string]*WorkerInstance
	workersMux   sync.RWMutex
	cgroupMgr    *CGroupManager
	dataChannel  chan *MonitorData
	serverClient *GRPCClient
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	stopping     bool // 标记是否正在停止
	stoppingMux  sync.RWMutex
	gomwMaster   *gomw.Master
}

type TLSConfig struct {
	Enabled    bool   `json:"enabled"`     // 是否启用 TLS
	CAFile     string `json:"ca_file"`     // CA证书文件（用于验证服务器）
	CertFile   string `json:"cert_file"`   // 客户端证书文件（双向TLS时使用）
	KeyFile    string `json:"key_file"`    // 客户端密钥文件（双向TLS时使用）
	ServerName string `json:"server_name"` // 服务器名称（用于证书验证）
}

type Config struct {
	ServerAddr     string         `json:"server_addr"`
	DataBuffSize   int            `json:"data_buff_size"`
	ReportInterval int            `json:"report_interval"` // 单位：秒
	Workers        []WorkerConfig `json:"workers"`
	TLS            *TLSConfig     `json:"tls"`
}

func NewAgentMaster(configPath string) (*AgentMaster, error) {
	// 加载配置
	var config *Config

	// 记录绝对路径，供后续 setConfig/saveConfig 复用，避免相对路径随进程工作目录变化而写错位置
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		absConfigPath = configPath
	}

	// 从文件加载配置
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %v", err)
	}

	config = &Config{}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parse config file: %v", err)
	}

	log.Printf("Loaded configuration from %s", configPath)

	// 限制Master自身资源
	masterLimit := ResourceLimit{
		MemoryMB:   150,
		CPUPercent: 2,
	}

	cgroupMgr := NewCGroupManager()
	if err := cgroupMgr.CreateAndLimit("hids-master", os.Getpid(), masterLimit); err != nil {
		log.Printf("Warning: Failed to limit master resources: %v", err)
	}

	// 优化Go运行时
	debug.SetGCPercent(50) // 降低GC目标
	runtime.GOMAXPROCS(1)  // 限制使用单核

	ctx, cancel := context.WithCancel(context.Background())

	agentMaster := &AgentMaster{
		config:      config,
		configPath:  absConfigPath,
		workers:     make(map[string]*WorkerInstance),
		cgroupMgr:   cgroupMgr,
		dataChannel: make(chan *MonitorData, config.DataBuffSize),
		ctx:         ctx,
		cancel:      cancel,
	}

	// 初始化 gomw Master
	// 获取当前可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}
	execDir := filepath.Dir(execPath)
	socketPath := filepath.Join(execDir, "artemis_agent.sock")
	gomwConfig := gomw.MasterConfig{
		SocketPath:        socketPath,
		HeartbeatInterval: 5 * time.Second,
		Logger:            log.StandardLogger(),
		LogDir:            LogDir,
	}
	agentMaster.gomwMaster = gomw.NewMaster(gomwConfig)
	agentMaster.gomwMaster.SetEventHandler(agentMaster.handleWorkerEvent)
	agentMaster.gomwMaster.SetProcessStartHandler(agentMaster.handleProcessStart)

	return agentMaster, nil
}

func (m *AgentMaster) handleProcessStart(name string, pid int) {
	m.workersMux.Lock()
	defer m.workersMux.Unlock()

	instance, ok := m.workers[name]
	if !ok {
		return
	}

	instance.pid = pid
	log.Printf("Worker %s started with PID %d", name, pid)

	// 应用 CGroup
	if err := m.cgroupMgr.CreateAndLimit(instance.cgroupName, pid, instance.config.ResourceLimit); err != nil {
		log.Printf("Warning: Failed to limit worker %s: %v", name, err)
	}
}

func (m *AgentMaster) handleWorkerEvent(event *protocol.Event) {
	// 更新 Worker 状态 (LastSeen)
	m.workersMux.RLock()
	instance, exists := m.workers[event.WorkerName]
	m.workersMux.RUnlock()

	if exists {
		instance.mu.Lock()
		instance.lastSeen = time.Unix(event.Timestamp, 0)
		instance.mu.Unlock()
	}

	// 构造 MonitorData
	data := &MonitorData{
		WorkerName: event.WorkerName,
		Timestamp:  time.Unix(event.Timestamp, 0),
		EventType:  event.Type,
	}

	if m, ok := event.Data.(map[string]interface{}); ok {
		data.Data = m
	} else {
		data.Data = map[string]interface{}{
			"payload": event.Data,
		}
	}

	select {
	case m.dataChannel <- data:
	default:
		log.Warnf("Data channel full, dropping event from %s", event.WorkerName)
	}
}

func (m *AgentMaster) Start() error {
	log.Println("Starting Agent Master...")

	// 0. 启动 gomw Master
	if err := m.gomwMaster.Start(); err != nil {
		return fmt.Errorf("start gomw master: %v", err)
	}

	// 1. 连接Server
	m.serverClient = NewGRPCClient(m.config.ServerAddr, m.config.TLS)
	if err := m.serverClient.Connect(); err != nil {
		return fmt.Errorf("connect to server: %v", err)
	}
	m.serverClient.executor = NewCommandExecutor(m, m.serverClient.client)

	// 2. 启动所有Worker
	for _, workerConfig := range m.config.Workers {
		if err := m.startWorker(workerConfig); err != nil {
			log.Printf("Failed to start worker %s: %v", workerConfig.Name, err)
			continue
		}
	}

	// 3. 启动数据收集器
	m.wg.Add(1)
	go m.dataCollector()

	// 4. 启动心跳
	m.wg.Add(1)
	go m.heartbeat()

	// 6. 启动资源监控
	m.wg.Add(1)
	go m.resourceMonitor()

	// 7. 启动健康检查
	m.wg.Add(1)
	go m.healthCheck()

	log.Println("Agent Master started successfully")
	return nil
}

// startWorker 启动Worker并限制资源
func (m *AgentMaster) startWorker(config WorkerConfig) error {
	log.Printf("启动 worker: %s", config.Name)

	// 检查二进制文件是否存在
	absPath, err := filepath.Abs(config.BinaryPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %v", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("worker binary not found: %v", err)
	}

	// gomw 启动 Worker
	if err := m.gomwMaster.AddWorker(config.Name, absPath, nil); err != nil {
		return fmt.Errorf("gomw add worker: %v", err)
	}

	// 保存Worker实例
	m.workersMux.Lock()
	oldRestartCount := 0
	if oldInstance, exists := m.workers[config.Name]; exists {
		oldRestartCount = oldInstance.restartCount
	}

	instance := &WorkerInstance{
		config:       config,
		cgroupName:   fmt.Sprintf("hids-worker-%s", config.Name),
		lastSeen:     time.Now(),
		restartCount: oldRestartCount,
	}

	m.workers[config.Name] = instance
	m.workersMux.Unlock()

	log.Printf("Worker %s added to gomw", config.Name)
	return nil
}

// restartWorker 重启Worker (deprecated by gomw)
func (m *AgentMaster) restartWorker(name string) error {
	return nil
}

// maxPendingRetryBatches 发送失败后暂存待重试的批次数上限，超出丢弃最旧的一批
const maxPendingRetryBatches = 20

// sendBatchWithRetryQueue 发送一个批次；失败时暂存到重试队列，成功前优先重发队列里积压的批次。
// batch 为 nil 时表示只尝试清空已积压的重试队列，不新增批次。
func (m *AgentMaster) sendBatchWithRetryQueue(pending *[][]*MonitorData, batch []*MonitorData) {
	if batch != nil {
		*pending = append(*pending, batch)
	}

	for len(*pending) > 0 {
		next := (*pending)[0]
		if err := m.serverClient.SendBatch(next); err != nil {
			log.Printf("Send batch failed (pending retries: %d): %v", len(*pending), err)
			if len(*pending) > maxPendingRetryBatches {
				dropped := (*pending)[0]
				*pending = (*pending)[1:]
				log.Printf("Pending retry queue full, dropping oldest batch (%d events)", len(dropped))
				continue
			}
			return
		}
		*pending = (*pending)[1:]
	}
}

// dataCollector 数据收集器
func (m *AgentMaster) dataCollector() {
	defer m.wg.Done()

	batch := make([]*MonitorData, 0, 100)
	pending := make([][]*MonitorData, 0, maxPendingRetryBatches)
	ticker := time.NewTicker(time.Duration(m.config.ReportInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			if len(batch) > 0 {
				m.sendBatchWithRetryQueue(&pending, batch)
			}
			return

		case data := <-m.dataChannel:

			batch = append(batch, data)

			if len(batch) >= 100 {
				m.sendBatchWithRetryQueue(&pending, batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				m.sendBatchWithRetryQueue(&pending, batch)
				batch = batch[:0]
			} else if len(pending) > 0 {
				m.sendBatchWithRetryQueue(&pending, nil)
			}
		}
	}
}

// heartbeat 心跳
func (m *AgentMaster) heartbeat() {
	defer m.wg.Done()

	interval := time.Duration(m.config.ReportInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			status := m.collectStatus()
			if err := m.serverClient.SendHeartbeat(status); err != nil {
				log.Printf("Send heartbeat failed: %v", err)
			}

			// set_config 命令可能已经把 ReportInterval 改到了内存里，检测到变化就重置 ticker，
			// 不需要重启进程即可让新的心跳间隔生效
			if newInterval := time.Duration(m.config.ReportInterval) * time.Second; newInterval > 0 && newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
				log.Printf("Heartbeat interval updated to %s", interval)
			}
		}
	}
}

// resourceMonitor 资源监控
func (m *AgentMaster) resourceMonitor() {
	defer m.wg.Done()

	ticker := time.NewTicker(300 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.printResourceUsage()
		}
	}
}

// printResourceUsage 打印资源使用情况（使用独立的历史数据，显示 5 分钟内的平均 CPU 使用率）
func (m *AgentMaster) printResourceUsage() {
	// Master资源使用（使用 ForLog 方法，独立的历史数据）
	masterMem, masterCPU, _ := m.cgroupMgr.GetResourceUsageForLog("hids-master")

	var totalMem int64 = masterMem
	var totalCPU float64 = masterCPU

	log.Printf("=== Resource Usage (5min avg) ===")
	log.Printf("Master: Memory=%dMB/150MB, CPU=%.2f%%", masterMem, masterCPU)

	// Worker资源使用（使用 ForLog 方法，独立的历史数据）
	m.workersMux.RLock()
	for name, instance := range m.workers {
		mem, cpu, err := m.cgroupMgr.GetResourceUsageForLog(instance.cgroupName)
		if err != nil {
			log.Printf("Worker %s: Failed to get usage: %v", name, err)
			continue
		}

		totalMem += mem
		totalCPU += cpu

		limit := instance.config.ResourceLimit
		log.Printf("Worker %s: Memory=%dMB/%dMB, CPU=%.2f%%",
			name, mem, limit.MemoryMB, cpu)
	}
	m.workersMux.RUnlock()

	log.Printf("Total: Memory=%dMB/500MB, CPU=%.2f%%", totalMem, totalCPU)
	log.Printf("=================================")
}

// healthCheck 健康检查
func (m *AgentMaster) healthCheck() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// gomw handles restarts
		}
	}
}

func (m *AgentMaster) collectStatus() map[string]interface{} {
	status := make(map[string]interface{})

	m.workersMux.RLock()
	defer m.workersMux.RUnlock()

	for name, instance := range m.workers {
		mem, cpu, _ := m.cgroupMgr.GetResourceUsage(instance.cgroupName)
		status[name] = map[string]interface{}{
			"pid":           instance.pid,
			"memory_mb":     mem,
			"cpu_percent":   cpu,
			"restart_count": instance.restartCount,
			"last_seen":     instance.lastSeen,
		}
	}

	return status
}

func (m *AgentMaster) Stop() {
	log.Println("Stopping Agent Master...")

	// 0. 设置停止标志，防止健康检查重启 worker
	m.stoppingMux.Lock()
	m.stopping = true
	m.stoppingMux.Unlock()

	// 立即停止所有后台 goroutine（包括健康检查）
	if m.cancel != nil {
		m.cancel()
	}

	// 1. 停止 gomw Master
	if m.gomwMaster != nil {
		m.gomwMaster.Stop()
	}

	// 2. 等待所有 goroutine 退出
	m.wg.Wait()
	log.Println("All goroutines stopped")

	// 3. 尝试清理 Master 的 CGroup
	// 注意: master 进程自己还在 cgroup 中，删除可能会失败
	// 这是正常的，操作系统会在进程退出后自动清理
	m.cgroupMgr.Remove("hids-master")

	log.Println("Agent Master stopped")
}

// ============================================
// gRPC客户端 (与Server通信)
// ============================================

type MonitorData struct {
	WorkerName string
	Timestamp  time.Time
	EventType  string
	Data       map[string]interface{}
}

type GRPCClient struct {
	serverAddr string
	conn       *grpc.ClientConn
	tlsConfig  *TLSConfig
	client     pb.MonitorServiceClient
	agentID    string
	hostname   string
	executor   *CommandExecutor // 处理心跳响应中携带的命令，Start() 里接线
}

func NewGRPCClient(addr string, tlsConfig *TLSConfig) *GRPCClient {
	return &GRPCClient{
		serverAddr: addr,
		tlsConfig:  tlsConfig,
	}
}

func (c *GRPCClient) Connect() error {
	log.Printf("Connecting to server: %s", c.serverAddr)

	var opts []grpc.DialOption

	// 配置 TLS
	if c.tlsConfig != nil && c.tlsConfig.Enabled {
		log.Println("TLS enabled, configuring secure connection")

		// 加载 CA 证书
		caCert, err := os.ReadFile(c.tlsConfig.CAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %v", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to append CA certificate")
		}

		config := &tls.Config{
			RootCAs:    certPool,
			ServerName: c.tlsConfig.ServerName,
		}

		// 如果配置了客户端证书（双向 TLS）
		if c.tlsConfig.CertFile != "" && c.tlsConfig.KeyFile != "" {
			log.Println("Loading client certificate for mutual TLS")
			cert, err := tls.LoadX509KeyPair(c.tlsConfig.CertFile, c.tlsConfig.KeyFile)
			if err != nil {
				return fmt.Errorf("failed to load client certificate: %v", err)
			}
			config.Certificates = []tls.Certificate{cert}
		}

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	} else {
		log.Println("TLS disabled, using insecure connection")
		opts = append(opts, grpc.WithInsecure())
	}

	// 建立连接
	conn, err := grpc.Dial(c.serverAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}

	c.conn = conn
	c.client = pb.NewMonitorServiceClient(conn)

	// 获取 agent ID 和 hostname
	c.hostname, _ = os.Hostname()
	c.agentID = c.getOrCreateAgentID()

	log.Printf("Successfully connected to server (AgentID: %s)", c.agentID)
	return nil
}

// isVirtualInterface 判断是否为虚拟网络接口（Docker、VPN等）
func isVirtualInterface(name string) bool {
	name = strings.ToLower(name)

	// Docker 相关接口
	if strings.HasPrefix(name, "docker") ||
		strings.HasPrefix(name, "br-") ||
		strings.HasPrefix(name, "veth") {
		return true
	}

	// Kubernetes 相关接口
	if strings.Contains(name, "cni") ||
		strings.Contains(name, "kube") ||
		strings.Contains(name, "flannel") ||
		strings.Contains(name, "calico") {
		return true
	}

	// VPN 相关接口
	if strings.HasPrefix(name, "tun") ||
		strings.HasPrefix(name, "tap") ||
		strings.HasPrefix(name, "ppp") ||
		strings.HasPrefix(name, "wg") ||
		strings.HasPrefix(name, "utun") {
		return true
	}

	// 虚拟机相关接口
	if strings.HasPrefix(name, "vbox") ||
		strings.HasPrefix(name, "vmnet") ||
		strings.HasPrefix(name, "virbr") {
		return true
	}

	// 其他虚拟接口
	if strings.HasPrefix(name, "dummy") ||
		strings.HasPrefix(name, "vir") {
		return true
	}

	return false
}

// isVirtualIP 判断 IP 是否为虚拟网络地址段
func isVirtualIP(ip net.IP) bool {
	// Docker 默认网段: 172.17.0.0/16 到 172.31.0.0/16 (172.16.0.0/12)
	_, dockerNet, _ := net.ParseCIDR("172.16.0.0/12")
	if dockerNet.Contains(ip) {
		return true
	}

	// 链路本地地址: 169.254.0.0/16
	_, linkLocalNet, _ := net.ParseCIDR("169.254.0.0/16")
	if linkLocalNet.Contains(ip) {
		return true
	}

	return false
}

// getLocalIPs 获取本机所有非回环、非虚拟网络的真实 IP 地址
func getLocalIPs() []string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Warning: failed to get network interfaces: %v", err)
		return ips
	}

	for _, iface := range interfaces {
		// 跳过 down 状态和回环接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 跳过虚拟网络接口
		if isVirtualInterface(iface.Name) {
			log.Printf("Debug: skipping virtual interface: %s", iface.Name)
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("Warning: failed to get addresses for interface %s: %v", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// 只取 IPv4 地址，排除回环地址和虚拟网络地址
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() && !isVirtualIP(ip) {
				log.Printf("Debug: found valid IP %s on interface %s", ip.String(), iface.Name)
				ips = append(ips, ip.String())
			}
		}
	}

	return ips
}

// generateUniqueID 生成指定长度的唯一标识符
// length: 期望的ID长度（不含连字符）
// 返回格式：xxxx-xxxx-xxxx-xxxx（每4个字符一组，小写）
func generateUniqueID(length int) string {
	var rawID string

	// 生成原始ID字符串
	if length >= 32 {
		// 使用完整 UUID（不含连字符）
		rawID = strings.ReplaceAll(uuid.New().String(), "-", "")
	} else if length > 0 {
		// 使用加密安全的随机数生成
		byteLen := (length + 1) / 2
		bytes := make([]byte, byteLen)

		if _, err := rand.Read(bytes); err != nil {
			// 如果加密随机失败，降级使用 UUID 的前 N 位
			log.Printf("Warning: crypto rand failed, using UUID fallback: %v", err)
			rawID = strings.ReplaceAll(uuid.New().String(), "-", "")
			if length <= len(rawID) {
				rawID = rawID[:length]
			}
		} else {
			// 转换为十六进制字符串
			hexStr := hex.EncodeToString(bytes)
			if len(hexStr) > length {
				rawID = hexStr[:length]
			} else {
				rawID = hexStr
			}
		}
	} else {
		// 默认 16 位
		return generateUniqueID(16)
	}

	// 转换为小写
	rawID = strings.ToLower(rawID)

	// 格式化为每4个字符一组，用连字符分隔
	var formatted strings.Builder
	for i, char := range rawID {
		if i > 0 && i%4 == 0 {
			formatted.WriteRune('-')
		}
		formatted.WriteRune(char)
	}

	return formatted.String()
}

// getOrCreateAgentID 获取或创建持久化的 agent ID
// 格式: ip1-ip2-...-uniqueID
func (c *GRPCClient) getOrCreateAgentID() string {
	// 获取用户 home 目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: failed to get user home directory: %v, using current directory", err)
		homeDir = "."
	}

	idFile := filepath.Join(homeDir, ".artemis_agent_id")

	// 尝试读取已存在的 ID
	if data, err := os.ReadFile(idFile); err == nil {
		agentID := strings.TrimSpace(string(data))
		if agentID != "" {
			log.Printf("Loaded existing agent ID from %s", idFile)
			return agentID
		}
	}

	// 生成新的 ID: ip1-ip2-...-uniqueID
	ips := getLocalIPs()
	uniqueID := generateUniqueID(UniqueIDLength)

	var agentID string
	if len(ips) > 0 {
		// 将所有 IP 用 - 连接，然后加上唯一ID
		agentID = strings.Join(ips, "-") + "-" + uniqueID
	} else {
		// 如果没有获取到 IP，使用 hostname 和唯一ID
		agentID = fmt.Sprintf("%s-%s", c.hostname, uniqueID)
	}

	// 保存到文件
	if err := os.WriteFile(idFile, []byte(agentID), 0600); err != nil {
		log.Printf("Warning: failed to save agent ID to %s: %v", idFile, err)
	} else {
		log.Printf("Created new agent ID and saved to %s", idFile)
	}

	return agentID
}

// convertToStructpbValue 递归转换 Go 值为 structpb.Value
func convertToStructpbValue(v interface{}) (*structpb.Value, error) {
	switch val := v.(type) {
	case nil:
		return structpb.NewNullValue(), nil
	case bool:
		return structpb.NewBoolValue(val), nil
	case int:
		return structpb.NewNumberValue(float64(val)), nil
	case int8:
		return structpb.NewNumberValue(float64(val)), nil
	case int16:
		return structpb.NewNumberValue(float64(val)), nil
	case int32:
		return structpb.NewNumberValue(float64(val)), nil
	case int64:
		return structpb.NewNumberValue(float64(val)), nil
	case uint:
		return structpb.NewNumberValue(float64(val)), nil
	case uint8:
		return structpb.NewNumberValue(float64(val)), nil
	case uint16:
		return structpb.NewNumberValue(float64(val)), nil
	case uint32:
		return structpb.NewNumberValue(float64(val)), nil
	case uint64:
		return structpb.NewNumberValue(float64(val)), nil
	case float32:
		return structpb.NewNumberValue(float64(val)), nil
	case float64:
		return structpb.NewNumberValue(val), nil
	case string:
		return structpb.NewStringValue(val), nil
	case []interface{}:
		// 处理 []interface{} 类型的数组
		values := make([]*structpb.Value, 0, len(val))
		for _, item := range val {
			itemVal, err := convertToStructpbValue(item)
			if err != nil {
				return nil, err
			}
			values = append(values, itemVal)
		}
		return structpb.NewListValue(&structpb.ListValue{Values: values}), nil
	case []string:
		// 处理 []string 类型的数组
		values := make([]*structpb.Value, 0, len(val))
		for _, item := range val {
			values = append(values, structpb.NewStringValue(item))
		}
		return structpb.NewListValue(&structpb.ListValue{Values: values}), nil
	case []map[string]interface{}:
		// 处理 []map[string]interface{} 类型的数组
		values := make([]*structpb.Value, 0, len(val))
		for _, item := range val {
			itemStruct, err := convertToStructpbStruct(item)
			if err != nil {
				return nil, err
			}
			values = append(values, structpb.NewStructValue(itemStruct))
		}
		return structpb.NewListValue(&structpb.ListValue{Values: values}), nil
	case map[string]interface{}:
		// 处理嵌套的 map
		s, err := convertToStructpbStruct(val)
		if err != nil {
			return nil, err
		}
		return structpb.NewStructValue(s), nil
	default:
		// 对于其他类型，转换为字符串
		return structpb.NewStringValue(fmt.Sprintf("%v", val)), nil
	}
}

// convertToStructpbStruct 转换 map[string]interface{} 为 structpb.Struct
func convertToStructpbStruct(m map[string]interface{}) (*structpb.Struct, error) {
	fields := make(map[string]*structpb.Value)
	for k, v := range m {
		val, err := convertToStructpbValue(v)
		if err != nil {
			return nil, fmt.Errorf("convert field %s: %v", k, err)
		}
		fields[k] = val
	}
	return &structpb.Struct{Fields: fields}, nil
}

func (c *GRPCClient) SendBatch(data []*MonitorData) error {
	if c.client == nil {
		return fmt.Errorf("client not connected")
	}

	// 转换为 protobuf 格式
	events := make([]*pb.MonitorEvent, 0, len(data))
	for _, d := range data {
		// 使用自定义转换函数处理复杂类型
		dataStruct, err := convertToStructpbStruct(d.Data)
		if err != nil {
			log.Printf("Failed to convert data to struct: %v", err)
			continue
		}

		event := &pb.MonitorEvent{
			WorkerName: d.WorkerName,
			Timestamp:  d.Timestamp.UnixNano(),
			EventType:  d.EventType,
			Data:       dataStruct,
		}

		events = append(events, event)
	}

	// 构建请求
	req := &pb.BatchRequest{
		AgentId:  c.agentID,
		Hostname: c.hostname,
		Events:   events,
	}

	// 发送请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.SendBatch(ctx, req)
	if err != nil {
		return fmt.Errorf("send batch: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("server error: %s", resp.Message)
	}

	// log.Printf("Sent batch: %d events, server received: %d", len(data), resp.AcceptedCount)
	return nil
}

func (c *GRPCClient) SendHeartbeat(status map[string]interface{}) error {
	if c.client == nil {
		return fmt.Errorf("client not connected")
	}

	// 使用自定义转换函数处理复杂类型
	statusStruct, err := convertToStructpbStruct(status)
	if err != nil {
		return fmt.Errorf("convert status to struct: %v", err)
	}

	// 构建请求
	req := &pb.HeartbeatRequest{
		AgentId:   c.agentID,
		Hostname:  c.hostname,
		Timestamp: time.Now().UnixNano(),
		Status:    statusStruct,
	}

	// 发送请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.SendHeartbeat(ctx, req)
	if err != nil {
		return fmt.Errorf("send heartbeat: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("server error: %s", resp.Message)
	}

	// 处理服务器返回的命令
	if len(resp.Commands) > 0 {
		log.Printf("Received %d commands from server", len(resp.Commands))
		if c.executor != nil {
			// 异步执行，避免 update_agent/uninstall_agent 等耗时命令阻塞心跳循环
			go c.executor.ProcessCommands(resp.Commands)
		}
	}

	// log.Printf("Heartbeat sent successfully")
	return nil
}

// ============================================
// 主函数
// ============================================

func main() {
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	// 配置 logrus 日志
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006/01/02 15:04:05",
	})
	log.SetLevel(log.InfoLevel)

	if err := os.MkdirAll(LogDir, 0755); err != nil {
		log.Printf("Warning: Failed to create log directory %s: %v", LogDir, err)
	}

	logPath := filepath.Join(LogDir, "artemis_agent.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// 只输出到标准输出
		log.SetOutput(os.Stdout)
		log.Warnf("Cannot open log file, using stdout only: %v", err)
	} else {
		log.SetOutput(logFile)
		defer logFile.Close()
		log.Infof("Logging to %s ", logPath)
	}

	master, err := NewAgentMaster(*configPath)
	if err != nil {
		log.Fatalf("Create agent master failed: %v", err)
	}

	// 确保在任何情况下都清理资源（包括 panic）
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic recovered: %v", r)
			log.Printf("Stack trace:\n%s", debug.Stack())
		}
		log.Println("Cleaning up resources...")
		master.Stop()
	}()

	if err := master.Start(); err != nil {
		log.Fatalf("Start agent master failed: %v", err)
	}

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// 第一次信号：优雅退出
	sig := <-sigChan
	log.Printf("Received signal: %v, shutting down gracefully (press Ctrl+C again to force quit)...", sig)

	// 在 goroutine 中执行 Stop，以便可以接收第二次信号
	done := make(chan struct{})
	go func() {
		master.Stop()
		close(done)
	}()

	// 等待优雅退出完成，或收到第二次信号强制退出
	select {
	case <-done:
		log.Println("Shutdown complete")
	case sig := <-sigChan:
		log.Printf("Received second signal: %v, forcing exit...", sig)
		os.Exit(1)
	}
}

// ============================================
// 部署和使用说明
// ============================================

/*
1. 编译:
   go build -o agent main.go

2. 目录结构:
   agent/
   ├── agent (Master二进制)
   └── workers/
       ├── file_monitor/
       │   └── file_monitor (Worker二进制)
       ├── process_monitor/
       │   └── process_monitor
       └── network_monitor/
           └── network_monitor

3. 运行 (需要root权限):
   sudo ./agent

4. 验证资源限制:
   # 查看Master
   cat /sys/fs/cgroup/memory/hids-master/memory.usage_in_bytes
   cat /sys/fs/cgroup/cpu/hids-master/cpu.cfs_quota_us

   # 查看Worker
   cat /sys/fs/cgroup/memory/hids-worker-file_monitor/memory.usage_in_bytes

   # 实时监控
   watch -n 1 'ps aux | grep -E "agent|worker"'

5. 资源分配总览:
   ┌────────────────────────────────────┐
   │  总资源: 500MB + 5% CPU             │
   ├────────────────────────────────────┤
   │ Master:         150MB + 2%         │
   │  - gRPC通信                         │
   │  - 数据汇总                         │
   │  - Worker管理                       │
   ├────────────────────────────────────┤
   │ File Worker:    100MB + 1%         │
   │ Process Worker: 100MB + 1%         │
   │ Network Worker: 100MB + 1%         │
   ├────────────────────────────────────┤
   │ 预留Buffer:      50MB              │
   └────────────────────────────────────┘

6. 优化建议:
    Worker使用go-plugin, 崩溃不影响Master
    CGroup严格限制每个进程资源
    内存OOM时暂停进程(不kill), Master自动重启
    资源超限告警
    自动重启机制(最多5次)
    批量上报减少网络开销

7. 监控指标:
   - 每个Worker独立监控
   - 总资源不超过500MB
   - 单个Worker超限会被限流/暂停
   - Master监控所有Worker健康状态

8. 故障处理:
   - Worker崩溃  Master自动重启
   - Worker OOM  CGroup暂停  Master检测  重启
   - Master崩溃  systemd重启 (配置systemd服务)
   - 重启超过5次  停止该Worker  告警

9. 进一步优化:
   - 使用 pprof 分析内存热点
   - 关键路径用 sync.Pool 复用对象
   - 降低日志级别(生产环境)
   - 使用 Protobuf 替代 JSON (节省30%)
*/
