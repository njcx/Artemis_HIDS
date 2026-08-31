// ============================================
// Agent 配置管理 API (V2 - 基于心跳下发)
// ============================================
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentConfig Agent 配置
type AgentConfig struct {
	AgentID   string                  `json:"agent_id"`
	Version   string                  `json:"version"`
	Workers   map[string]WorkerConfig `json:"workers"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// ConfigCommand 配置命令
type ConfigCommand struct {
	CommandID   string                 `json:"command_id"`
	AgentID     string                 `json:"agent_id"`
	CommandType string                 `json:"command_type"` // stop_worker, start_worker, update_agent, update_worker, set_config
	Payload     map[string]interface{} `json:"payload"`
	Timestamp   int64                  `json:"timestamp"`
}

// setupConfigRoutes 设置配置管理路由
func (s *APIServer) setupConfigRoutes() {
	config := s.router.Group("/api/v1/config")
	config.Use(AuthMiddleware(s.db))
	config.Use(RequirePermission(PermManageConfig))
	{
		// 获取 agent 配置
		config.GET("/agents/:id", s.getAgentConfig)

		// 更新 agent 配置
		config.PUT("/agents/:id", s.updateAgentConfig)

		// 停止 worker
		config.POST("/agents/:id/workers/:worker/stop", s.stopWorker)

		// 启动 worker
		config.POST("/agents/:id/workers/:worker/start", s.startWorker)

		// 更新 agent
		config.POST("/agents/:id/update", s.updateAgent)

		// 更新 worker
		config.POST("/agents/:id/workers/:worker/update", s.updateWorker)

		// 批量操作
		config.POST("/batch", s.batchCommand)

		// 查询命令执行结果
		config.GET("/commands/:id/result", s.getCommandResult)

		// 查询命令历史
		config.GET("/agents/:id/commands", s.getCommandHistory)
	}
}

// updateAgentConfig 更新 agent 配置
func (s *APIServer) updateAgentConfig(c *gin.Context) {
	agentID := c.Param("id")
	ctx := c.Request.Context()

	var config AgentConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config.AgentID = agentID
	config.UpdatedAt = time.Now()

	// 保存到 Redis
	data, _ := json.Marshal(config)
	key := fmt.Sprintf("agent:config:%s", agentID)
	s.redisClient.Set(ctx, key, data, 0)

	// 发送配置更新命令
	cmd := &ConfigCommand{
		CommandID:   uuid.New().String(),
		AgentID:     agentID,
		CommandType: "set_config",
		Payload: map[string]interface{}{
			"config": config,
		},
		Timestamp: time.Now().UnixNano(),
	}

	if err := s.sendCommand(ctx, cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "config updated",
		"command_id": cmd.CommandID,
	})
}

// stopWorker 停止 worker
func (s *APIServer) stopWorker(c *gin.Context) {
	agentID := c.Param("id")
	workerName := c.Param("worker")
	ctx := c.Request.Context()

	cmd := &ConfigCommand{
		CommandID:   uuid.New().String(),
		AgentID:     agentID,
		CommandType: "stop_worker",
		Payload: map[string]interface{}{
			"worker_name": workerName,
		},
		Timestamp: time.Now().UnixNano(),
	}

	if err := s.sendCommand(ctx, cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "worker stop command sent",
		"command_id": cmd.CommandID,
	})
}

// startWorker 启动 worker
func (s *APIServer) startWorker(c *gin.Context) {
	agentID := c.Param("id")
	workerName := c.Param("worker")
	ctx := c.Request.Context()

	cmd := &ConfigCommand{
		CommandID:   uuid.New().String(),
		AgentID:     agentID,
		CommandType: "start_worker",
		Payload: map[string]interface{}{
			"worker_name": workerName,
		},
		Timestamp: time.Now().UnixNano(),
	}

	if err := s.sendCommand(ctx, cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "worker start command sent",
		"command_id": cmd.CommandID,
		"eta":        "within 30 seconds",
	})
}

// updateAgent 更新 agent
func (s *APIServer) updateAgent(c *gin.Context) {
	agentID := c.Param("id")
	ctx := c.Request.Context()

	var req struct {
		Version     string `json:"version"`
		DownloadURL string `json:"download_url"`
		Checksum    string `json:"checksum"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd := &ConfigCommand{
		CommandID:   uuid.New().String(),
		AgentID:     agentID,
		CommandType: "update_agent",
		Payload: map[string]interface{}{
			"version":      req.Version,
			"download_url": req.DownloadURL,
			"checksum":     req.Checksum,
		},
		Timestamp: time.Now().UnixNano(),
	}

	if err := s.sendCommand(ctx, cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "agent update command sent",
		"command_id": cmd.CommandID,
		"eta":        "within 30 seconds",
	})
}

