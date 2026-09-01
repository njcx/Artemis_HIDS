package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mongo_syncer/config"
	"mongo_syncer/database"
	"mongo_syncer/syncer"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	log.Println("========================================")
	log.Println("  Kafka 到 MongoDB 同步工具")
	log.Println("========================================")

	// 加载配置
	log.Printf("[INFO] 加载配置文件: %s", *configPath)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[ERROR] 加载配置失败: %v", err)
	}
	log.Println("[INFO] 配置加载成功")

	// 连接 MongoDB
	log.Println("[INFO] 连接 MongoDB...")
	mongodb, err := database.NewMongoDB(&cfg.MongoDB)
	if err != nil {
		log.Fatalf("[ERROR] 连接 MongoDB 失败: %v", err)
	}
	defer func() {
		log.Println("[INFO] 断开 MongoDB 连接...")
		if err := mongodb.Close(context.Background()); err != nil {
			log.Printf("[ERROR] 关闭 MongoDB 连接失败: %v", err)
		}
	}()

	// 创建同步器
	log.Println("[INFO] 创建同步器...")
	sync := syncer.NewSyncer(cfg, mongodb)
	defer func() {
		log.Println("[INFO] 关闭同步器...")
		if err := sync.Close(); err != nil {
			log.Printf("[ERROR] 关闭同步器失败: %v", err)
		}
	}()

	// 创建 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动同步器
	go sync.Start(ctx)

	// 打印配置信息
	log.Println("========================================")
	log.Println("[INFO] 配置信息:")
	log.Printf("  Kafka Brokers: %v", cfg.Kafka.Brokers)
	log.Printf("  Kafka Topics: %v", cfg.Kafka.Topics)
	log.Printf("  Kafka Group ID: %s", cfg.Kafka.GroupID)
	log.Printf("  MongoDB URI: %s", maskURI(cfg.MongoDB.URI))
	log.Printf("  MongoDB Database: %s", cfg.MongoDB.Database)
	log.Printf("  Batch Size: %d", cfg.Sync.BatchSize)
	log.Printf("  Flush Interval: %d 秒", cfg.Sync.FlushInterval)
	log.Printf("  Workers: %d", cfg.Sync.Workers)
	log.Printf("  System Info Dedup: %v", cfg.Sync.SystemInfoDedup)
	log.Printf("  Alert Dedup: %v", cfg.Sync.AlertDedup)
	log.Println("========================================")
	log.Println("[INFO] 同步器已启动，按 Ctrl+C 退出")

	// 等待退出信号
	sig := <-sigChan
	log.Printf("[INFO] 收到信号: %v", sig)
	log.Println("[INFO] 正在优雅退出...")

	// 取消 context
	cancel()

	log.Println("[INFO] 同步器已停止")
}

// maskURI 隐藏 URI 中的密码
func maskURI(uri string) string {
	// 简单的密码隐藏逻辑
	if len(uri) > 20 {
		return uri[:20] + "***"
	}
	return "***"
}
