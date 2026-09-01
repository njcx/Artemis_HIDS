# Artemis HIDS 规则编写指南

## 目录

- [概述](#概述)
- [规则文件结构](#规则文件结构)
- [规则类型](#规则类型)
- [字段说明](#字段说明)
- [条件配置](#条件配置)
- [阈值规则](#阈值规则)
- [关联规则](#关联规则)
- [动作配置](#动作配置)
- [最佳实践](#最佳实践)
- [示例规则](#示例规则)

---

## 概述

Artemis HIDS 规则引擎支持 YAML 格式的规则定义，提供灵活的威胁检测能力。规则文件位于 `rules/` 目录下，引擎会在启动时自动加载。

### 架构说明

规则引擎从 Kafka 消费事件数据，支持多 Topic 订阅：
- **Server** 按 worker 类型分发事件到不同 topic：`hids_events-file_monitor`、`hids_events-process_monitor` 等
- **规则引擎** 订阅所有相关 topics，统一进行规则匹配和告警
- **告警输出** 支持多种方式：标准输出、文件、Kafka、Webhook 等

### 支持的规则类型

- **Pattern** - 模式匹配规则（正则表达式、字段匹配）
- **Threshold** - 阈值检测规则（频率、数量限制）
- **Correlation** - 关联分析规则（多事件关联）
- **Statistical** - 统计分析规则（预留）

---

## 规则文件结构

每个规则文件必须包含以下基本字段：

```yaml
id: "rule-001-example" # 规则唯一标识符
name: "规则名称" # 人类可读的规则名称
description: "规则描述" # 详细说明规则用途
severity: "high" # 严重级别
enabled: true # 是否启用
type: "pattern" # 规则类型
tags: # 标签（可选）
 - "tag1"
 - "tag2"
conditions: # 匹配条件
 # ... 条件配置
actions: # 触发动作
 - type: alert
    config:
      priority: high
author: "Security Team" # 作者
created_at: 2025-12-17T00:00:00Z # 创建时间
updated_at: 2025-12-17T00:00:00Z # 更新时间
```

---

## 规则类型

### 1. Pattern - 模式匹配规则

用于检测符合特定模式的单个事件。

**适用场景：**
- 敏感文件访问检测
- 可疑进程执行检测
- 危险命令执行检测
- WebShell 检测

**配置要求：**
```yaml
type: "pattern"
conditions:
 worker_name: "file_monitor" # 监控器名称
 event_type: "file_read" # 事件类型
 patterns: # 正则表达式匹配
    path: "^/etc/(passwd|shadow)$"
 fields: # 精确字段匹配（可选）
    user: "www-data"
```

### 2. Threshold - 阈值检测规则

用于检测在指定时间窗口内超过阈值的事件。

**适用场景：**
- 暴力破解检测（多次登录失败）
- 端口扫描检测（短时间大量连接）
- DDoS 攻击检测
- 异常访问频率检测

**配置要求：**
```yaml
type: "threshold"
conditions:
 worker_name: "network_monitor"
 event_type: "ssh_login_failed"
threshold:
 field: "count" # 统计字段
 operator: ">=" # 比较操作符: >, <, >=, <=, ==, !=
 value: 5 # 阈值
 time_window: 300 # 时间窗口（秒）
 group_by: # 分组字段
    - "agent_id"
    - "data.src_ip"
```

### 3. Correlation - 关联分析规则

用于检测多个相关事件组合形成的攻击链。

**适用场景：**
- APT 攻击检测
- 多阶段攻击检测
- 杀伤链分析
- 复杂攻击场景

**配置要求：**
```yaml
type: "correlation"
conditions:
 worker_name: "*" # 匹配所有监控器
correlation:
 events: # 需要关联的事件类型
    - "port_scan"
    - "process_create"
    - "file_modify"
 time_window: 600 # 时间窗口（秒）
 group_by: # 分组字段
    - "agent_id"
 min_count: 1 # 最小事件数量
```

### 4. Statistical - 统计分析规则

（预留功能，暂未实现）

---

## 字段说明

### 基础字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | String | | 规则唯一标识符，建议格式：`rule-<编号>-<名称>` |
| `name` | String | | 规则的人类可读名称 |
| `description` | String | | 规则的详细描述 |
| `severity` | Enum | | 严重级别：`critical`, `high`, `medium`, `low`, `info` |
| `enabled` | Boolean | | 是否启用规则 |
| `type` | Enum | | 规则类型：`pattern`, `threshold`, `correlation`, `statistical` |
| `tags` | Array | | 标签列表，用于分类和检索 |
| `conditions` | Object | | 匹配条件配置 |
| `threshold` | Object | | 阈值配置（仅 threshold 类型需要） |
| `correlation` | Object | | 关联配置（仅 correlation 类型需要） |
| `actions` | Array | | 触发的动作列表 |
| `author` | String | | 规则作者 |
| `created_at` | DateTime | | 创建时间（ISO 8601 格式） |
| `updated_at` | DateTime | | 更新时间（ISO 8601 格式） |

### 严重级别说明

| 级别 | 描述 | 典型场景 |
|------|------|----------|
| `critical` | 严重威胁 | 反弹 Shell、WebShell、成功入侵 |
| `high` | 高风险 | 权限提升、敏感文件访问、可疑进程 |
| `medium` | 中风险 | 端口扫描、配置文件修改 |
| `low` | 低风险 | 异常但可能正常的行为 |
| `info` | 信息记录 | 审计日志、统计信息 |

---

## 条件配置

### Conditions 字段

```yaml
conditions:
 worker_name: "file_monitor" # 监控器名称（可选）
 event_type: "file_read" # 事件类型（可选）
 fields: # 字段精确匹配（可选）
    user: "root"
    action: "delete"
 patterns: # 字段正则匹配（可选）
    path: "^/etc/.*"
    command: "(wget|curl).*http"
 expression: "data['size'] > 1000" # 自定义表达式（可选）
 exclude: # 排除条件（可选）
    fields:
      user: "system"
    patterns:
      path: "^/tmp/.*"
```

### 支持的监控器

| Worker Name | 说明 | 主要事件类型 |
|-------------|------|-------------|
| `file_monitor` | 文件监控 | `file_read`, `file_write`, `file_create`, `file_delete`, `file_modify` |
| `process_monitor` | 进程监控 | `process_create`, `process_exit` |
| `network_monitor` | 网络监控 | `connection_attempt`, `ssh_login_failed`, `ssh_login_success` |
| `security_log_monitor` | 安全日志监控 | `auth_failed`, `auth_success` |
| `system_info_monitor` | 系统信息监控 | `cpu_high`, `memory_high`, `disk_high` |
| `*` | 所有监控器 | 用于关联规则 |

### 正则表达式示例

```yaml
patterns:
 # 匹配敏感文件
 path: "^/etc/(passwd|shadow|sudoers)$"
  
 # 匹配可疑命令
 command: "(nc -l|bash -i|python.*pty\\.spawn)"
  
 # 匹配 IP 地址
 src_ip: "^192\\.168\\.1\\."
  
 # 匹配 Web Shell 路径
 path: "^/(var/www|usr/share/nginx)/.*\\.(php|jsp|asp)$"
```

### 排除条件（Exclude）

用于过滤误报，排除已知的正常行为：

```yaml
conditions:
 worker_name: "file_monitor"
 event_type: "file_read"
 patterns:
    path: "^/etc/passwd$"
  
 exclude:
    fields:
      user: "root"              # 排除 root 用户
    patterns:
      process: "^/usr/bin/ssh"  # 排除 SSH 进程
```

---

## 阈值规则

### Threshold 配置

```yaml
threshold:
 field: "count" # 统计字段
 operator: ">=" # 比较操作符
 value: 5 # 阈值
 time_window: 300 # 时间窗口（秒）
 group_by: # 分组字段
    - "agent_id"
    - "data.src_ip"
```

### 操作符说明

| 操作符 | 说明 | 示例 |
|--------|------|------|
| `>` | 大于 | `value > 10` |
| `<` | 小于 | `value < 5` |
| `>=` | 大于等于 | `value >= 5` |
| `<=` | 小于等于 | `value <= 100` |
| `==` | 等于 | `value == 0` |
| `!=` | 不等于 | `value != 0` |

### 时间窗口

时间窗口必须使用**秒数**表示：

| 时间 | 秒数 | 示例场景 |
|------|------|----------|
| 1分钟 | 60 | 端口扫描检测 |
| 5分钟 | 300 | 暴力破解检测 |
| 10分钟 | 600 | 多阶段攻击检测 |
| 1小时 | 3600 | 异常访问频率 |
| 1天 | 86400 | 长期趋势分析 |

### 分组字段（Group By）

指定如何分组统计事件：

```yaml
group_by:
 - "agent_id" # 按主机分组
 - "data.src_ip" # 按源 IP 分组
 - "data.user" # 按用户分组
 - "data.dest_port" # 按目标端口分组
```

---

## 关联规则

### Correlation 配置

```yaml
correlation:
 events: # 关联的事件类型列表
    - "port_scan"
    - "process_create"
    - "file_modify"
 time_window: 600 # 时间窗口（秒）
 group_by: # 分组字段
    - "agent_id"
 min_count: 1 # 最小匹配事件数
```

### 关联规则工作原理

1. 在时间窗口内收集所有匹配的事件
2. 按照 `group_by` 字段分组
3. 检查每组中是否包含所有指定的事件类型
4. 至少需要 `min_count` 个事件才触发告警

### 典型攻击链场景

```yaml
# APT 攻击检测
correlation:
 events:
    - "port_scan"           # 侦察阶段
    - "exploit_attempt"     # 利用阶段
    - "privilege_escalation" # 权限提升
    - "persistence"         # 持久化
 time_window: 3600
 group_by:
    - "agent_id"
 min_count: 3 # 至少匹配3个阶段
```

---

## 动作配置

### Actions 字段

每个规则可以配置多个动作：

```yaml
actions:
 - type: alert # 告警
    config:
      priority: high
  
 - type: log # 日志记录
    config:
      file: "/var/log/artemis-hids/alerts.log"
  
 - type: kafka # 发送到 Kafka
    config:
      topic: "security-alerts"
  
 - type: webhook # Webhook 通知
    config:
      url: "https://api.example.com/webhook"
      method: "POST"
```

### 支持的动作类型

| 类型 | 说明 | 配置参数 |
|------|------|----------|
| `alert` | 生成告警 | `priority`: 优先级 |
| `log` | 写入日志文件 | `file`: 日志文件路径 |
| `kafka` | 发送到 Kafka | `topic`: Kafka 主题 |
| `webhook` | HTTP 回调 | `url`, `method` |
| `email` | 邮件通知 | `to`, `subject` |
| `block` | 阻断操作 | `action`: `kill_process`, `block_ip` |

---

## 最佳实践

### 1. 命名规范

```yaml
# 规则 ID 格式
id: "rule-<编号>-<简短描述>"

# 示例
id: "rule-001-sensitive-file-access"
id: "rule-002-ssh-brute-force"
id: "rule-003-suspicious-process"
```

### 2. 标签分类

使用标签对规则进行分类，便于管理和检索：

```yaml
tags:
 # MITRE ATT&CK 战术
 - "credential-access"
 - "privilege-escalation"
 - "persistence"
 - "command-and-control"
  
 # 资源类型
 - "file"
 - "process"
 - "network"
  
 # 攻击类型
 - "brute-force"
 - "malware"
 - "webshell"
 - "apt"
```

### 3. 避免误报

使用 `exclude` 条件排除已知正常行为：

```yaml
conditions:
 patterns:
    command: "sudo"
  
 exclude:
    fields:
      user: "root"                    # 排除 root
    patterns:
      parent_process: "^/usr/bin/ansible" # 排除自动化工具
```

### 4. 性能优化

- 优先使用字段精确匹配，减少正则计算
- 合理设置时间窗口，避免过长
- 避免过于宽泛的正则表达式
- 使用 `enabled: false` 临时禁用规则

```yaml
# 推荐：精确匹配 + 简单正则
conditions:
 worker_name: "file_monitor" # 精确匹配
 event_type: "file_read" # 精确匹配
 patterns:
    path: "^/etc/passwd$"         # 简单正则

# 避免：复杂正则
conditions:
 patterns:
    path: "^/(etc|var|usr|opt|home)/.*/(passwd|shadow|.*key|.*conf)$"
```

### 5. 测试验证

创建规则后进行充分测试：

```bash
# 1. 检查规则语法
cargo run --release

# 2. 观察加载日志
# 表示成功， 表示失败

# 3. 触发测试事件
# 验证规则是否正确触发

# 4. 检查误报率
# 观察一段时间，调整规则参数
```

### 6. 文档化

在规则文件中添加详细注释：

```yaml
# ============================================
# 规则名称：SSH 暴力破解检测
# 功能：检测短时间内多次 SSH 登录失败
# 阈值：5分钟内失败5次
# 误报场景：
# - 用户忘记密码多次尝试（可接受）
# - 自动化脚本配置错误（需排除）
# 更新历史：
# 2025-12-17: 初始版本
# 2026-01-15: 调整阈值从3次到5次
# ============================================
```

---

## 示例规则

### 示例 1：敏感文件访问检测（Pattern）

```yaml
id: "rule-001-sensitive-file-access"
name: "敏感文件访问检测"
description: "检测对 /etc/passwd、/etc/shadow 等敏感文件的访问"
severity: "high"
enabled: true
type: "pattern"
tags:
 - "file"
 - "privilege-escalation"
 - "credential-access"

conditions:
 worker_name: "file_monitor"
 event_type: "file_read"
 patterns:
    path: "^/etc/(passwd|shadow|sudoers|ssh/.*_key)$"
  
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

### 示例 2：SSH 暴力破解检测（Threshold）

```yaml
id: "rule-002-ssh-brute-force"
name: "SSH暴力破解检测"
description: "检测短时间内多次 SSH 登录失败"
severity: "critical"
enabled: true
type: "threshold"
tags:
 - "network"
 - "brute-force"
 - "credential-access"

conditions:
 worker_name: "network_monitor"
 event_type: "ssh_login_failed"

threshold:
 field: "count"
 operator: ">="
 value: 5
 time_window: 300 # 5分钟
 group_by:
    - "agent_id"
    - "data.src_ip"

actions:
 - type: alert
    config:
      priority: critical
 - type: log
    config:
      file: "/var/log/artemis-hids/ssh-attacks.log"

author: "Security Team"
created_at: 2025-12-17T00:00:00Z
updated_at: 2025-12-17T00:00:00Z
```

### 示例 3：可疑进程执行检测（Pattern）

```yaml
id: "rule-003-suspicious-process"
name: "可疑进程执行检测"
description: "检测常见的攻击工具和可疑命令执行"
severity: "high"
enabled: true
type: "pattern"
tags:
 - "process"
 - "execution"
 - "malware"

conditions:
 worker_name: "process_monitor"
 event_type: "process_create"
 patterns:
    command: "(nmap|masscan|metasploit|nc -l|/bin/bash -i|python.*pty\\.spawn)"

actions:
 - type: alert
    config:
      priority: high

author: "Security Team"
created_at: 2025-12-17T00:00:00Z
updated_at: 2025-12-17T00:00:00Z
```

### 示例 4：端口扫描检测（Threshold）

```yaml
id: "rule-005-port-scan"
name: "端口扫描检测"
description: "检测短时间内访问大量不同端口"
severity: "medium"
enabled: true
type: "threshold"
tags:
 - "network"
 - "reconnaissance"

conditions:
 worker_name: "network_monitor"
 event_type: "connection_attempt"

threshold:
 field: "count"
 operator: ">="
 value: 20
 time_window: 60 # 1分钟
 group_by:
    - "agent_id"
    - "data.src_ip"

actions:
 - type: alert
    config:
      priority: medium

author: "Security Team"
created_at: 2025-12-17T00:00:00Z
updated_at: 2025-12-17T00:00:00Z
```

### 示例 5：反弹 Shell 检测（Pattern）

```yaml
id: "rule-007-reverse-shell"
name: "反弹Shell检测"
description: "检测常见的反弹 Shell 特征"
severity: "critical"
enabled: true
type: "pattern"
tags:
 - "process"
 - "command-and-control"
 - "execution"

conditions:
 worker_name: "process_monitor"
 event_type: "process_create"
 patterns:
    command: "(bash -i >& /dev/tcp/|nc.*-e.*/bin/|python.*socket.*subprocess|perl.*socket.*open)"

actions:
 - type: alert
    config:
      priority: critical

author: "Security Team"
created_at: 2025-12-17T00:00:00Z
updated_at: 2025-12-17T00:00:00Z
```

### 示例 6：WebShell 检测（Pattern）

```yaml
id: "rule-008-webshell-detection"
name: "WebShell检测"
description: "检测 Web 目录下的可疑文件创建"
severity: "critical"
enabled: true
type: "pattern"
tags:
 - "file"
 - "persistence"
 - "webshell"

conditions:
 worker_name: "file_monitor"
 event_type: "file_create"
 patterns:
    path: "^/(var/www|usr/share/nginx|home/.*/public_html)/.*\\.(php|jsp|asp|aspx)$"
 expression: "data['size'] != '' && int(data['size']) < 10240" # 小于10KB

actions:
 - type: alert
    config:
      priority: critical

author: "Security Team"
created_at: 2025-12-17T00:00:00Z
updated_at: 2025-12-17T00:00:00Z
```

### 示例 7：多阶段攻击检测（Correlation）

```yaml
id: "rule-009-multi-stage-attack"
name: "多阶段攻击检测"
description: "检测连续的侦察、利用和持久化行为"
severity: "critical"
enabled: true
type: "correlation"
tags:
 - "correlation"
 - "apt"
 - "kill-chain"

conditions:
 worker_name: "*"

correlation:
 events:
    - "port_scan"
    - "process_create"
    - "file_modify"
 time_window: 600 # 10分钟
 group_by:
    - "agent_id"
 min_count: 1

actions:
 - type: alert
    config:
      priority: critical
 - type: kafka
    config:
      topic: "high-priority-alerts"

author: "Security Team"
created_at: 2025-12-17T00:00:00Z
updated_at: 2025-12-17T00:00:00Z
```

---

## 配置说明

### Kafka 消费者配置

规则引擎支持三种 Kafka Topic 订阅方式：

**方式 1：单个 Topic（兼容旧版）**
```yaml
consumer:
 brokers:
    - "10.185.25.205:19092"
 topic: "hids_alerts"
 group_id: "rule-engine-group"
```

**方式 2：通配符模式（推荐）**
```yaml
consumer:
 brokers:
    - "10.185.25.205:19092"
 topic: "hids_events-*" # 自动订阅所有 worker topics
 group_id: "rule-engine-group"
```
使用通配符会自动订阅以下 topics：
- `hids_events-file_monitor`
- `hids_events-process_monitor`
- `hids_events-network_monitor`
- `hids_events-security_log_monitor`
- `hids_events-system_info_monitor`
- `hids_events-heartbeat`

**方式 3：显式多 Topics**
```yaml
consumer:
 brokers:
    - "10.185.25.205:19092"
 topics: # 使用 topics 替代 topic
    - "hids_events-file_monitor"
    - "hids_events-process_monitor"
    - "hids_events-network_monitor"
 group_id: "rule-engine-group"
```

### 多 Topic 架构优势

1. **按类型分离** - 不同 worker 的事件分散到不同 topic，避免单 topic 压力过大
2. **灵活订阅** - 可以选择只订阅感兴趣的 worker events
3. **便于扩展** - 方便添加新的 worker 类型而不影响现有 topics
4. **性能优化** - Kafka 可以更好地并行处理多个 topic 的数据

---

## 故障排查

### 规则加载失败

**问题：** 启动时显示 ` 加载规则失败`

**常见原因：**
1. YAML 语法错误（缩进、引号）
2. 必填字段缺失
3. `time_window` 使用了字符串格式（如 `5m`）而非秒数（如 `300`）
4. 枚举值错误（`severity`, `type` 等）
5. 日期格式错误

**解决方法：**
```bash
# 1. 检查 YAML 语法
yamllint rules/your-rule.yaml

# 2. 查看详细错误日志
RUST_LOG=debug ./target/release/rule-enginx

# 3. 对比正常加载的规则文件
```

### 规则未触发

**问题：** 规则加载成功但未触发告警

**检查清单：**
1. `enabled` 是否为 `true`
2. `worker_name` 和 `event_type` 是否匹配事件
3. 正则表达式是否正确
4. `exclude` 条件是否过于宽泛
5. 阈值设置是否过高
6. 时间窗口是否过短

### 误报过多

**问题：** 规则频繁触发误报

**优化方法：**
1. 添加 `exclude` 条件排除正常行为
2. 提高阈值
3. 延长时间窗口
4. 使用更精确的正则表达式
5. 添加更多匹配条件

---

## 参考资源

- **MITRE ATT&CK**: https://attack.mitre.org/
- **正则表达式测试**: https://regex101.com/
- **YAML 语法**: https://yaml.org/
- **Rust Regex 文档**: https://docs.rs/regex/

---

## 更新日志

- **2026-01-28**: 初始版本发布
- 包含完整的规则编写指南和示例

---

如有问题或建议，请联系安全团队。
