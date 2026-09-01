// MongoDB 初始化脚本
// 使用方法: mongosh < init_mongodb.js

// 切换到 artemis_hids 数据库
use artemis_hids;

// 创建集合
db.createCollection("users");
db.createCollection("roles");
db.createCollection("user_roles");
db.createCollection("agents");
db.createCollection("agent_processes");
db.createCollection("agent_ports");
db.createCollection("agent_software");
db.createCollection("agent_services");
db.createCollection("agent_containers");
db.createCollection("agent_connections");
db.createCollection("agent_security_events");
db.createCollection("dashboard_stats");

// 创建索引
// users 集合
db.users.createIndex({ "username": 1 }, { unique: true });
db.users.createIndex({ "status": 1 });
db.users.createIndex({ "deleted_at": 1 });

// roles 集合
db.roles.createIndex({ "name": 1 }, { unique: true });
db.roles.createIndex({ "deleted_at": 1 });

// user_roles 集合
db.user_roles.createIndex({ "user_id": 1, "role_id": 1 }, { unique: true });

// agents 集合
db.agents.createIndex({ "agent_id": 1 }, { unique: true });
db.agents.createIndex({ "status": 1 });
db.agents.createIndex({ "online": 1 });
db.agents.createIndex({ "last_heartbeat": 1 });
db.agents.createIndex({ "deleted_at": 1 });

// agent_processes 集合
db.agent_processes.createIndex({ "agent_id": 1 });
db.agent_processes.createIndex({ "pid": 1 });
db.agent_processes.createIndex({ "name": 1 });
db.agent_processes.createIndex({ "user": 1 });

// agent_ports 集合
db.agent_ports.createIndex({ "agent_id": 1 });
db.agent_ports.createIndex({ "port": 1 });
db.agent_ports.createIndex({ "protocol": 1 });

// agent_software 集合
db.agent_software.createIndex({ "agent_id": 1 });
db.agent_software.createIndex({ "name": 1 });
db.agent_software.createIndex({ "type": 1 });

// agent_services 集合
db.agent_services.createIndex({ "agent_id": 1 });
db.agent_services.createIndex({ "name": 1 });
db.agent_services.createIndex({ "status": 1 });

// agent_containers 集合
db.agent_containers.createIndex({ "agent_id": 1 });
db.agent_containers.createIndex({ "container_id": 1 });
db.agent_containers.createIndex({ "status": 1 });

// agent_connections 集合
db.agent_connections.createIndex({ "agent_id": 1 });
db.agent_connections.createIndex({ "protocol": 1 });
db.agent_connections.createIndex({ "remote_address": 1 });

// agent_security_events 集合
db.agent_security_events.createIndex({ "agent_id": 1 });
db.agent_security_events.createIndex({ "severity": 1 });
db.agent_security_events.createIndex({ "status": 1 });
db.agent_security_events.createIndex({ "timestamp": -1 });
db.agent_security_events.createIndex({ "event_type": 1 });

// dashboard_stats 集合
db.dashboard_stats.createIndex({ "stat_date": 1 }, { unique: true });
db.dashboard_stats.createIndex({ "updated_at": -1 });

print("MongoDB 初始化完成！");
print("集合和索引已创建。");
