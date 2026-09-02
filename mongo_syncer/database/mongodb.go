package database

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"mongo_syncer/config"
	oo "mongo_syncer/openobserve"

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
// 注意：数据来自 OpenObserve（原 Kafka JSON 里嵌套的 data.xxx 字段被扁平化成顶层 data_xxx），
// 所以这里从 data["data_xxx"] 直接取值，而不是 data["data"].(map)["xxx"]。
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

	// 从扁平化的 data_xxx 字段中提取信息
	{
		dataField := data

		// 如果顶层没有 hostname，尝试从 data_hostname 中获取
		if _, exists := doc["hostname"]; !exists {
			if v, ok := dataField["data_hostname"].(string); ok && v != "" {
				doc["hostname"] = v
			}
		}
		// 如果顶层没有 ip，尝试从 data_xxx 中获取
		if _, exists := doc["ip"]; !exists {
			if v, ok := dataField["data_ip"].(string); ok && v != "" {
				doc["ip"] = v
			} else if v, ok := dataField["data_local_ip"].(string); ok && v != "" {
				doc["ip"] = v
			} else if v, ok := dataField["data_ip_address"].(string); ok && v != "" {
				doc["ip"] = v
			} else if ips, ok := dataField["data_local_ips"].([]interface{}); ok && len(ips) > 0 {
				// 从 local_ips 数组中获取第一个 IP
				if ip, ok := ips[0].(string); ok && ip != "" {
					doc["ip"] = ip
				}
			}
		}

		// 操作系统信息 - 支持多种字段名
		if v, ok := dataField["data_os_type"].(string); ok && v != "" {
			doc["os"] = v
		} else if v, ok := dataField["data_os"].(string); ok && v != "" {
			doc["os"] = v
		} else if v, ok := dataField["data_os_name"].(string); ok && v != "" {
			doc["os"] = v
		}

		if v, ok := dataField["data_os_platform"].(string); ok && v != "" {
			doc["platform"] = v
		} else if v, ok := dataField["data_platform"].(string); ok && v != "" {
			doc["platform"] = v
		}

		if v, ok := dataField["data_os_version"].(string); ok && v != "" {
			doc["version"] = v
		} else if v, ok := dataField["data_version"].(string); ok && v != "" {
			doc["version"] = v
		}

		// CPU 信息
		if v, ok := dataField["data_cpu_model"].(string); ok && v != "" {
			doc["cpu"] = v
			doc["cpu_model"] = v // 新增：保存 CPU 型号
		} else if v, ok := dataField["data_cpu"].(string); ok && v != "" {
			doc["cpu"] = v
			doc["cpu_model"] = v
		}
		// CPU 物理个数
		if v, ok := dataField["data_cpu_physical_count"].(float64); ok && v > 0 {
			doc["cpu_physical_count"] = int(v)
		}
		// CPU 核数（兼容旧字段）
		if v, ok := dataField["data_cpu_count"].(float64); ok && v > 0 {
			doc["cpu_count"] = int(v)
		} else if v, ok := dataField["data_num_cpu"].(float64); ok && v > 0 {
			doc["cpu_count"] = int(v)
		} else if v, ok := dataField["data_cpu_cores"].(float64); ok && v > 0 {
			doc["cpu_count"] = int(v)
		}
		// CPU 负载百分比
		if v, ok := dataField["data_cpu_load_percent"].(float64); ok {
			doc["cpu_usage"] = v
			doc["cpu_load_percent"] = v // 新增：保存 CPU 负载
		} else if v, ok := dataField["data_cpu_load_percent"].(string); ok {
			// 支持字符串格式的百分比
			doc["cpu_load_percent"] = v
		} else if v, ok := dataField["data_cpu_percent"].(float64); ok {
			doc["cpu_usage"] = v
			doc["cpu_load_percent"] = v
		}

		// 内存信息
		if v, ok := dataField["data_memory_total_gb"].(float64); ok {
			doc["memory"] = fmt.Sprintf("%.2f GB", v)
			doc["memory_total_gb"] = fmt.Sprintf("%.1f", v) // 新增：保存内存大小
		} else if v, ok := dataField["data_memory_total_gb"].(string); ok {
			doc["memory_total_gb"] = v
			doc["memory"] = v + " GB"
		} else if v, ok := dataField["data_memory_total"].(float64); ok {
			// 假设是字节，转换为 GB
			gb := v / 1024 / 1024 / 1024
			doc["memory"] = fmt.Sprintf("%.2f GB", gb)
			doc["memory_total_gb"] = fmt.Sprintf("%.1f", gb)
		} else if v, ok := dataField["data_memory"].(string); ok && v != "" {
			doc["memory"] = v
		}
		// 内存占用百分比
		if v, ok := dataField["data_memory_percent"].(float64); ok {
			doc["memory_usage"] = v
			doc["memory_percent"] = v // 新增：保存内存占用
		} else if v, ok := dataField["data_memory_percent"].(string); ok {
			doc["memory_percent"] = v
		}

		// 磁盘信息
		if v, ok := dataField["data_disk_total_gb"].(float64); ok {
			doc["disk"] = fmt.Sprintf("%.2f GB", v)
			doc["disk_total_gb"] = fmt.Sprintf("%.1f", v) // 新增：保存磁盘大小
		} else if v, ok := dataField["data_disk_total_gb"].(string); ok {
			doc["disk_total_gb"] = v
			doc["disk"] = v + " GB"
		} else if v, ok := dataField["data_disk_total"].(float64); ok {
			// 假设是字节，转换为 GB
			gb := v / 1024 / 1024 / 1024
			doc["disk"] = fmt.Sprintf("%.2f GB", gb)
			doc["disk_total_gb"] = fmt.Sprintf("%.1f", gb)
		} else if v, ok := dataField["data_disk"].(string); ok && v != "" {
			doc["disk"] = v
		}
		// 磁盘个数
		if v, ok := dataField["data_disk_count"].(float64); ok {
			doc["disk_count"] = int(v) // 新增：保存磁盘个数
		}
		// 磁盘占用百分比
		if v, ok := dataField["data_disk_percent"].(float64); ok {
			doc["disk_usage"] = v
			doc["disk_percent"] = v // 新增：保存磁盘占比
		} else if v, ok := dataField["data_disk_percent"].(string); ok {
			doc["disk_percent"] = v
		}

		// IP 信息
		if v, ok := dataField["data_ip_count"].(float64); ok {
			doc["ip_count"] = int(v) // 新增：保存 IP 个数
		}
		// 本地 IP 列表
		if ips, ok := dataField["data_local_ips"].([]interface{}); ok {
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
		if v, ok := dataField["data_load_average_1min"].(float64); ok {
			doc["load_average"] = v
		} else if v, ok := dataField["data_load_average"].(float64); ok {
			doc["load_average"] = v
		}

		// 运行时间
		if v, ok := dataField["data_uptime_days"].(float64); ok {
			doc["uptime"] = int64(v * 24 * 3600) // 转换为秒
		} else if v, ok := dataField["data_uptime_days"].(string); ok {
			doc["uptime_days"] = v
		} else if v, ok := dataField["data_uptime"].(float64); ok {
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

	// 从扁平化的 data_xxx 字段中提取详细信息
	// （原 Kafka JSON 里嵌套的 data.xxx 被 OpenObserve 摄入后变成顶层 data_xxx）
	{
		if v, ok := data["data_process_name"].(string); ok {
			doc["process_name"] = v
		} else if v, ok := data["data_command"].(string); ok {
			doc["process_name"] = v
		}
		if v, ok := data["data_process_path"].(string); ok {
			doc["process_path"] = v
		} else if v, ok := data["data_exe"].(string); ok {
			doc["process_path"] = v
		}
		if v, ok := data["data_user"].(string); ok {
			doc["user"] = v
		}

		// 收集所有 data_ 前缀字段整体作为 details JSON 存储
		detailsField := make(map[string]interface{})
		for k, v := range data {
			if strings.HasPrefix(k, "data_") {
				detailsField[k] = v
			}
		}
		if detailsJSON, err := bson.MarshalExtJSON(detailsField, false, false); err == nil {
			doc["details"] = string(detailsJSON)
		}
	}

	return doc
}

// ============================================
// 资产数据落库（端口/软件/容器为快照型：先删旧再插新；进程为流水型：按 (agent_id, pid) upsert）
// ============================================

// ReplacePorts 替换某个 agent 的端口快照
// ports 每项字段来自 agent/workers/system_info_monitor/port_collector.go 的 ListeningPort：
// protocol/local_addr/local_port/pid/process_name/command_line/user
func (m *MongoDB) ReplacePorts(ctx context.Context, agentID string, ports []map[string]interface{}) error {
	coll := m.db.Collection("agent_ports")
	if _, err := coll.DeleteMany(ctx, bson.M{"agent_id": agentID}); err != nil {
		return fmt.Errorf("清理旧端口记录失败: %w", err)
	}
	if len(ports) == 0 {
		return nil
	}

	now := time.Now()
	docs := make([]interface{}, 0, len(ports))
	for _, p := range ports {
		port, _ := strconv.Atoi(fmt.Sprintf("%v", p["local_port"]))
		pid, _ := strconv.Atoi(fmt.Sprintf("%v", p["pid"]))
		docs = append(docs, bson.M{
			"agent_id":      agentID,
			"port":          port,
			"protocol":      fmt.Sprintf("%v", p["protocol"]),
			"local_address": fmt.Sprintf("%v", p["local_addr"]),
			"pid":           pid,
			"process_name":  fmt.Sprintf("%v", p["process_name"]),
			"created_at":    now,
			"updated_at":    now,
		})
	}

	if _, err := coll.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("插入端口快照失败: %w", err)
	}
	return nil
}

// ReplaceSoftware 替换某个 agent 的软件包快照
// packages 每项字段来自 agent/workers/system_info_monitor/package_collector.go：
// name/version/architecture（均为 string），packageManager 由外层统一传入
func (m *MongoDB) ReplaceSoftware(ctx context.Context, agentID, packageManager string, packages []map[string]interface{}) error {
	coll := m.db.Collection("agent_software")
	if _, err := coll.DeleteMany(ctx, bson.M{"agent_id": agentID}); err != nil {
		return fmt.Errorf("清理旧软件记录失败: %w", err)
	}
	if len(packages) == 0 {
		return nil
	}

	now := time.Now()
	docs := make([]interface{}, 0, len(packages))
	for _, p := range packages {
		docs = append(docs, bson.M{
			"agent_id":        agentID,
			"name":            fmt.Sprintf("%v", p["name"]),
			"version":         fmt.Sprintf("%v", p["version"]),
			"package_manager": packageManager,
			"architecture":    fmt.Sprintf("%v", p["architecture"]),
			"created_at":      now,
			"updated_at":      now,
		})
	}

	if _, err := coll.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("插入软件快照失败: %w", err)
	}
	return nil
}

