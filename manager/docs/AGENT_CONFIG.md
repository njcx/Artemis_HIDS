# Agent 配置管理文档

## 概述

通过 Manager API 远程管理 Agent 配置，支持：
- 启动/停止 Worker 模块
- 更新 Agent 主程序
- 更新 Worker 插件
- 动态修改配置
- 批量操作

## 架构设计

```
┌─────────────┐
│   Manager   │
│     API     │
└──────┬──────┘
       │ 下发配置命令
       ▼
┌──────────────┐
│    Kafka     │
│hids-commands │ (新增 topic)
└──────┬───────┘
       │ 消费命令
       ▼
┌──────────────┐
│    Agent     │
│  配置监听器   │
│ - 停止 Worker │
│ - 启动 Worker │
│ - 更新自己    │
│ - 更新插件    │
└──────────────┘
```

## API 接口

### 1. 获取 Agent 配置

```bash
GET /api/v1/config/agents/:id

# 示例
curl http://localhost:8080/api/v1/config/agents/agent-001
```

**响应**:
```json
{
 "agent_id": "agent-001",
 "version": "1.0.0",
 "workers": {
    "file_monitor": {
      "enabled": true,
      "config": {
        "watch_paths": ["/etc", "/var/log"],
        "interval": 5
      }
    },
    "network_monitor": {
      "enabled": true,
      "config": {}
    },
    "process_monitor": {
      "enabled": false,
      "config": {}
    }
 },
 "updated_at": "2025-12-17T10:30:00Z"
}
```

### 2. 停止 Worker

```bash
POST /api/v1/config/agents/:id/workers/:worker/stop

# 示例：停止 file_monitor
curl -X POST http://localhost:8080/api/v1/config/agents/agent-001/workers/file_monitor/stop
```

**响应**:
```json
{
 "message": "worker stop command sent",
 "command_id": "cmd-123456"
}
```

### 3. 启动 Worker

```bash
POST /api/v1/config/agents/:id/workers/:worker/start

# 示例：启动 file_monitor
curl -X POST http://localhost:8080/api/v1/config/agents/agent-001/workers/file_monitor/start
```

### 4. 更新 Agent

```bash
POST /api/v1/config/agents/:id/update
Content-Type: application/json

{
 "version": "1.1.0",
 "download_url": "https://releases.example.com/agent-1.1.0.tar.gz",
 "checksum": "sha256:abc123..."
}
```

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/config/agents/agent-001/update \
 -H "Content-Type: application/json" \
 -d '{
    "version": "1.1.0",
    "download_url": "https://releases.example.com/agent-1.1.0.tar.gz",
    "checksum": "sha256:abc123..."
 }'
```

### 5. 更新 Worker

```bash
POST /api/v1/config/agents/:id/workers/:worker/update
Content-Type: application/json

{
 "version": "1.2.0",
 "download_url": "https://releases.example.com/worker-1.2.0.so",
 "checksum": "sha256:def456..."
}
```

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/config/agents/agent-001/workers/network_monitor/update \
 -H "Content-Type: application/json" \
 -d '{
    "version": "1.2.0",
    "download_url": "https://releases.example.com/network_monitor-1.2.0.so",
    "checksum": "sha256:def456..."
 }'
```

### 6. 更新完整配置

```bash
PUT /api/v1/config/agents/:id
Content-Type: application/json

{
 "version": "1.0.0",
 "workers": {
    "file_monitor": {
      "enabled": true,
      "config": {
        "watch_paths": ["/etc", "/var/log"],
        "interval": 5
      }
    }
 }
}
```

### 7. 批量操作

```bash
POST /api/v1/config/batch
Content-Type: application/json

{
 "agent_ids": ["agent-001", "agent-002", "agent-003"],
 "command_type": "stop_worker",
 "payload": {
    "worker_name": "file_monitor"
 }
}
```

**示例**:
```bash
# 批量停止所有 agent 的 file_monitor
curl -X POST http://localhost:8080/api/v1/config/batch \
 -H "Content-Type: application/json" \
 -d '{
    "agent_ids": ["agent-001", "agent-002", "agent-003"],
    "command_type": "stop_worker",
    "payload": {
      "worker_name": "file_monitor"
    }
 }'

# 批量更新所有 agent
curl -X POST http://localhost:8080/api/v1/config/batch \
 -H "Content-Type: application/json" \
 -d '{
    "agent_ids": ["agent-001", "agent-002"],
    "command_type": "update_agent",
    "payload": {
      "version": "1.1.0",
      "download_url": "https://releases.example.com/agent-1.1.0.tar.gz"
    }
 }'
```

