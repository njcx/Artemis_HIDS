# Kafka 到 MongoDB 同步工具

从 Kafka 消费数据并同步到 MongoDB 的高性能工具。

## 功能特性

- 从 Kafka 消费 `hids-system-info` 和 `hids_alert` 两个 topic
- `hids-system-info` 按 `agent_id` 去重，已有则更新
- `hids_alert` 不去重，直接插入
- 批量写入 MongoDB，提高性能
- 支持 Kafka SASL 认证（PLAIN, SCRAM-SHA-256, SCRAM-SHA-512）
- 多工作线程并发消费
- 优雅退出，确保数据不丢失

## 快速开始

### 1. 配置文件

复制配置示例：
```bash
cp config.example.yaml config.yaml
```

编辑 `config.yaml`：
```yaml
# Kafka 配置
kafka:
 brokers:
    - "localhost:9092"
 group_id: "mongo-syncer-group"
 topics:
    - "hids-system-info"
    - "hids_alert"

# MongoDB 配置
mongodb:
 uri: "mongodb://admin:password123@localhost:27017"
 database: "artemis_hids"

# 同步配置
sync:
 batch_size: 100
 flush_interval: 5
 workers: 4
```

### 2. 构建

```bash
chmod +x build.sh
./build.sh
```

### 3. 运行

```bash
./mongo_syncer -config config.yaml
```

## 数据流

```
Kafka Topics MongoDB Collections
┌─────────────────┐ ┌──────────────────────┐
│ hids-system-info│───────▶│ agents │
│ │ │ (按 agent_id 去重) │
└─────────────────┘ └──────────────────────┘

┌─────────────────┐ ┌──────────────────────┐
│ hids_alert │───────▶│ agent_security_events│
│ │ │ (不去重，直接插入) │
└─────────────────┘ └──────────────────────┘
```

## 配置说明

### Kafka 配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `brokers` | Kafka broker 地址列表 | - |
| `group_id` | 消费者组 ID | - |
| `topics` | 要消费的 topic 列表 | - |
| `sasl.enabled` | 是否启用 SASL 认证 | false |
| `sasl.mechanism` | SASL 认证机制 | PLAIN |
| `sasl.username` | SASL 用户名 | - |
| `sasl.password` | SASL 密码 | - |

### MongoDB 配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `uri` | MongoDB 连接 URI | - |
| `database` | 数据库名称 | - |
| `max_pool_size` | 最大连接池大小 | 100 |
| `min_pool_size` | 最小连接池大小 | 10 |
| `max_conn_idle_time` | 连接最大空闲时间（秒） | 3600 |

### 同步配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `batch_size` | 批量写入大小 | 100 |
| `flush_interval` | 批量刷新间隔（秒） | 5 |
| `workers` | 工作线程数 | 4 |
| `system_info_dedup` | system_info 是否去重 | true |
| `alert_dedup` | alert 是否去重 | false |

## 数据格式

### hids-system-info

```json
{
 "agent_id": "agent-001",
 "hostname": "web-server-01",
 "ip": "192.168.1.100",
 "event_type": "system_info",
 "data": {
    "os_type": "Linux",
    "os_platform": "Ubuntu",
    "os_version": "20.04",
    "cpu_model": "Intel Xeon",
    "cpu_load_percent": 45.2,
    "memory_total_gb": 32.0,
    "memory_percent": 68.5,
    "disk_total_gb": 500.0,
    "disk_percent": 55.3,
    "load_average_1min": 2.5,
    "uptime_days": 10.5
 }
}
```

### hids_alert

```json
{
 "agent_id": "agent-001",
 "event_type": "alert",
 "severity": "high",
 "title": "可疑进程执行",
 "description": "检测到未知进程",
 "source": "file_monitor",
 "timestamp": 1706601234567890000,
 "data": {
    "process_name": "unknown_proc",
    "process_path": "/tmp/unknown_proc",
    "user": "www-data"
 }
}
```

## 监控

### 日志输出

程序会输出详细的日志信息：

```
 连接到 MongoDB 成功: artemis_hids
 创建 Kafka reader: topic=hids-system-info
 创建 Kafka reader: topic=hids_alert
 Worker 0 开始消费 topic: hids-system-info
 插入新 agent: agent-001
 更新 agent: agent-001
 插入告警: agent=agent-001, severity=high
 批量 upsert system_info: 成功=100, 失败=0
 批量插入告警: 100 条
```

### 性能指标

- **吞吐量**: 每秒可处理数千条消息
- **延迟**: 批量写入延迟 < 100ms
- **资源占用**: 内存 < 100MB，CPU < 10%

## 故障排查

### 问题1: 连接 Kafka 失败

```bash
# 检查 Kafka 是否运行
docker ps | grep kafka

# 测试连接
telnet localhost 9092
```

### 问题2: 连接 MongoDB 失败

```bash
# 检查 MongoDB 是否运行
docker ps | grep mongo

# 测试连接
mongosh "mongodb://admin:password123@localhost:27017"
```

### 问题3: 消费延迟

- 增加 `workers` 数量
- 增加 `batch_size`
- 减少 `flush_interval`

## 相关文档

- [Manager API MongoDB 迁移指南](../manager/MONGODB_MIGRATION_COMPLETE.md)
- [Kafka 配置文档](https://kafka.apache.org/documentation/)
- [MongoDB Go Driver 文档](https://www.mongodb.com/docs/drivers/go/current/)

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License

---

**最后更新**: 2026-01-30 
**版本**: v1.0.0 
**维护者**: Artemis HIDS Team
