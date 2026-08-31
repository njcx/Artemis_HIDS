package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"artemis_agent/shared"

	"github.com/florianl/go-conntrack"
)

// NetworkMonitorWorker 网络监控Worker (使用 conntrack)
type NetworkMonitorWorker struct {
	ctx             context.Context
	cancel          context.CancelFunc
	healthy         atomic.Bool
	checkCount      atomic.Uint64
	suspiciousCount atomic.Uint64
	startTime       time.Time
	mu              sync.RWMutex
	callback        shared.DataCallback

	// conntrack 连接
	nfct *conntrack.Nfct

	// 连接跟踪
	seenConnections map[string]bool

	// 本机IP地址列表
	localIPs map[string]bool

	// 事件队列
	eventQueue   []*shared.MonitorData
	queueMutex   sync.Mutex
	maxQueueSize int
}

type ConnectionInfo struct {
	SrcIP    string
	SrcPort  uint16
	DstIP    string
	DstPort  uint16
	Protocol string
	Status   string
}

func NewNetworkMonitorWorker() *NetworkMonitorWorker {
	w := &NetworkMonitorWorker{
		startTime:       time.Now(),
		seenConnections: make(map[string]bool),
		localIPs:        make(map[string]bool),
		eventQueue:      make([]*shared.MonitorData, 0, 1000),
		maxQueueSize:    1000,
	}
	w.healthy.Store(true)

	// 获取本机IP地址
	w.updateLocalIPs()

	return w
}

func (w *NetworkMonitorWorker) Name() string {
	return "network_monitor"
}

// updateLocalIPs 获取本机所有网卡的IP地址
func (w *NetworkMonitorWorker) updateLocalIPs() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.localIPs = make(map[string]bool)

	// 添加回环地址
	w.localIPs["127.0.0.1"] = true
	w.localIPs["::1"] = true

	// 获取所有网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("[NetworkMonitor] Failed to get interfaces: %v", err)
		return
	}

	for _, iface := range interfaces {
		// 跳过禁用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				ipStr := ipNet.IP.String()
				w.localIPs[ipStr] = true
				log.Printf("[NetworkMonitor] Local IP detected: %s (%s)", ipStr, iface.Name)
			}
		}
	}
}

func (w *NetworkMonitorWorker) Start(ctx context.Context, callback shared.DataCallback) error {
	// 确保 log 输出到 stderr，以便被主进程捕获
	log.SetOutput(os.Stderr)

	log.Println("[NetworkMonitor] Starting...")

	w.callback = callback
	w.ctx, w.cancel = context.WithCancel(ctx)

	// 初始化 conntrack
	nfct, err := conntrack.Open(&conntrack.Config{})
	if err != nil {
		return fmt.Errorf("failed to open conntrack: %v", err)
	}
	w.nfct = nfct

	// 启动父进程监控
	go shared.WatchParentProcess(w.ctx)

	// 启动事件监听
	go w.listenEvents()

	// 启动定期扫描（作为补充）
	go w.scanLoop()

	// 定期更新本机IP列表（处理DHCP/网络变化）
	go w.ipUpdateLoop()

	log.Println("[NetworkMonitor] Started successfully")
	return nil
}

// ipUpdateLoop 定期更新本机IP列表
func (w *NetworkMonitorWorker) ipUpdateLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.updateLocalIPs()
		}
	}
}

// listenEvents 实时监听 conntrack 事件
func (w *NetworkMonitorWorker) listenEvents() {
	log.Println("[NetworkMonitor] Starting conntrack event listener...")

	// 订阅新连接事件 - 增大缓冲区防止阻塞
	events := make(chan conntrack.Con, 2000)
	droppedEvents := uint64(0)

	err := w.nfct.Register(w.ctx, conntrack.Conntrack, conntrack.NetlinkCtNew, func(c conntrack.Con) int {
		// 使用非阻塞发送，防止回调阻塞导致内核 netlink 缓冲区溢出
		select {
		case events <- c:
			// 成功发送
		default:
			// channel 满了，丢弃事件，避免阻塞
			droppedEvents++
			if droppedEvents%1000 == 0 {
				log.Printf("[NetworkMonitor] WARNING: Event channel full, dropped %d events total", droppedEvents)
			}
		}
		return 0
	})
	if err != nil {
		log.Printf("[NetworkMonitor] Failed to register for events: %v", err)
		return
	}

	for {
		select {
		case <-w.ctx.Done():
			return
		case conn := <-events:
			w.handleConntrackEvent(conn)
		}
	}
}

