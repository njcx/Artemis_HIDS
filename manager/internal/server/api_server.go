package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// APIServer API 服务器
type APIServer struct {
	redisClient *redis.Client
	db          *mongo.Database
	router      *gin.Engine
}

// NewAPIServer 创建 API 服务器
func NewAPIServer(redisClient *redis.Client, db *mongo.Database) *APIServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 全局 CORS 中间件（必须最先设置）
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Captcha-ID")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// 调试日志：打印请求头，用于排查 403 问题
		// log.Printf("[DEBUG] Request Headers: %v", c.Request.Header)

		c.Next()
	})

	server := &APIServer{
		redisClient: redisClient,
		db:          db,
		router:      router,
	}

	// 启动验证码清理协程
	captchaStore.CleanupExpiredCaptchas()

	server.setupAuthRoutes()   // 认证路由
	server.setupRoutes()       // 主要路由
	server.setupConfigRoutes() // 配置管理路由
	server.setupRuleRoutes()   // 规则管理路由
	server.setupAssetRoutes()  // 资产识别路由
	return server
}

// setupRoutes 设置路由
func (s *APIServer) setupRoutes() {
	// 静态文件服务 - 前端页面
	s.router.Static("/web", "./web")
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/web/login.html")
	})

	// 健康检查（无需认证）
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API 路由组（需要认证）
	api := s.router.Group("/api/v1")
	api.Use(AuthMiddleware(s.db)) // 所有 API 需要认证
	{
		// 统计信息
		api.GET("/stats", RequirePermission(PermViewDashboard), s.getStats)
		api.GET("/stats/realtime", RequirePermission(PermViewDashboard), s.getRealtimeStats)
		api.GET("/stats/dashboard", RequirePermission(PermViewDashboard), s.getDashboardStats)

		// Agent 管理
		api.GET("/agents", RequirePermission(PermViewAgents), s.listAgents)
		api.GET("/agents/:id", RequirePermission(PermViewAgents), s.getAgent)
		api.GET("/agents/:id/status", RequirePermission(PermViewAgents), s.getAgentStatus)
		api.GET("/agents/online", RequirePermission(PermViewAgents), s.getOnlineAgents)

		// 告警管理
		api.GET("/alerts", RequirePermission(PermViewDashboard), s.listAlerts)
		api.GET("/alerts/:id", RequirePermission(PermViewDashboard), s.getAlert)
		api.PUT("/alerts/:id/status", RequirePermission(PermViewDashboard), s.updateAlertStatus)
	}
}

// Start 启动服务器
func (s *APIServer) Start(addr string) error {
	server := &http.Server{
		Addr:           addr,
		Handler:        s.router,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 26, // 64MB
	}
	return server.ListenAndServe()
}

// getStats 获取统计信息
func (s *APIServer) getStats(c *gin.Context) {
	ctx := c.Request.Context()

	// 从 Redis 获取统计数据
	pipe := s.redisClient.Pipeline()
	onlineCmd := pipe.Get(ctx, "stats:agents:online")
	totalCmd := pipe.Get(ctx, "stats:agents:total")
	_, err := pipe.Exec(ctx)

	stats := gin.H{
		"agents": gin.H{
			"online": 0,
			"total":  0,
		},
		"timestamp": time.Now().Unix(),
	}

	if err == nil {
		online, _ := onlineCmd.Int64()
		total, _ := totalCmd.Int64()
		stats["agents"] = gin.H{
			"online": online,
			"total":  total,
		}
	}

	c.JSON(http.StatusOK, stats)
}

// getRealtimeStats 获取实时统计（WebSocket）
func (s *APIServer) getRealtimeStats(c *gin.Context) {
	// TODO: 实现 WebSocket 推送
	c.JSON(http.StatusOK, gin.H{"message": "WebSocket endpoint"})
}

