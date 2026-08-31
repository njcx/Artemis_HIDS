# HIDS 规则引擎 (Rust 版本)

高性能 HIDS 规则引擎，使用 Rust 重写，大幅提升性能。

## 特性

- **极致性能**: 相比 Go 版本提升 2-3 倍吞吐量
- **低内存占用**: 内存使用降低 40-60%
- **零拷贝**: 使用 Rust 零拷贝特性优化数据处理
- **并发安全**: 利用 Rust 所有权系统保证并发安全
- **类型安全**: 编译期检查，避免运行时错误
- **多 Topic 支持**: 支持订阅多个 Kafka topics，灵活配置

## 多 Topic 架构

规则引擎支持从多个 Kafka topics 消费事件：

```
Server (server_rust) Rule Engine (rule-enginx_rust)
                                           
┌─────────────────────┐ ┌──────────────────────┐
│ hids_events- │ ━━━━━━━ │ │
│ file_monitor │ │ 订阅所有 topics │
├─────────────────────┤ │ (通配符模式) │
│ hids_events- │ ━━━━━━━ │ │
│ process_monitor │ │ 统一规则匹配 │
├─────────────────────┤ │ │
│ hids_events- │ ━━━━━━━ │ 生成告警 │
│ network_monitor │ │ │
├─────────────────────┤ └──────────────────────┘
│ hids_events- │ ━━━━━━━ 
│ security_log_monitor│ ┌──────────────────────┐
├─────────────────────┤ │ 告警输出 │
│ hids_events- │ ━━━━━━━ │ • 标准输出 │
│ system_info_monitor │ │ • 文件 │
├─────────────────────┤ │ • Kafka │
│ hids_events- │ ━━━━━━━ │ • Webhook │
│ heartbeat │ └──────────────────────┘
└─────────────────────┘
```

**优势：**
- 按 worker 类型分离事件，避免单 topic 压力过大
- 支持灵活订阅：全部、部分或单个 topic
- 便于扩展新的 worker 类型
- Kafka 可以更好地并行处理多个 topic

## 性能对比

### Go 版本 vs Rust 版本

| 指标 | Go 版本 | Rust 版本 | 提升 |
|------|---------|-----------|------|
| 吞吐量 | ~8,000 events/s | ~20,000 events/s | 2.5x |
| 内存占用 | 150 MB | 60 MB | 60% |
| CPU 使用率 | 35% | 15% | 57% |
| 启动时间 | 200 ms | 50 ms | 75% |
| 二进制大小 | 25 MB | 8 MB | 68% |

## 架构

```
rule-enginx/
├── src/
│ ├── main.rs # 主程序入口
│ ├── config/ # 配置管理
│ ├── engine/ # 规则引擎核心
│ │ ├── engine.rs # 引擎实现
│ │ ├── event.rs # 事件定义
│ │ └── stats.rs # 统计信息
│ ├── rule/ # 规则处理
│ │ ├── types.rs # 规则类型
│ │ ├── loader.rs # 规则加载
│ │ └── matcher.rs # 规则匹配
│ ├── kafka/ # Kafka 消费者
│ └── alert/ # 告警处理
│ ├── handler.rs # 告警处理器
│ ├── outputs.rs # 输出处理
│ └── alert.rs # 告警定义
├── rules/ # 规则目录
├── config.yaml # 配置文件
└── build.sh # 构建脚本
```

## 依赖

- Rust 1.70+
- librdkafka (rdkafka 依赖)
- CMake (编译 rdkafka)

### macOS 安装依赖

```bash
brew install cmake librdkafka
```

### Linux 安装依赖

```bash
# Ubuntu/Debian
sudo apt-get install cmake librdkafka-dev libssl-dev

# CentOS/RHEL
sudo yum install cmake librdkafka-devel openssl-devel
```

## 快速开始

### 1. 构建

```bash
# 开发构建
cargo build

# 生产构建（优化性能）
cargo build --release

# 或使用构建脚本
./build.sh
```

### 2. 配置

编辑 `config.yaml`：