// updateWorker 更新 worker
func (s *APIServer) updateWorker(c *gin.Context) {
	agentID := c.Param("id")
	workerName := c.Param("worker")
	ctx := c.Request.Context()

	var req struct {
		Version     string `json:"version"`
		DownloadURL string `json:"download_url"`
		Checksum    string `json:"checksum"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd := &ConfigCommand{
		CommandID:   uuid.New().String(),
		AgentID:     agentID,
		CommandType: "update_worker",
		Payload: map[string]interface{}{
			"worker_name":  workerName,
			"version":      req.Version,
			"download_url": req.DownloadURL,
			"checksum":     req.Checksum,
		},
		Timestamp: time.Now().UnixNano(),
	}

	if err := s.sendCommand(ctx, cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "worker update command sent",
		"command_id": cmd.CommandID,
		"eta":        "within 30 seconds",
	})
}

// batchCommand 批量执行命令
func (s *APIServer) batchCommand(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		AgentIDs    []string               `json:"agent_ids"`
		CommandType string                 `json:"command_type"`
		Payload     map[string]interface{} `json:"payload"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := make([]gin.H, 0, len(req.AgentIDs))
	for _, agentID := range req.AgentIDs {
		cmd := &ConfigCommand{
			CommandID:   uuid.New().String(),
			AgentID:     agentID,
			CommandType: req.CommandType,
			Payload:     req.Payload,
			Timestamp:   time.Now().UnixNano(),
		}

		err := s.sendCommand(ctx, cmd)
		results = append(results, gin.H{
			"agent_id":   agentID,
			"command_id": cmd.CommandID,
			"success":    err == nil,
			"error": func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(req.AgentIDs),
		"results": results,
	})
}

// sendCommand 发送命令到 Redis 队列
func (s *APIServer) sendCommand(ctx context.Context, cmd *ConfigCommand) error {
	data, _ := json.Marshal(cmd)

	// 保存到 Redis 队列
	key := fmt.Sprintf("agent:commands:%s", cmd.AgentID)
	if err := s.redisClient.RPush(ctx, key, data).Err(); err != nil {
		return err
	}

	// 设置过期时间 (24小时未执行自动删除)
	s.redisClient.Expire(ctx, key, 24*time.Hour)

	// 保存命令历史
	historyKey := fmt.Sprintf("agent:command:history:%s", cmd.AgentID)
	s.redisClient.LPush(ctx, historyKey, data)
	s.redisClient.LTrim(ctx, historyKey, 0, 99)            // 保留最近 100 条
	s.redisClient.Expire(ctx, historyKey, 30*24*time.Hour) // 保留 30 天

	return nil
}

// getCommandResult 查询命令执行结果
func (s *APIServer) getCommandResult(c *gin.Context) {
	commandID := c.Param("id")
	ctx := c.Request.Context()

	key := fmt.Sprintf("agent:command:result:%s", commandID)
	result, err := s.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(result) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "result not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"command_id": commandID,
		"agent_id":   result["agent_id"],
		"success":    result["success"] == "1" || result["success"] == "true",
		"message":    result["message"],
		"timestamp":  result["timestamp"],
	})
}

// getCommandHistory 查询命令历史
func (s *APIServer) getCommandHistory(c *gin.Context) {
	agentID := c.Param("id")
	ctx := c.Request.Context()

	key := fmt.Sprintf("agent:command:history:%s", agentID)
	history, err := s.redisClient.LRange(ctx, key, 0, 49).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get history"})
		return
	}

	commands := make([]map[string]interface{}, 0, len(history))
	for _, cmdData := range history {
		var cmd map[string]interface{}
		if err := json.Unmarshal([]byte(cmdData), &cmd); err == nil {
			commands = append(commands, cmd)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"total":    len(commands),
		"commands": commands,
	})
}

// getAgentConfig 获取 agent 配置
func (s *APIServer) getAgentConfig(c *gin.Context) {
	agentID := c.Param("id")
	ctx := c.Request.Context()

	// 从 Redis 获取 agent 状态（包含配置信息）
	key := fmt.Sprintf("agent:status:%s", agentID)
	data, err := s.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"config":   data,
	})
}
