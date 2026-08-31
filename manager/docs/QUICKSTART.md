# MongoDB 迁移 - 快速启动指南

## 前置条件

- Docker 和 Docker Compose 已安装
- Go 1.19+ 已安装
- 端口 27017 (MongoDB) 和 8081 (Mongo Express) 未被占用

---

## 快速启动（5分钟）

### 步骤 1: 启动 MongoDB

```bash
cd manager
chmod +x start_mongodb.sh
./start_mongodb.sh
```

**预期输出:**
```
 MongoDB 容器已启动
 Mongo Express 已启动
 数据库初始化完成
```

### 步骤 2: 安装 Go 依赖

```bash
cd manager
go mod tidy
```

**预期输出:**
```
go: downloading go.mongodb.org/mongo-driver v1.13.1
...
```

### 步骤 3: 运行迁移测试

```bash
cd manager
chmod +x test_migration.sh
./test_migration.sh
```

**预期输出:**
```
测试: Docker 服务状态 ... 通过
测试: MongoDB 容器状态 ... 通过
测试: MongoDB 连接测试 ... 通过
...
 所有测试通过！MongoDB 迁移成功！
```

### 步骤 4: 启动 API Server

```bash
cd manager
go run cmd/api/main.go
```

**预期输出:**
```
2026/01/30 15:00:00 连接到 MongoDB 成功
2026/01/30 15:00:00 数据库索引创建完成
2026/01/30 15:00:00 默认数据初始化完成
2026/01/30 15:00:00 API Server 启动在 :8080
```

### 步骤 5: 验证服务

```bash
# 健康检查
curl http://localhost:8080/health

# 预期响应
{"status":"ok"}
```

---

## 访问服务

### API Server
- **地址**: http://localhost:8080
- **健康检查**: http://localhost:8080/health
- **API 文档**: 参考 API_SERVER_MIGRATION_GUIDE.md

### Mongo Express (数据库管理界面)
- **地址**: http://localhost:8081
- **用户名**: admin
- **密码**: pass123

---

## 测试 API 端点

### 1. 获取验证码
```bash
curl -X POST http://localhost:8080/api/v1/auth/captcha
```

### 2. 用户登录
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
 -H "Content-Type: application/json" \
 -d '{
    "username": "admin",
    "password": "admin123",
    "captcha_id": "your-captcha-id",
    "captcha_code": "your-captcha-code"
 }'
```

### 3. 获取仪表盘统计（需要 Token）
```bash
curl -X GET http://localhost:8080/api/v1/stats/dashboard \
 -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 4. 获取 Agent 列表（需要 Token）
```bash
curl -X GET http://localhost:8080/api/v1/agents \
 -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

---

## 数据库管理

### 查看集合
```bash
docker exec -it artemis-mongodb mongosh artemis_hids --eval "db.getCollectionNames()"
```

### 查看用户数据
```bash
docker exec -it artemis-mongodb mongosh artemis_hids --eval "db.users.find().pretty()"
```

### 查看 Agent 数据
```bash
docker exec -it artemis-mongodb mongosh artemis_hids --eval "db.agents.find().pretty()"
```

### 查看索引
```bash
docker exec -it artemis-mongodb mongosh artemis_hids --eval "db.users.getIndexes()"
```

---

## 常见问题

### Q1: MongoDB 启动失败
```bash
# 检查端口占用
lsof -i :27017

# 停止旧容器
docker stop artemis-mongodb artemis-mongo-express
docker rm artemis-mongodb artemis-mongo-express

# 重新启动
./start_mongodb.sh
```

### Q2: API Server 连接失败
```bash
# 检查配置文件
cat config.yaml | grep mongodb

# 确保配置正确
database:
 mongodb:
    uri: "mongodb://admin:password123@localhost:27017"
    database: "artemis_hids"
```

### Q3: 编译错误
```bash
# 清理缓存
go clean -cache -modcache

# 重新下载依赖
go mod tidy
go mod download

# 重新编译
go build cmd/api/main.go
```

### Q4: 权限错误
```bash
# 给脚本添加执行权限
chmod +x start_mongodb.sh
chmod +x test_migration.sh
```

---

## 详细文档

- **[MONGODB_MIGRATION_COMPLETE.md](MONGODB_MIGRATION_COMPLETE.md)** - 完整迁移总结
- **[MONGODB_MIGRATION.md](MONGODB_MIGRATION.md)** - 详细迁移指南
- **[API_SERVER_MIGRATION_GUIDE.md](API_SERVER_MIGRATION_GUIDE.md)** - API 迁移示例
- **[MONGODB_README.md](MONGODB_README.md)** - 项目说明

---

## 成功标志

如果看到以下内容，说明迁移成功：

1. MongoDB 容器正常运行
2. 数据库和集合已创建
3. 索引已创建
4. 默认管理员账号已创建（admin/admin123）
5. API Server 正常启动
6. 健康检查返回 `{"status":"ok"}`
7. 可以正常登录和访问 API

---

## 回滚方案

如果需要回滚到 MySQL：

```bash
# 1. 停止 MongoDB
docker-compose -f docker-compose-mongodb.yaml down

# 2. 恢复 MySQL 配置
git checkout go.mod
git checkout config.yaml
git checkout internal/server/

# 3. 启动 MySQL
# (根据原有的 MySQL 启动方式)
```

---

## 技术支持

如遇到问题，请检查：
1. Docker 日志: `docker logs artemis-mongodb`
2. API Server 日志: 查看终端输出
3. MongoDB 连接: 使用 Mongo Express 验证

---

**最后更新**: 2026-01-30 
**版本**: v1.0 
**状态**: 迁移完成，待测试验证
