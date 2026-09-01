package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"mongo_syncer/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB MongoDB 客户端
type MongoDB struct {
	client *mongo.Client
	db     *mongo.Database
	config *config.MongoDBConfig
}

// NewMongoDB 创建 MongoDB 客户端
func NewMongoDB(cfg *config.MongoDBConfig) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 设置客户端选项
	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxConnIdleTime(time.Duration(cfg.MaxConnIdleTime) * time.Second)

	// 连接到 MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("连接 MongoDB 失败: %w", err)
	}

	// 测试连接
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("Ping MongoDB 失败: %w", err)
	}

	db := client.Database(cfg.Database)
	log.Printf("[INFO] 连接到 MongoDB 成功: %s", cfg.Database)

	return &MongoDB{
		client: client,
		db:     db,
		config: cfg,
	}, nil
}

// UpsertSystemInfo 插入或更新系统信息（按 agent_id 去重）
func (m *MongoDB) UpsertSystemInfo(ctx context.Context, data map[string]interface{}) error {
	agentID, ok := data["agent_id"].(string)
	if !ok || agentID == "" {
		return fmt.Errorf("invalid agent_id")
	}

	// 提取并转换数据
	doc := m.convertSystemInfoToDoc(data)

	// 使用 agent_id 作为唯一标识进行 upsert
	filter := bson.M{"agent_id": agentID, "deleted_at": nil}
	update := bson.M{
		"$set":         doc,
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}

	opts := options.Update().SetUpsert(true)
	result, err := m.db.Collection("agents").UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("upsert system_info 失败: %w", err)
	}

	if result.UpsertedCount > 0 {
		log.Printf("[INFO] 插入新 agent: %s", agentID)
	} else if result.ModifiedCount > 0 {
		log.Printf("[INFO] 更新 agent: %s", agentID)
	}

	return nil
}

// InsertAlert 插入告警（不去重）
func (m *MongoDB) InsertAlert(ctx context.Context, data map[string]interface{}) error {
	// 转换数据
	doc := m.convertAlertToDoc(data)

	// 直接插入
	_, err := m.db.Collection("agent_security_events").InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("insert alert 失败: %w", err)
	}

	agentID, _ := data["agent_id"].(string)
	severity, _ := data["severity"].(string)
	log.Printf("[INFO] 插入告警: agent=%s, severity=%s", agentID, severity)

	return nil
}

// BatchUpsertSystemInfo 批量插入或更新系统信息
func (m *MongoDB) BatchUpsertSystemInfo(ctx context.Context, dataList []map[string]interface{}) error {
	if len(dataList) == 0 {
		return nil
	}

	successCount := 0
	errorCount := 0

	for _, data := range dataList {
		if err := m.UpsertSystemInfo(ctx, data); err != nil {
			log.Printf("[ERROR] upsert system_info 失败: %v", err)
			errorCount++
		} else {
			successCount++
		}
	}

	log.Printf("[INFO] 批量 upsert system_info: 成功=%d, 失败=%d", successCount, errorCount)
	return nil
}

// BatchInsertAlerts 批量插入告警
func (m *MongoDB) BatchInsertAlerts(ctx context.Context, dataList []map[string]interface{}) error {
	if len(dataList) == 0 {
		return nil
	}

	docs := make([]interface{}, 0, len(dataList))
	for _, data := range dataList {
		doc := m.convertAlertToDoc(data)
		docs = append(docs, doc)
	}

	result, err := m.db.Collection("agent_security_events").InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("batch insert alerts 失败: %w", err)
	}

	log.Printf("[INFO] 批量插入告警: %d 条", len(result.InsertedIDs))
	return nil
}