// getDashboardStats 获取仪表盘统计数据（从缓存表读取）
func (s *APIServer) getDashboardStats(c *gin.Context) {
	ctx := context.Background()
	var stats DashboardStats

	// 从统计表中读取最新的统计数据
	opts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	err := s.db.Collection("dashboard_stats").FindOne(ctx, bson.M{}, opts).Decode(&stats)

	if err != nil {
		// 如果表中没有数据，则实时计算一次
		stats = s.calculateDashboardStats()
		stats.CreatedAt = time.Now()
		stats.UpdatedAt = time.Now()
		// 保存到数据库
		s.db.Collection("dashboard_stats").InsertOne(ctx, stats)
	}

	c.JSON(http.StatusOK, gin.H{
		"total_agents":    stats.TotalAgents,
		"online_agents":   stats.OnlineAgents,
		"online_rate":     stats.OnlineRate,
		"today_alerts":    stats.TodayAlerts,
		"pending_alerts":  stats.PendingAlerts,
		"total_alerts":    stats.TotalAlerts,
		"critical_alerts": stats.CriticalAlerts,
		"high_alerts":     stats.HighAlerts,
		"medium_alerts":   stats.MediumAlerts,
		"low_alerts":      stats.LowAlerts,
		"timestamp":       stats.UpdatedAt.Unix(),
	})
}

// calculateDashboardStats 计算仪表盘统计数据
func (s *APIServer) calculateDashboardStats() DashboardStats {
	ctx := context.Background()
	var stats DashboardStats

	// 统计总主机数
	filter := bson.M{"deleted_at": nil}
	stats.TotalAgents, _ = s.db.Collection("agents").CountDocuments(ctx, filter)

	// 统计在线主机（最近5分钟有心跳）
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	onlineFilter := bson.M{
		"last_heartbeat": bson.M{"$gt": fiveMinutesAgo},
		"deleted_at":     nil,
	}
	stats.OnlineAgents, _ = s.db.Collection("agents").CountDocuments(ctx, onlineFilter)

	// 计算在线率
	if stats.TotalAgents > 0 {
		stats.OnlineRate = float64(stats.OnlineAgents) / float64(stats.TotalAgents) * 100
	}

	// 统计今日安全事件数（从 agent_security_events 表）
	startOfDay := time.Now().Truncate(24 * time.Hour)
	todayFilter := bson.M{
		"created_at": bson.M{"$gte": startOfDay},
	}
	stats.TodayAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, todayFilter)

	// 统计待处理告警（状态为 open 的安全事件）
	pendingFilter := bson.M{"status": "open"}
	stats.PendingAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, pendingFilter)

	// 统计告警总条数
	stats.TotalAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{})

	// 统计不同严重程度的告警
	stats.CriticalAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "critical"})
	stats.HighAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "high"})
	stats.MediumAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "medium"})
	stats.LowAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "low"})

	stats.UpdatedAt = time.Now()
	return stats
}

// UpdateDashboardStats 定期更新仪表盘统计数据
func (s *APIServer) UpdateDashboardStats() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒更新一次
	defer ticker.Stop()

	// 首次启动时立即更新一次
	s.upsertDashboardStats()
	log.Println("仪表盘统计数据初始化完成")

	for range ticker.C {
		s.upsertDashboardStats()
	}
}

// upsertDashboardStats 插入或更新当天的统计数据
func (s *APIServer) upsertDashboardStats() {
	ctx := context.Background()
	stats := s.calculateDashboardStats()

	// 获取当天日期（只保留日期部分）
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	stats.StatDate = today
	stats.CreatedAt = now
	stats.UpdatedAt = now

	// 使用 Upsert 实现插入或更新
	filter := bson.M{"stat_date": today}
	update := bson.M{"$set": stats}
	opts := options.Update().SetUpsert(true)

	result, err := s.db.Collection("dashboard_stats").UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Printf("更新仪表盘统计数据失败: %v", err)
		return
	}

	if result.ModifiedCount > 0 || result.UpsertedCount > 0 {
		log.Printf("仪表盘统计数据已更新: 日期=%s, 总主机=%d, 在线=%d, 今日告警=%d, 待处理=%d, 总告警=%d, 严重=%d, 高危=%d, 中危=%d, 低危=%d",
			today.Format("2006-01-02"), stats.TotalAgents, stats.OnlineAgents, stats.TodayAlerts,
			stats.PendingAlerts, stats.TotalAlerts, stats.CriticalAlerts, stats.HighAlerts,
			stats.MediumAlerts, stats.LowAlerts)
	}
}

