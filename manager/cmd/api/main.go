// ============================================
// Manager API Server - 独立 API 服务进程
// ============================================
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v3"

	"manager/internal/server"
)

func main() {
	// 加载配置
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化 Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.Redis.Addr,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
		PoolSize: config.Redis.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis 连接失败: %v", err)
	}
	log.Println("Redis 连接成功")

	// 初始化 MongoDB 数据库
	db, err := server.InitDatabase(&config.Database)
	if err != nil {
		log.Fatalf("MongoDB 连接失败: %v", err)
	}
	log.Println("MongoDB 连接成功")

	// 创建索引
	if err := server.CreateIndexes(db); err != nil {
		log.Fatalf("创建索引失败: %v", err)
	}
	log.Println("MongoDB 索引创建完成")

	// 初始化默认数据（角色和管理员账号）
	if err := server.InitDefaultData(db); err != nil {
		log.Fatalf("默认数据初始化失败: %v", err)
	}
	log.Println("默认数据初始化完成")

	// 初始化 API 服务器
	apiServer := server.NewAPIServer(redisClient, db)

	// 启动统计任务
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	go updateStats(ctx, redisClient)
	log.Println("统计任务已启动")

	// 启动仪表盘统计更新任务
	go apiServer.UpdateDashboardStats()
	log.Println("仪表盘统计更新任务已启动")

	// 启动 API 服务
	go func() {
		addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
		log.Printf("API Server 启动在 %s\n", addr)
		if err := apiServer.Start(addr); err != nil {
			log.Printf("API Server 错误: %v", err)
		}
	}()

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("正在关闭 API Server...")
	cancel()
	time.Sleep(2 * time.Second)
	redisClient.Close()
	log.Println("API Server 已关闭")
}

// updateStats 更新统计数据
func updateStats(ctx context.Context, redisClient *redis.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := calculateStats(ctx, redisClient); err != nil {
				log.Printf("更新统计失败: %v", err)
			}
		}
	}
}

// calculateStats 计算统计数据
func calculateStats(ctx context.Context, redisClient *redis.Client) error {
	// 统计在线 agent 数量
	minScore := float64(time.Now().Add(-5 * time.Minute).Unix())
	onlineCount, err := redisClient.ZCount(ctx, "agents:online", fmt.Sprintf("%f", minScore), "+inf").Result()
	if err != nil {
		return err
	}

	// 统计总 agent 数
	totalCount, err := redisClient.ZCard(ctx, "agents:online").Result()
	if err != nil {
		return err
	}

	// 保存统计数据
	pipe := redisClient.Pipeline()
	pipe.Set(ctx, "stats:agents:online", onlineCount, 0)
	pipe.Set(ctx, "stats:agents:total", totalCount, 0)
	_, err = pipe.Exec(ctx)

	if err == nil {
		// 发布实时统计
		stats := map[string]interface{}{
			"online_agents": onlineCount,
			"total_agents":  totalCount,
			"timestamp":     time.Now().Unix(),
		}
		redisClient.Publish(ctx, "stats:realtime", stats)
	}

	return err
}

// loadConfig 加载配置文件
func loadConfig(path string) (*server.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config server.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
