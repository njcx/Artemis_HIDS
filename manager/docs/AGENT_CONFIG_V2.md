# Agent 配置管理设计 V2 (优雅版)

## 问题分析

### V1 方案的问题
1. **Agent 无法获取命令**: 只在 Manager 端保存到 Redis，Agent 无法感知
2. **Kafka topic 设计不当**: 每个 agent 独立 consumer group 会导致大量连接
3. **没有利用现有连接**: Agent 已经有 gRPC 连接到 Server

### V2 方案 - 基于心跳的命令下发

```
┌─────────────┐        ┌──────────────┐        ┌──────────────┐
│   Manager   │        │     Redis    │        │    Server    │
│     API     │        │   (命令队列)  │        │    (gRPC)    │
└──────┬──────┘        └──────┬───────┘        └──────┬───────┘
       │ 1. 下发命令           │                        │
       │ POST /config/...      │                        │
       ├──────────────────────>│                        │
       │ Set agent:commands:ID │                        │
       │                       │                        │
       │                       │  3. Server 从 Redis     │
       │                       │     拉取命令            │
       │                       │<───────────────────────┤
       │                       │                        │
       │                       │                        │ 2. Agent 心跳
       │                       │                        │<────────────┐
       │                       │                        │             │
       │                       │  4. 响应中返回命令       │             │
       │                       │───────────────────────>│─────────────┤
       │                       │                        │    Agent    │
       │                       │                        │             │
       │                       │  5. Agent 上报执行结果   │             │
       │                       │<───────────────────────┤<────────────┘
       │  6. Server 写入结果    │                        │
       │<──────────────────────┤                        │
       │                       │                        │
```

## 核心设计

### 1. 在心跳响应中下发命令

**优势**:
- 利用现有 gRPC 连接，无额外开销
- Agent 每 30 秒心跳一次，命令延迟最多 30 秒
- 无需 Kafka 额外 topic
- Server 统一管理命令分发

**流程**:
```go
// Agent 心跳
heartbeat := &HeartbeatRequest{
    AgentID: "agent-001",
    Version: "1.0.0",
    Workers: map[string]WorkerStatus{
        "file_monitor": {Enabled: true, Status: "running"},
    },
}

response := client.Heartbeat(ctx, heartbeat)

// 响应中包含待执行的命令
for _, cmd := range response.Commands {
    switch cmd.CommandType {
    case "stop_worker":
        master.StopWorker(cmd.Payload["worker_name"])
    case "update_agent":
        updateAgent(cmd.Payload["version"], cmd.Payload["download_url"])
    }
    
    // 上报执行结果
    client.ReportCommandResult(ctx, &CommandResult{
        CommandID: cmd.CommandID,
        Success: true,
        Message: "executed",
    })
}
```

### 2. Redis 作为命令队列

```
# 数据结构
agent:commands:{agent_id} -> List [cmd1, cmd2, cmd3]
agent:command:result:{cmd_id} -> Hash {success, message, timestamp}
```

**Manager API 下发命令**:
```go
func (s *APIServer) stopWorker(c *gin.Context) {
    cmd := &ConfigCommand{
        CommandID: uuid.New().String(),
        AgentID: agentID,
        CommandType: "stop_worker",
        Payload: map[string]interface{}{"worker_name": workerName},
        Timestamp: time.Now().UnixNano(),
    }
    
    // 保存到 Redis 队列
    key := fmt.Sprintf("agent:commands:%s", agentID)
    data, _ := json.Marshal(cmd)
    s.redisClient.RPush(ctx, key, data)
    
    // 设置过期时间(24小时未执行自动删除)
    s.redisClient.Expire(ctx, key, 24*time.Hour)
}
```

**Server 在心跳响应时获取命令**:
```go
func (s *HIDSServer) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
    // 从 Redis 获取待执行的命令
    key := fmt.Sprintf("agent:commands:%s", req.AgentID)
    
    cmds := s.redisClient.LRange(ctx, key, 0, 9).Val() // 最多返回 10 条
    commands := make([]*Command, 0, len(cmds))
    
    for _, cmdData := range cmds {
        var cmd ConfigCommand
        json.Unmarshal([]byte(cmdData), &cmd)
        
        commands = append(commands, &Command{
            CommandID: cmd.CommandID,
            CommandType: cmd.CommandType,
            Payload: cmd.Payload,
            Timestamp: cmd.Timestamp,
        })
        
        // 从队列中移除
        s.redisClient.LRem(ctx, key, 1, cmdData)
    }
    
    return &HeartbeatResponse{
        Success: true,
        Interval: 30,
        Commands: commands,
    }, nil
}
```

