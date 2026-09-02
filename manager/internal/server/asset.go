// ============================================
// 资产识别 API —— 跨主机聚合查询端口/软件/容器/进程
// ============================================
package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// setupAssetRoutes 设置资产查询路由
func (s *APIServer) setupAssetRoutes() {
	assets := s.router.Group("/api/v1/assets")
	assets.Use(AuthMiddleware(s.db))
	{
		assets.GET("/ports", s.listAssetPorts)
		assets.GET("/software", s.listAssetSoftware)
		assets.GET("/containers", s.listAssetContainers)
		assets.GET("/processes", s.listAssetProcesses)
	}
}

// parsePagination 从 query 参数解析分页信息，默认第1页每页20条
func parsePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ = strconv.Atoi(c.Query("page_size"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	return page, pageSize
}

// listAssetPorts 查询端口资产列表
func (s *APIServer) listAssetPorts(c *gin.Context) {
	ctx := c.Request.Context()

	filter := bson.M{}
	if agentID := c.Query("agent_id"); agentID != "" {
		filter["agent_id"] = agentID
	}
	if search := c.Query("search"); search != "" {
		filter["$or"] = []bson.M{
			{"process_name": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	page, pageSize := parsePagination(c)
	total, err := s.db.Collection("agent_ports").CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cursor, err := s.db.Collection("agent_ports").Find(ctx, filter,
		options.Find().SetSkip(int64((page-1)*pageSize)).SetLimit(int64(pageSize)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var ports []AgentPort
	if err := cursor.All(ctx, &ports); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"total": total, "items": ports})
}

// listAssetSoftware 查询软件资产列表
func (s *APIServer) listAssetSoftware(c *gin.Context) {
	ctx := c.Request.Context()

	filter := bson.M{}
	if agentID := c.Query("agent_id"); agentID != "" {
		filter["agent_id"] = agentID
	}
	if search := c.Query("search"); search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	page, pageSize := parsePagination(c)
	total, err := s.db.Collection("agent_software").CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cursor, err := s.db.Collection("agent_software").Find(ctx, filter,
		options.Find().SetSkip(int64((page-1)*pageSize)).SetLimit(int64(pageSize)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var software []AgentSoftware
	if err := cursor.All(ctx, &software); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"total": total, "items": software})
}

// listAssetContainers 查询容器资产列表
func (s *APIServer) listAssetContainers(c *gin.Context) {
	ctx := c.Request.Context()

	filter := bson.M{}
	if agentID := c.Query("agent_id"); agentID != "" {
		filter["agent_id"] = agentID
	}
	if search := c.Query("search"); search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": search, "$options": "i"}},
			{"image": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	page, pageSize := parsePagination(c)
	total, err := s.db.Collection("agent_containers").CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cursor, err := s.db.Collection("agent_containers").Find(ctx, filter,
		options.Find().SetSkip(int64((page-1)*pageSize)).SetLimit(int64(pageSize)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var containers []AgentContainer
	if err := cursor.All(ctx, &containers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"total": total, "items": containers})
}

// listAssetProcesses 查询进程资产列表
func (s *APIServer) listAssetProcesses(c *gin.Context) {
	ctx := c.Request.Context()

	filter := bson.M{}
	if agentID := c.Query("agent_id"); agentID != "" {
		filter["agent_id"] = agentID
	}
	if search := c.Query("search"); search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	page, pageSize := parsePagination(c)
	total, err := s.db.Collection("agent_processes").CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cursor, err := s.db.Collection("agent_processes").Find(ctx, filter,
		options.Find().SetSkip(int64((page-1)*pageSize)).SetLimit(int64(pageSize)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var processes []AgentProcess
	if err := cursor.All(ctx, &processes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"total": total, "items": processes})
}
