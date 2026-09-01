# 规则引擎性能优化指南

## 性能优化概述

本次优化将规则引擎的处理能力从 **~10,000 条/分钟** 提升到 **100,000+ 条/分钟**，主要通过以下技术实现：

## 优化措施

### 1. 多线程并行处理

**问题**：原实现是单线程串行处理，每次只能处理一条消息。

**解决方案**：
- 使用 `flume` 高性能 MPMC (多生产者多消费者) channel
- 启动多个 worker 线程并行处理事件
- 消费和处理解耦，互不阻塞

**配置参数**：
```yaml
consumer:
 # 处理线程数，建议设置为 CPU 核心数的 1-2 倍
 num_workers: 16
  
 # Channel 缓冲区大小
 channel_size: 50000
```

### 2. Kafka 高性能配置

**优化的 Kafka 参数**：
```
fetch.min.bytes: 1 # 最小获取字节数
fetch.max.bytes: 52428800 # 50MB 最大获取
max.partition.fetch.bytes: 10485760 # 10MB 每分区
fetch.wait.max.ms: 100 # 最大等待时间 100ms
queued.min.messages: 100000 # 预取队列最小消息数
queued.max.messages.kbytes: 1048576 # 1GB 预取队列大小
```

### 3. 规则引擎优化

#### 3.1 规则索引

根据 `worker_name` 和 `event_type` 建立索引，快速定位相关规则：

```
事件: worker_name=exec_monitor, event_type=syscall_exec
  
只检查相关规则（而不是遍历所有规则）
```

#### 3.2 正则表达式预编译

所有规则的正则表达式在加载时预编译，避免每次匹配都重新编译：

```rust
// 加载时预编译
let compiled_patterns: HashMap<String, Regex> = ...;

// 匹配时直接使用
for (field, re) in &compiled.compiled_patterns {
    if !re.is_match(&event_value) {
        return None;
    }
}
```

### 4. 批量提交 Offset

每处理 1000 条消息才提交一次 offset，减少 Kafka 通信开销：

```rust
const COMMIT_BATCH_SIZE: u64 = 1000;

if count % COMMIT_BATCH_SIZE == 0 {
    consumer.commit_message(&message, CommitMode::Async);
}
```

## 性能调优建议

### CPU 核心数与 Worker 数量

| CPU 核心数 | 建议 num_workers | 预期吞吐量 |
|-----------|-----------------|-----------|
| 4 核 | 4-8 | 50,000/min |
| 8 核 | 8-16 | 100,000/min |
| 16 核 | 16-32 | 200,000/min |
| 32 核 | 32-64 | 400,000/min |

### Channel 缓冲区大小

| 场景 | 建议 channel_size | 内存占用 |
|-----|------------------|---------|
| 低延迟 | 1,000-5,000 | ~10MB |
| 平衡 | 10,000-50,000 | ~50MB |
| 高吞吐 | 50,000-100,000 | ~100MB |

### 监控指标

运行时会输出以下统计信息：

```
统计: 消费=120000/min, 处理=118000/min, 待处理=2000, 总处理=1000000, 匹配=150
```

- **消费**: 从 Kafka 消费的消息数/分钟
- **处理**: 规则引擎处理的消息数/分钟
- **待处理**: Channel 中等待处理的消息数
- **总处理**: 累计处理的消息总数
- **匹配**: 触发告警的匹配数

### 性能瓶颈排查

1. **消费 > 处理**：增加 `num_workers`
2. **消费 < 处理**：Kafka 是瓶颈，检查网络或增加分区
3. **待处理持续增长**：处理能力不足，增加 `num_workers`
4. **CPU 使用率低**：增加 `num_workers` 或 `channel_size`

## 配置示例

### 高性能配置（16核服务器）

```yaml
consumer:
 brokers:
    - "kafka:9092"
 topic: "hids_events-*"
 group_id: "rule-engine-group"
 auto_offset_reset: "latest"
 num_workers: 32
 channel_size: 100000
```

### 低延迟配置

```yaml
consumer:
 brokers:
    - "kafka:9092"
 topic: "hids_events-*"
 group_id: "rule-engine-group"
 auto_offset_reset: "latest"
 num_workers: 8
 channel_size: 1000
```

## 架构图

```
                    ┌─────────────────────────────────────────┐
                    │           Kafka Consumer                │
                    │  (高性能预取: 100K msgs / 1GB buffer)   │
                    └─────────────────┬───────────────────────┘
                                      │
                                      ▼
                    ┌─────────────────────────────────────────┐
                    │         Flume MPMC Channel              │
                    │      (缓冲区: 50,000 消息)              │
                    └─────────────────┬───────────────────────┘
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          │                           │                           │
          ▼                           ▼                           ▼
    ┌───────────┐               ┌───────────┐               ┌───────────┐
    │ Worker 0  │               │ Worker 1  │      ...      │ Worker N  │
    │           │               │           │               │           │
    │ ┌───────┐ │               │ ┌───────┐ │               │ ┌───────┐ │
    │ │ JSON  │ │               │ │ JSON  │ │               │ │ JSON  │ │
    │ │ Parse │ │               │ │ Parse │ │               │ │ Parse │ │
    │ └───┬───┘ │               │ └───┬───┘ │               │ └───┬───┘ │
    │     │     │               │     │     │               │     │     │
    │     ▼     │               │     ▼     │               │     ▼     │
    │ ┌───────┐ │               │ ┌───────┐ │               │ ┌───────┐ │
    │ │ Rule  │ │               │ │ Rule  │ │               │ │ Rule  │ │
    │ │ Match │ │               │ │ Match │ │               │ │ Match │ │
    │ └───┬───┘ │               │ └───┬───┘ │               │ └───┬───┘ │
    │     │     │               │     │     │               │     │     │
    └─────┼─────┘               └─────┼─────┘               └─────┼─────┘
          │                           │                           │
          └───────────────────────────┼───────────────────────────┘
                                      │
                                      ▼
                    ┌─────────────────────────────────────────┐
                    │           Alert Handler                 │
                    │   (stdout / file / kafka output)        │
                    └─────────────────────────────────────────┘
```

## 版本历史

- **v1.0**: 单线程串行处理，~10,000 条/分钟
- **v2.0**: 多线程并行处理 + 规则索引 + 正则预编译，100,000+ 条/分钟
