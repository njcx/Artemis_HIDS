package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"mongo_syncer/config"
	"mongo_syncer/database"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// Syncer Kafka 到 MongoDB 同步器
type Syncer struct {
	config          *config.Config
	mongodb         *database.MongoDB
	readers         []*kafka.Reader
	systemInfoBatch []map[string]interface{}
	alertBatch      []map[string]interface{}
	mu              sync.Mutex
	lastFlush       time.Time
}

// NewSyncer 创建同步器
func NewSyncer(cfg *config.Config, mongodb *database.MongoDB) *Syncer {
	readers := make([]*kafka.Reader, 0, len(cfg.Kafka.Topics))

	// 为每个 topic 创建一个 reader
	for _, topic := range cfg.Kafka.Topics {
		readerConfig := kafka.ReaderConfig{
			Brokers:        cfg.Kafka.Brokers,
			GroupID:        cfg.Kafka.GroupID,
			Topic:          topic,
			MinBytes:       1,
			MaxBytes:       10e6, // 10MB
			CommitInterval: time.Second,
			StartOffset:    kafka.LastOffset,
			MaxWait:        500 * time.Millisecond,
		}

		// 配置 SASL 认证
		if cfg.Kafka.SASL.Enabled && cfg.Kafka.SASL.Username != "" {
			mechanism, err := getSASLMechanism(
				cfg.Kafka.SASL.Mechanism,
				cfg.Kafka.SASL.Username,
				cfg.Kafka.SASL.Password,
			)
			if err != nil {
				log.Fatalf("创建 SASL 认证失败: %v", err)
			}
			readerConfig.Dialer = &kafka.Dialer{
				SASLMechanism: mechanism,
				Timeout:       10 * time.Second,
			}
			log.Printf("[INFO] Kafka SASL 已启用: mechanism=%s, username=%s",
				cfg.Kafka.SASL.Mechanism, cfg.Kafka.SASL.Username)
		}

		reader := kafka.NewReader(readerConfig)
		readers = append(readers, reader)
		log.Printf("[INFO] 创建 Kafka reader: topic=%s", topic)
	}

	return &Syncer{
		config:          cfg,
		mongodb:         mongodb,
		readers:         readers,
		systemInfoBatch: make([]map[string]interface{}, 0, cfg.Sync.BatchSize),
		alertBatch:      make([]map[string]interface{}, 0, cfg.Sync.BatchSize),
		lastFlush:       time.Now(),
	}
}

// Start 启动同步器
func (s *Syncer) Start(ctx context.Context) {
	log.Println("[INFO] 启动 Kafka 到 MongoDB 同步器")

	// 启动定时刷新 goroutine
	go s.periodicFlush(ctx)

	// 为每个 reader 启动多个工作线程
	var wg sync.WaitGroup
	for _, reader := range s.readers {
		for i := 0; i < s.config.Sync.Workers; i++ {
			wg.Add(1)
			go func(r *kafka.Reader, workerID int) {
				defer wg.Done()
				s.consumeFromReader(ctx, r, workerID)
			}(reader, i)
		}
	}

	// 等待 context 取消
	<-ctx.Done()
	log.Println("[INFO] 收到退出信号，停止同步")

	// 刷新剩余批次
	s.flushBatches(context.Background())

	// 等待所有工作线程结束
	wg.Wait()
	log.Println("[INFO] 所有工作线程已停止")
}

// consumeFromReader 从指定的 reader 消费消息
func (s *Syncer) consumeFromReader(ctx context.Context, reader *kafka.Reader, workerID int) {
	topic := reader.Config().Topic
	log.Printf("[INFO] Worker %d 开始消费 topic: %s", workerID, topic)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] Worker %d 停止消费 topic: %s", workerID, topic)
			return
		default:
			// 读取消息
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Printf("[ERROR] Worker %d 读取消息失败 (topic=%s): %v", workerID, topic, err)
				time.Sleep(time.Second)
				continue
			}

			// 解析消息
			var event map[string]interface{}
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[ERROR] Worker %d 解析消息失败: %v", workerID, err)
				reader.CommitMessages(ctx, msg)
				continue
			}

			// 根据 topic 分类处理
			if err := s.handleMessage(ctx, topic, event); err != nil {
				log.Printf("[ERROR] Worker %d 处理消息失败: %v", workerID, err)
			}

			// 提交偏移量
			if err := reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[ERROR] Worker %d 提交偏移量失败: %v", workerID, err)
			}
		}
	}
}