// convertSystemInfoToDoc 转换系统信息为 MongoDB 文档
func (m *MongoDB) convertSystemInfoToDoc(data map[string]interface{}) bson.M {
	doc := bson.M{
		"updated_at": time.Now(),
	}

	// 基本字段 - 优先从顶层获取
	if v, ok := data["agent_id"].(string); ok {
		doc["agent_id"] = v
	}
	if v, ok := data["hostname"].(string); ok && v != "" {
		doc["hostname"] = v
	}
	if v, ok := data["ip"].(string); ok && v != "" {
		doc["ip"] = v
	}

	// 从 data 字段中提取信息
	if dataField, ok := data["data"].(map[string]interface{}); ok {
		// 如果顶层没有 hostname，尝试从 data 中获取
		if _, exists := doc["hostname"]; !exists {
			if v, ok := dataField["hostname"].(string); ok && v != "" {
				doc["hostname"] = v
			}
		}
		// 如果顶层没有 ip，尝试从 data 中获取
		if _, exists := doc["ip"]; !exists {
			if v, ok := dataField["ip"].(string); ok && v != "" {
				doc["ip"] = v
			} else if v, ok := dataField["local_ip"].(string); ok && v != "" {
				doc["ip"] = v
			} else if v, ok := dataField["ip_address"].(string); ok && v != "" {
				doc["ip"] = v
			} else if ips, ok := dataField["local_ips"].([]interface{}); ok && len(ips) > 0 {
				// 从 local_ips 数组中获取第一个 IP
				if ip, ok := ips[0].(string); ok && ip != "" {
					doc["ip"] = ip
				}
			}
		}

		// 操作系统信息 - 支持多种字段名
		if v, ok := dataField["os_type"].(string); ok && v != "" {
			doc["os"] = v
		} else if v, ok := dataField["os"].(string); ok && v != "" {
			doc["os"] = v
		} else if v, ok := dataField["os_name"].(string); ok && v != "" {
			doc["os"] = v
		}

		if v, ok := dataField["os_platform"].(string); ok && v != "" {
			doc["platform"] = v
		} else if v, ok := dataField["platform"].(string); ok && v != "" {
			doc["platform"] = v
		}

		if v, ok := dataField["os_version"].(string); ok && v != "" {
			doc["version"] = v
		} else if v, ok := dataField["version"].(string); ok && v != "" {
			doc["version"] = v
		}

		// CPU 信息
		if v, ok := dataField["cpu_model"].(string); ok && v != "" {
			doc["cpu"] = v
			doc["cpu_model"] = v // 新增：保存 CPU 型号
		} else if v, ok := dataField["cpu"].(string); ok && v != "" {
			doc["cpu"] = v
			doc["cpu_model"] = v
		}
		// CPU 物理个数
		if v, ok := dataField["cpu_physical_count"].(float64); ok && v > 0 {
			doc["cpu_physical_count"] = int(v)
		}
		// CPU 核数（兼容旧字段）
		if v, ok := dataField["cpu_count"].(float64); ok && v > 0 {
			doc["cpu_count"] = int(v)
		} else if v, ok := dataField["num_cpu"].(float64); ok && v > 0 {
			doc["cpu_count"] = int(v)
		} else if v, ok := dataField["cpu_cores"].(float64); ok && v > 0 {
			doc["cpu_count"] = int(v)
		}
		// CPU 负载百分比
		if v, ok := dataField["cpu_load_percent"].(float64); ok {
			doc["cpu_usage"] = v
			doc["cpu_load_percent"] = v // 新增：保存 CPU 负载
		} else if v, ok := dataField["cpu_load_percent"].(string); ok {
			// 支持字符串格式的百分比
			doc["cpu_load_percent"] = v
		} else if v, ok := dataField["cpu_percent"].(float64); ok {
			doc["cpu_usage"] = v
			doc["cpu_load_percent"] = v
		}

		// 内存信息
		if v, ok := dataField["memory_total_gb"].(float64); ok {
			doc["memory"] = fmt.Sprintf("%.2f GB", v)
			doc["memory_total_gb"] = fmt.Sprintf("%.1f", v) // 新增：保存内存大小
		} else if v, ok := dataField["memory_total_gb"].(string); ok {
			doc["memory_total_gb"] = v
			doc["memory"] = v + " GB"
		} else if v, ok := dataField["memory_total"].(float64); ok {
			// 假设是字节，转换为 GB
			gb := v / 1024 / 1024 / 1024
			doc["memory"] = fmt.Sprintf("%.2f GB", gb)
			doc["memory_total_gb"] = fmt.Sprintf("%.1f", gb)
		} else if v, ok := dataField["memory"].(string); ok && v != "" {
			doc["memory"] = v
		}
		// 内存占用百分比
		if v, ok := dataField["memory_percent"].(float64); ok {
			doc["memory_usage"] = v
			doc["memory_percent"] = v // 新增：保存内存占用
		} else if v, ok := dataField["memory_percent"].(string); ok {
			doc["memory_percent"] = v
		}

		// 磁盘信息
		if v, ok := dataField["disk_total_gb"].(float64); ok {
			doc["disk"] = fmt.Sprintf("%.2f GB", v)
			doc["disk_total_gb"] = fmt.Sprintf("%.1f", v) // 新增：保存磁盘大小
		} else if v, ok := dataField["disk_total_gb"].(string); ok {
			doc["disk_total_gb"] = v
			doc["disk"] = v + " GB"
		} else if v, ok := dataField["disk_total"].(float64); ok {
			// 假设是字节，转换为 GB
			gb := v / 1024 / 1024 / 1024
			doc["disk"] = fmt.Sprintf("%.2f GB", gb)
			doc["disk_total_gb"] = fmt.Sprintf("%.1f", gb)
		} else if v, ok := dataField["disk"].(string); ok && v != "" {
			doc["disk"] = v
		}
		// 磁盘个数
		if v, ok := dataField["disk_count"].(float64); ok {
			doc["disk_count"] = int(v) // 新增：保存磁盘个数
		}
		// 磁盘占用百分比
		if v, ok := dataField["disk_percent"].(float64); ok {
			doc["disk_usage"] = v
			doc["disk_percent"] = v // 新增：保存磁盘占比
		} else if v, ok := dataField["disk_percent"].(string); ok {
			doc["disk_percent"] = v
		}

		// IP 信息
		if v, ok := dataField["ip_count"].(float64); ok {
			doc["ip_count"] = int(v) // 新增：保存 IP 个数
		}
		// 本地 IP 列表
		if ips, ok := dataField["local_ips"].([]interface{}); ok {
			ipList := make([]string, 0, len(ips))
			for _, ip := range ips {
				if ipStr, ok := ip.(string); ok && ipStr != "" {
					ipList = append(ipList, ipStr)
				}
			}
			if len(ipList) > 0 {
				doc["local_ips"] = ipList // 新增：保存 IP 列表
			}
		}

		// 负载信息
		if v, ok := dataField["load_average_1min"].(float64); ok {
			doc["load_average"] = v
		} else if v, ok := dataField["load_average"].(float64); ok {
			doc["load_average"] = v
		}

		// 运行时间
		if v, ok := dataField["uptime_days"].(float64); ok {
			doc["uptime"] = int64(v * 24 * 3600) // 转换为秒
		} else if v, ok := dataField["uptime_days"].(string); ok {
			doc["uptime_days"] = v
		} else if v, ok := dataField["uptime"].(float64); ok {
			doc["uptime"] = int64(v)
		}

		// 保存完整的 data 字段用于前端直接访问
		// doc["data"] = dataField
	}

	// 状态信息
	doc["status"] = "online"
	doc["online"] = true
	now := time.Now()
	doc["last_heartbeat"] = &now

	return doc
}

