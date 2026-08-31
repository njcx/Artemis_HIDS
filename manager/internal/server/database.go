package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// InitDatabase 初始化 MongoDB 连接
func InitDatabase(config *DatabaseConfig) (*mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 构建 MongoDB 连接字符串
	var uri string
	if config.User != "" && config.Password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin",
			config.User,
			config.Password,
			config.Host,
			config.Port,
			config.Database,
		)
	} else {
		uri = fmt.Sprintf("mongodb://%s:%d/%s",
			config.Host,
			config.Port,
			config.Database,
		)
	}

	// 创建客户端选项
	clientOptions := options.Client().ApplyURI(uri)
	clientOptions.SetMaxPoolSize(100)
	clientOptions.SetMinPoolSize(10)
	clientOptions.SetMaxConnIdleTime(time.Hour)

	// 连接到 MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// 验证连接
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	log.Printf("成功连接到 MongoDB: %s:%d/%s", config.Host, config.Port, config.Database)

	db := client.Database(config.Database)
	return db, nil
}

// CreateIndexes 创建索引
func CreateIndexes(db *mongo.Database) error {
	ctx := context.Background()

	// users 集合索引
	usersIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
	}
	if _, err := db.Collection("users").Indexes().CreateMany(ctx, usersIndexes); err != nil {
		return fmt.Errorf("failed to create users indexes: %v", err)
	}

	// roles 集合索引
	rolesIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	if _, err := db.Collection("roles").Indexes().CreateMany(ctx, rolesIndexes); err != nil {
		return fmt.Errorf("failed to create roles indexes: %v", err)
	}

	// agents 集合索引
	agentsIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agent_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "online", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "last_heartbeat", Value: 1}},
		},
	}
	if _, err := db.Collection("agents").Indexes().CreateMany(ctx, agentsIndexes); err != nil {
		return fmt.Errorf("failed to create agents indexes: %v", err)
	}

	// agent_processes 集合索引
	processesIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agent_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "pid", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "name", Value: 1}},
		},
	}
	if _, err := db.Collection("agent_processes").Indexes().CreateMany(ctx, processesIndexes); err != nil {
		return fmt.Errorf("failed to create agent_processes indexes: %v", err)
	}

	// agent_ports 集合索引
	portsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agent_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "port", Value: 1}},
		},
	}
	if _, err := db.Collection("agent_ports").Indexes().CreateMany(ctx, portsIndexes); err != nil {
		return fmt.Errorf("failed to create agent_ports indexes: %v", err)
	}

	// agent_software 集合索引
	softwareIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agent_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "name", Value: 1}},
		},
	}
	if _, err := db.Collection("agent_software").Indexes().CreateMany(ctx, softwareIndexes); err != nil {
		return fmt.Errorf("failed to create agent_software indexes: %v", err)
	}

	// agent_services 集合索引
	servicesIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agent_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "name", Value: 1}},
		},
	}
	if _, err := db.Collection("agent_services").Indexes().CreateMany(ctx, servicesIndexes); err != nil {
		return fmt.Errorf("failed to create agent_services indexes: %v", err)
	}

	// agent_containers 集合索引
	containersIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agent_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "container_id", Value: 1}},
		},
	}
	if _, err := db.Collection("agent_containers").Indexes().CreateMany(ctx, containersIndexes); err != nil {
		return fmt.Errorf("failed to create agent_containers indexes: %v", err)
	}

	// agent_connections 集合索引
	connectionsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agent_id", Value: 1}},
		},
	}
	if _, err := db.Collection("agent_connections").Indexes().CreateMany(ctx, connectionsIndexes); err != nil {
		return fmt.Errorf("failed to create agent_connections indexes: %v", err)
	}

	// agent_security_events 集合索引
	eventsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agent_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "severity", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "timestamp", Value: -1}},
		},
	}
	if _, err := db.Collection("agent_security_events").Indexes().CreateMany(ctx, eventsIndexes); err != nil {
		return fmt.Errorf("failed to create agent_security_events indexes: %v", err)
	}

	// dashboard_stats 集合索引
	statsIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "stat_date", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "updated_at", Value: -1}},
		},
	}
	if _, err := db.Collection("dashboard_stats").Indexes().CreateMany(ctx, statsIndexes); err != nil {
		return fmt.Errorf("failed to create dashboard_stats indexes: %v", err)
	}

	log.Println("MongoDB 索引创建完成")
	return nil
}

// InitDefaultData 初始化默认数据
func InitDefaultData(db *mongo.Database) error {
	ctx := context.Background()

	// 创建默认角色
	rolesCollection := db.Collection("roles")
	for _, role := range DefaultRoles {
		filter := bson.M{"name": role.Name}
		var existingRole Role
		err := rolesCollection.FindOne(ctx, filter).Decode(&existingRole)

		if err == mongo.ErrNoDocuments {
			// 创建新角色
			role.CreatedAt = time.Now()
			role.UpdatedAt = time.Now()
			if _, err := rolesCollection.InsertOne(ctx, role); err != nil {
				return fmt.Errorf("failed to create role %s: %v", role.Name, err)
			}
			log.Printf("创建默认角色: %s", role.Name)
		} else if err == nil {
			// 更新现有角色的权限
			update := bson.M{
				"$set": bson.M{
					"permissions": role.Permissions,
					"updated_at":  time.Now(),
				},
			}
			if _, err := rolesCollection.UpdateOne(ctx, filter, update); err != nil {
				log.Printf("警告: 更新角色 %s 权限失败: %v", role.Name, err)
			} else {
				log.Printf("已更新角色 %s 的默认权限", role.Name)
			}
		} else {
			return fmt.Errorf("failed to check role %s: %v", role.Name, err)
		}
	}

	// 创建默认管理员账号
	usersCollection := db.Collection("users")
	filter := bson.M{"username": "admin"}
	count, err := usersCollection.CountDocuments(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to check admin user: %v", err)
	}

	if count == 0 {
		adminPassword := os.Getenv("MANAGER_ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "admin123"
		}
		hashedPassword, _ := HashPassword(adminPassword)

		adminUser := User{
			Username:  "admin",
			Password:  hashedPassword,
			Email:     "admin@artemis-hids.local",
			FullName:  "系统管理员",
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		result, err := usersCollection.InsertOne(ctx, adminUser)
		if err != nil {
			return fmt.Errorf("failed to create admin user: %v", err)
		}

		// 获取插入的用户ID
		adminUser.ID = result.InsertedID

		// 关联管理员角色 - 直接将角色ID存入用户的 role_ids 字段
		var adminRole Role
		if err := rolesCollection.FindOne(ctx, bson.M{"name": "admin"}).Decode(&adminRole); err == nil {
			// 更新用户的 role_ids 字段
			update := bson.M{
				"$set": bson.M{
					"role_ids": []interface{}{adminRole.ID},
				},
			}
			if _, err := usersCollection.UpdateOne(ctx, bson.M{"_id": adminUser.ID}, update); err != nil {
				log.Printf("警告: 关联管理员角色失败: %v", err)
			} else {
				log.Printf("已为 admin 用户关联管理员角色")
			}
		}

		log.Printf("创建默认管理员账号: admin / %s", adminPassword)
	}

	return nil
}