// ReplaceContainers 替换某个 agent 的容器快照
// containers 每项字段来自 agent/workers/system_info_monitor/container_collector.go 的 ContainerInfo：
// id/name/image/status/created(字符串时间)/ports
func (m *MongoDB) ReplaceContainers(ctx context.Context, agentID string, containers []map[string]interface{}) error {
	coll := m.db.Collection("agent_containers")
	if _, err := coll.DeleteMany(ctx, bson.M{"agent_id": agentID}); err != nil {
		return fmt.Errorf("清理旧容器记录失败: %w", err)
	}
	if len(containers) == 0 {
		return nil
	}

	now := time.Now()
	docs := make([]interface{}, 0, len(containers))
	for _, c := range containers {
		var createdUnix int64
		if createdStr := fmt.Sprintf("%v", c["created"]); createdStr != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
				createdUnix = t.Unix()
			}
		}
		docs = append(docs, bson.M{
			"agent_id":     agentID,
			"container_id": fmt.Sprintf("%v", c["id"]),
			"name":         fmt.Sprintf("%v", c["name"]),
			"image":        fmt.Sprintf("%v", c["image"]),
			"status":       fmt.Sprintf("%v", c["status"]),
			"created":      createdUnix,
			"ports":        fmt.Sprintf("%v", c["ports"]),
			"created_at":   now,
			"updated_at":   now,
		})
	}

	if _, err := coll.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("插入容器快照失败: %w", err)
	}
	return nil
}

