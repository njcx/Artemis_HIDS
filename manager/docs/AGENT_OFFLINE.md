# Agent 在线/离线机制说明

## 工作原理

### 1. 心跳包上报
```
Agent ──> Kafka (heartbeat) ──> Manager Consumer ──> Redis
```

- Agent 定期发送心跳包到 Kafka
- Consumer 收到心跳后更新 Redis `agents:online` sorted set
- **Key**: `agents:online`
- **Score**: 心跳时间戳（Unix秒）
- **Member**: agent_id

### 2. 在线判断

```go
// 查询最近5分钟有心跳的 agent
minScore := 当前时间 - 5分钟
agents := ZRANGEBYSCORE agents:online minScore +inf
```

**在线标准**：最近5分钟内有心跳包

### 3. 离线清理（新增）

Manager Consumer 每 **30秒** 自动执行一次清理任务：

```go
// 删除超过10分钟没有心跳的 agent
cutoffTime := 当前时间 - 10分钟
ZREMRANGEBYSCORE agents:online -inf cutoffTime
```

**清理标准**：超过10分钟无心跳的记录

## 完整流程

```
时间轴：
┌─────────────────────────────────────────────────────────────┐
│                                                               │
│    0分钟        5分钟        10分钟        15分钟             │
│     │            │            │            │                │
│     │            │            │            │                │
│    心跳         在线          离线         清理               │
│    上报         显示          显示         记录               │
│                                                               │
└─────────────────────────────────────────────────────────────┘

详细说明：
- 0-5分钟：Agent 正常，显示为【在线】
- 5-10分钟：Agent 停止心跳，显示为【离线】
- 10分钟后：Consumer 自动清理该记录
```

## Redis 数据结构

### agents:online (Sorted Set)
```
ZADD agents:online 1734598800 "test-agent-001"
ZADD agents:online 1734598801 "test-agent-002"

Score: Unix 时间戳（最后心跳时间）
Member: Agent ID
```

### agent:{id} (Hash)
```
HSET agent:test-agent-001 
 agent_id "test-agent-001"
 last_heartbeat "1734598800"
 status "online"
 hostname "host-001"
 ip "192.168.1.101"
 os "Linux"
 version "1.0.0"
 cpu_usage "15.5"
 memory_usage "45.3"
 ...
```

## 查询示例

### 1. 获取所有在线 Agent
```bash
# 获取最近5分钟有心跳的
redis-cli ZRANGEBYSCORE agents:online $(date -d '-5 minutes' +%s) +inf
```

### 2. 获取 Agent 详情
```bash
redis-cli HGETALL agent:test-agent-001
```

### 3. 统计在线数量
```bash
redis-cli ZCOUNT agents:online $(date -d '-5 minutes' +%s) +inf
```

### 4. 查看即将清理的离线 Agent
```bash
# 5-10分钟前的心跳（已离线但未清理）
redis-cli ZRANGEBYSCORE agents:online -inf $(date -d '-10 minutes' +%s)
```

## 配置参数

| 参数 | 值 | 说明 |
|------|-----|------|
| 在线阈值 | 5分钟 | 最近5分钟有心跳视为在线 |
| 清理阈值 | 10分钟 | 超过10分钟无心跳自动清理 |
| 清理周期 | 30秒 | 每30秒执行一次清理任务 |

## 手动管理

### 强制下线某个 Agent
```bash
redis-cli ZREM agents:online "test-agent-001"
redis-cli DEL agent:test-agent-001
```

### 清理所有离线记录
```bash
# 删除10分钟前的所有记录
redis-cli ZREMRANGEBYSCORE agents:online -inf $(date -d '-10 minutes' +%s)
```

### 查看离线但未清理的 Agent
```bash
# 离线时间区间：5-10分钟
redis-cli ZRANGEBYSCORE agents:online \
 $(date -d '-10 minutes' +%s) \
 $(date -d '-5 minutes' +%s)
```

## 日志示例

```
# 心跳包处理
 心跳包: agent=test-agent-001, timestamp=1734598800, status=online

# 自动清理
 清理了 3 个离线 agent (超过10分钟无心跳)
```

## 优化建议

1. **调整在线阈值**：根据实际心跳频率调整（建议 2-3倍心跳间隔）
2. **调整清理阈值**：保留更长历史可用于故障分析
3. **监控告警**：当大量 Agent 同时离线时触发告警
4. **断线重连**：Agent 重连后自动恢复在线状态

## 安全考虑

- 自动清理防止 Redis 无限增长
- Score 使用时间戳便于范围查询
- 清理阈值大于在线阈值，避免误删
- 清理任务独立 goroutine，不影响消费性能