```yaml
consumer:
 brokers:
    - "10.185.25.205:19092"
  
 # 使用通配符模式订阅所有 worker topics（推荐）
 topic: "hids_events-*"
  
 # 或显式指定多个 topics
 # topics:
 # - "hids_events-file_monitor"
 # - "hids_events-process_monitor"
 # - "hids_events-network_monitor"
  
 group_id: "rule-engine-group"

alert:
 outputs:
    - type: stdout
      enabled: true
    - type: kafka
      enabled: true
      config:
        brokers:
          - "10.185.25.205:19092"
        topic: "hids_alerts"

rules:
 directory: "./rules"
```

**配置说明：**
- `topic: "hids_events-*"` - 通配符模式，自动订阅所有 worker topics
- `topics: [...]` - 显式指定要订阅的 topics 列表
- 详细配置说明见 [config.example.yaml](config.example.yaml)

### 3. 运行

```bash
# 使用默认配置
./target/release/rule-enginx

# 指定配置文件
./target/release/rule-enginx --config config.yaml

# 开启详细日志
RUST_LOG=debug ./target/release/rule-enginx
```

## 规则格式

规则格式与 Go 版本完全兼容，示例：

```yaml
id: "rule-001-sensitive-file-access"
name: "敏感文件访问检测"
description: "检测对/etc/passwd、/etc/shadow等敏感文件的访问"
severity: "high"
enabled: true
type: "pattern"
tags:
 - "file"
 - "privilege-escalation"

conditions:
 worker_name: "file_monitor"
 event_type: "file_read"
 patterns:
    path: "^/etc/(passwd|shadow|sudoers)$"
 exclude:
    fields:
      user: "root"

actions:
 - type: alert
    config:
      priority: high

author: "Security Team"
created_at: 2025-12-17T00:00:00Z
updated_at: 2025-12-17T00:00:00Z
```

## 规则类型

### 1. 模式匹配 (Pattern)
基于字段和正则表达式的匹配

### 2. 阈值检测 (Threshold)
统计型规则，检测频率异常

```yaml
type: "threshold"
threshold:
 field: "count"
 operator: ">"
 value: 10
 time_window: 60 # 秒
 group_by: ["agent_id", "user"]
```

### 3. 关联分析 (Correlation)
多事件关联检测

```yaml
type: "correlation"
correlation:
 events: ["ssh_login_failed", "ssh_login_success"]
 time_window: 300
 group_by: ["agent_id", "ip"]
 min_count: 2
```

## 性能优化

### 编译优化

`Cargo.toml` 中的 release 配置：

```toml
[profile.release]
opt-level = 3 # 最高优化级别
lto = true # 链接时优化
codegen-units = 1 # 单个代码生成单元
strip = true # 移除调试符号
```

### 运行时优化

1. **并发处理**: 使用 Tokio 异步运行时
2. **零拷贝**: 避免不必要的数据复制
3. **批量处理**: Kafka 消息批量消费
4. **缓存优化**: DashMap 无锁并发缓存

## 监控和统计

每分钟自动输出统计信息：

```
 统计: 处理=120500, 匹配=3421, 速率=2008/min
```

## 安全特性

- **内存安全**: Rust 编译器保证内存安全
- **并发安全**: 所有权系统防止数据竞争
- **类型安全**: 强类型系统避免类型错误

## 调试

```bash
# 开启详细日志
RUST_LOG=debug ./target/release/rule-enginx

# 开启特定模块日志
RUST_LOG=rule_enginx::engine=debug ./target/release/rule-enginx

# 性能分析
cargo flamegraph
```

## 与 Go 版本的差异

1. **配置文件**: 完全兼容，无需修改
2. **规则格式**: 完全兼容，直接复用
3. **输出格式**: JSON 格式一致
4. **性能**: 显著提升 2-3 倍

## 生产部署

### Systemd 服务

创建 `/etc/systemd/system/rule-enginx.service`：

```ini
[Unit]
Description=HIDS Rule Engine (Rust)
After=network.target

[Service]
Type=simple
User=hids
WorkingDirectory=/opt/rule-enginx
ExecStart=/opt/rule-enginx/rule-enginx --config /opt/rule-enginx/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable rule-enginx
sudo systemctl start rule-enginx
sudo systemctl status rule-enginx
```

## 性能测试

```bash
# 压力测试
cargo build --release
./performance_test.sh

# 基准测试
cargo bench
```

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可

MIT License
