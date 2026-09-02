# OpenObserve 到 MongoDB 同步工具

定时轮询查询 OpenObserve，把系统信息、告警、资产数据（端口/软件包/容器/进程）同步落地到 MongoDB 的工具。

## 功能特性

- 定时轮询查询 OpenObserve `_search` API（默认每 30 秒一轮），不依赖 Kafka 消费者
- 快照型数据（系统信息、端口、软件包、容器）：每轮取每个 `agent_id` 最新一条记录，先清空该 agent 的旧记录再插入新快照（delete-then-insert），保证数据始终反映主机当前状态，不会残留已消失的端口/软件
- 流水型数据（告警、进程）：按内存游标增量拉取新记录；告警纯追加不去重，进程按 `(agent_id, pid)` upsert
- 批量写入 MongoDB，提高性能
- 优雅退出，确保当前一轮同步完整落库

## 快速开始

### 1. 配置文件

复制配置示例：
```bash
cp config.yaml config.local.yaml
```

编辑配置文件：
```yaml
# OpenObserve 配置
openobserve:
  url: "http://localhost:5080"
  org: "default"
  username: "${OPENOBSERVE_ROOT_EMAIL}"
  password: "${OPENOBSERVE_ROOT_PASS}"

# MongoDB 配置
mongodb:
  uri: "mongodb://admin:password123@localhost:27017"
  database: "artemis_hids"

# 同步配置
sync:
  poll_interval: 30    # 轮询间隔（秒）
  lookback_secs: 300   # 首次启动/游标丢失时的回溯窗口（秒）
  batch_size: 100
```

真实凭据不要写进 `config.yaml`，通过环境变量注入（Docker Compose 环境下由 `docker/init-env.sh` 生成并写入 `.env`）。

### 2. 构建

```bash
chmod +x build.sh
./build.sh
```

### 3. 运行

```bash
./mongo_syncer -config config.yaml
```

## 数据流

```
OpenObserve Stream              同步方式                MongoDB Collection
┌──────────────────┐                                    ┌──────────────────────┐
│ hids_system_info  │ ──── 快照型：GROUP BY agent_id ───▶│ agents                │
│                    │      取最新一条 + upsert          │ (按 agent_id 去重)     │
└──────────────────┘                                    └──────────────────────┘

┌──────────────────┐                                    ┌──────────────────────┐
│ hids_alerts        │ ──── 流水型：按游标增量拉取 ─────▶│ agent_security_events │
│                    │      纯追加，不去重                │ (不去重，直接插入)     │
└──────────────────┘                                    └──────────────────────┘

┌──────────────────┐                                    ┌──────────────────────┐
│ hids_port_monitor  │ ──── 快照型：delete-then-insert ─▶│ agent_ports           │
└──────────────────┘                                    └──────────────────────┘

┌──────────────────┐                                    ┌──────────────────────┐
│ hids_package_info  │ ──── 快照型：delete-then-insert ─▶│ agent_software        │
└──────────────────┘                                    └──────────────────────┘

┌──────────────────┐                                    ┌──────────────────────┐
│ hids_container_info│ ──── 快照型：delete-then-insert ─▶│ agent_containers      │
└──────────────────┘                                    └──────────────────────┘

┌──────────────────┐                                    ┌──────────────────────┐
│ hids_process_monitor│ ─── 流水型：按 (agent_id,pid) ──▶│ agent_processes       │
│                    │      upsert                        │                       │
└──────────────────┘                                    └──────────────────────┘
```

## 配置说明

### OpenObserve 配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `url` | OpenObserve 服务地址 | - |
| `org` | 组织名 | default |
| `username` | Basic Auth 用户名 | - |
| `password` | Basic Auth 密码 | - |

### MongoDB 配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `uri` | MongoDB 连接 URI | - |
| `database` | 数据库名称 | - |
| `max_pool_size` | 最大连接池大小 | 100 |
| `min_pool_size` | 最小连接池大小 | 10 |
| `max_conn_idle_time` | 连接最大空闲时间（秒） | 3600 |

### 同步配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `poll_interval` | 轮询间隔（秒） | 30 |
| `lookback_secs` | 首次启动/游标丢失时的回溯窗口（秒） | 300 |
| `batch_size` | 批量写入 MongoDB 时的分批大小 | 100 |

## 数据格式说明

OpenObserve 摄入数据时会对原始事件做两点转换，落库转换代码（`database/mongodb.go`）需要相应处理：

