# Artemis HIDS 认证系统

## 已实现功能

### 1. **用户认证**
- JWT Token 认证
- 登录/登出
- Token 自动过期（24小时）
- 密码 bcrypt 加密

### 2. **角色管理 (RBAC)**
- 4个预定义角色:
 - **admin**: 系统管理员（所有权限）
 - **operator**: 运维人员（查看+配置管理）
 - **analyst**: 安全分析师（只读）
 - **viewer**: 只读用户（Dashboard）
  
- 8个权限点:
 - `view:dashboard` - 查看Dashboard
 - `view:agents` - 查看Agent
 - `manage:agents` - 管理Agent
 - `view:events` - 查看事件
 - `view:alerts` - 查看告警
 - `manage:config` - 管理配置
 - `manage:users` - 管理用户
 - `manage:roles` - 管理角色

### 3. **数据库存储 (MySQL + GORM)**
- users 表
- roles 表
- user_roles 多对多关联表
- 自动迁移
- 默认数据初始化

### 4. **前端登录页面**
- 美观的登录界面
- Token 自动存储
- 过期自动登出
- 所有API请求携带认证头

## 数据库结构

```sql
CREATE TABLE `users` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,
 `created_at` datetime,
 `updated_at` datetime,
 `deleted_at` datetime,
 `username` varchar(50) UNIQUE NOT NULL,
 `password` varchar(255) NOT NULL,
 `email` varchar(100),
 `full_name` varchar(100),
 `status` varchar(20) DEFAULT 'active',
 PRIMARY KEY (`id`)
);

CREATE TABLE `roles` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,
 `created_at` datetime,
 `updated_at` datetime,
 `deleted_at` datetime,
 `name` varchar(50) UNIQUE NOT NULL,
 `display_name` varchar(100),
 `description` varchar(255),
 `permissions` text,
 PRIMARY KEY (`id`)
);

CREATE TABLE `user_roles` (
 `user_id` bigint unsigned NOT NULL,
 `role_id` bigint unsigned NOT NULL,
 PRIMARY KEY (`user_id`,`role_id`)
);
```

## API 接口

### 认证接口

```bash
# 登录
POST /api/v1/auth/login
{
 "username": "admin",
 "password": "admin123"
}

# 响应
{
 "token": "eyJhbGciOiJIUzI1NiIsInR...",
 "expires_at": 1734499200,
 "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@artemis-hids.local",
    "full_name": "系统管理员",
    "roles": ["admin"]
 }
}

# 登出
POST /api/v1/auth/logout
Authorization: Bearer <token>

# 获取当前用户
GET /api/v1/auth/me
Authorization: Bearer <token>

# 修改密码
POST /api/v1/auth/change-password
Authorization: Bearer <token>
{
 "old_password": "admin123",
 "new_password": "newpassword"
}
```

### 用户管理（需要 manage:users 权限）

```bash
# 获取用户列表
GET /api/v1/users
Authorization: Bearer <token>

# 创建用户
POST /api/v1/users
{
 "username": "operator1",
 "password": "password123",
 "email": "operator1@example.com",
 "full_name": "运维人员1"
}

# 获取用户详情
GET /api/v1/users/:id

# 更新用户
PUT /api/v1/users/:id
{
 "email": "newemail@example.com",
 "full_name": "新名字",
 "status": "active"
}

# 删除用户
DELETE /api/v1/users/:id

# 分配角色
POST /api/v1/users/:id/roles
{
 "role_ids": [1, 2]
}
```

### 角色管理（需要 manage:roles 权限）

```bash
# 获取角色列表
GET /api/v1/roles

# 创建角色
POST /api/v1/roles
{
 "name": "custom_role",
 "display_name": "自定义角色",
 "description": "描述",
 "permissions": "[\"view:dashboard\",\"view:agents\"]"
}

# 获取角色详情
GET /api/v1/roles/:id

# 更新角色
PUT /api/v1/roles/:id

# 删除角色
DELETE /api/v1/roles/:id
```

## 部署步骤

### 1. 准备 MySQL 数据库

```bash
# 创建数据库
mysql -u root -p
CREATE DATABASE artemis_hids CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. 配置文件

创建 `config.yaml`:

```yaml
database:
 host: "localhost"
 port: 3306
 user: "root"
 password: "your_password"
 database: "artemis_hids"
```

### 3. 更新 cmd/api/main.go

```go
// 初始化数据库
db, err := InitDatabase(&config.Database)
if err != nil {
    log.Fatalf("Failed to connect database: %v", err)
}

// 自动迁移
if err := AutoMigrate(db); err != nil {
    log.Fatalf("Failed to migrate database: %v", err)
}

// 初始化默认数据（角色和管理员账号）
if err := InitDefaultData(db); err != nil {
    log.Fatalf("Failed to init default data: %v", err)
}

// 创建 API 服务器（传入 db）
apiServer := NewAPIServer(redisClient, o2Client, db)
```

### 4. 启动服务

```bash
cd /Users/njcx/Artemis_HIDS/manager
go run cmd/api/main.go config.go api_server.go agent_config.go auth.go models.go database.go openobserve.go
```

### 5. 访问登录页面

```bash
open http://localhost:8080
```

## 默认账号

```
用户名: admin
密码: admin123
角色: 系统管理员（所有权限）
```

** 首次登录后请立即修改密码！**

## 权限控制示例

```go
// 只允许有 view:dashboard 权限的用户访问
api.GET("/stats", RequirePermission(PermViewDashboard), s.getStats)

// 只允许有 manage:config 权限的用户访问
config.POST("/agents/:id/workers/:worker/stop", 
    RequirePermission(PermManageConfig), s.stopWorker)
```

## 安全建议

1. **JWT 密钥**: 修改 `auth.go` 中的 `jwtSecret`，使用强随机密钥
2. **密码策略**: 增加密码复杂度验证
3. **Token 刷新**: 实现 Token 刷新机制
4. **审计日志**: 记录所有认证和授权操作
5. **HTTPS**: 生产环境必须使用 HTTPS
6. **限流**: 添加登录失败次数限制

## 权限矩阵

| 角色 | Dashboard | Agent查看 | Agent管理 | 事件查看 | 告警查看 | 配置管理 | 用户管理 | 角色管理 |
|------|----------|----------|----------|---------|---------|---------|---------|---------|
| admin | | | | | | | | |
| operator | | | | | | | | |
| analyst | | | | | | | | |
| viewer | | | | | | | | |

## 前端集成

前端已自动集成认证：
1. 登录页面自动跳转
2. Token 存储在 localStorage
3. 所有API请求自动携带 Authorization 头
4. Token 过期自动登出
5. 显示当前用户名

## 测试

```bash
# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
 -H "Content-Type: application/json" \
 -d '{"username":"admin","password":"admin123"}'

# 使用 Token 访问 API
curl http://localhost:8080/api/v1/stats \
 -H "Authorization: Bearer <your_token>"

# 创建新用户
curl -X POST http://localhost:8080/api/v1/users \
 -H "Authorization: Bearer <admin_token>" \
 -H "Content-Type: application/json" \
 -d '{
    "username": "analyst1",
    "password": "password123",
    "full_name": "分析师1"
 }'
```

## 下一步扩展

- [ ] OAuth2/OIDC 集成
- [ ] LDAP/AD 集成
- [ ] 双因素认证 (2FA)
- [ ] 登录日志和审计
- [ ] IP 白名单
- [ ] Session 管理
