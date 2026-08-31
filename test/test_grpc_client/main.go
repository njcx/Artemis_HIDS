// ============================================
// HIDS Test gRPC Client
// 模拟 Agent 向 Server 发送数据
// ============================================
package main

import (
	"artemis-hids/server/pb"
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// 命令行参数
var (
	serverAddr    = flag.String("server", "localhost:50051", "Server address")
	agentID       = flag.String("agent-id", "", "Agent ID (auto-generated if empty)")
	hostname      = flag.String("hostname", "", "Hostname (auto-generated if empty)")
	heartbeatInt  = flag.Int("heartbeat", 30, "Heartbeat interval in seconds")
	batchInt      = flag.Int("batch", 5, "Batch send interval in seconds")
	batchSize     = flag.Int("batch-size", 10, "Number of events per batch")
	enableFile    = flag.Bool("file", true, "Enable file monitor events")
	enableProcess = flag.Bool("process", true, "Enable process monitor events")
	enableNetwork = flag.Bool("network", true, "Enable network monitor events")
	// TLS 配置
	enableTLS  = flag.Bool("tls", true, "Enable TLS")
	caFile     = flag.String("ca-file", "../../server/certs/ca-cert.pem", "CA certificate file")
	certFile   = flag.String("cert-file", "../../server/certs/client-cert.pem", "Client certificate file (for mutual TLS)")
	keyFile    = flag.String("key-file", "../../server/certs/client-key.pem", "Client key file (for mutual TLS)")
	skipVerify = flag.Bool("skip-verify", false, "Skip TLS certificate verification (insecure)")
)

// TestClient 测试客户端
type TestClient struct {
	client         pb.MonitorServiceClient
	agentID        string
	hostname       string
	eventGenerator *EventGenerator
	stats          *Stats
}

// Stats 统计信息
type Stats struct {
	totalEvents      int64
	totalBatches     int64
	totalHeartbeats  int64
	failedBatches    int64
	failedHeartbeats int64
	startTime        time.Time
}

func main() {
	flag.Parse()

	// 生成默认值
	if *agentID == "" {
		*agentID = fmt.Sprintf("test-agent-%d", rand.Intn(10000))
	}
	if *hostname == "" {
		*hostname, _ = os.Hostname()
		if *hostname == "" {
			*hostname = fmt.Sprintf("test-host-%d", rand.Intn(1000))
		}
	}

	log.Printf("Starting HIDS Test Client")
	log.Printf("Agent ID: %s", *agentID)
	log.Printf("Hostname: %s", *hostname)
	log.Printf("Server: %s", *serverAddr)
	log.Printf("TLS: %v", *enableTLS)
	log.Printf("Heartbeat Interval: %ds", *heartbeatInt)
	log.Printf("Batch Interval: %ds", *batchInt)
	log.Printf("Events per Batch: %d", *batchSize)
	log.Printf("Enabled Workers: file=%v, process=%v, network=%v", *enableFile, *enableProcess, *enableNetwork)

	// 配置连接选项
	var opts []grpc.DialOption

	// 配置 TLS
	if *enableTLS {
		creds, err := loadClientTLSCredentials(*caFile, *certFile, *keyFile, *skipVerify)
		if err != nil {
			log.Fatalf("Failed to load TLS credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
		log.Printf("TLS enabled (CA: %s)", *caFile)
		if *certFile != "" && *keyFile != "" {
			log.Printf("Client certificate auth enabled")
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		log.Printf("Warning: TLS is disabled, connection is not secure!")
	}

	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
		grpc.MaxCallSendMsgSize(10*1024*1024), // 10MB
	))

	// 连接到 Server
	conn, err := grpc.NewClient(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to server %s", *serverAddr)

	// 创建客户端
	client := &TestClient{
		client:   pb.NewMonitorServiceClient(conn),
		agentID:  *agentID,
		hostname: *hostname,
		eventGenerator: &EventGenerator{
			agentID:       *agentID,
			hostname:      *hostname,
			enableFile:    *enableFile,
			enableProcess: *enableProcess,
			enableNetwork: *enableNetwork,
		},
		stats: &Stats{
			startTime: time.Now(),
		},
	}

	// 启动心跳
	go client.startHeartbeat(*heartbeatInt)

	// 启动批量发送
	go client.startBatchSender(*batchInt, *batchSize)

	// 启动统计输出
	go client.printStats()

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down...")
	client.printFinalStats()
}

// startHeartbeat 启动心跳
func (c *TestClient) startHeartbeat(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		req := &pb.HeartbeatRequest{
			AgentId:   c.agentID,
			Hostname:  c.hostname,
			Timestamp: time.Now().UnixNano(),
			Status: map[string]string{
				"cpu_usage":    fmt.Sprintf("%.1f%%", rand.Float64()*10),
				"memory_usage": fmt.Sprintf("%dMB", 150+rand.Intn(100)),
				"uptime":       fmt.Sprintf("%ds", int(time.Since(c.stats.startTime).Seconds())),
				"events_sent":  fmt.Sprintf("%d", c.stats.totalEvents),
				"batches_sent": fmt.Sprintf("%d", c.stats.totalBatches),
			},
		}

		resp, err := c.client.SendHeartbeat(ctx, req)
		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
			c.stats.failedHeartbeats++
		} else {
			c.stats.totalHeartbeats++
			log.Printf("Heartbeat sent (commands: %d)", len(resp.Commands))

			// 处理命令
			if len(resp.Commands) > 0 {
				c.handleCommands(resp.Commands)
			}
		}

		cancel()
	}
}

// startBatchSender 启动批量发送
func (c *TestClient) startBatchSender(interval int, batchSize int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		// 生成事件
		events := c.eventGenerator.GenerateBatch(batchSize)

		req := &pb.BatchRequest{
			AgentId:  c.agentID,
			Hostname: c.hostname,
			Events:   events,
		}

		resp, err := c.client.SendBatch(ctx, req)
		if err != nil {
			log.Printf("Batch send failed: %v", err)
			c.stats.failedBatches++
		} else {
			c.stats.totalBatches++
			c.stats.totalEvents += int64(len(events))
			log.Printf("Batch sent: %d events, accepted: %d", len(events), resp.AcceptedCount)
		}

		cancel()
	}
}