1. **字段扁平化**：原始事件里嵌套的 `data.xxx` 字段，在 OpenObserve 中变成顶层的 `data_xxx` 字段。例如 Agent 上报的 `{"data": {"cpu_model": "..."}}` 在 OpenObserve 里查出来是 `{"data_cpu_model": "..."}`。
2. **数组字段字符串化**：原始事件里的嵌套数组（如端口列表、软件包列表），在 OpenObserve 里变成字符串化的 JSON（如 `"data_ports": "[{\"local_port\":\"8080\",...}]"`），读取时需要二次 `json.Unmarshal`（见 `openobserve.ExtractJSONArray`）。
3. **数字类型统一为 float64**：包括原本是 int 的字段（如 `pid`/`port_count`），OpenObserve 返回时统一是 float64，需要显式转换（见 `openobserve.ToInt64` 等辅助函数）。

### hids_system_info 示例（OpenObserve 实际返回格式）

```json
{
  "_timestamp": 1788318449412000,
  "agent_id": "agent-001",
  "worker_name": "system_monitor",
  "event_type": "system_info",
  "timestamp": 1788318448000000000,
  "data_hostname": "web-server-01",
  "data_os_platform": "ubuntu",
  "data_cpu_model": "Intel Xeon",
  "data_cpu_load_percent": 45.2,
  "data_memory_total_gb": 32.0,
  "data_memory_percent": 68.5,
  "data_local_ips": ["192.168.1.100"]
}
```

### hids_alerts 示例

```json
{
  "_timestamp": 1788318514425000,
  "agent_id": "agent-001",
  "event_type": "syscall_exec",
  "worker_name": "exec_monitor",
  "severity": "high",
  "rule_id": "rule-xxx",
  "rule_name": "可疑进程执行",
  "description": "检测到未知进程",
  "timestamp": 1788318511000000000,
  "data_process_name": "unknown_proc",
  "data_exe": "/tmp/unknown_proc",
  "data_user": "www-data"
}
```

### hids_port_monitor 示例（快照型，`data_ports` 为字符串化 JSON 数组）

```json
{
  "agent_id": "agent-001",
  "data_port_count": 3.0,
  "data_ports": "[{\"protocol\":\"tcp\",\"local_addr\":\"0.0.0.0\",\"local_port\":\"22\",\"pid\":\"1234\",\"process_name\":\"sshd\",\"user\":\"root\"}]"
}
```

## 监控

### 日志输出

程序会输出详细的日志信息：

```
[INFO] 连接到 MongoDB 成功: artemis_hids
[INFO] 启动 OpenObserve 到 MongoDB 轮询同步器
[INFO] 开始一轮同步
[INFO] 插入新 agent: agent-001
[INFO] 系统信息同步: 成功=1, 失败=0
[INFO] 批量插入告警: 35 条
[INFO] 端口快照同步: agent_id=agent-001, count=47
[INFO] 软件包快照同步: agent_id=agent-001, count=2324
[INFO] 容器快照同步: agent_id=agent-001, count=20
[INFO] 进程同步: 成功=161, 失败=0
[INFO] 本轮同步完成
```

## 故障排查

### 问题1: 连接 OpenObserve 失败

```bash
# 检查 OpenObserve 是否运行
docker ps | grep openobserve

# 手动测试查询
curl -u "$OPENOBSERVE_ROOT_EMAIL:$OPENOBSERVE_ROOT_PASS" \
  http://localhost:5080/api/default/streams
```

### 问题2: 连接 MongoDB 失败

```bash
# 检查 MongoDB 是否运行
docker ps | grep mongo

# 测试连接
mongosh "mongodb://admin:password123@localhost:27017"
```

### 问题3: 数据没有更新 / 落库延迟

- 确认 OpenObserve 里对应 stream 确实有新数据写入（Agent 端采集是否正常）
- 减小 `poll_interval` 提高轮询频率
- 快照型 collection（`agent_ports` 等）记录数不应随轮询次数持续增长——如果观察到持续增长，说明 delete-then-insert 逻辑异常，需要检查 `agent_id` 匹配是否正确
- 流水型 collection（`agent_processes`/`agent_security_events`）如果长期没有新数据，检查内存游标是否卡住（重启进程会重置游标，回退到 `lookback_secs` 窗口重新拉取）

## 相关文档

- [OpenObserve 查询 API 文档](https://openobserve.ai/docs/api/)
- [MongoDB Go Driver 文档](https://www.mongodb.com/docs/drivers/go/current/)

## 许可证

MIT License