// listAgents 列出 agents
func (s *APIServer) listAgents(c *gin.Context) {
	ctx := context.Background()
	var agents []Agent

	// 从MongoDB查询所有agent（排除已删除）
	filter := bson.M{"deleted_at": nil}
	cursor, err := s.db.Collection("agents").Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &agents); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为响应格式
	results := make([]gin.H, 0, len(agents))
	for _, agent := range agents {
		// 判断是否在线（最近5分钟有心跳）
		online := false
		if agent.LastHeartbeat != nil {
			online = time.Since(*agent.LastHeartbeat) < 5*time.Minute
		}

		results = append(results, gin.H{
			"agent_id":       agent.AgentID,
			"hostname":       agent.Hostname,
			"ip":             agent.IP,
			"os":             agent.OS,
			"platform":       agent.Platform,
			"cpu":            agent.CPU,
			"memory":         agent.Memory,
			"disk":           agent.Disk,
			"version":        agent.Version,
			"status":         agent.Status,
			"online":         online,
			"last_heartbeat": agent.LastHeartbeat,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  len(results),
		"agents": results,
	})
}

// getAgent 获取 agent 详情
func (s *APIServer) getAgent(c *gin.Context) {
	ctx := context.Background()
	agentID := c.Param("id")

	var agent Agent
	filter := bson.M{"agent_id": agentID, "deleted_at": nil}
	if err := s.db.Collection("agents").FindOne(ctx, filter).Decode(&agent); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// 手动加载关联数据
	// 1. 加载进程
	processCursor, _ := s.db.Collection("agent_processes").Find(ctx, bson.M{"agent_id": agentID})
	processCursor.All(ctx, &agent.Processes)

	// 2. 加载端口
	portCursor, _ := s.db.Collection("agent_ports").Find(ctx, bson.M{"agent_id": agentID})
	portCursor.All(ctx, &agent.Ports)

	// 3. 加载软件
	softwareCursor, _ := s.db.Collection("agent_software").Find(ctx, bson.M{"agent_id": agentID})
	softwareCursor.All(ctx, &agent.Software)

	// 4. 加载服务
	serviceCursor, _ := s.db.Collection("agent_services").Find(ctx, bson.M{"agent_id": agentID})
	serviceCursor.All(ctx, &agent.Services)

	// 5. 加载容器
	containerCursor, _ := s.db.Collection("agent_containers").Find(ctx, bson.M{"agent_id": agentID})
	containerCursor.All(ctx, &agent.Containers)

	// 6. 加载连接
	connectionCursor, _ := s.db.Collection("agent_connections").Find(ctx, bson.M{"agent_id": agentID})
	connectionCursor.All(ctx, &agent.Connections)

	// 7. 加载安全事件
	eventCursor, _ := s.db.Collection("agent_security_events").Find(ctx, bson.M{"agent_id": agentID})
	eventCursor.All(ctx, &agent.SecurityEvents)

	// 判断是否在线（最近5分钟有心跳）
	online := false
	if agent.LastHeartbeat != nil {
		online = time.Since(*agent.LastHeartbeat) < 5*time.Minute
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_id":        agent.AgentID,
		"hostname":        agent.Hostname,
		"ip":              agent.IP,
		"os":              agent.OS,
		"platform":        agent.Platform,
		"cpu":             agent.CPU,
		"memory":          agent.Memory,
		"disk":            agent.Disk,
		"version":         agent.Version,
		"status":          agent.Status,
		"online":          online,
		"last_heartbeat":  agent.LastHeartbeat,
		"processes":       agent.Processes,
		"ports":           agent.Ports,
		"software":        agent.Software,
		"services":        agent.Services,
		"containers":      agent.Containers,
		"connections":     agent.Connections,
		"security_events": agent.SecurityEvents,
		// 新增硬件详细信息
		"cpu_model":          agent.CPUModel,
		"cpu_physical_count": agent.CPUPhysicalCount,
		"cpu_load_percent":   agent.CPULoadPercent,
		"memory_total_gb":    agent.MemoryTotalGB,
		"memory_percent":     agent.MemoryPercent,
		"disk_total_gb":      agent.DiskTotalGB,
		"disk_count":         agent.DiskCount,
		"disk_percent":       agent.DiskPercent,
		"ip_count":           agent.IPCount,
		"local_ips":          agent.LocalIPs,
		"data":               agent.Data,
	})
}

