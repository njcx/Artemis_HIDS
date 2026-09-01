# 10万+ Agent管理端架构设计方案

## 核心挑战

管理10万+agent需要解决的核心问题：

1. **实时状态查询**: 快速查询任意agent的在线状态和健康信息 
2. **海量数据存储**: 每天产生数TB的事件和告警数据
3. **高并发API**: 支持数千管理员同时查询
4. **实时监控**: 大屏展示、统计图表
5. **策略下发**: 规则配置、升级控制
6. **搜索分析**: 复杂条件查询、关联分析

## 技术架构（使用 OpenObserve 简化版）

```
┌──────────────┐
│   用户界面    │  React / Vue.js / OpenObserve 内置UI
└──────┬───────┘
       │
┌──────▼─────────────────────────────────────────┐
│           API Gateway (Kong/Nginx)              │
│    - 认证/授权  - 限流  - 路由  - 缓存           │
└──────┬───────────────────────────────────────────┘
       │
┌──────▼───────────────────────────────────────────┐
│            Manager API (Go微服务集群)             │
├──────────────┬───────────────┬────────────────────┤
│   Agent管理   │    事件查询    │      告警管理       │
│  - 状态查询   │   - 日志检索   │     - 规则管理      │
│  - 配置下发   │   - 趋势分析   │     - 通知处理      │
│  - 升级控制   │   - 关联查询   │     - 工单创建      │
└──────┬───────┴───────┬───────┴────────────────────┘
       │               │
┌──────▼──────┐ ┌──────▼─────────────────┐
│    Redis    │ │      OpenObserve        │
│ 实时状态缓存 │ │ - 事件日志存储(Parquet) │
│  agent在线   │ │ - 告警历史              │
│  心跳数据    │ │ - Agent元数据           │
│  统计数据    │ │ - SQL查询/全文搜索      │
└─────────────┘ │ - 内置可视化Dashboard   │
       ▲        └────────────────────────┘
       │               ▲
┌──────┴───────────────┴────────────────────────┐
│          Kafka Consumer (数据采集器)            │
│   - 消费hids-events和hids-alerts               │
│   - 写入OpenObserve (HTTP API)                 │
│   - 更新Redis缓存                              │
└─────────────────────────────────────────────────┘
       ▲
       │
┌──────┴──────┐
│    Kafka    │  从Server和规则引擎接收数据
└─────────────┘
```

**为什么选择 OpenObserve？**

1. **更低成本**: 存储压缩率 140:1，比 ES 节省 80% 存储
2. **更少资源**: 单节点即可支撑 10 万 agent，内存需求仅 ES 的 1/10
3. **开箱即用**: 内置 UI、可视化、告警，无需额外开发
4. **SQL 查询**: 支持标准 SQL，比 ES DSL 更简单
5. **统一存储**: 日志、指标、元数据都存在一个系统，架构更简单

## 数据库设计（使用 OpenObserve）

### 1. Redis (实时状态 - 保持不变)

管理10万agent的在线状态和最新心跳：

```
# Agent在线状态 (Hash)
agent:status:{agent_id}
 - online: true/false
 - last_heartbeat: timestamp
 - version: "1.0.0"
 - hostname: "server-001"
 - ip: "10.0.1.100"
 - cpu: "5%"
 - memory: "120MB"
 - workers: "3"

# Agent在线列表 (Sorted Set, 按时间戳排序)
agents:online 
 score: last_heartbeat_timestamp
 member: agent_id

# 统计数据 (String)
stats:agents:total
stats:agents:online
stats:events:today
stats:alerts:critical

# 地域分布 (Hash)
agents:region
 cn-beijing: 25000
 cn-shanghai: 30000
 us-west: 20000
```

**容量估算**:
- 每个agent状态 ~500 bytes
- 10万agent = 50MB
- 考虑索引和开销 = ~200MB
- **结论**: 单Redis实例足够，可使用主从+哨兵保证高可用

### 2. OpenObserve (统一数据存储)

OpenObserve 使用 **Stream** 概念组织数据，类似表或索引：

#### Stream 1: hids-events (事件日志)
```json
{
 "agent_id": "agent-001",
 "hostname": "server-001",
 "worker_name": "file_monitor",
 "event_type": "file_access",
 "timestamp": "2025-12-17T10:30:00Z",
 "severity": "info",
 "region": "cn-beijing",
 "data": {
    "path": "/etc/passwd",
    "operation": "read",
    "user": "root"
 },
 "tags": ["security", "file"]
}
```

#### Stream 2: hids-alerts (告警)
```json
{
 "alert_id": "alert-12345",
 "rule_id": "rule-001",
 "rule_name": "敏感文件访问",
 "agent_id": "agent-001",
 "severity": "critical",
 "description": "检测到敏感文件被访问",
 "timestamp": "2025-12-17T10:30:00Z",
 "status": "new",
 "assignee": "",
 "matched_events": [...]
}
```