// handleConntrackEvent 处理 conntrack 事件
func (w *NetworkMonitorWorker) handleConntrackEvent(conn conntrack.Con) {
	w.checkCount.Add(1)

	info := w.parseConntrackFlow(conn)
	if info == nil || w.shouldSkipConnection(info) {
		return
	}

	// 生成连接唯一标识
	key := fmt.Sprintf("%s:%d-%s:%d-%s",
		info.SrcIP, info.SrcPort, info.DstIP, info.DstPort, info.Protocol)

	// 检查是否已处理过
	w.mu.Lock()
	if w.seenConnections[key] {
		w.mu.Unlock()
		return
	}
	w.seenConnections[key] = true
	w.mu.Unlock()

	// 处理新连接
	w.handleNewConnection(info)
}

// parseConntrackFlow 解析 conntrack 流信息
func (w *NetworkMonitorWorker) parseConntrackFlow(conn conntrack.Con) *ConnectionInfo {
	if conn.Origin == nil || conn.Origin.Proto == nil || conn.Origin.Proto.Number == nil {
		return nil
	}

	protocol := getProtocolName(*conn.Origin.Proto.Number)

	info := &ConnectionInfo{
		Protocol: protocol,
		Status:   "ESTABLISHED",
	}

	// 解析源地址和端口
	if conn.Origin.Src != nil {
		info.SrcIP = conn.Origin.Src.String()
	}
	if conn.Origin.Proto.SrcPort != nil {
		info.SrcPort = *conn.Origin.Proto.SrcPort
	}

	// 解析目标地址和端口
	if conn.Origin.Dst != nil {
		info.DstIP = conn.Origin.Dst.String()
	}
	if conn.Origin.Proto.DstPort != nil {
		info.DstPort = *conn.Origin.Proto.DstPort
	}

	return info
}

func getProtocolName(protoNum uint8) string {
	switch protoNum {
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	case 1:
		return "ICMP"
	default:
		return fmt.Sprintf("PROTO_%d", protoNum)
	}
}

func (w *NetworkMonitorWorker) scanLoop() {
	// 每60秒扫描一次（补充实时监听）
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	log.Printf("[NetworkMonitor] scanLoop started, will scan every 60 seconds")

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.scanConnections()
		}
	}
}

func (w *NetworkMonitorWorker) scanConnections() {
	// 获取所有连接
	sessions, err := w.nfct.Dump(conntrack.Conntrack, conntrack.IPv4)
	if err != nil {
		log.Printf("[NetworkMonitor] Failed to dump conntrack: %v", err)
		return
	}

	log.Printf("[NetworkMonitor] Scanned %d conntrack entries", len(sessions))

	for _, session := range sessions {
		info := w.parseConntrackFlow(session)
		if info == nil || w.shouldSkipConnection(info) {
			continue
		}

		// 生成连接唯一标识
		key := fmt.Sprintf("%s:%d-%s:%d-%s",
			info.SrcIP, info.SrcPort, info.DstIP, info.DstPort, info.Protocol)

		w.mu.Lock()
		if !w.seenConnections[key] {
			w.seenConnections[key] = true
			w.mu.Unlock()
			w.handleNewConnection(info)
		} else {
			w.mu.Unlock()
		}
	}
}

// shouldSkipConnection 判断是否应该跳过该连接
func (w *NetworkMonitorWorker) shouldSkipConnection(info *ConnectionInfo) bool {
	// 跳过回环地址连接 (127.0.0.1, ::1)
	if info.SrcIP == "127.0.0.1" || info.SrcIP == "::1" {
		return true
	}
	if info.DstIP == "127.0.0.1" || info.DstIP == "::1" {
		return true
	}

	// 跳过空地址
	if info.DstIP == "" || info.DstIP == "0.0.0.0" || info.DstIP == "::" {
		return true
	}

	// **关键：只监控源IP或目标IP是本机IP的连接**
	w.mu.RLock()
	isSrcLocal := w.localIPs[info.SrcIP]
	isDstLocal := w.localIPs[info.DstIP]
	w.mu.RUnlock()

	// 如果源IP和目标IP都不是本机IP，则跳过（过滤转发流量）
	if !isSrcLocal && !isDstLocal {
		return true
	}

	return false
}

