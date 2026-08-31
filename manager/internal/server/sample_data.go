package server

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// InitSampleAgentData 初始化样例Agent数据到MongoDB (使用新的关联表结构)
func InitSampleAgentData(db *mongo.Database) error {
	log.Println("开始插入样例Agent数据...")
	ctx := context.Background()

	// Agent 1: Web服务器
	agent1 := Agent{
		ID:            primitive.NewObjectID(),
		AgentID:       "agent-001",
		Hostname:      "web-server-01",
		IP:            "192.168.1.101",
		OS:            "Ubuntu 20.04 LTS",
		Platform:      "Linux",
		CPU:           "Intel Xeon E5-2680 v4 @ 2.40GHz (8 cores)",
		Memory:        "34359738368",  // 32GB in bytes
		Disk:          "536870912000", // 500GB in bytes
		Version:       "1.0.0",
		Status:        "online",
		Online:        true,
		LastHeartbeat: timePtr(time.Now()),
		CPUUsage:      45.2,
		MemoryUsage:   68.5,
		DiskUsage:     55.3,
		LoadAverage:   2.5,
		Uptime:        864000, // 10天
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Agent 1 的进程数据
	agent1Processes := []AgentProcess{
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-001",
			PID:           1234,
			Name:          "nginx",
			Cmdline:       "nginx: master process /usr/sbin/nginx",
			Exe:           "/usr/sbin/nginx",
			User:          "root",
			Status:        "running",
			CPUPercent:    2.5,
			MemoryPercent: 1.2,
			MemoryRSS:     52428800, // 50MB
			NumThreads:    4,
			CreateTime:    time.Now().Add(-24 * time.Hour).Unix(),
			ParentPID:     1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-001",
			PID:           5678,
			Name:          "node",
			Cmdline:       "node /app/server.js",
			Exe:           "/usr/bin/node",
			User:          "www-data",
			Status:        "running",
			CPUPercent:    15.3,
			MemoryPercent: 8.5,
			MemoryRSS:     268435456, // 256MB
			NumThreads:    8,
			CreateTime:    time.Now().Add(-12 * time.Hour).Unix(),
			ParentPID:     1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	// Agent 1 的端口数据
	agent1Ports := []AgentPort{
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-001",
			Port:          80,
			Protocol:      "TCP",
			LocalAddress:  "0.0.0.0:80",
			RemoteAddress: "",
			Status:        "LISTEN",
			PID:           1234,
			ProcessName:   "nginx",
			Service:       "http",
			RiskLevel:     "low",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-001",
			Port:          443,
			Protocol:      "TCP",
			LocalAddress:  "0.0.0.0:443",
			RemoteAddress: "",
			Status:        "LISTEN",
			PID:           1234,
			ProcessName:   "nginx",
			Service:       "https",
			RiskLevel:     "low",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:           primitive.NewObjectID(),
			AgentID:      "agent-001",
			Port:         3000,
			Protocol:     "TCP",
			LocalAddress: "127.0.0.1:3000",
			Status:       "LISTEN",
			PID:          5678,
			ProcessName:  "node",
			Service:      "node-app",
			RiskLevel:    "medium",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	// Agent 1 的软件数据
	agent1Software := []AgentSoftware{
		{
			ID:             primitive.NewObjectID(),
			AgentID:        "agent-001",
			Name:           "nginx",
			Version:        "1.18.0",
			Type:           "package",
			PackageManager: "apt",
			Architecture:   "amd64",
			InstallDate:    timePtr(time.Now().Add(-90 * 24 * time.Hour)),
			Description:    "high performance web server",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             primitive.NewObjectID(),
			AgentID:        "agent-001",
			Name:           "nodejs",
			Version:        "14.17.0",
			Type:           "package",
			PackageManager: "apt",
			Architecture:   "amd64",
			InstallDate:    timePtr(time.Now().Add(-60 * 24 * time.Hour)),
			Description:    "JavaScript runtime built on Chrome's V8",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             primitive.NewObjectID(),
			AgentID:        "agent-001",
			Name:           "docker",
			Version:        "20.10.7",
			Type:           "package",
			PackageManager: "apt",
			Architecture:   "amd64",
			InstallDate:    timePtr(time.Now().Add(-30 * 24 * time.Hour)),
			Description:    "Docker container runtime",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Agent 1 的服务数据
	agent1Services := []AgentService{
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			Name:        "nginx",
			DisplayName: "Nginx Web Server",
			Description: "A high performance web server",
			Status:      "running",
			State:       "active",
			StartMode:   "auto",
			PID:         1234,
			User:        "root",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			Name:        "ssh",
			DisplayName: "OpenSSH Server",
			Description: "OpenSSH Secure Shell server",
			Status:      "running",
			State:       "active",
			StartMode:   "auto",
			PID:         890,
			User:        "root",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Agent 1 的容器数据
	agent1Containers := []AgentContainer{
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			ContainerID: "abc123def456",
			Name:        "redis-cache",
			Image:       "redis:6.2-alpine",
			ImageID:     "sha256:abc123",
			Command:     "redis-server",
			Created:     time.Now().Add(-48 * time.Hour).Unix(),
			Status:      "running",
			State:       "running",
			Ports:       `[{"HostPort":"6379","ContainerPort":"6379","Protocol":"tcp"}]`,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Agent 1 的连接数据
	agent1Connections := []AgentConnection{
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-001",
			Protocol:      "TCP",
			LocalAddress:  "192.168.1.101",
			LocalPort:     80,
			RemoteAddress: "192.168.1.200",
			RemotePort:    45678,
			Status:        "ESTABLISHED",
			PID:           1234,
			ProcessName:   "nginx",
			Family:        "IPv4",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-001",
			Protocol:      "TCP",
			LocalAddress:  "192.168.1.101",
			LocalPort:     443,
			RemoteAddress: "192.168.1.201",
			RemotePort:    56789,
			Status:        "ESTABLISHED",
			PID:           1234,
			ProcessName:   "nginx",
			Family:        "IPv4",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	// Agent 1 的安全事件数据（更多今日告警）
	agent1SecurityEvents := []AgentSecurityEvent{
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "suspicious_process",
			Severity:    "high",
			Title:       "检测到可疑进程执行",
			Description: "发现未知进程尝试访问敏感文件 /etc/shadow",
			Source:      "file_monitor",
			Timestamp:   time.Now().Add(-2 * time.Hour).UnixNano(),
			ProcessName: "unknown_proc",
			ProcessPath: "/tmp/unknown_proc",
			User:        "www-data",
			Details:     `{"file":"/etc/shadow","access_type":"read","denied":true}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "brute_force_attempt",
			Severity:    "critical",
			Title:       "SSH暴力破解尝试",
			Description: "检测到来自 192.168.1.250 的SSH暴力破解尝试，失败次数: 15",
			Source:      "auth_monitor",
			Timestamp:   time.Now().Add(-1 * time.Hour).UnixNano(),
			ProcessName: "sshd",
			ProcessPath: "/usr/sbin/sshd",
			User:        "root",
			Details:     `{"source_ip":"192.168.1.250","failed_attempts":15,"usernames":["root","admin","test"]}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "port_scan",
			Severity:    "medium",
			Title:       "端口扫描检测",
			Description: "检测到来自 10.0.0.88 的端口扫描行为",
			Source:      "network_monitor",
			Timestamp:   time.Now().Add(-30 * time.Minute).UnixNano(),
			ProcessName: "nginx",
			ProcessPath: "/usr/sbin/nginx",
			User:        "root",
			Details:     `{"source_ip":"10.0.0.88","scanned_ports":[22,80,443,3306,8080],"scan_type":"SYN"}`,
			Status:      "investigating",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "file_modification",
			Severity:    "high",
			Title:       "关键文件被修改",
			Description: "系统配置文件 /etc/hosts 被意外修改",
			Source:      "file_integrity",
			Timestamp:   time.Now().Add(-15 * time.Minute).UnixNano(),
			ProcessName: "vim",
			ProcessPath: "/usr/bin/vim",
			User:        "admin",
			Details:     `{"file":"/etc/hosts","old_hash":"abc123","new_hash":"def456","change_type":"content"}`,
			Status:      "resolved",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "malware_detected",
			Severity:    "critical",
			Title:       "检测到恶意软件",
			Description: "在 /tmp 目录发现可疑的挖矿程序",
			Source:      "malware_scanner",
			Timestamp:   time.Now().Add(-45 * time.Minute).UnixNano(),
			ProcessName: "xmrig",
			ProcessPath: "/tmp/.xmrig",
			User:        "nobody",
			Details:     `{"hash":"5d41402abc4b2a76b9719d911017c592","type":"cryptominer","action":"quarantined"}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "unauthorized_access",
			Severity:    "high",
			Title:       "未授权访问尝试",
			Description: "检测到未授权的sudo命令执行尝试",
			Source:      "access_control",
			Timestamp:   time.Now().Add(-3 * time.Hour).UnixNano(),
			ProcessName: "sudo",
			ProcessPath: "/usr/bin/sudo",
			User:        "webuser",
			Details:     `{"command":"sudo cat /etc/shadow","result":"denied"}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "network_anomaly",
			Severity:    "medium",
			Title:       "异常网络流量",
			Description: "检测到大量出站连接",
			Source:      "network_monitor",
			Timestamp:   time.Now().Add(-20 * time.Minute).UnixNano(),
			ProcessName: "unknown",
			ProcessPath: "",
			User:        "root",
			Details:     `{"connections":500,"destination":"multiple","protocol":"TCP"}`,
			Status:      "investigating",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-001",
			EventType:   "privilege_escalation",
			Severity:    "critical",
			Title:       "权限提升检测",
			Description: "检测到疑似提权漏洞利用",
			Source:      "kernel_monitor",
			Timestamp:   time.Now().Add(-10 * time.Minute).UnixNano(),
			ProcessName: "exploit",
			ProcessPath: "/tmp/exploit.sh",
			User:        "www-data",
			Details:     `{"technique":"SUID_abuse","target":"/usr/bin/passwd"}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Agent 2: 数据库服务器
	agent2 := Agent{
		ID:            primitive.NewObjectID(),
		AgentID:       "agent-002",
		Hostname:      "db-server-01",
		IP:            "192.168.1.102",
		OS:            "CentOS 7.9",
		Platform:      "Linux",
		CPU:           "AMD EPYC 7542 @ 2.90GHz (16 cores)",
		Memory:        "68719476736",   // 64GB in bytes
		Disk:          "1073741824000", // 1TB in bytes
		Version:       "1.0.0",
		Status:        "online",
		Online:        true,
		LastHeartbeat: timePtr(time.Now()),
		CPUUsage:      35.8,
		MemoryUsage:   72.3,
		DiskUsage:     45.7,
		LoadAverage:   4.2,
		Uptime:        1728000, // 20天
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Agent 2 的进程数据
	agent2Processes := []AgentProcess{
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-002",
			PID:           2345,
			Name:          "mysqld",
			Cmdline:       "/usr/sbin/mysqld",
			Exe:           "/usr/sbin/mysqld",
			User:          "mysql",
			Status:        "running",
			CPUPercent:    25.5,
			MemoryPercent: 45.2,
			MemoryRSS:     4294967296, // 4GB
			NumThreads:    64,
			CreateTime:    time.Now().Add(-20 * 24 * time.Hour).Unix(),
			ParentPID:     1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            primitive.NewObjectID(),
			AgentID:       "agent-002",
			PID:           3456,
			Name:          "redis-server",
			Cmdline:       "/usr/bin/redis-server",
			Exe:           "/usr/bin/redis-server",
			User:          "redis",
			Status:        "running",
			CPUPercent:    5.2,
			MemoryPercent: 8.5,
			MemoryRSS:     536870912, // 512MB
			NumThreads:    4,
			CreateTime:    time.Now().Add(-15 * 24 * time.Hour).Unix(),
			ParentPID:     1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	// Agent 2 的端口数据
	agent2Ports := []AgentPort{
		{
			ID:           primitive.NewObjectID(),
			AgentID:      "agent-002",
			Port:         3306,
			Protocol:     "TCP",
			LocalAddress: "0.0.0.0:3306",
			Status:       "LISTEN",
			PID:          2345,
			ProcessName:  "mysqld",
			Service:      "mysql",
			RiskLevel:    "medium",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           primitive.NewObjectID(),
			AgentID:      "agent-002",
			Port:         6379,
			Protocol:     "TCP",
			LocalAddress: "127.0.0.1:6379",
			Status:       "LISTEN",
			PID:          3456,
			ProcessName:  "redis-server",
			Service:      "redis",
			RiskLevel:    "low",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	// Agent 2 的软件数据
	agent2Software := []AgentSoftware{
		{
			ID:             primitive.NewObjectID(),
			AgentID:        "agent-002",
			Name:           "mysql-server",
			Version:        "8.0.26",
			Type:           "package",
			PackageManager: "yum",
			Architecture:   "x86_64",
			InstallDate:    timePtr(time.Now().Add(-100 * 24 * time.Hour)),
			Description:    "MySQL Database Server",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             primitive.NewObjectID(),
			AgentID:        "agent-002",
			Name:           "redis",
			Version:        "6.2.5",
			Type:           "package",
			PackageManager: "yum",
			Architecture:   "x86_64",
			InstallDate:    timePtr(time.Now().Add(-80 * 24 * time.Hour)),
			Description:    "Redis in-memory database",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Agent 2 的服务数据
	agent2Services := []AgentService{
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			Name:        "mysqld",
			DisplayName: "MySQL Server",
			Description: "MySQL Database Server",
			Status:      "running",
			State:       "active",
			StartMode:   "auto",
			PID:         2345,
			User:        "mysql",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			Name:        "redis",
			DisplayName: "Redis Server",
			Description: "Redis in-memory database",
			Status:      "running",
			State:       "active",
			StartMode:   "auto",
			PID:         3456,
			User:        "redis",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Agent 2 的安全事件数据
	agent2SecurityEvents := []AgentSecurityEvent{
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			EventType:   "sql_injection_attempt",
			Severity:    "critical",
			Title:       "SQL注入攻击尝试",
			Description: "检测到针对MySQL数据库的SQL注入尝试",
			Source:      "database_monitor",
			Timestamp:   time.Now().Add(-3 * time.Hour).UnixNano(),
			ProcessName: "mysqld",
			ProcessPath: "/usr/sbin/mysqld",
			User:        "mysql",
			Details:     `{"query":"SELECT * FROM users WHERE id='1' OR '1'='1'","source_ip":"192.168.1.88","blocked":true}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			EventType:   "unauthorized_access",
			Severity:    "high",
			Title:       "未授权数据库访问",
			Description: "检测到来自未知IP的数据库访问尝试",
			Source:      "access_control",
			Timestamp:   time.Now().Add(-45 * time.Minute).UnixNano(),
			ProcessName: "mysqld",
			ProcessPath: "/usr/sbin/mysqld",
			User:        "mysql",
			Details:     `{"source_ip":"203.0.113.45","username":"admin","database":"production","denied":true}`,
			Status:      "investigating",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			EventType:   "privilege_escalation",
			Severity:    "critical",
			Title:       "权限提升尝试",
			Description: "普通用户尝试获取root权限",
			Source:      "privilege_monitor",
			Timestamp:   time.Now().Add(-20 * time.Minute).UnixNano(),
			ProcessName: "sudo",
			ProcessPath: "/usr/bin/sudo",
			User:        "dbuser",
			Details:     `{"command":"sudo su -","result":"denied","attempts":3}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			EventType:   "data_exfiltration",
			Severity:    "critical",
			Title:       "数据泄露尝试",
			Description: "检测到异常大量数据传输",
			Source:      "network_monitor",
			Timestamp:   time.Now().Add(-1 * time.Hour).UnixNano(),
			ProcessName: "mysqld",
			ProcessPath: "/usr/sbin/mysqld",
			User:        "mysql",
			Details:     `{"bytes_transferred":10737418240,"destination":"unknown_server","duration":300}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			EventType:   "brute_force_attempt",
			Severity:    "high",
			Title:       "数据库暴力破解",
			Description: "检测到针对MySQL的暴力破解攻击",
			Source:      "auth_monitor",
			Timestamp:   time.Now().Add(-2 * time.Hour).UnixNano(),
			ProcessName: "mysqld",
			ProcessPath: "/usr/sbin/mysqld",
			User:        "mysql",
			Details:     `{"source_ip":"45.76.88.123","failed_attempts":50,"usernames":["root","admin","user"]}`,
			Status:      "open",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			AgentID:     "agent-002",
			EventType:   "suspicious_query",
			Severity:    "medium",
			Title:       "可疑SQL查询",
			Description: "检测到可能的数据库扫描行为",
			Source:      "database_monitor",
			Timestamp:   time.Now().Add(-30 * time.Minute).UnixNano(),
			ProcessName: "mysqld",
			ProcessPath: "/usr/sbin/mysqld",
			User:        "mysql",
			Details:     `{"query":"SELECT table_name FROM information_schema.tables","source":"internal"}`,
			Status:      "investigating",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Agent 3: 离线服务器
	agent3 := Agent{
		ID:            primitive.NewObjectID(),
		AgentID:       "agent-003",
		Hostname:      "app-server-01",
		IP:            "192.168.1.103",
		OS:            "Ubuntu 22.04 LTS",
		Platform:      "Linux",
		CPU:           "Intel Core i7-9700K @ 3.60GHz (8 cores)",
		Memory:        "17179869184",  // 16GB in bytes
		Disk:          "268435456000", // 250GB in bytes
		Version:       "1.0.0",
		Status:        "offline",
		Online:        false,
		LastHeartbeat: timePtr(time.Now().Add(-10 * time.Minute)),
		CPUUsage:      0,
		MemoryUsage:   0,
		DiskUsage:     0,
		LoadAverage:   0,
		Uptime:        0,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 使用 MongoDB 事务插入数据
	session, err := db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// 插入 Agent 1（带安全事件）
		if err := createOrUpdateAgent(sessCtx, db, &agent1, agent1Processes, agent1Ports, agent1Software, agent1Services, agent1Containers, agent1Connections, agent1SecurityEvents); err != nil {
			return nil, err
		}

		// 插入 Agent 2（带安全事件）
		if err := createOrUpdateAgent(sessCtx, db, &agent2, agent2Processes, agent2Ports, agent2Software, agent2Services, nil, nil, agent2SecurityEvents); err != nil {
			return nil, err
		}

		// 插入 Agent 3 (离线，无详细数据)
		if err := createOrUpdateAgent(sessCtx, db, &agent3, nil, nil, nil, nil, nil, nil, nil); err != nil {
			return nil, err
		}

		log.Println("样例Agent数据插入完成")
		return nil, nil
	})

	return err
}

// createOrUpdateAgent 创建或更新Agent及其关联数据
func createOrUpdateAgent(
	ctx context.Context,
	db *mongo.Database,
	agent *Agent,
	processes []AgentProcess,
	ports []AgentPort,
	software []AgentSoftware,
	services []AgentService,
	containers []AgentContainer,
	connections []AgentConnection,
	securityEvents []AgentSecurityEvent,
) error {
	// 检查Agent是否存在
	agentColl := db.Collection("agents")
	var existingAgent Agent
	err := agentColl.FindOne(ctx, map[string]interface{}{"agent_id": agent.AgentID, "deleted_at": nil}).Decode(&existingAgent)

	if err == mongo.ErrNoDocuments {
		// 不存在，创建新记录
		if _, err := agentColl.InsertOne(ctx, agent); err != nil {
			log.Printf("插入Agent失败 %s: %v", agent.AgentID, err)
			return err
		}
		log.Printf("插入样例Agent: %s (%s)", agent.AgentID, agent.Hostname)
	} else if err != nil {
		return err
	} else {
		// 已存在，更新Agent基本信息
		agent.ID = existingAgent.ID
		agent.CreatedAt = existingAgent.CreatedAt
		agent.UpdatedAt = time.Now()

		if _, err := agentColl.ReplaceOne(ctx, map[string]interface{}{"_id": existingAgent.ID}, agent); err != nil {
			log.Printf("更新Agent失败 %s: %v", agent.AgentID, err)
			return err
		}
		log.Printf("更新样例Agent: %s (%s)", agent.AgentID, agent.Hostname)

		// 删除旧的关联数据
		db.Collection("agent_processes").DeleteMany(ctx, map[string]interface{}{"agent_id": agent.AgentID})
		db.Collection("agent_ports").DeleteMany(ctx, map[string]interface{}{"agent_id": agent.AgentID})
		db.Collection("agent_software").DeleteMany(ctx, map[string]interface{}{"agent_id": agent.AgentID})
		db.Collection("agent_services").DeleteMany(ctx, map[string]interface{}{"agent_id": agent.AgentID})
		db.Collection("agent_containers").DeleteMany(ctx, map[string]interface{}{"agent_id": agent.AgentID})
		db.Collection("agent_connections").DeleteMany(ctx, map[string]interface{}{"agent_id": agent.AgentID})
		db.Collection("agent_security_events").DeleteMany(ctx, map[string]interface{}{"agent_id": agent.AgentID})
	}

	// 插入关联数据
	if len(processes) > 0 {
		docs := make([]interface{}, len(processes))
		for i := range processes {
			docs[i] = processes[i]
		}
		if _, err := db.Collection("agent_processes").InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	if len(ports) > 0 {
		docs := make([]interface{}, len(ports))
		for i := range ports {
			docs[i] = ports[i]
		}
		if _, err := db.Collection("agent_ports").InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	if len(software) > 0 {
		docs := make([]interface{}, len(software))
		for i := range software {
			docs[i] = software[i]
		}
		if _, err := db.Collection("agent_software").InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	if len(services) > 0 {
		docs := make([]interface{}, len(services))
		for i := range services {
			docs[i] = services[i]
		}
		if _, err := db.Collection("agent_services").InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	if len(containers) > 0 {
		docs := make([]interface{}, len(containers))
		for i := range containers {
			docs[i] = containers[i]
		}
		if _, err := db.Collection("agent_containers").InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	if len(connections) > 0 {
		docs := make([]interface{}, len(connections))
		for i := range connections {
			docs[i] = connections[i]
		}
		if _, err := db.Collection("agent_connections").InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	if len(securityEvents) > 0 {
		docs := make([]interface{}, len(securityEvents))
		for i := range securityEvents {
			docs[i] = securityEvents[i]
		}
		if _, err := db.Collection("agent_security_events").InsertMany(ctx, docs); err != nil {
			return err
		}
	}

	return nil
}

// timePtr 辅助函数，返回time.Time指针
func timePtr(t time.Time) *time.Time {
	return &t
}
