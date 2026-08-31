# HIDS Manager - 管理端服务

基于 OpenObserve 的轻量级 HIDS 管理平台，支持 10 万+ agent 管理。

## 架构特点

- **Redis**: 实时 agent 状态缓存
- **OpenObserve**: 统一日志/事件/元数据存储
- **Kafka**: 事件流传输
- **Go + Gin**: 高性能 API 服务

## 快速启动

### 1. 启动基础设施

```bash
# 启动 Redis + OpenObserve + Kafka + Manager
docker-compose up -d

# 查看服务状态
docker-compose ps
```

### 2. 访问服务

- **Manager API**: http://localhost:8080
- **OpenObserve UI**: http://localhost:5080
  - 用户名: admin@example.com
  - 密码: admin123

### 3. 本地开发运行

```bash
# 安装依赖
go mod tidy

# 运行服务
go run *.go
```

## API 接口

### 统计信息

```bash
# 获取总体统计
curl http://localhost:8080/api/v1/stats

# 响应示例
{
  "agents": {
    "online": 85234,
    "total": 100000
  },
  "timestamp": 1702800000
}
```

### Agent 管理

```bash
# 列出所有 agents
curl http://localhost:8080/api/v1/agents

# 获取指定 agent 详情
curl http://localhost:8080/api/v1/agents/agent-001

# 获取在线 agents
curl http://localhost:8080/api/v1/agents/online

# 响应示例
{
  "agent_id": "agent-001",
  "hostname": "server-001",
  "online": true,
  "last_heartbeat": "1702800000",
  "version": "1.0.0",
  "cpu": "5%",
  "memory": "120MB"
}
```

### 事件查询

```bash
# 搜索事件
curl "http://localhost:8080/api/v1/events?agent_id=agent-001&severity=critical&size=20"

# 时间范围查询
curl "http://localhost:8080/api/v1/events?start_time=1702800000000&end_time=1702810000000"

# 关键词搜索
curl "http://localhost:8080/api/v1/events?keyword=/etc/passwd"

# 获取事件统计
curl "http://localhost:8080/api/v1/events/stats?range=24%20hours"
```

### 告警查询

```bash
# 搜索告警
curl "http://localhost:8080/api/v1/alerts?severity=critical"

# 更新告警状态
curl -X PUT http://localhost:8080/api/v1/alerts/alert-123/status \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved", "assignee": "admin"}'
```

## OpenObserve 查询示例

### 使用 SQL 查询事件

登录 OpenObserve UI，在 SQL 编辑器中执行：

```sql
-- 查询最近1小时的高危事件
SELECT * FROM hids_events 
WHERE severity = 'critical' 
  AND timestamp >= NOW() - INTERVAL '1 hour'
ORDER BY timestamp DESC;

-- 统计每个 region 的事件量
SELECT region, COUNT(*) as event_count 
FROM hids_events 
WHERE timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY region;

-- 查询特定 agent 的所有告警
SELECT * FROM hids_alerts
WHERE agent_id = 'agent-001'
ORDER BY timestamp DESC
LIMIT 100;

-- 统计告警趋势
SELECT 
  DATE_TRUNC('hour', timestamp) as hour,
  severity,
  COUNT(*) as count
FROM hids_alerts
WHERE timestamp >= NOW() - INTERVAL '7 days'
GROUP BY hour, severity
ORDER BY hour DESC;
```

### 使用 Dashboard

OpenObserve 内置可视化功能，可以创建：

1. **实时监控大屏**
   - 在线 agent 数量
   - 事件/告警趋势图
   - 地域分布图

2. **安全态势面板**
   - 高危告警列表
   - 攻击类型分布
   - TOP 10 受攻击主机

3. **性能监控**
   - 事件写入速率
   - 查询响应时间
   - 存储使用情况

## 配置说明

编辑 `config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 100

openobserve:
  url: "http://localhost:5080"
  org: "default"
  email: "admin@example.com"
  password: "admin123"

kafka:
  brokers:
    - "localhost:9092"
  group_id: "manager-consumer"
  topics:
    - "hids-events"
    - "hids-alerts"
```

## 性能优化

### Redis 优化

```bash
# 设置最大内存（用于 10 万 agent）
redis-cli CONFIG SET maxmemory 2gb
redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

### OpenObserve 优化

在 `docker-compose.yaml` 中调整资源：

```yaml
openobserve:
  environment:
    - ZO_MEM_TABLE_MAX_SIZE=256   # 内存表大小 (MB)
    - ZO_COMPACT_MAX_FILE_SIZE=64 # 压缩文件大小 (MB)
  deploy:
    resources:
      limits:
        memory: 16G
        cpus: '8'
```

## 监控指标

Manager 服务暴露以下指标：

- 在线 agent 数量
- 事件处理速率
- API 请求延迟
- Kafka 消费延迟
- OpenObserve 写入成功率

## 故障排查

### Manager 无法连接 Redis

```bash
# 检查 Redis 状态
docker-compose logs redis

# 测试连接
redis-cli ping
```

### OpenObserve 写入失败

```bash
# 查看 Manager 日志
docker-compose logs manager

# 检查 OpenObserve 状态
curl http://localhost:5080/healthz
```

### Kafka 消费延迟

```bash
# 检查 consumer lag
docker exec -it hids-kafka kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --describe --group manager-consumer
```

## 扩展部署

### 水平扩展 Manager API

```bash
# 启动多个 Manager 实例
docker-compose up -d --scale manager=3
```

### OpenObserve 集群部署

```yaml
# docker-compose-cluster.yaml
openobserve-1:
  image: public.ecr.aws/zinclabs/openobserve:latest
  environment:
    - ZO_CLUSTER_NAME=hids-cluster
    - ZO_NODE_ROLE=all
    - ZO_ETCD_ENDPOINTS=etcd:2379

openobserve-2:
  # 同上配置...
```

## 成本估算

10 万 agent 场景：

| 组件 | 配置 | 月成本 |
|------|------|--------|
| Manager API (3节点) | 2核4GB | $300 |
| Redis (主从) | 4核16GB | $400 |
| OpenObserve | 8核16GB+200GB | $600 |
| Kafka | 4核8GB | $250 |
| **总计** | - | **$1550** |

相比 ES+PG 方案节省 **76%**！

## 许可证

MIT License