// handleCommands 处理服务器下发的命令
func (c *TestClient) handleCommands(commands []*pb.Command) {
	for _, cmd := range commands {
		log.Printf("Received command: %s (type: %s)", cmd.CommandId, cmd.CommandType)

		// 模拟命令执行
		go c.executeCommand(cmd)
	}
}

// executeCommand 执行命令
func (c *TestClient) executeCommand(cmd *pb.Command) {
	// 模拟执行延迟
	time.Sleep(time.Duration(500+rand.Intn(1500)) * time.Millisecond)

	success := rand.Float64() > 0.1 // 90% 成功率
	message := "Command executed successfully"
	if !success {
		message = "Command execution failed: simulated error"
	}

	result := &pb.CommandResult{
		AgentId:   c.agentID,
		CommandId: cmd.CommandId,
		Success:   success,
		Message:   message,
		Timestamp: time.Now().UnixNano(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.ReportCommandResult(ctx, result)
	if err != nil {
		log.Printf("Failed to report command result: %v", err)
	} else {
		if success {
			log.Printf("Command %s executed successfully", cmd.CommandId)
		} else {
			log.Printf("Command %s failed", cmd.CommandId)
		}
	}
}

// printStats 定期打印统计信息
func (c *TestClient) printStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.printFinalStats()
	}
}

// printFinalStats 打印最终统计
func (c *TestClient) printFinalStats() {
	uptime := time.Since(c.stats.startTime)

	log.Println("\n" + strings.Repeat("=", 60))
	log.Printf("Statistics (Uptime: %s)", uptime.Round(time.Second))
	log.Println(strings.Repeat("=", 60))
	log.Printf("Total Events:      %d", c.stats.totalEvents)
	log.Printf("Total Batches:     %d (Failed: %d)", c.stats.totalBatches, c.stats.failedBatches)
	log.Printf("Total Heartbeats:  %d (Failed: %d)", c.stats.totalHeartbeats, c.stats.failedHeartbeats)

	if uptime.Seconds() > 0 {
		log.Printf("Event Rate:        %.2f events/sec", float64(c.stats.totalEvents)/uptime.Seconds())
		log.Printf("Batch Rate:        %.2f batches/sec", float64(c.stats.totalBatches)/uptime.Seconds())
	}
	log.Println(strings.Repeat("=", 60) + "\n")
}

// EventGenerator 事件生成器
type EventGenerator struct {
	agentID       string
	hostname      string
	enableFile    bool
	enableProcess bool
	enableNetwork bool
}

