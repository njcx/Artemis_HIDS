# API Server MongoDB 迁移示例

本文档展示如何将 `api_server.go` 中的关键函数从 GORM 迁移到 MongoDB。

## 已修改的部分

### 1. 导入和结构体定义

```go
// 旧的导入
import (
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

// 新的导入
import (
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// APIServer 结构体
type APIServer struct {
    redisClient *redis.Client
    db          *mongo.Database  // 改为 *mongo.Database
    router      *gin.Engine
}
```

## 需要迁移的函数

### 1. getDashboardStats() - 获取仪表盘统计

**GORM 版本:**
```go
func (s *APIServer) getDashboardStats(c *gin.Context) {
    var stats DashboardStats
    if err := s.db.Order("updated_at DESC").First(&stats).Error; err != nil {
        stats = s.calculateDashboardStats()
        s.db.Create(&stats)
    }
    // ...
}
```

**MongoDB 版本:**
```go
func (s *APIServer) getDashboardStats(c *gin.Context) {
    ctx := context.Background()
    var stats DashboardStats
    
    // 查询最新的统计数据
    opts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
    err := s.db.Collection("dashboard_stats").FindOne(ctx, bson.M{}, opts).Decode(&stats)
    
    if err != nil {
        // 如果没有数据，实时计算
        stats = s.calculateDashboardStats()
        stats.CreatedAt = time.Now()
        stats.UpdatedAt = time.Now()
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
```

### 2. calculateDashboardStats() - 计算统计数据

**GORM 版本:**
```go
func (s *APIServer) calculateDashboardStats() DashboardStats {
    var stats DashboardStats
    s.db.Model(&Agent{}).Count(&stats.TotalAgents)
    fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
    s.db.Model(&Agent{}).Where("last_heartbeat > ?", fiveMinutesAgo).Count(&stats.OnlineAgents)
    // ...
}
```

**MongoDB 版本:**
```go
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
    
    // 统计今日安全事件数
    startOfDay := time.Now().Truncate(24 * time.Hour)
    todayFilter := bson.M{
        "created_at": bson.M{"$gte": startOfDay},
    }
    stats.TodayAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, todayFilter)
    
    // 统计待处理告警
    pendingFilter := bson.M{"status": "open"}
    stats.PendingAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, pendingFilter)
    
    // 统计告警总数
    stats.TotalAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{})
    
    // 统计不同严重程度的告警
    stats.CriticalAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "critical"})
    stats.HighAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "high"})
    stats.MediumAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "medium"})
    stats.LowAlerts, _ = s.db.Collection("agent_security_events").CountDocuments(ctx, bson.M{"severity": "low"})
    
    stats.UpdatedAt = time.Now()
    return stats
}
```

### 3. upsertDashboardStats() - 插入或更新统计数据

**GORM 版本:**
```go
func (s *APIServer) upsertDashboardStats() {
    stats := s.calculateDashboardStats()
    now := time.Now()
    today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
    stats.StatDate = today
    
    result := s.db.Clauses(clause.OnConflict{
        Columns: []clause.Column{{Name: "stat_date"}},
        DoUpdates: clause.AssignmentColumns([]string{...}),
    }).Create(&stats)
}
```

**MongoDB 版本:**
```go
func (s *APIServer) upsertDashboardStats() {
    ctx := context.Background()
    stats := s.calculateDashboardStats()
    
    // 获取当天日期
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
        log.Printf("仪表盘统计数据已更新: 日期=%s, 总主机=%d, 在线=%d",
            today.Format("2006-01-02"), stats.TotalAgents, stats.OnlineAgents)
    }
}
```

### 4. listAgents() - 列出 Agents

**GORM 版本:**
```go
func (s *APIServer) listAgents(c *gin.Context) {
    var agents []Agent
    if err := s.db.Find(&agents).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    // ...
}
```

**MongoDB 版本:**
```go
func (s *APIServer) listAgents(c *gin.Context) {
    ctx := context.Background()
    var agents []Agent
    
    // 查询所有 agent（排除已删除）
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
        // 判断是否在线
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
```

### 5. getAgent() - 获取 Agent 详情（包含关联数据）

**GORM 版本:**
```go
func (s *APIServer) getAgent(c *gin.Context) {
    agentID := c.Param("id")
    var agent Agent
    
    if err := s.db.Preload("Processes").
        Preload("Ports").
        Preload("Software").
        // ...
        Where("agent_id = ?", agentID).First(&agent).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
        return
    }
    // ...
}
```

**MongoDB 版本:**
```go
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
    
    // 判断是否在线
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
    })
}
```

### 6. listAlerts() - 获取告警列表（分页、筛选、排序）

**GORM 版本:**
```go
func (s *APIServer) listAlerts(c *gin.Context) {
    // ...
    query := s.db.Model(&AgentSecurityEvent{})
    if severity != "" && severity != "all" {
        query = query.Where("severity = ?", severity)
    }
    query.Count(&total)
    offset := (page - 1) * pageSize
    query.Order("timestamp DESC").Offset(offset).Limit(pageSize).Find(&alerts)
}
```

**MongoDB 版本:**
```go
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
    
    // 转换响应格式
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
```

### 7. updateAlertStatus() - 更新告警状态

**GORM 版本:**
```go
func (s *APIServer) updateAlertStatus(c *gin.Context) {
    id := c.Param("id")
    var alert AgentSecurityEvent
    if err := s.db.First(&alert, id).Error; err != nil {
        // ...
    }
    alert.Status = req.Status
    s.db.Save(&alert)
}
```

**MongoDB 版本:**
```go
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
```

## 使用 MongoHelper 简化代码

可以使用 `mongo_helper.go` 中的辅助函数简化上述代码：

```go
// 创建 helper
helper := NewMongoHelper(s.db)

// 查询列表
var agents []Agent
filter := BuildFilter(map[string]interface{}{})
helper.Find("agents", filter, &agents)

// 分页查询
var alerts []AgentSecurityEvent
total, err := helper.Paginate("agent_security_events", filter, page, pageSize, &alerts)

// 统计
count, err := helper.Count("agents", filter)

// 更新
updates := BuildUpdate(map[string]interface{}{"status": "resolved"})
helper.UpdateByID("agent_security_events", alertID, updates)
```

## 完整迁移步骤

1. 修改所有函数的数据库操作
2. 处理 ID 类型转换（interface{} ObjectID string）
3. 处理关联查询（多次查询代替 Preload）
4. 添加适当的错误处理
5. 测试每个 API 端点

## 注意事项

1. **ID 类型**: MongoDB 使用 ObjectID，需要转换
2. **关联查询**: 需要多次查询，不能使用 Preload
3. **软删除**: 所有查询都要添加 `deleted_at: nil` 条件
4. **索引**: 确保为常用查询字段创建索引
5. **错误处理**: MongoDB 的错误类型与 GORM 不同

## 测试建议

每迁移一个函数后立即测试：

```bash
# 测试 Agent 列表
curl -H "Authorization: Bearer <token>" \
 http://localhost:8080/api/v1/agents

# 测试告警列表
curl -H "Authorization: Bearer <token>" \
 "http://localhost:8080/api/v1/alerts?page=1&page_size=20&severity=high"
```