#### Stream 3: agent-metadata (Agent 元数据)
```json
{
 "agent_id": "agent-001",
 "hostname": "server-001",
 "ip_address": "10.0.1.100",
 "region": "cn-beijing",
 "version": "1.0.0",
 "os_type": "Linux",
 "created_at": "2025-12-01T00:00:00Z",
 "updated_at": "2025-12-17T10:30:00Z",
 "tags": ["production", "web-server"],
 "config": {...}
}
```

#### Stream 4: rules (规则配置)
```json
{
 "rule_id": "rule-001",
 "name": "SSH暴力破解检测",
 "description": "检测短时间内多次失败登录",
 "severity": "high",
 "type": "threshold",
 "enabled": true,
 "conditions": {...},
 "actions": {...},
 "created_by": "admin",
 "created_at": "2025-12-01T00:00:00Z"
}
```

**OpenObserve 优势**:

1. **SQL 查询**: 
```sql
-- 查询最近1小时高危告警
SELECT * FROM hids_alerts 
WHERE severity = 'critical' 
 AND timestamp >= NOW() - INTERVAL '1 hour'
ORDER BY timestamp DESC;

-- 统计每个region的事件量
SELECT region, COUNT(*) as event_count 
FROM hids_events 
WHERE timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY region;
```

2. **极致压缩**: Parquet 列式存储 + Snappy 压缩
 - 10万agent, 每天1000万事件
 - 原始大小: 10GB/天
 - OpenObserve 压缩后: **~70MB/天** (140:1)
 - 1年历史: 仅需 **25GB** (vs ES 的 1TB)

3. **内置功能**:
 - Dashboard 可视化
 - 告警规则（补充规则引擎）
 - 日志查询 UI
 - API 完全兼容

**容量估算**:
- 事件数据(1年): 25GB
- 告警数据(1年): 5GB
- 元数据: 1GB
- **总计**: 约 30-50GB
- **推荐**: 单节点 OpenObserve，8核16GB+200GB SSD 即可

## 性能优化策略

### 1. Agent状态查询优化

**问题**: 10万agent状态需要快速查询

**解决方案**:
```go
// 使用Redis Pipeline批量查询
func GetAgentsStatus(agentIDs []string) (map[string]*AgentStatus, error) {
    pipe := redisClient.Pipeline()
    cmds := make([]*redis.StringStringMapCmd, len(agentIDs))
    
    for i, id := range agentIDs {
        cmds[i] = pipe.HGetAll(ctx, fmt.Sprintf("agent:status:%s", id))
    }
    
    pipe.Exec(ctx)
    
    results := make(map[string]*AgentStatus)
    for i, cmd := range cmds {
        data, _ := cmd.Result()
        results[agentIDs[i]] = parseAgentStatus(data)
    }
    
    return results, nil
}

// 使用Sorted Set快速获取在线agent
func GetOnlineAgents(limit int) ([]string, error) {
    // 获取最近5分钟有心跳的agent
    minScore := time.Now().Add(-5 * time.Minute).Unix()
    return redisClient.ZRangeByScore(ctx, "agents:online", &redis.ZRangeBy{
        Min:   fmt.Sprintf("%d", minScore),
        Max:   "+inf",
        Count: int64(limit),
    }).Result()
}
```

### 2. 大屏实时监控

**方案**: WebSocket + Redis Pub/Sub

```go
// Server端
func ServeRealtimeStats(ws *websocket.Conn) {
    pubsub := redisClient.Subscribe(ctx, "stats:realtime")
    defer pubsub.Close()
    
    for msg := range pubsub.Channel() {
        ws.WriteJSON(msg.Payload)
    }
}

// 数据更新器（每秒更新）
func UpdateRealtimeStats() {
    ticker := time.NewTicker(1 * time.Second)
    for range ticker.C {
        stats := collectStats()
        redisClient.Publish(ctx, "stats:realtime", stats)
    }
}
```

### 3. 搜索性能优化

```go
// ES查询优化
func SearchEvents(query *EventQuery) (*SearchResult, error) {
    // 1. 使用索引别名按日期分区
    indices := generateIndices(query.TimeRange) // hids-events-2025-12-*
    
    // 2. 只返回需要的字段
    seaOpenObserve 查询优化

```go
// OpenObserve SQL查询（比ES更简单）
func SearchEvents(query *EventQuery) (*SearchResult, error) {
    // 构建SQL查询
    sqlQuery := fmt.Sprintf(`
        SELECT agent_id, event_type, timestamp, severity, data
        FROM hids_events
        WHERE timestamp BETWEEN '%s' AND '%s'
    `, query.StartTime, query.EndTime)
    
    // 添加过滤条件
    if query.AgentID != "" {
        sqlQuery += fmt.Sprintf(" AND agent_id = '%s'", query.AgentID)
    }
    if query.Severity != "" {
        sqlQuery += fmt.Sprintf(" AND severity = '%s'", query.Severity)
    }
    
    // 全文搜索
    if query.Keyword != "" {
        sqlQuery += fmt.Sprintf(" AND match_all('%s')", query.Keyword)
    }
    
    sqlQuery += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT %d OFFSET %d", 
        query.Size, query.From)
    
    // 调用OpenObserve API
    req := &OpenObserveQueryRequest{
        SQL:    sqlQuery,
        Stream: "hids_events",
    }
    
    return o2Client.Query(req)
}

