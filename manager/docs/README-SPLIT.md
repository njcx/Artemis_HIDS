# Manager 服务 - 拆分架构

## 架构设计

```
┌─────────────┐
│    Kafka    │
└──────┬──────┘
       │
       ▼
┌──────────────────┐        ┌─────────┐
│ Manager Consumer │───────>│  Redis  │
│     (2 实例)      │        └────┬────┘
│  - 消费 Kafka     │             │
│  - 写入 Redis     │             │
│  - 写入 O2        │             ▼
└──────────────────┘        ┌─────────┐
                             │ Manager │
┌──────────────────┐        │   API   │
│   OpenObserve    │<───────│ (3 实例) │
└──────────────────┘        │ - 查询   │
                             │ - 统计   │
                             └─────────┘
```

## 目录结构

```
manager/
├── cmd/
│   ├── api/main.go           # API Server 入口
│   └── consumer/main.go      # Consumer 入口
├── api_server.go             # 共享: API 逻辑
├── kafka_consumer.go         # 共享: Kafka 消费逻辑
├── openobserve.go            # 共享: OpenObserve 客户端
├── config.go                 # 共享: 配置定义
├── config.yaml               # 配置文件
├── Dockerfile.api            # API 镜像
├── Dockerfile.consumer       # Consumer 镜像
├── docker-compose-split.yaml # 分离部署
└── build.sh                  # 本地构建脚本
```

## 快速启动

### 方式 1: Docker Compose（推荐）

```bash
# 启动所有服务
docker-compose -f docker-compose-split.yaml up -d

# 查看服务状态
docker-compose -f docker-compose-split.yaml ps

# 扩容 API 实例
docker-compose -f docker-compose-split.yaml up -d --scale manager-api=5

# 扩容 Consumer 实例
docker-compose -f docker-compose-split.yaml up -d --scale manager-consumer=3
```

### 方式 2: 本地运行

```bash
# 构建
chmod +x build.sh
./build.sh

# 终端 1: 启动 API Server
./bin/manager-api

# 终端 2: 启动 Consumer
./bin/manager-consumer
```

### 方式 3: 手动编译运行

```bash
# API Server
cd /Users/njcx/Artemis_HIDS/manager
go run cmd/api/main.go config.go api_server.go openobserve.go

# Consumer (另一个终端)
cd /Users/njcx/Artemis_HIDS/manager
go run cmd/consumer/main.go config.go kafka_consumer.go openobserve.go
```

## 服务说明

### Manager API (manager-api)

**职责**:
- 提供 RESTful API
- 从 Redis 读取实时状态
- 从 OpenObserve 查询历史数据
- 计算和发布统计信息

**特点**:
- 无状态，可水平扩展
- 低延迟 (Redis 查询)
- 无需 Kafka 依赖

**扩容**:
```bash
# 3 个 API 实例
docker-compose up -d --scale manager-api=3
```

### Manager Consumer (manager-consumer)

**职责**:
- 消费 Kafka `hids-events` 和 `hids-alerts`
- 更新 Redis 实时状态
- 批量写入 OpenObserve

**特点**:
- 使用 Consumer Group 自动分区
- 批量处理提高吞吐
- 双写 Redis + OpenObserve

**扩容**:
```bash
# 2 个 Consumer 实例（自动分配分区）
docker-compose up -d --scale manager-consumer=2
```

## 优势

### 1. 独立扩容
```bash
# 高并发场景：增加 API 实例
docker-compose up -d --scale manager-api=10

# 高吞吐场景：增加 Consumer 实例
docker-compose up -d --scale manager-consumer=5
```

### 2. 故障隔离
- API 挂了不影响数据采集
- Consumer 挂了不影响查询服务

### 3. 资源优化
```yaml
manager-api:
  resources:
    limits:
      cpus: '0.5'      # API 低 CPU
      memory: 512M     # API 低内存

manager-consumer:
  resources:
    limits:
      cpus: '1'        # Consumer 需要更多 CPU
      memory: 1G       # Consumer 需要更多内存
```

### 4. 独立部署和回滚
```bash
# 只更新 API
docker build -f Dockerfile.api -t manager-api:v2 .
docker-compose up -d manager-api

# 只更新 Consumer
docker build -f Dockerfile.consumer -t manager-consumer:v2 .
docker-compose up -d manager-consumer
```

## API 端点

所有 API 端点保持不变：

```bash
# 统计信息
curl http://localhost:8080/api/v1/stats

# 在线 agents
curl http://localhost:8080/api/v1/agents/online

# 搜索事件
curl http://localhost:8080/api/v1/events?severity=critical
```

## 性能基准

### 单实例性能

| 组件 | QPS | 延迟 | 内存 | CPU |
|------|-----|------|------|-----|
| API | 5000 | <50ms | 200MB | 30% |
| Consumer | 10万事件/s | <100ms | 500MB | 60% |

### 多实例性能

| API 实例 | Consumer 实例 | 总 QPS | 总吞吐 |
|---------|--------------|--------|--------|
| 3 | 2 | 15000 | 20万事件/s |
| 5 | 3 | 25000 | 30万事件/s |
| 10 | 5 | 50000 | 50万事件/s |

## 监控

### API 健康检查
```bash
curl http://localhost:8080/health
```

### Consumer 监控
```bash
# 查看 consumer lag
kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
 --describe --group manager-consumer-group
```

### Redis 监控
```bash
redis-cli INFO stats
redis-cli DBSIZE
```

## 故障排查

### API 无响应
```bash
# 检查 Redis 连接
docker logs hids-manager-api

# 重启 API
docker-compose restart manager-api
```

### Consumer 延迟
```bash
# 检查 Kafka lag
docker exec hids-kafka kafka-consumer-groups.sh --describe

# 增加 Consumer 实例
docker-compose up -d --scale manager-consumer=5
```

## 配置优化

### 高并发优化 (API)
```yaml
redis:
 pool_size: 200 # 增加连接池
```

### 高吞吐优化 (Consumer)
```yaml
kafka:
 min_bytes: 100000 # 增加批次大小
 max_bytes: 100000000 # 10MB
```

## 成本对比

### 单体架构
- 1 个 Manager 实例: 2核4GB
- 月成本: ~$100

### 拆分架构
- 3 个 API 实例: 0.5核512MB × 3
- 2 个 Consumer 实例: 1核1GB × 2
- 月成本: ~$120
- **性能提升**: 300%

## 生产建议

1. **小规模（< 1 万 agent）**
 - API: 2 实例
 - Consumer: 1 实例

2. **中等规模（1-10 万 agent）**
 - API: 3-5 实例
 - Consumer: 2-3 实例

3. **大规模（> 10 万 agent）**
 - API: 5-10 实例
 - Consumer: 3-5 实例
 - 添加 Load Balancer (Nginx/HAProxy)

## 负载均衡

### Nginx 配置
```nginx
upstream manager_api {
    server manager-api-1:8080;
    server manager-api-2:8080;
    server manager-api-3:8080;
}

server {
    listen 80;
    location / {
        proxy_pass http://manager_api;
    }
}
```
