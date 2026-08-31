# Quick Start Guide

## 前置条件

确保 HIDS Server 正在运行：

```bash
# 启动 Server
cd /Users/njcx/Artemis_HIDS/server
./hids-server
```

## 快速测试

### 1. 构建客户端

```bash
cd /Users/njcx/Artemis_HIDS/test/test_grpc_client
./build.sh
```

### 2. 运行单个客户端

```bash
./test-client
```

预期输出：
```
Starting HIDS Test Client
Agent ID: test-agent-1234
Hostname: MacBook-Pro
Server: localhost:50051
Heartbeat Interval: 30s
Batch Interval: 5s
Events per Batch: 10
Enabled Workers: file=true, process=true, network=true
Connected to server localhost:50051
 Batch sent: 10 events, accepted: 10
 Batch sent: 10 events, accepted: 10
 Heartbeat sent (commands: 0)
```

### 3. 运行多个客户端（压力测试）

```bash
# 启动 5 个客户端
./run_multiple.sh 5

# 启动 10 个客户端
./run_multiple.sh 10

# 查看日志
tail -f logs/client-1.log
```

### 4. 停止客户端

单个客户端：按 `Ctrl+C`

多个客户端：
```bash
pkill -f test-client
```

## 常用测试场景

### 场景 1: 低频测试
适合开发调试：
```bash
./test-client -batch 10 -batch-size 5 -heartbeat 60
```

### 场景 2: 中等负载
模拟正常生产环境：
```bash
./test-client -batch 5 -batch-size 10 -heartbeat 30
```

### 场景 3: 高负载压测
测试系统承载能力：
```bash
# 单客户端高频
./test-client -batch 1 -batch-size 100

# 多客户端并发
./run_multiple.sh 20 localhost:50051 1 50
```

### 场景 4: 特定事件类型测试
只测试文件监控：
```bash
./test-client -process=false -network=false
```

## 验证数据

### 检查 Server 日志
```bash
# Server 应该显示接收到的事件
tail -f /Users/njcx/Artemis_HIDS/server/logs/server.log
```

### 检查 Kafka
```bash
# 消费 Kafka 消息
kafka-console-consumer.sh \
 --bootstrap-server localhost:9092 \
 --topic hids.events \
 --from-beginning
```

### 检查 Redis
```bash
# 连接 Redis 查看心跳数据
redis-cli
> KEYS agent:*
> HGETALL agent:test-agent-1234
```

## 性能指标

正常运行的测试客户端每30秒会输出统计信息：

```
============================================================
Statistics (Uptime: 2m30s)
============================================================
Total Events: 300
Total Batches: 30 (Failed: 0)
Total Heartbeats: 5 (Failed: 0)
Event Rate: 2.00 events/sec
Batch Rate: 0.20 batches/sec
============================================================
```

## 故障排查

### 连接失败
如果看到：`Failed to connect: connection refused`

解决方法：
1. 确认 Server 正在运行
2. 检查端口：`lsof -i :50051`
3. 确认防火墙设置

### 心跳失败
如果看到：` Heartbeat failed`

解决方法：
1. 检查 Server 日志
2. 确认 Redis 可访问
3. 检查网络连接

### 批量发送失败
如果看到：` Batch send failed`

解决方法：
1. 检查 Kafka 状态
2. 查看 Server 日志
3. 确认消息大小未超限

## 清理

停止所有测试客户端：
```bash
pkill -f test-client
rm -rf logs/
```

## 下一步

- 查看 [README.md](README.md) 了解完整文档
- 查看 [examples.sh](examples.sh) 了解更多使用示例
- 修改源代码 [main.go](main.go) 自定义测试逻辑