// handleMessage 处理消息
func (s *Syncer) handleMessage(ctx context.Context, topic string, event map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 根据 topic 名称判断消息类型
	switch topic {
	case "hids_events-system_monitor":
		// 添加到 system_info 批次
		s.systemInfoBatch = append(s.systemInfoBatch, event)

		// 检查是否需要刷新
		if len(s.systemInfoBatch) >= s.config.Sync.BatchSize {
			return s.flushSystemInfoBatch(ctx)
		}

	case "hids_alerts":
		// 添加到 alert 批次
		s.alertBatch = append(s.alertBatch, event)

		// 检查是否需要刷新
		if len(s.alertBatch) >= s.config.Sync.BatchSize {
			return s.flushAlertBatch(ctx)
		}

	default:
		log.Printf("[WARN] 未知的 topic: %s", topic)
	}

	return nil
}

// periodicFlush 定期刷新批次
func (s *Syncer) periodicFlush(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.config.Sync.FlushInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			if time.Since(s.lastFlush) >= time.Duration(s.config.Sync.FlushInterval)*time.Second {
				s.flushBatches(ctx)
			}
			s.mu.Unlock()
		}
	}
}

// flushBatches 刷新所有批次
func (s *Syncer) flushBatches(ctx context.Context) {
	s.flushSystemInfoBatch(ctx)
	s.flushAlertBatch(ctx)
	s.lastFlush = time.Now()
}

// flushSystemInfoBatch 刷新 system_info 批次
func (s *Syncer) flushSystemInfoBatch(ctx context.Context) error {
	if len(s.systemInfoBatch) == 0 {
		return nil
	}

	batch := s.systemInfoBatch
	s.systemInfoBatch = make([]map[string]interface{}, 0, s.config.Sync.BatchSize)

	// 释放锁后再执行数据库操作
	go func() {
		if err := s.mongodb.BatchUpsertSystemInfo(context.Background(), batch); err != nil {
			log.Printf("[ERROR] 刷新 system_info 批次失败: %v", err)
		}
	}()

	return nil
}

// flushAlertBatch 刷新 alert 批次
func (s *Syncer) flushAlertBatch(ctx context.Context) error {
	if len(s.alertBatch) == 0 {
		return nil
	}

	batch := s.alertBatch
	s.alertBatch = make([]map[string]interface{}, 0, s.config.Sync.BatchSize)

	// 释放锁后再执行数据库操作
	go func() {
		if err := s.mongodb.BatchInsertAlerts(context.Background(), batch); err != nil {
			log.Printf("[ERROR] 刷新 alert 批次失败: %v", err)
		}
	}()

	return nil
}

// Close 关闭同步器
func (s *Syncer) Close() error {
	log.Println("[INFO] 关闭同步器")

	// 刷新剩余数据
	s.mu.Lock()
	s.flushBatches(context.Background())
	s.mu.Unlock()

	// 关闭所有 readers
	for _, reader := range s.readers {
		if err := reader.Close(); err != nil {
			log.Printf("[ERROR] 关闭 reader 失败: %v", err)
		}
	}

	return nil
}

// getSASLMechanism 获取 SASL 认证机制
func getSASLMechanism(mechanism, username, password string) (sasl.Mechanism, error) {
	switch mechanism {
	case "PLAIN":
		return &plain.Mechanism{
			Username: username,
			Password: password,
		}, nil
	case "SCRAM-SHA-256":
		m, err := scram.Mechanism(scram.SHA256, username, password)
		if err != nil {
			return nil, fmt.Errorf("create SCRAM-SHA-256: %w", err)
		}
		return m, nil
	case "SCRAM-SHA-512":
		m, err := scram.Mechanism(scram.SHA512, username, password)
		if err != nil {
			return nil, fmt.Errorf("create SCRAM-SHA-512: %w", err)
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %s", mechanism)
	}
}
