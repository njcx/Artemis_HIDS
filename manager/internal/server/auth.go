package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecret = []byte("artemis-hids-secret-key-868732")
)

// Claims JWT Claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Captcha   string `json:"captcha" binding:"required"`
	CaptchaID string `json:"captcha_id" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string   `json:"token"`
	ExpiresAt int64    `json:"expires_at"`
	User      UserInfo `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	Roles    []string `json:"roles"`
}

// HashPassword 密码哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// GenerateToken 生成 JWT Token
func GenerateToken(user *User, roles []string) (string, int64, error) {
	expiresAt := time.Now().Add(24 * time.Hour)

	userID := ""
	if oid, ok := user.ID.(primitive.ObjectID); ok {
		userID = oid.Hex()
	} else if str, ok := user.ID.(string); ok {
		userID = str
	}

	claims := Claims{
		UserID:   userID,
		Username: user.Username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "artemis-hids",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", 0, err
	}

	return tokenString, expiresAt.Unix(), nil
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// 验证用户是否存在且状态正常
		ctx := context.Background()
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
			c.Abort()
			return
		}

		var user User
		filter := bson.M{"_id": userID, "status": "active", "deleted_at": nil}
		if err := db.Collection("users").FindOne(ctx, filter).Decode(&user); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found or disabled"})
			c.Abort()
			return
		}

		// 加载用户角色
		if len(user.RoleIDs) > 0 {
			cursor, err := db.Collection("roles").Find(ctx, bson.M{"_id": bson.M{"$in": user.RoleIDs}})
			if err == nil {
				cursor.All(ctx, &user.Roles)
			}
		}

		// 将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)
		c.Set("user", &user)
		c.Next()
	}
}

// RequirePermission 权限检查中间件
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInterface, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "user not found in context"})
			c.Abort()
			return
		}

		user, ok := userInterface.(*User)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid user type"})
			c.Abort()
			return
		}

		// 检查用户是否有该权限
		if !hasPermission(user, permission) {
			// 调试日志: 打印当前用户的角色和权限，方便排查 403
			userRoles := make([]string, 0)
			userPermissions := make([]string, 0)
			for _, role := range user.Roles {
				userRoles = append(userRoles, role.Name)
				userPermissions = append(userPermissions, role.Permissions)
			}
			log.Printf("[403 Debug] User: %s, Roles: %v, Permissions: %v, Required: %s",
				user.Username, userRoles, userPermissions, permission)

			// 临时允许通过，以便前端调试，生产环境需移除
			// c.Next()
			// return

			c.JSON(http.StatusForbidden, gin.H{
				"error":    "insufficient permissions",
				"required": permission,
				"debug": gin.H{
					"user":        user.Username,
					"roles":       userRoles,
					"permissions": userPermissions,
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// hasPermission 检查用户是否有指定权限
func hasPermission(user *User, permission string) bool {
	for _, role := range user.Roles {
		var permissions []string
		if err := json.Unmarshal([]byte(role.Permissions), &permissions); err == nil {
			for _, p := range permissions {
				if p == permission {
					return true
				}
			}
		}
	}
	return false
}

// setupAuthRoutes 设置认证相关路由
func (s *APIServer) setupAuthRoutes() {
	auth := s.router.Group("/api/v1/auth")
	{
		auth.GET("/captcha", s.getCaptcha)
		auth.POST("/login", s.login)
		auth.POST("/logout", s.logout)
		auth.GET("/me", AuthMiddleware(s.db), s.getCurrentUser)
		auth.POST("/change-password", AuthMiddleware(s.db), s.changePassword)
	}

	// 用户管理路由 (需要管理员权限)
	users := s.router.Group("/api/v1/users")
	users.Use(AuthMiddleware(s.db))
	users.Use(RequirePermission(PermManageUsers))
	{
		users.GET("", s.listUsers)
		users.POST("", s.createUser)
		users.GET("/:id", s.getUser)
		users.PUT("/:id", s.updateUser)
		users.DELETE("/:id", s.deleteUser)
		users.POST("/:id/roles", s.assignRoles)
		users.POST("/:id/reset-password", s.resetUserPassword)
	}

	// 角色管理路由
	roles := s.router.Group("/api/v1/roles")
	roles.Use(AuthMiddleware(s.db))
	roles.Use(RequirePermission(PermManageRoles))
	{
		roles.GET("", s.listRoles)
		roles.POST("", s.createRole)
		roles.GET("/:id", s.getRole)
		roles.PUT("/:id", s.updateRole)
		roles.DELETE("/:id", s.deleteRole)
	}
}

// login 登录
func (s *APIServer) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证验证码
	if !captchaStore.VerifyCaptcha(req.CaptchaID, req.Captcha) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid captcha", "message": "验证码错误或已过期"})
		return
	}

	ctx := context.Background()

	// 查找用户
	var user User
	filter := bson.M{"username": req.Username, "deleted_at": nil}
	if err := s.db.Collection("users").FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	// 验证密码
	if err := CheckPassword(user.Password, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	// 检查用户状态
	if user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "user account is disabled"})
		return
	}

	// 加载用户角色
	if len(user.RoleIDs) > 0 {
		cursor, err := s.db.Collection("roles").Find(ctx, bson.M{"_id": bson.M{"$in": user.RoleIDs}})
		if err == nil {
			cursor.All(ctx, &user.Roles)
		}
	}

	// 获取角色列表
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	// 生成 Token
	token, expiresAt, err := GenerateToken(&user, roleNames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// 更新最后登录时间
	now := time.Now()
	updateFilter := bson.M{"_id": user.ID}
	updateData := bson.M{"$set": bson.M{"last_login": now}}
	s.db.Collection("users").UpdateOne(ctx, updateFilter, updateData)

	userIDStr := ""
	if oid, ok := user.ID.(primitive.ObjectID); ok {
		userIDStr = oid.Hex()
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:       userIDStr,
			Username: user.Username,
			Email:    user.Email,
			FullName: user.FullName,
			Roles:    roleNames,
		},
	})
}