## 命令类型

| 命令类型 | 说明 | Payload 参数 |
|---------|------|-------------|
| `stop_worker` | 停止 Worker | `worker_name` |
| `start_worker` | 启动 Worker | `worker_name` |
| `update_agent` | 更新 Agent | `version`, `download_url`, `checksum` |
| `update_worker` | 更新 Worker | `worker_name`, `version`, `download_url`, `checksum` |
| `set_config` | 更新配置 | `config` (完整配置对象) |

## Agent 端实现

Agent 需要添加配置监听器：

```go
// agent/config_listener.go
type ConfigListener struct {
    reader *kafka.Reader
    master *AgentMaster
}

func (l *ConfigListener) Start(ctx context.Context) {
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers: []string{"localhost:9092"},
        Topic:   "hids-commands",
        GroupID: "agent-" + getAgentID(), // 每个 agent 独立的 group
    })

    for {
        msg, err := reader.ReadMessage(ctx)
        if err != nil {
            continue
        }

        var cmd ConfigCommand
        json.Unmarshal(msg.Value, &cmd)

        // 只处理发给自己的命令
        if cmd.AgentID != getAgentID() {
            continue
        }

        l.handleCommand(&cmd)
    }
}

func (l *ConfigListener) handleCommand(cmd *ConfigCommand) {
    switch cmd.CommandType {
    case "stop_worker":
        workerName := cmd.Payload["worker_name"].(string)
        l.master.StopWorker(workerName)
        
    case "start_worker":
        workerName := cmd.Payload["worker_name"].(string)
        l.master.StartWorker(workerName)
        
    case "update_agent":
        version := cmd.Payload["version"].(string)
        downloadURL := cmd.Payload["download_url"].(string)
        l.updateAgent(version, downloadURL)
        
    case "update_worker":
        workerName := cmd.Payload["worker_name"].(string)
        version := cmd.Payload["version"].(string)
        downloadURL := cmd.Payload["download_url"].(string)
        l.updateWorker(workerName, version, downloadURL)
    }
}
```

## 使用场景

### 场景 1: 紧急停止某个模块

```bash
# 发现 file_monitor 导致 CPU 高，紧急停止
curl -X POST http://localhost:8080/api/v1/config/agents/agent-001/workers/file_monitor/stop
```

### 场景 2: 灰度发布新版本

```bash
# 先更新 10% 的 agent
curl -X POST http://localhost:8080/api/v1/config/batch \
 -d '{
    "agent_ids": ["agent-001", "agent-002"],
    "command_type": "update_agent",
    "payload": {"version": "1.1.0", "download_url": "..."}
 }'

# 观察一段时间后，更新剩余 90%
```

### 场景 3: 动态调整监控配置

```bash
# 增加监控路径
curl -X PUT http://localhost:8080/api/v1/config/agents/agent-001 \
 -d '{
    "workers": {
      "file_monitor": {
        "enabled": true,
        "config": {
          "watch_paths": ["/etc", "/var/log", "/home"]
        }
      }
    }
 }'
```

## 安全建议

1. **命令认证**: 添加 JWT 认证
2. **命令签名**: 防止命令被篡改
3. **版本校验**: 验证更新包的 checksum
4. **回滚机制**: 保存旧版本，支持快速回滚
5. **审计日志**: 记录所有配置变更

## 测试

```bash
# 运行测试脚本
cd /Users/njcx/Artemis_HIDS/manager
chmod +x scripts/test_config.sh
./scripts/test_config.sh
```

## 监控

查看命令执行历史：

```bash
# 从 Redis 查看
redis-cli LRANGE agent:commands:agent-001 0 10
```

## 下一步

1. 在 Agent 端实现 ConfigListener
2. 添加 Kafka `hids-commands` topic
3. 实现 Agent 更新逻辑（下载、校验、替换、重启）
4. 添加命令执行结果反馈机制
