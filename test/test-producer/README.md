# Test Producer - Kafka测试数据生成器

## 功能说明

为 Artemis HIDS 生成测试数据，支持多种安全事件模拟和心跳包发送。

## 配置文件

配置文件 `config.yaml`：

```yaml
# Kafka生产者配置
kafka:
 brokers:
    - "10.185.25.205:19092"
 topic: "hids-events"
 # SASL认证配置（可选）
 sasl:
    enabled: false
    mechanism: "PLAIN"  # PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
    username: ""
    password: ""

# 日志配置
logging:
 level: "info"
```

## 安装依赖

```bash
go mod tidy
```

## 编译

```bash
go build -o test-producer
```

## 使用方法

### 基本用法

```bash
# 使用默认配置文件 (config.yaml)
./test-producer -test=all

# 指定配置文件
./test-producer -config=/path/to/config.yaml -test=all
```

### 测试用例

| 参数 | 说明 |
|------|------|
| `all` | 发送所有测试用例（包含心跳包） |
| `heartbeat` | 只发送心跳包 |
| `001` | 敏感文件访问检测 |
| `002` | SSH暴力破解检测 |
| `003` | 可疑进程执行检测 |
| `004` | 系统文件修改检测 |
| `005` | 端口扫描检测 |

### 示例

```bash
# 发送所有测试
./test-producer -test=all

# 只发送心跳包
./test-producer -test=heartbeat

# 测试SSH暴力破解
./test-producer -test=002

# 使用自定义配置
./test-producer -config=custom.yaml -test=all
```

## 测试数据说明

### 心跳包
- 模拟5个Agent (test-agent-001 ~ test-agent-005)
- 包含主机名、IP、操作系统、版本信息
- 包含CPU、内存、磁盘使用率等监控数据

### 安全事件
1. **敏感文件访问** - 检测对 `/etc/passwd`, `/etc/shadow` 的读取
2. **SSH暴力破解** - 模拟6次失败登录尝试（触发规则需要5次）
3. **可疑进程** - 检测 nmap 扫描、反弹shell等命令
4. **系统文件修改** - 检测对 `/etc/hosts`, `/etc/resolv.conf` 的修改
5. **端口扫描** - 检测 masscan 等端口扫描工具

## SASL认证

如果Kafka启用了SASL认证，修改配置文件：

```yaml
kafka:
 brokers:
    - "kafka.example.com:9092"
 topic: "hids-events"
 sasl:
    enabled: true
    mechanism: "SCRAM-SHA-256"  # 或 PLAIN, SCRAM-SHA-512
    username: "your-username"
    password: "your-password"
```

## 输出示例

```
 启动测试数据生成器
 配置文件: config.yaml
 Brokers: [10.185.25.205:19092]
 Topic: hids-events
 测试: all

 发送所有测试用例...

[心跳包] 发送Agent心跳信息
 发送: host-001 (192.168.1.101) - online
 发送: host-002 (192.168.1.102) - online
 ...

[测试001] 敏感文件访问检测
 发送: suspicious_user 读取 /etc/passwd
 发送: attacker 读取 /etc/shadow

 测试数据发送完成！
```