func (w *NetworkMonitorWorker) handleNewConnection(info *ConnectionInfo) {
	severity := w.calculateConnectionSeverity(info)

	data := &shared.MonitorData{
		WorkerName: w.Name(),
		Timestamp:  time.Now(),
		EventType:  "connection_established",
		Severity:   severity,
		Data: map[string]interface{}{
			"src_ip":   info.SrcIP,
			"src_port": info.SrcPort,
			"dst_ip":   info.DstIP,
			"dst_port": info.DstPort,
			"protocol": info.Protocol,
			"status":   info.Status,
		},
	}

	if severity >= 3 {
		w.suspiciousCount.Add(1)
		log.Printf("[NetworkMonitor] Suspicious connection: %s:%d -> %s:%d (%s) (severity: %d)",
			info.SrcIP, info.SrcPort, info.DstIP, info.DstPort, info.Protocol, severity)
	}

	if w.callback != nil {
		if err := w.callback.Send(data); err != nil {
			log.Printf("[NetworkMonitor] Failed to send data: %v", err)
			// 发送失败，添加到队列作为备份
			w.addToQueue(data)
		}
	} else {
		w.addToQueue(data)
	}
}

func (w *NetworkMonitorWorker) calculateConnectionSeverity(info *ConnectionInfo) int {
	// 连接到可疑端口 - 高危
	suspiciousPorts := []uint16{4444, 5555, 6666, 7777, 8888, 31337}
	for _, port := range suspiciousPorts {
		if info.DstPort == port {
			return 5
		}
	}

	// 连接到Tor网络 - 中等
	if info.DstPort == 9001 || info.DstPort == 9030 {
		return 3
	}

	// 非标准SSH端口 - 低
	if info.DstPort != 22 && info.DstPort >= 2200 && info.DstPort <= 2299 {
		return 2
	}

	// 检查是否是私网地址
	if isPrivateIP(info.DstIP) {
		return 1
	}

	// 外网连接 - 信息级
	return 1
}

func parseIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range private {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

func (w *NetworkMonitorWorker) Stop() error {
	log.Println("[NetworkMonitor] Stopping...")
	if w.cancel != nil {
		w.cancel()
	}
	if w.nfct != nil {
		w.nfct.Close()
	}
	return nil
}

func (w *NetworkMonitorWorker) GetEvents() ([]*shared.MonitorData, error) {
	w.queueMutex.Lock()
	defer w.queueMutex.Unlock()

	events := make([]*shared.MonitorData, len(w.eventQueue))
	copy(events, w.eventQueue)
	w.eventQueue = w.eventQueue[:0]

	return events, nil
}

// addToQueue 添加事件到队列
func (w *NetworkMonitorWorker) addToQueue(data *shared.MonitorData) {
	w.queueMutex.Lock()
	defer w.queueMutex.Unlock()

	if len(w.eventQueue) >= w.maxQueueSize {
		// 队列满了，移除最旧的事件
		w.eventQueue = w.eventQueue[1:]
	}
	w.eventQueue = append(w.eventQueue, data)
}

func (w *NetworkMonitorWorker) GetMetrics() (map[string]interface{}, error) {
	uptime := time.Since(w.startTime)
	checkCount := w.checkCount.Load()
	suspiciousCount := w.suspiciousCount.Load()

	w.mu.RLock()
	connCount := len(w.seenConnections)
	w.mu.RUnlock()

	w.queueMutex.Lock()
	queueLen := len(w.eventQueue)
	w.queueMutex.Unlock()

	return map[string]interface{}{
		"uptime_seconds":   uptime.Seconds(),
		"check_count":      checkCount,
		"suspicious_count": suspiciousCount,
		"connection_count": connCount,
		"queue_length":     queueLen,
		"max_queue":        w.maxQueueSize,
		"healthy":          w.healthy.Load(),
	}, nil
}

func (w *NetworkMonitorWorker) Health() bool {
	return w.healthy.Load()
}

func (w *NetworkMonitorWorker) HealthCheck() (bool, string) {
	healthy := w.healthy.Load()
	if !healthy {
		return false, "worker unhealthy"
	}

	uptime := time.Since(w.startTime)
	checkCount := w.checkCount.Load()

	w.mu.RLock()
	connCount := len(w.seenConnections)
	w.mu.RUnlock()

	return true, fmt.Sprintf("uptime=%s checks=%d suspicious=%d connections=%d",
		uptime.Round(time.Second), checkCount, w.suspiciousCount.Load(), connCount)
}

// 实现 gomw 接口
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--standalone" {
		shared.StartStandalone(NewNetworkMonitorWorker())
		return
	}
	shared.StartGomwWorker("network_monitor", "1.0.0", NewNetworkMonitorWorker())
}