// convertAlertToDoc 转换告警为 MongoDB 文档
func (m *MongoDB) convertAlertToDoc(data map[string]interface{}) bson.M {
	now := time.Now()
	doc := bson.M{
		"_id":        primitive.NewObjectID(),
		"created_at": now,
		"updated_at": now,
		"status":     "open",
	}

	// 基本字段
	if v, ok := data["agent_id"].(string); ok {
		doc["agent_id"] = v
	}
	if v, ok := data["event_type"].(string); ok {
		doc["event_type"] = v
	}
	if v, ok := data["severity"].(string); ok {
		doc["severity"] = v
	}
	// 优先使用 title 字段，如果没有则使用 rule_name 字段作为标题
	if v, ok := data["title"].(string); ok && v != "" {
		doc["title"] = v
	} else if v, ok := data["rule_name"].(string); ok && v != "" {
		doc["title"] = v
	}
	if v, ok := data["description"].(string); ok {
		doc["description"] = v
	}
	if v, ok := data["source"].(string); ok {
		doc["source"] = v
	}
	// 保存 rule_id 用于追溯
	if v, ok := data["rule_id"].(string); ok {
		doc["rule_id"] = v
	}

	// 时间戳
	if v, ok := data["timestamp"].(float64); ok {
		doc["timestamp"] = int64(v)
	} else if v, ok := data["timestamp"].(int64); ok {
		doc["timestamp"] = v
	} else {
		doc["timestamp"] = now.UnixNano()
	}

	// 从 data 字段中提取详细信息
	if dataField, ok := data["data"].(map[string]interface{}); ok {
		if v, ok := dataField["process_name"].(string); ok {
			doc["process_name"] = v
		}
		if v, ok := dataField["process_path"].(string); ok {
			doc["process_path"] = v
		}
		if v, ok := dataField["user"].(string); ok {
			doc["user"] = v
		}

		// 将整个 data 字段作为 details JSON 存储
		if detailsJSON, err := bson.MarshalExtJSON(dataField, false, false); err == nil {
			doc["details"] = string(detailsJSON)
		}
	}

	return doc
}

// Close 关闭 MongoDB 连接
func (m *MongoDB) Close(ctx context.Context) error {
	if m.client != nil {
		return m.client.Disconnect(ctx)
	}
	return nil
}

// GetDatabase 获取数据库实例
func (m *MongoDB) GetDatabase() *mongo.Database {
	return m.db
}
