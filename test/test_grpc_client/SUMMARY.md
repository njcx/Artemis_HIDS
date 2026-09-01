# HIDS Test gRPC Client - 项目总结

## 项目结构

```
test_grpc_client/
├── main.go # 主程序 - gRPC 客户端实现
├── go.mod # Go 模块定义
├── go.sum # 依赖锁定文件
├── build.sh # 构建脚本
├── run_multiple.sh # 多客户端启动脚本
├── examples.sh # 使用示例脚本
├── Makefile # Make 命令定义
├── .gitignore # Git 忽略文件
├── README.md # 完整文档
├── QUICKSTART.md # 快速开始指南
└── test-client # 编译后的可执行文件
```

## 核心功能

### 1. 心跳发送
- 定期向 Server 发送心跳（默认 30 秒）
- 携带 Agent 状态信息（CPU、内存、运行时间等）
- 接收并处理 Server 下发的命令
- 异步执行命令并上报结果

### 2. 批量事件上报
- 定期批量发送监控事件（默认 5 秒发送 10 个事件）
- 支持三种事件类型：
 - 文件监控事件（CREATE、MODIFY、DELETE、RENAME、CHMOD）
 - 进程监控事件（PROCESS_START、PROCESS_EXIT、PROCESS_CONNECT）
 - 网络监控事件（TCP_CONNECT、TCP_ACCEPT、UDP_SEND）

### 3. 命令处理
- 接收 Server 命令：stop_worker、start_worker、update_agent 等
- 模拟命令执行（500-2000ms 延迟）
- 90% 成功率模拟真实场景
- 自动上报执行结果

### 4. 统计信息
- 实时统计事件、批次、心跳数量
- 计算事件发送速率
- 定期输出统计报告（每 30 秒）
- 优雅退出时显示最终统计

## 快速开始

### 构建
```bash
make build
# 或
./build.sh
```

### 运行
```bash
# 使用默认参数
make run

# 快速测试
make fast

# 压力测试
make stress

# 多客户端并发
make multiple n=10
```

## 主要类型和结构

### TestClient
核心客户端结构，包含：
- gRPC 客户端连接
- Agent 标识信息
- 事件生成器
- 统计信息

### EventGenerator
事件生成器，负责：
- 随机生成各类监控事件
- 模拟真实的文件、进程、网络活动
- 可配置启用/禁用特定类型

### Stats
统计信息，跟踪：
- 总事件数
- 总批次数
- 总心跳数
- 失败计数
- 运行时间

## 使用场景

### 1. 开发测试
```bash
./test-client -batch 10 -batch-size 5
```
低频率发送，便于调试和观察日志

### 2. 功能验证
```bash
# 测试文件监控
make file-only

# 测试进程监控
make process-only

# 测试网络监控
make network-only
```

### 3. 性能压测
```bash
# 单客户端高频
./test-client -batch 1 -batch-size 100

# 多客户端并发
./run_multiple.sh 20 localhost:50051 1 50
```

### 4. 长期稳定性测试
```bash
./test-client -batch 30 -batch-size 20 -heartbeat 60
```

## 性能指标

### 单客户端
- 默认配置：2 events/sec
- 高频配置：100 events/sec
- 最大配置：1000+ events/sec

### 多客户端
- 10 客户端：20-100 events/sec
- 50 客户端：100-500 events/sec 
- 100 客户端：200-1000 events/sec

## 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| -server | localhost:50051 | Server 地址 |
| -agent-id | 自动生成 | Agent ID |
| -hostname | 系统主机名 | 主机名 |
| -heartbeat | 30 | 心跳间隔（秒） |
| -batch | 5 | 批量发送间隔（秒） |
| -batch-size | 10 | 每批事件数 |
| -file | true | 启用文件监控 |
| -process | true | 启用进程监控 |
| -network | true | 启用网络监控 |

## Makefile 命令

```bash
make help # 显示帮助
make build # 构建
make run # 运行
make fast # 快速测试
make stress # 压力测试
make multiple # 多客户端
make stop # 停止所有客户端
make logs # 查看日志
make clean # 清理
```

## 日志示例

```
Starting HIDS Test Client
Agent ID: test-agent-5432
Hostname: MacBook-Pro
Server: localhost:50051
Connected to server localhost:50051

 Batch sent: 10 events, accepted: 10
 Batch sent: 10 events, accepted: 10
 Heartbeat sent (commands: 0)
 Batch sent: 10 events, accepted: 10

============================================================
Statistics (Uptime: 1m30s)
============================================================
Total Events: 180
Total Batches: 18 (Failed: 0)
Total Heartbeats: 3 (Failed: 0)
Event Rate: 2.00 events/sec
Batch Rate: 0.20 batches/sec
============================================================
```

## 故障排查

### 连接失败
```
Failed to connect: connection refused
```
**解决：** 检查 Server 是否运行，端口是否正确

### 心跳失败
```
 Heartbeat failed: context deadline exceeded
```
**解决：** 检查网络连接，Redis 可访问性

### 批量发送失败
```
 Batch send failed: rpc error
```
**解决：** 检查 Kafka 状态，Server 日志

## 扩展开发

### 添加新事件类型
在 `EventGenerator` 中添加生成函数：
```go
func (g *EventGenerator) generateCustomEvent() *pb.MonitorEvent {
    return &pb.MonitorEvent{
        WorkerName: "custom_monitor",
        EventType:  "CUSTOM_EVENT",
        Data: map[string]string{
            "key": "value",
        },
    }
}
```

### 修改命令处理
在 `executeCommand` 中实现具体逻辑

### 自定义统计
在 `Stats` 中添加新字段和计算方法

## 相关文档

- [README.md](README.md) - 完整使用文档
- [QUICKSTART.md](QUICKSTART.md) - 快速开始指南
- [examples.sh](examples.sh) - 使用示例

## 测试清单

- [x] 构建成功
- [x] 单客户端运行正常
- [x] 多客户端并发测试
- [x] 心跳功能正常
- [x] 批量发送功能正常
- [x] 命令接收和执行
- [x] 统计信息准确
- [x] 优雅退出

## 总结

这是一个功能完整的 HIDS 测试客户端，支持：
- 模拟真实 Agent 行为
- 灵活的配置选项
- 完善的统计和日志
- 多客户端并发支持
- 易于使用和扩展

可用于开发测试、性能压测、功能验证等多种场景。
