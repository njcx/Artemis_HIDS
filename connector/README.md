# HIDS Connector

Kafka 数据同步工具，将 HIDS 数据从 Kafka 同步到 OpenObserve 和 Redis。

## 功能特性

- **多 Topic 消费**：同时消费多个 Kafka topic
- **OpenObserve 写入**：按 worker 类型写入不同索引
- **Redis 状态管理**：心跳数据写入 Redis，实时更新 agent 状态
- **批量写入**：支持批量写入 OpenObserve，提升性能
- **SASL 认证**：支持 Kafka SASL 认证
- **自动清理**：定期清理 Redis 中的过期 agent

## 架构设计

```
Kafka Topics Connector 目标存储
───────────── ───────── ──────────
hids_events-file_monitor ─► ─► OpenObserve (hids-file-monitor)
hids_events-exec_monitor ─► ─► OpenObserve (hids-exec-monitor)
hids_events-network_monitor ─► Kafka Consumer ───────► OpenObserve (hids-network-monitor)
hids_events-security_log ─► ─► OpenObserve (hids-security-log)
hids_events-system_info ─► ─► OpenObserve (hids-system-info)
hids_events-heartbeat ─► ─► OpenObserve + Redis (agent status)
```

## 快速开始

### 1. 编译

```bash
bash build.sh
```

### 2. 配置

编辑 `config.yaml`：

```yaml
kafka:
 brokers:
    - "localhost:9092"
 group_id: "hids-connector"
 topics:
    - name: "hids_events-file_monitor"
      index: "hids-file-monitor"
      is_heartbeat: false
    # ... 更多 topic

openobserve:
 url: "http://localhost:5080"
 org: "default"
 username: "root@example.com"
 password: "Complexpass#123"

redis:
 url: "redis://127.0.0.1:6379"
 key_prefix: "hids:agent:"
 ttl_secs: 300
```

### 3. 运行

```bash
RUST_LOG=info ./target/release/hids-connector
```

## 配置说明

### Kafka 配置

- `brokers`: Kafka broker 地址列表
- `group_id`: 消费者组 ID
- `topics`: Topic 配置列表
 - `name`: Kafka topic 名称
 - `index`: OpenObserve 索引名称
 - `is_heartbeat`: 是否为心跳数据（心跳数据会写入 Redis）

### OpenObserve 配置

- `url`: OpenObserve 服务地址
- `org`: 组织名称
- `username`: 用户名
- `password`: 密码
- `batch_size`: 批量写入大小（默认 100）
- `flush_interval_secs`: 刷新间隔秒数（默认 5）

### Redis 配置

- `url`: Redis 连接 URL
- `key_prefix`: Key 前缀（默认 `hids:agent:`）
- `ttl_secs`: 数据 TTL 秒数（默认 300）

## Redis 数据结构

### Agent 状态

Key: `hids:agent:{agent_id}`

```json
{
 "agent_id": "server-1",
 "hostname": "web-01",
 "last_heartbeat": 1737544800000,
 "status": {
    "file_monitor": {...},
    "exec_monitor": {...},
    ...
 },
 "updated_at": 1737544800
}
```

### Agent 列表

Key: `hids:agent:list` (Sorted Set)

- Score: 最后心跳时间戳
- Member: agent_id

## OpenObserve 索引

- `hids-file-monitor`: 文件监控事件
- `hids-exec-monitor`: 进程执行事件
- `hids-network-monitor`: 网络连接事件
- `hids-security-log`: 安全日志事件
- `hids-system-info`: 系统信息事件
- `hids-heartbeat`: 心跳数据

## 性能优化

- 批量写入 OpenObserve（默认 100 条/批次）
- 定时刷新（默认 5 秒）
- 异步并发处理多个 topic
- Redis 连接池管理

## 监控指标

日志输出包含：
- Kafka 消费速度
- OpenObserve 写入成功率
- Redis 更新状态
- 错误统计

## 故障处理

- Kafka 连接失败：自动重连
- OpenObserve 写入失败：记录错误日志，继续处理
- Redis 更新失败：记录错误日志，不影响数据写入
- 数据解析失败：跳过错误数据，继续处理

## 依赖

- Rust 1.70+
- Kafka 2.0+
- OpenObserve 0.5+
- Redis 6.0+