// getAgentStatus 获取 agent 状态
func (s *APIServer) getAgentStatus(c *gin.Context) {
	ctx := context.Background()
	agentID := c.Param("id")

	var agent Agent
	filter := bson.M{"agent_id": agentID, "deleted_at": nil}
	if err := s.db.Collection("agents").FindOne(ctx, filter).Decode(&agent); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// 判断是否在线（最近5分钟有心跳）
	online := false
	if agent.LastHeartbeat != nil {
		online = time.Since(*agent.LastHeartbeat) < 5*time.Minute
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_id":       agent.AgentID,
		"hostname":       agent.Hostname,
		"ip":             agent.IP,
		"os":             agent.OS,
		"version":        agent.Version,
		"status":         agent.Status,
		"online":         online,
		"last_heartbeat": agent.LastHeartbeat,
	})
}

// getAgentStatusByID 从 Redis 获取 agent 状态
func (s *APIServer) getAgentStatusByID(ctx context.Context, agentID string) (gin.H, error) {
	key := fmt.Sprintf("agent:status:%s", agentID)
	data, err := s.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("agent not found")
	}

	status := gin.H{
		"agent_id": agentID,
	}
	for k, v := range data {
		status[k] = v
	}

	// 判断是否在线（最近5分钟有心跳）
	if lastHeartbeat, ok := data["last_heartbeat"]; ok {
		timestamp, _ := strconv.ParseInt(lastHeartbeat, 10, 64)
		online := time.Now().Unix()-timestamp < 300 // 5分钟
		status["online"] = online
	}

	return status, nil
}

// getOnlineAgents 获取在线 agents
func (s *APIServer) getOnlineAgents(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取最近5分钟有心跳的 agent
	minScore := float64(time.Now().Add(-5 * time.Minute).Unix())
	agents, err := s.redisClient.ZRangeByScoreWithScores(ctx, "agents:online", &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", minScore),
		Max: "+inf",
	}).Result()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]gin.H, 0, len(agents))
	for _, z := range agents {
		agentID := z.Member.(string)
		status, err := s.getAgentStatusByID(ctx, agentID)
		if err != nil {
			continue
		}
		results = append(results, status)
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  len(results),
		"agents": results,
	})
}

