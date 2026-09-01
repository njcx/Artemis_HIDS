package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

// setupRuleRoutes 规则管理路由
func (s *APIServer) setupRuleRoutes() {
	rules := s.router.Group("/api/v1/rules")
	rules.Use(AuthMiddleware(s.db))
	{
		rules.GET("", RequirePermission(PermViewRules), s.listRules)
		rules.GET("/:id", RequirePermission(PermViewRules), s.getRule)
		rules.POST("", RequirePermission(PermManageRules), s.createRule)
		rules.PUT("/:id", RequirePermission(PermManageRules), s.updateRule)
		rules.DELETE("/:id", RequirePermission(PermManageRules), s.deleteRule)
	}
}

// listRules 获取规则列表
func (s *APIServer) listRules(c *gin.Context) {
	ctx := context.Background()

	filter := bson.M{}
	if name := c.Query("name"); name != "" {
		filter["name"] = bson.M{"$regex": name, "$options": "i"}
	}
	if severity := c.Query("severity"); severity != "" {
		filter["severity"] = severity
	}
	if ruleType := c.Query("type"); ruleType != "" {
		filter["type"] = ruleType
	}
	if enabled := c.Query("enabled"); enabled != "" {
		filter["enabled"] = enabled == "true"
	}

	cursor, err := s.db.Collection("rules").Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var rules []Rule
	if err := cursor.All(ctx, &rules); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// getRule 获取规则详情
func (s *APIServer) getRule(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	var rule Rule
	if err := s.db.Collection("rules").FindOne(ctx, bson.M{"id": id}).Decode(&rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// createRule 创建规则
func (s *APIServer) createRule(c *gin.Context) {
	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if rule.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	ctx := context.Background()

	count, err := s.db.Collection("rules").CountDocuments(ctx, bson.M{"id": rule.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "rule id already exists"})
		return
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	if _, err := s.db.Collection("rules").InsertOne(ctx, rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create rule"})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// updateRule 更新规则
func (s *APIServer) updateRule(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	filter := bson.M{"id": id}

	var existing Rule
	if err := s.db.Collection("rules").FindOne(ctx, filter).Decode(&existing); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	update := bson.M{"$set": rule}
	if _, err := s.db.Collection("rules").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update rule"})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// deleteRule 删除规则（硬删除）
func (s *APIServer) deleteRule(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	result, err := s.db.Collection("rules").DeleteOne(ctx, bson.M{"id": id})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete rule"})
		return
	}
	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "rule deleted successfully"})
}
