# HIDS Test gRPC Client

模拟 Agent 向 Server 发送监控数据的测试客户端。

## 功能特性

- 模拟心跳发送 (支持接收和执行命令)
- 模拟批量事件上报
- 支持三种监控类型：文件监控、进程监控、网络监控
- 实时统计信息输出
- 可配置的参数

## 快速开始

### 1. 构建程序

```bash
cd /Users/njcx/Artemis_HIDS/test/test_grpc_client
go build -o test-client
```

### 2. 运行测试客户端

**使用默认参数运行：**
```bash
./test-client
```

**自定义参数运行：**
```bash
./test-client \
 -server localhost:50051 \
 -agent-id my-test-agent \
 -hostname test-server-01 \
 -heartbeat 30 \
 -batch 5 \
 -batch-size 10
```

## 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-server` | string | localhost:50051 | gRPC Server 地址 |
| `-agent-id` | string | 自动生成 | Agent ID |
| `-hostname` | string | 系统主机名 | 主机名 |
| `-heartbeat` | int | 30 | 心跳间隔（秒） |
| `-batch` | int | 5 | 批量发送间隔（秒） |
| `-batch-size` | int | 10 | 每批事件数量 |
| `-file` | bool | true | 启用文件监控事件 |
| `-process` | bool | true | 启用进程监控事件 |
| `-network` | bool | true | 启用网络监控事件 |
| **TLS 参数** | | | |
| `-tls` | bool | true | 启用 TLS/HTTPS |
| `-ca-file` | string | ../../server/certs/ca-cert.pem | CA 证书文件 |
| `-cert-file` | string | ../../server/certs/client-cert.pem | 客户端证书（双向TLS） |
| `-key-file` | string | ../../server/certs/client-key.pem | 客户端私钥（双向TLS） |
| `-skip-verify` | bool | false | 跳过证书验证（仅测试用） |

## 使用示例

### 示例 1: 高频率测试
模拟高负载场景，每秒发送一批 50 个事件：
```bash
./test-client -batch 1 -batch-size 50 -heartbeat 10
```

### 示例 2: 只测试文件监控
只发送文件监控事件：
```bash
./test-client -process=false -network=false
```

### 示例 3: 多客户端并发测试
启动多个客户端模拟多个 Agent：
```bash
# 终端 1
./test-client -agent-id agent-001 -hostname host-001

# 终端 2 
./test-client -agent-id agent-002 -hostname host-002

# 终端 3
./test-client -agent-id agent-003 -hostname host-003
```

### 示例 4: 连接远程服务器
```bash
./test-client -server 192.168.1.100:50051
```

### 示例 5: 使用 TLS 连接
```bash
# 单向 TLS（默认）
./test-client -ca-file ../../server/certs/ca-cert.pem

# 双向 TLS
./test-client \
 -ca-file ../../server/certs/ca-cert.pem \
 -cert-file ../../server/certs/client-cert.pem \
 -key-file ../../server/certs/client-key.pem

# 跳过证书验证（测试用）
./test-client -skip-verify

# 禁用 TLS（不安全）
./test-client -tls=false
```

## 输出说明

程序运行时会输出以下信息：

- ` Heartbeat sent` - 心跳发送成功
- ` Batch sent` - 批量事件发送成功
- ` Received command` - 收到服务器命令
- ` Command executed` - 命令执行成功
- ` Failed` - 操作失败

### 统计信息输出

每 30 秒输出一次统计信息：
```
============================================================
Statistics (Uptime: 2m30s)
============================================================
Total Events: 1500
Total Batches: 30 (Failed: 0)
Total Heartbeats: 5 (Failed: 0)
Event Rate: 10.00 events/sec
Batch Rate: 0.20 batches/sec
============================================================
```

## 模拟的事件类型

### 1. 文件监控事件
- CREATE - 文件创建
- MODIFY - 文件修改
- DELETE - 文件删除
- RENAME - 文件重命名
- CHMOD - 权限变更

示例路径：
- `/etc/passwd`
- `/etc/shadow`
- `/home/user/document.txt`
- `/var/log/system.log`

### 2. 进程监控事件
- PROCESS_START - 进程启动
- PROCESS_EXIT - 进程退出
- PROCESS_CONNECT - 进程连接

模拟进程：
- nginx
- sshd
- mysqld
- redis-server
- python

### 3. 网络监控事件
- TCP_CONNECT - TCP 连接
- TCP_ACCEPT - TCP 接受连接
- UDP_SEND - UDP 发送

模拟网络连接：
- 本地 IP: 192.168.1.10
- 远程 IP: 192.168.1.100, 10.0.0.50, 172.16.0.10, 8.8.8.8

## 心跳与命令处理

客户端会定期发送心跳，并在心跳响应中接收服务器下发的命令。

支持的命令类型：
- `stop_worker` - 停止 Worker
- `start_worker` - 启动 Worker
- `update_agent` - 更新 Agent
- `update_worker` - 更新 Worker
- `set_config` - 设置配置

命令执行特性：
- 异步执行（不阻塞主流程）
- 模拟执行延迟（500-2000ms）
- 90% 成功率模拟
- 自动上报执行结果

## 停止程序

使用 Ctrl+C 优雅退出，程序会打印最终统计信息。

## 故障排查

### 连接失败
```
Failed to connect: connection refused
```
**解决方案：** 确保 HIDS Server 正在运行，并检查地址和端口是否正确。

### 心跳失败
```
 Heartbeat failed: context deadline exceeded
```
**解决方案：** 检查网络连接，或增加超时时间。

### 批量发送失败
```
 Batch send failed: rpc error
```
**解决方案：** 检查 Server 日志，确认 Kafka 和 Redis 服务是否正常。

## 性能测试建议

### 单客户端压测
```bash
# 每秒 100 个事件
./test-client -batch 1 -batch-size 100

# 每秒 1000 个事件
./test-client -batch 1 -batch-size 1000
```

### 多客户端并发压测
使用脚本启动多个客户端：

```bash
#!/bin/bash
# start_multiple_clients.sh

for i in {1..10}; do
 ./test-client \
    -agent-id "agent-$(printf %03d $i)" \
    -hostname "host-$(printf %03d $i)" \
    -batch 5 \
    -batch-size 20 \
    > logs/client-$i.log 2>&1 &
done

echo "Started 10 test clients"
```

## 依赖项

- Go 1.24+
- gRPC
- Protocol Buffers

所有依赖都在 `go.mod` 中定义，运行 `go mod tidy` 自动下载。

## 开发说明

### 添加新的事件类型

在 `EventGenerator` 中添加新的生成函数：

```go
func (g *EventGenerator) generateCustomEvent() *pb.MonitorEvent {
    return &pb.MonitorEvent{
        WorkerName: "custom_monitor",
        Timestamp:  time.Now().UnixNano(),
        EventType:  "CUSTOM_EVENT",
        Data: map[string]string{
            "key": "value",
        },
    }
}
```

### 修改命令处理逻辑

在 `executeCommand` 函数中添加具体的命令处理逻辑。

## 许可证

与 Artemis HIDS 项目保持一致。
