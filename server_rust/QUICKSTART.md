# HIDS Server 快速入门

## 5 分钟快速部署

### 1. 安装依赖

**macOS**:
```bash
brew install cmake librdkafka protobuf
```

**Ubuntu/Debian**:
```bash
sudo apt-get install cmake librdkafka-dev libssl-dev protobuf-compiler
```

### 2. 克隆并构建

```bash
cd /path/to/Artemis_HIDS/server_rust
./build.sh
```

### 3. 配置

编辑 `config.yaml`，修改 Kafka 和 Redis 地址：

```yaml
server:
 host: "0.0.0.0"
 port: 50051

kafka:
 brokers:
    - "your-kafka-host:9092"
 topic: "hids-events"

redis:
 host: "your-redis-host"
 port: 6379
 password: ""
```

### 4. 运行

```bash
./run.sh
```

## 验证部署

### 检查服务状态

```bash
# 查看进程
ps aux | grep hids-server

# 查看端口
lsof -i :50051

# 查看日志
tail -f /var/log/artemis-hids/server.log
```

### 测试连接

使用 grpcurl 测试：

```bash
# 安装 grpcurl
brew install grpcurl # macOS
# 或
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# 测试心跳
grpcurl -plaintext \
 -d '{"agent_id":"test-001","hostname":"test-host","timestamp":1703232000,"status":{}}' \
 localhost:50051 \
 hids.MonitorService/SendHeartbeat
```

## 常见问题

### Q1: 编译失败 - protoc not found

**解决方案**:
```bash
# macOS
brew install protobuf

# Linux
sudo apt-get install protobuf-compiler
```

### Q2: 运行时错误 - librdkafka.so not found

**解决方案**:
```bash
# macOS
brew install librdkafka

# Linux
sudo apt-get install librdkafka-dev
```

### Q3: 连接 Kafka 失败

**检查清单**:
1. Kafka 服务是否运行
2. 网络是否可达
3. 配置文件中的 brokers 地址是否正确
4. 如果使用 SASL，检查认证配置

### Q4: 性能不如预期

**优化建议**:
1. 增大 Kafka batch_size
2. 启用 compression (lz4)
3. 调整 max_concurrent_streams
4. 使用 release 构建 (--release)

## 生产环境部署

### 使用 Systemd

1. 复制二进制文件：
```bash
sudo cp target/release/hids-server /usr/local/bin/
sudo cp config.yaml /etc/hids-server/
```

2. 创建服务文件 `/etc/systemd/system/hids-server.service`：
```ini
[Unit]
Description=HIDS Server
After=network.target

[Service]
Type=simple
User=hids
ExecStart=/usr/local/bin/hids-server --config /etc/hids-server/config.yaml
Restart=always
Environment=RUST_LOG=info

[Install]
WantedBy=multi-user.target
```

3. 启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable hids-server
sudo systemctl start hids-server
```

### 使用 Docker

```bash
# 构建镜像
docker build -t hids-server:rust .

# 运行容器
docker run -d \
 --name hids-server \
 -p 50051:50051 \
 -v $(pwd)/config.yaml:/etc/hids-server/config.yaml \
 hids-server:rust
```

## 监控和维护

### 日志级别

```bash
# 开启 Debug 日志
RUST_LOG=debug ./run.sh

# 只查看特定模块
RUST_LOG=hids_server::grpc=debug ./run.sh
```

### 性能监控

```bash
# 实时查看统计信息（每分钟输出一次）
tail -f /var/log/artemis-hids/server.log | grep " 统计"

# 使用 htop 查看资源使用
htop -p $(pidof hids-server)
```

### 健康检查

创建健康检查脚本 `health_check.sh`：

```bash
#!/bin/bash
grpcurl -plaintext \
 -d '{"agent_id":"health-check","hostname":"health-check","timestamp":'$(date +%s)',"status":{}}' \
 localhost:50051 \
 hids.MonitorService/SendHeartbeat > /dev/null 2>&1

if [ $? -eq 0 ]; then
 echo " Health check passed"
 exit 0
else
 echo " Health check failed"
 exit 1
fi
```

## 下一步

- 查看 [README.md](README.md) 了解完整功能
- 查看 [PERFORMANCE.md](PERFORMANCE.md) 了解性能对比
- 配置 Agent 连接到此服务器
- 设置监控告警

## 技术支持

遇到问题？请提交 Issue 或查看文档。