// logout 登出
func (s *APIServer) logout(c *gin.Context) {
	// JWT 是无状态的，客户端删除 token 即可
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// getCurrentUser 获取当前用户信息
func (s *APIServer) getCurrentUser(c *gin.Context) {
	userInterface, _ := c.Get("user")
	user := userInterface.(*User)

	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	userIDStr := ""
	if oid, ok := user.ID.(primitive.ObjectID); ok {
		userIDStr = oid.Hex()
	}

	c.JSON(http.StatusOK, UserInfo{
		ID:       userIDStr,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.FullName,
		Roles:    roleNames,
	})
}

// changePassword 修改密码
func (s *APIServer) changePassword(c *gin.Context) {
	userInterface, _ := c.Get("user")
	user := userInterface.(*User)

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证旧密码
	if err := CheckPassword(user.Password, req.OldPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "incorrect old password"})
		return
	}

	// 哈希新密码
	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// 更新密码
	ctx := context.Background()
	filter := bson.M{"_id": user.ID}
	update := bson.M{"$set": bson.M{"password": hashedPassword, "updated_at": time.Now()}}
	if _, err := s.db.Collection("users").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

// listUsers 获取用户列表
func (s *APIServer) listUsers(c *gin.Context) {
	ctx := context.Background()

	// 查询所有用户（排除已删除）
	filter := bson.M{"deleted_at": nil}
	cursor, err := s.db.Collection("users").Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var users []User
	if err := cursor.All(ctx, &users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 加载每个用户的角色
	for i := range users {
		if len(users[i].RoleIDs) > 0 {
			roleCursor, err := s.db.Collection("roles").Find(ctx, bson.M{"_id": bson.M{"$in": users[i].RoleIDs}})
			if err == nil {
				roleCursor.All(ctx, &users[i].Roles)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// createUser 创建用户
func (s *APIServer) createUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, _ := HashPassword(req.Password)

	user := User{
		Username:  req.Username,
		Password:  hashedPassword,
		Email:     req.Email,
		FullName:  req.FullName,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	result, err := s.db.Collection("users").InsertOne(ctx, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	user.ID = result.InsertedID
	c.JSON(http.StatusCreated, user)
}

// getUser 获取用户详情
func (s *APIServer) getUser(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var user User
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	if err := s.db.Collection("users").FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 加载角色
	if len(user.RoleIDs) > 0 {
		cursor, err := s.db.Collection("roles").Find(ctx, bson.M{"_id": bson.M{"$in": user.RoleIDs}})
		if err == nil {
			cursor.All(ctx, &user.Roles)
		}
	}

	c.JSON(http.StatusOK, user)
}

// updateUser 更新用户
func (s *APIServer) updateUser(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var user User
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	if err := s.db.Collection("users").FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Status   string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := bson.M{"updated_at": time.Now()}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.FullName != "" {
		updates["full_name"] = req.FullName
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	update := bson.M{"$set": updates}
	if _, err := s.db.Collection("users").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	// 重新查询更新后的用户
	s.db.Collection("users").FindOne(ctx, filter).Decode(&user)
	c.JSON(http.StatusOK, user)
}

// deleteUser 删除用户（软删除）
func (s *APIServer) deleteUser(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// 软删除
	filter := bson.M{"_id": objectID}
	now := time.Now()
	update := bson.M{"$set": bson.M{"deleted_at": &now, "updated_at": now}}
	if _, err := s.db.Collection("users").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

// assignRoles 分配角色
func (s *APIServer) assignRoles(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var user User
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	if err := s.db.Collection("users").FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		RoleIDs []string `json:"role_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 转换角色ID
	roleObjectIDs := make([]interface{}, len(req.RoleIDs))
	for i, roleID := range req.RoleIDs {
		oid, err := primitive.ObjectIDFromHex(roleID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role id"})
			return
		}
		roleObjectIDs[i] = oid
	}

	// 验证角色是否存在
	count, err := s.db.Collection("roles").CountDocuments(ctx, bson.M{"_id": bson.M{"$in": roleObjectIDs}})
	if err != nil || count != int64(len(roleObjectIDs)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "some roles not found"})
		return
	}

	// 更新用户的角色ID列表
	update := bson.M{"$set": bson.M{"role_ids": roleObjectIDs, "updated_at": time.Now()}}
	if _, err := s.db.Collection("users").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign roles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "roles assigned successfully"})
}

// listRoles 获取角色列表
func (s *APIServer) listRoles(c *gin.Context) {
	ctx := context.Background()

	filter := bson.M{"deleted_at": nil}
	cursor, err := s.db.Collection("roles").Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var roles []Role
	if err := cursor.All(ctx, &roles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

// createRole 创建角色
func (s *APIServer) createRole(c *gin.Context) {
	var role Role

	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()

	ctx := context.Background()
	result, err := s.db.Collection("roles").InsertOne(ctx, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create role"})
		return
	}

	role.ID = result.InsertedID
	c.JSON(http.StatusCreated, role)
}

// getRole 获取角色详情
func (s *APIServer) getRole(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role id"})
		return
	}

	var role Role
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	if err := s.db.Collection("roles").FindOne(ctx, filter).Decode(&role); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
		return
	}

	c.JSON(http.StatusOK, role)
}

// updateRole 更新角色
func (s *APIServer) updateRole(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role id"})
		return
	}

	var role Role
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	if err := s.db.Collection("roles").FindOne(ctx, filter).Decode(&role); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
		return
	}

	var req Role
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := bson.M{
		"name":         req.Name,
		"display_name": req.DisplayName,
		"description":  req.Description,
		"permissions":  req.Permissions,
		"updated_at":   time.Now(),
	}

	update := bson.M{"$set": updates}
	if _, err := s.db.Collection("roles").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		return
	}

	// 重新查询更新后的角色
	s.db.Collection("roles").FindOne(ctx, filter).Decode(&role)
	c.JSON(http.StatusOK, role)
}

// deleteRole 删除角色（软删除）
func (s *APIServer) deleteRole(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role id"})
		return
	}

	// 软删除
	filter := bson.M{"_id": objectID}
	now := time.Now()
	update := bson.M{"$set": bson.M{"deleted_at": &now, "updated_at": now}}
	if _, err := s.db.Collection("roles").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "role deleted"})
}

// resetUserPassword 管理员重置用户密码
func (s *APIServer) resetUserPassword(c *gin.Context) {
	id := c.Param("id")
	ctx := context.Background()

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var user User
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	if err := s.db.Collection("users").FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 哈希新密码
	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// 更新密码
	update := bson.M{"$set": bson.M{"password": hashedPassword, "updated_at": time.Now()}}
	if _, err := s.db.Collection("users").UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset password"})
		return
	}

	log.Printf("用户 %s 的密码已被管理员重置", user.Username)
	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}