// UpsertProcess 更新或插入一条进程记录（流水型，按 agent_id+pid 唯一）
// 字段来自 agent/workers/system_info_monitor/process_collector.go 的 ProcessInfo，
// 在 OpenObserve 中已扁平化为顶层 data_xxx（本方法接收的是已提取好的扁平 map）
func (m *MongoDB) UpsertProcess(ctx context.Context, agentID string, raw map[string]interface{}) error {
	pid := int(oo.ToInt64(raw["data_pid"]))
	filter := bson.M{"agent_id": agentID, "pid": pid}

	update := bson.M{
		"$set": bson.M{
			"agent_id":    agentID,
			"pid":         pid,
			"name":        oo.ToString(raw["data_name"]),
			"cmdline":     oo.ToString(raw["data_cmdline"]),
			"exe":         oo.ToString(raw["data_exe"]),
			"user":        oo.ToString(raw["data_username"]),
			"status":      oo.ToString(raw["data_status"]),
			"cpu_percent": oo.ToFloat64(raw["data_cpu_percent"]),
			// Agent 侧只上报内存 MB 绝对值，没有采集占系统总内存的百分比，
			// 这里存进 memory_rss（语义为内存占用量），memory_percent 留空不填
			"memory_rss":  int64(oo.ToFloat64(raw["data_memory_mb"])),
			"num_threads": int(oo.ToInt64(raw["data_num_threads"])),
			"create_time": oo.ToInt64(raw["data_create_time"]),
			"parent_pid":  int(oo.ToInt64(raw["data_ppid"])),
			"updated_at":  time.Now(),
		},
		"$setOnInsert": bson.M{
			"created_at": time.Now(),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := m.db.Collection("agent_processes").UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("upsert 进程记录失败: %w", err)
	}
	return nil
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