### 3. 命令执行结果反馈

```go
func (s *HIDSServer) ReportCommandResult(ctx context.Context, result *CommandResult) (*CommandResultAck, error) {
    // 保存执行结果到 Redis
    key := fmt.Sprintf("agent:command:result:%s", result.CommandID)
    s.redisClient.HSet(ctx, key, map[string]interface{}{
        "agent_id": result.AgentID,
        "success": result.Success,
        "message": result.Message,
        "timestamp": result.Timestamp,
    })
    s.redisClient.Expire(ctx, key, 7*24*time.Hour) // 保留7天
    
    return &CommandResultAck{Received: true}, nil
}
```

## 完整流程示例

### 场景: 紧急停止 file_monitor 模块

```bash
# 1. 管理员通过 Manager API 下发命令
curl -X POST http://localhost:8080/api/v1/config/agents/agent-001/workers/file_monitor/stop

# Manager 返回
{
 "command_id": "cmd-123456",
 "message": "command queued",
 "eta": "within 30 seconds" # Agent 下次心跳时执行
}

# 2. Agent 在 30 秒内发送心跳到 Server
# 3. Server 从 Redis 获取命令并在响应中返回
# 4. Agent 执行命令: master.StopWorker("file_monitor")
# 5. Agent 上报执行结果: ReportCommandResult(...)
# 6. Server 将结果写入 Redis

# 7. 管理员查询命令执行结果
curl http://localhost:8080/api/v1/config/commands/cmd-123456/result

# Manager 返回
{
 "command_id": "cmd-123456",
 "agent_id": "agent-001",
 "success": true,
 "message": "worker stopped",
 "executed_at": "2025-12-17T10:35:22Z"
}
```

## 优势对比

| 特性 | V1 方案 (Kafka) | V2 方案 (心跳) |
|------|----------------|---------------|
| **网络连接** | Agent 需要连接 Kafka | 利用现有 gRPC 连接 |
| **延迟** | 实时 | 最多 30 秒 (心跳间隔) |
| **复杂度** | 高 (Kafka consumer groups) | 低 (Redis + gRPC) |
| **可靠性** | 需要保证 Kafka 可用 | Server/Redis 已经是依赖 |
| **成本** | 额外 Kafka 连接开销 | 几乎无额外开销 |
| **紧急命令** | 支持实时推送 | 30秒延迟可接受 |

## 处理紧急命令 (可选)

如果 30 秒延迟不可接受，可以添加 **轮询接口**:

```go
// Agent 启动独立协程，每 5 秒轮询一次
go func() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        resp, err := client.GetCommands(ctx, &CommandRequest{
            AgentID: agentID,
        })
        if err == nil && len(resp.Commands) > 0 {
            for _, cmd := range resp.Commands {
                executeCommand(cmd)
            }
        }
    }
}()
```

**权衡**:
- 5 秒轮询 vs 30 秒心跳
- 网络开销增加 6 倍 vs 命令延迟降低 6 倍

## 实现清单

### Server 端
- [ ] 添加 proto 定义
- [ ] 实现 Heartbeat 返回命令逻辑
- [ ] 实现 ReportCommandResult RPC
- [ ] 连接 Redis 获取命令队列

### Manager 端
- [ ] API 下发命令到 Redis
- [ ] 提供命令结果查询接口
- [ ] 命令历史记录查询

### Agent 端
- [ ] 修改心跳逻辑处理命令
- [ ] 实现命令执行器 (stop/start worker)
- [ ] 上报执行结果
- [ ] (可选) 实现快速轮询

## 下一步

推荐使用 **V2 方案 (心跳下发)**，理由：
1. 最小改动 - 利用现有架构
2. 零额外成本 - 无新增连接
3. 30秒延迟完全可接受 (配置管理不是实时性要求高的场景)
4. 简单可靠 - Redis + gRPC

如果需要，我可以立即实现 V2 方案的完整代码！