// 聚合查询示例
func GetEventStats(timeRange string) (*Stats, error) {
    sql := fmt.Sprintf(`
        SELECT 
            region,
            severity,
            COUNT(*) as count,
            COUNT(DISTINCT agent_id) as unique_agents
        FROM hids_events
        WHERE timestamp >= NOW() - INTERVAL '%s'
        GROUP BY region, severity
    `, timeRange)
    
    return o2Client.Query(&OpenObserveQueryRequest{SQL: sql}10MB)
    l2Cache *redis.Client    // Redis缓存
}

func (c *CacheManager) GetAgentInfo(agentID string) (*AgentInfo, error) {
    // L1: 本地缓（使用 OpenObserve）

| 资源 | 配置 | 数量 | 总计 |
|------|------|------|------|
| API Server | 2核4GB | 3 | 6核12GB |
| Redis | 4核16GB | 2 (主从) | 8核32GB |
| **OpenObserve** | **8核16GB+200GB SSD** | **1** | **8核16GB+200GB** |
| Kafka Consumer | 2核4GB | 2 | 4核8GB |
| **总计** | - | - | **26核68GB+200GB** |

**对比传统方案**:
- CPU: 26核 vs 64核 (节省 **60%**)
- 内存: 68GB vs 224GB (节省 **70%**)
- 存储: 200GB vs 7TB (节省 **97%**)

### 成本估算

- **云服务器**: 约$1500/月 (AWS/阿里云)
- **存储**: 约$50/月
- **带宽**: 约$500/月
- **总计**: 约$2050/月
 (Redis)
- **事件搜索**: < 500ms (OpenObserve)
- **并发能力**: 5000 QPS
- **数据延迟**: < 3s (从采集到可查询)
- **写入吞吐**: 100万事件/秒

## 扩展性

使用 OpenObserve 后，架构更简单，扩展更容易：

- **10万agent**: 上述配置
- **50万agent**: OpenObserve 集群(3节点) + API Server × 2
- **100万agent**: OpenObserve 集群(5节点) + API Server × 4

关键优势：
1. **OpenObserve 单节点即可支撑 10万+ agent**
2. 无需维护复杂的 ES 集群和 PostgreSQL
3. 存储成本降低 97%，运维成本降低 80%
4. 内置 UI，无需开发前端

## 快速部署

```bash
# 1. 启动 OpenObserve
docker run -d -p 5080:5080 \
 -v /data/openobserve:/data \
 -e ZO_ROOT_USER_EMAIL=admin@example.com \
 -e ZO_ROOT_USER_PASSWORD=admin123 \
 public.ecr.aws/zinclabs/openobserve:latest

# 2. 创建 Stream (自动创建，首次写入时)

# 3. 配置 Kafka Consumer 写入 OpenObserve
# (修改 consumer 代码，调用 OpenObserve HTTP API)

# 4. 访问 http://localhost:5080 查看数据
```

| 资源 | 配置 | 数量 | 总计 |
|------|------|------|------|
| API Server | 2核4GB | 5 | 10核20GB |
| Redis | 8核32GB | 2 (主从) | 16核64GB |
| ElasticSearch | 8核32GB+2TB SSD | 3 | 24核96GB+6TB |
| PostgreSQL | 4核16GB+500GB | 2 (主从) | 8核32GB+1TB |
| Kafka Consumer | 2核4GB | 3 | 6核12GB |
| **总计** | - | - | **64核224GB+7TB** |

### 成本估算

- **云服务器**: 约$5000/月 (AWS/阿里云)
- **存储**: 约$500/月
- **带宽**: 约$1000/月
- **总计**: 约$6500/月

### 性能指标

- **API响应时间**: P95 < 200ms
- **Agent状态查询**: < 50ms
- **事件搜索**: < 1s
- **并发能力**: 5000 QPS
- **数据延迟**: < 5s (从采集到可查询)

## 扩展性

这个架构可以支撑到50万甚至100万agent，只需线性增加资源：

- **10万agent**: 上述配置
- **50万agent**: 资源 × 3
- **100万agent**: 资源 × 5 + 分片优化

关键是采用**无状态** + **分布式存储**设计，可以水平扩展所有组件。