// listAlerts 获取告警列表
func (s *APIServer) listAlerts(c *gin.Context) {
	ctx := context.Background()
	severity := c.Query("severity")
	status := c.Query("status")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 构建查询过滤器
	filter := bson.M{}

	// 按严重级别筛选
	if severity != "" && severity != "all" {
		filter["severity"] = severity
	}

	// 按状态筛选
	if status != "" && status != "all" {
		if status == "pending" {
			filter["status"] = "open"
		} else {
			filter["status"] = status
		}
	}

	// 获取总数
	total, _ := s.db.Collection("agent_security_events").CountDocuments(ctx, filter)

	// 分页查询
	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	var alerts []AgentSecurityEvent
	cursor, err := s.db.Collection("agent_security_events").Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询告警列表失败"})
		return
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &alerts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析告警数据失败"})
		return
	}

	// 转换timestamp为标准时间格式
	type AlertResponse struct {
		ID          string    `json:"id"`
		AgentID     string    `json:"agent_id"`
		EventType   string    `json:"event_type"`
		Severity    string    `json:"severity"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Source      string    `json:"source"`
		Timestamp   time.Time `json:"timestamp"`
		ProcessName string    `json:"process_name"`
		ProcessPath string    `json:"process_path"`
		User        string    `json:"user"`
		Details     string    `json:"details"`
		Status      string    `json:"status"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	items := make([]AlertResponse, len(alerts))
	for i, alert := range alerts {
		items[i] = AlertResponse{
			ID:          ObjectIDToString(alert.ID),
			AgentID:     alert.AgentID,
			EventType:   alert.EventType,
			Severity:    alert.Severity,
			Title:       alert.Title,
			Description: alert.Description,
			Source:      alert.Source,
			Timestamp:   time.Unix(0, alert.Timestamp),
			ProcessName: alert.ProcessName,
			ProcessPath: alert.ProcessPath,
			User:        alert.User,
			Details:     alert.Details,
			Status:      alert.Status,
			CreatedAt:   alert.CreatedAt,
			UpdatedAt:   alert.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// getAlert 获取告警详情
func (s *APIServer) getAlert(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的告警ID"})
		return
	}

	var alert AgentSecurityEvent
	filter := bson.M{"_id": objectID}
	if err := s.db.Collection("agent_security_events").FindOne(ctx, filter).Decode(&alert); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "告警不存在"})
		return
	}

	// 查询主机信息
	var agent Agent
	agentFilter := bson.M{"agent_id": alert.AgentID, "deleted_at": nil}
	s.db.Collection("agents").FindOne(ctx, agentFilter).Decode(&agent)

	c.JSON(http.StatusOK, gin.H{
		"id":           ObjectIDToString(alert.ID),
		"agent_id":     alert.AgentID,
		"hostname":     agent.Hostname,
		"event_type":   alert.EventType,
		"severity":     alert.Severity,
		"title":        alert.Title,
		"description":  alert.Description,
		"source":       alert.Source,
		"timestamp":    time.Unix(0, alert.Timestamp),
		"process_name": alert.ProcessName,
		"process_path": alert.ProcessPath,
		"user":         alert.User,
		"details":      alert.Details,
		"status":       alert.Status,
		"created_at":   alert.CreatedAt,
		"updated_at":   alert.UpdatedAt,
	})
}

// updateAlertStatus 更新告警状态
func (s *APIServer) updateAlertStatus(c *gin.Context) {
	ctx := context.Background()
	id := c.Param("id")

	var req struct {
		Status  string `json:"status" binding:"required"`
		Comment string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 验证状态值
	if req.Status != "open" && req.Status != "resolved" && req.Status != "ignored" && req.Status != "investigating" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的状态值"})
		return
	}

	// 转换 ID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的告警ID"})
		return
	}

	// 检查告警是否存在
	var alert AgentSecurityEvent
	filter := bson.M{"_id": objectID}
	if err := s.db.Collection("agent_security_events").FindOne(ctx, filter).Decode(&alert); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "告警不存在"})
		return
	}

	// 更新状态
	update := bson.M{
		"$set": bson.M{
			"status":     req.Status,
			"updated_at": time.Now(),
		},
	}

	if _, err := s.db.Collection("agent_security_events").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新告警状态失败"})
		return
	}

	log.Printf("告警 %s 状态已更新为: %s", id, req.Status)
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "status": req.Status})
}