// GenerateBatch 生成一批事件
func (g *EventGenerator) GenerateBatch(count int) []*pb.MonitorEvent {
	events := make([]*pb.MonitorEvent, 0, count)

	for i := 0; i < count; i++ {
		var event *pb.MonitorEvent

		// 随机选择事件类型
		workers := []string{}
		if g.enableFile {
			workers = append(workers, "file")
		}
		if g.enableProcess {
			workers = append(workers, "process")
		}
		if g.enableNetwork {
			workers = append(workers, "network")
		}

		if len(workers) == 0 {
			continue
		}

		workerType := workers[rand.Intn(len(workers))]

		switch workerType {
		case "file":
			event = g.generateFileEvent()
		case "process":
			event = g.generateProcessEvent()
		case "network":
			event = g.generateNetworkEvent()
		}

		if event != nil {
			events = append(events, event)
		}
	}

	return events
}

// generateFileEvent 生成文件监控事件
func (g *EventGenerator) generateFileEvent() *pb.MonitorEvent {
	operations := []string{"CREATE", "MODIFY", "DELETE", "RENAME", "CHMOD"}
	paths := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/home/user/document.txt",
		"/var/log/system.log",
		"/tmp/temp_file.tmp",
	}

	op := operations[rand.Intn(len(operations))]
	path := paths[rand.Intn(len(paths))]

	return &pb.MonitorEvent{
		WorkerName: "file_monitor",
		Timestamp:  time.Now().UnixNano(),
		EventType:  op,
		Data: map[string]string{
			"path":    path,
			"pid":     fmt.Sprintf("%d", 1000+rand.Intn(9000)),
			"process": "vim",
			"user":    "root",
			"size":    fmt.Sprintf("%d", rand.Intn(1000000)),
		},
	}
}

// generateProcessEvent 生成进程监控事件
func (g *EventGenerator) generateProcessEvent() *pb.MonitorEvent {
	events := []string{"PROCESS_START", "PROCESS_EXIT", "PROCESS_CONNECT"}
	processes := []string{"nginx", "sshd", "mysqld", "redis-server", "python"}

	eventType := events[rand.Intn(len(events))]
	process := processes[rand.Intn(len(processes))]

	return &pb.MonitorEvent{
		WorkerName: "process_monitor",
		Timestamp:  time.Now().UnixNano(),
		EventType:  eventType,
		Data: map[string]string{
			"pid":     fmt.Sprintf("%d", 1000+rand.Intn(9000)),
			"ppid":    fmt.Sprintf("%d", 1+rand.Intn(1000)),
			"name":    process,
			"cmdline": fmt.Sprintf("/usr/bin/%s --config /etc/%s.conf", process, process),
			"user":    "root",
			"cwd":     "/var/lib/" + process,
		},
	}
}

// generateNetworkEvent 生成网络监控事件
func (g *EventGenerator) generateNetworkEvent() *pb.MonitorEvent {
	events := []string{"TCP_CONNECT", "TCP_ACCEPT", "UDP_SEND"}
	remoteIPs := []string{"192.168.1.100", "10.0.0.50", "172.16.0.10", "8.8.8.8"}

	eventType := events[rand.Intn(len(events))]
	remoteIP := remoteIPs[rand.Intn(len(remoteIPs))]

	return &pb.MonitorEvent{
		WorkerName: "network_monitor",
		Timestamp:  time.Now().UnixNano(),
		EventType:  eventType,
		Data: map[string]string{
			"pid":         fmt.Sprintf("%d", 1000+rand.Intn(9000)),
			"process":     "nginx",
			"local_ip":    "192.168.1.10",
			"local_port":  fmt.Sprintf("%d", 3000+rand.Intn(60000)),
			"remote_ip":   remoteIP,
			"remote_port": fmt.Sprintf("%d", 80+rand.Intn(65000)),
			"protocol":    "TCP",
		},
	}
}

// ============================================
// TLS 配置加载
// ============================================

func loadClientTLSCredentials(caFile, certFile, keyFile string, skipVerify bool) (credentials.TransportCredentials, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: skipVerify,
	}

	// 加载 CA 证书
	if caFile != "" && !skipVerify {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("load CA cert: %v", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to add CA cert to pool")
		}

		tlsConfig.RootCAs = certPool
	}

	// 如果提供了客户端证书和密钥（双向TLS）
	if certFile != "" && keyFile != "" {
		clientCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert/key: %v", err)
		}

		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return credentials.NewTLS(tlsConfig), nil
}
