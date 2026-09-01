package server

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User 用户模型
type User struct {
	ID        interface{} `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time   `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time  `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`

	Username  string     `bson:"username" json:"username"`
	Password  string     `bson:"password" json:"-"` // bcrypt hash
	Email     string     `bson:"email" json:"email"`
	FullName  string     `bson:"full_name" json:"full_name"`
	Status    string     `bson:"status" json:"status"` // active, disabled
	LastLogin *time.Time `bson:"last_login,omitempty" json:"last_login,omitempty"`

	// 关联角色 (存储角色ID列表)
	RoleIDs []interface{} `bson:"role_ids,omitempty" json:"role_ids,omitempty"`
	Roles   []Role        `bson:"-" json:"roles,omitempty"` // 不存储在数据库，仅用于查询时填充
}

// Role 角色模型
type Role struct {
	ID        interface{} `bson:"_id,omitempty" json:"id"`
	CreatedAt time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time   `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time  `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`

	Name        string `bson:"name" json:"name"`
	DisplayName string `bson:"display_name" json:"display_name"`
	Description string `bson:"description" json:"description"`

	// 权限列表 (JSON array)
	Permissions string `bson:"permissions" json:"permissions"`
}

// Agent Agent主机信息模型
type Agent struct {
	ID            interface{} `bson:"_id,omitempty" json:"id"`
	AgentID       string      `bson:"agent_id" json:"agent_id"`
	Hostname      string      `bson:"hostname" json:"hostname"`
	IP            string      `bson:"ip" json:"ip"`
	OS            string      `bson:"os" json:"os"`
	Platform      string      `bson:"platform" json:"platform"`
	CPU           string      `bson:"cpu" json:"cpu"`
	CPUCount      int         `bson:"cpu_count" json:"cpu_count"`
	Memory        string      `bson:"memory" json:"memory"`
	Disk          string      `bson:"disk" json:"disk"`
	Version       string      `bson:"version" json:"version"`
	Status        string      `bson:"status" json:"status"` // online, offline
	Online        bool        `bson:"online" json:"online"`
	LastHeartbeat *time.Time  `bson:"last_heartbeat,omitempty" json:"last_heartbeat,omitempty"`
	CPUUsage      float64     `bson:"cpu_usage" json:"cpu_usage"`
	MemoryUsage   float64     `bson:"memory_usage" json:"memory_usage"`
	DiskUsage     float64     `bson:"disk_usage" json:"disk_usage"`
	LoadAverage   float64     `bson:"load_average" json:"load_average"`
	Uptime        int64       `bson:"uptime" json:"uptime"`

	// 详细硬件信息（来自 system_info 数据）
	// 使用 interface{} 类型以兼容字符串和数字格式
	CPUModel         string      `bson:"cpu_model" json:"cpu_model"`
	CPUPhysicalCount interface{} `bson:"cpu_physical_count" json:"cpu_physical_count"`
	CPULoadPercent   interface{} `bson:"cpu_load_percent" json:"cpu_load_percent"`
	MemoryTotalGB    interface{} `bson:"memory_total_gb" json:"memory_total_gb"`
	MemoryPercent    interface{} `bson:"memory_percent" json:"memory_percent"`
	DiskTotalGB      interface{} `bson:"disk_total_gb" json:"disk_total_gb"`
	DiskCount        interface{} `bson:"disk_count" json:"disk_count"`
	DiskPercent      interface{} `bson:"disk_percent" json:"disk_percent"`
	IPCount          interface{} `bson:"ip_count" json:"ip_count"`
	LocalIPs         interface{} `bson:"local_ips" json:"local_ips"`

	// 系统信息原始数据（用于存储完整的 system_info data）
	Data interface{} `bson:"data,omitempty" json:"data,omitempty"`

	// 关联关系 (不存储在数据库，仅用于查询时填充)
	Processes      []AgentProcess       `bson:"-" json:"processes,omitempty"`
	Ports          []AgentPort          `bson:"-" json:"ports,omitempty"`
	Software       []AgentSoftware      `bson:"-" json:"software,omitempty"`
	Services       []AgentService       `bson:"-" json:"services,omitempty"`
	Containers     []AgentContainer     `bson:"-" json:"containers,omitempty"`
	Connections    []AgentConnection    `bson:"-" json:"connections,omitempty"`
	SecurityEvents []AgentSecurityEvent `bson:"-" json:"security_events,omitempty"`

	CreatedAt time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

// AgentProcess 进程模型
type AgentProcess struct {
	ID            interface{} `bson:"_id,omitempty" json:"id"`
	AgentID       string      `bson:"agent_id" json:"agent_id"`
	PID           int         `bson:"pid" json:"pid"`
	Name          string      `bson:"name" json:"name"`
	Cmdline       string      `bson:"cmdline" json:"cmdline"`
	Exe           string      `bson:"exe" json:"exe"`
	User          string      `bson:"user" json:"user"`
	Status        string      `bson:"status" json:"status"`
	CPUPercent    float64     `bson:"cpu_percent" json:"cpu_percent"`
	MemoryPercent float64     `bson:"memory_percent" json:"memory_percent"`
	MemoryRSS     int64       `bson:"memory_rss" json:"memory_rss"`
	NumThreads    int         `bson:"num_threads" json:"num_threads"`
	CreateTime    int64       `bson:"create_time" json:"create_time"`
	ParentPID     int         `bson:"parent_pid" json:"parent_pid"`
	CreatedAt     time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time   `bson:"updated_at" json:"updated_at"`
}

// AgentPort 端口模型
type AgentPort struct {
	ID            interface{} `bson:"_id,omitempty" json:"id"`
	AgentID       string      `bson:"agent_id" json:"agent_id"`
	Port          int         `bson:"port" json:"port"`
	Protocol      string      `bson:"protocol" json:"protocol"`
	LocalAddress  string      `bson:"local_address" json:"local_address"`
	RemoteAddress string      `bson:"remote_address" json:"remote_address"`
	Status        string      `bson:"status" json:"status"`
	PID           int         `bson:"pid" json:"pid"`
	ProcessName   string      `bson:"process_name" json:"process_name"`
	Service       string      `bson:"service" json:"service"`
	RiskLevel     string      `bson:"risk_level" json:"risk_level"`
	CreatedAt     time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time   `bson:"updated_at" json:"updated_at"`
}

// AgentSoftware 软件模型
type AgentSoftware struct {
	ID             interface{} `bson:"_id,omitempty" json:"id"`
	AgentID        string      `bson:"agent_id" json:"agent_id"`
	Name           string      `bson:"name" json:"name"`
	Version        string      `bson:"version" json:"version"`
	Type           string      `bson:"type" json:"type"`
	PackageManager string      `bson:"package_manager" json:"package_manager"`
	Architecture   string      `bson:"architecture" json:"architecture"`
	InstallDate    *time.Time  `bson:"install_date,omitempty" json:"install_date,omitempty"`
	Size           int64       `bson:"size" json:"size"`
	Description    string      `bson:"description" json:"description"`
	Vendor         string      `bson:"vendor" json:"vendor"`
	CreatedAt      time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time   `bson:"updated_at" json:"updated_at"`
}

// AgentService 服务模型
type AgentService struct {
	ID          interface{} `bson:"_id,omitempty" json:"id"`
	AgentID     string      `bson:"agent_id" json:"agent_id"`
	Name        string      `bson:"name" json:"name"`
	DisplayName string      `bson:"display_name" json:"display_name"`
	Description string      `bson:"description" json:"description"`
	Status      string      `bson:"status" json:"status"`
	State       string      `bson:"state" json:"state"`
	StartMode   string      `bson:"start_mode" json:"start_mode"`
	PID         int         `bson:"pid" json:"pid"`
	User        string      `bson:"user" json:"user"`
	Path        string      `bson:"path" json:"path"`
	CreatedAt   time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time   `bson:"updated_at" json:"updated_at"`
}

// AgentContainer 容器模型
type AgentContainer struct {
	ID          interface{} `bson:"_id,omitempty" json:"id"`
	AgentID     string      `bson:"agent_id" json:"agent_id"`
	ContainerID string      `bson:"container_id" json:"container_id"`
	Name        string      `bson:"name" json:"name"`
	Image       string      `bson:"image" json:"image"`
	ImageID     string      `bson:"image_id" json:"image_id"`
	Command     string      `bson:"command" json:"command"`
	Created     int64       `bson:"created" json:"created"`
	Status      string      `bson:"status" json:"status"`
	State       string      `bson:"state" json:"state"`
	Ports       string      `bson:"ports" json:"ports"`       // JSON
	Labels      string      `bson:"labels" json:"labels"`     // JSON
	Networks    string      `bson:"networks" json:"networks"` // JSON
	Mounts      string      `bson:"mounts" json:"mounts"`     // JSON
	CreatedAt   time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time   `bson:"updated_at" json:"updated_at"`
}

// AgentConnection 网络连接模型
type AgentConnection struct {
	ID            interface{} `bson:"_id,omitempty" json:"id"`
	AgentID       string      `bson:"agent_id" json:"agent_id"`
	Protocol      string      `bson:"protocol" json:"protocol"`
	LocalAddress  string      `bson:"local_address" json:"local_address"`
	LocalPort     int         `bson:"local_port" json:"local_port"`
	RemoteAddress string      `bson:"remote_address" json:"remote_address"`
	RemotePort    int         `bson:"remote_port" json:"remote_port"`
	Status        string      `bson:"status" json:"status"`
	PID           int         `bson:"pid" json:"pid"`
	ProcessName   string      `bson:"process_name" json:"process_name"`
	Family        string      `bson:"family" json:"family"`
	CreatedAt     time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time   `bson:"updated_at" json:"updated_at"`
}

// AgentSecurityEvent 安全事件模型
type AgentSecurityEvent struct {
	ID          interface{} `bson:"_id,omitempty" json:"id"`
	AgentID     string      `bson:"agent_id" json:"agent_id"`
	EventType   string      `bson:"event_type" json:"event_type"`
	Severity    string      `bson:"severity" json:"severity"`
	Title       string      `bson:"title" json:"title"`
	Description string      `bson:"description" json:"description"`
	Source      string      `bson:"source" json:"source"`
	Timestamp   int64       `bson:"timestamp" json:"timestamp"`
	ProcessName string      `bson:"process_name" json:"process_name"`
	ProcessPath string      `bson:"process_path" json:"process_path"`
	User        string      `bson:"user" json:"user"`
	Details     string      `bson:"details" json:"details"`
	Status      string      `bson:"status" json:"status"`
	CreatedAt   time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time   `bson:"updated_at" json:"updated_at"`
}

// Permission 权限定义
const (
	PermViewDashboard = "view:dashboard"
	PermViewAgents    = "view:agents"
	PermManageAgents  = "manage:agents"
	PermManageConfig  = "manage:config"
	PermManageUsers   = "manage:users"
	PermManageRoles   = "manage:roles"
	PermViewRules     = "view:rules"
	PermManageRules   = "manage:rules"
)

// 预定义角色
var DefaultRoles = []Role{
	{
		Name:        "admin",
		DisplayName: "系统管理员",
		Description: "拥有所有权限",
		Permissions: `["view:dashboard","view:agents","manage:agents","manage:config","manage:users","manage:roles","view:rules","manage:rules"]`,
	},
	{
		Name:        "operator",
		DisplayName: "运维人员",
		Description: "可以查看所有数据和管理Agent配置",
		Permissions: `["view:dashboard","view:agents","manage:agents","manage:config","view:rules","manage:rules"]`,
	},
	{
		Name:        "analyst",
		DisplayName: "安全分析师",
		Description: "只能查看数据，不能修改配置",
		Permissions: `["view:dashboard","view:agents","view:rules"]`,
	},
	{
		Name:        "viewer",
		DisplayName: "只读用户",
		Description: "只能查看Dashboard",
		Permissions: `["view:dashboard"]`,
	},
}

// Rule 检测规则模型（与 rule-enginx_rust 的 src/rule/types.rs::Rule 字段一一对应）
type Rule struct {
	ID          string   `bson:"id" json:"id"`
	Name        string   `bson:"name" json:"name"`
	Description string   `bson:"description" json:"description"`
	Severity    string   `bson:"severity" json:"severity"` // critical, high, medium, low, info
	Enabled     bool     `bson:"enabled" json:"enabled"`
	Type        string   `bson:"type" json:"type"` // pattern, threshold, correlation, statistical
	Tags        []string `bson:"tags" json:"tags"`

	Conditions  RuleConditions     `bson:"conditions" json:"conditions"`
	Threshold   *ThresholdConfig   `bson:"threshold,omitempty" json:"threshold,omitempty"`
	Correlation *CorrelationConfig `bson:"correlation,omitempty" json:"correlation,omitempty"`
	Actions     []Action           `bson:"actions" json:"actions"`

	Author    string    `bson:"author" json:"author"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// RuleConditions 规则匹配条件
type RuleConditions struct {
	WorkerName string            `bson:"worker_name,omitempty" json:"worker_name,omitempty"`
	EventType  string            `bson:"event_type,omitempty" json:"event_type,omitempty"`
	Fields     map[string]string `bson:"fields" json:"fields"`
	Patterns   map[string]string `bson:"patterns" json:"patterns"`
	Expression string            `bson:"expression,omitempty" json:"expression,omitempty"`
	Exclude    *ExcludeCondition `bson:"exclude,omitempty" json:"exclude,omitempty"`
}

// ExcludeCondition 排除条件
type ExcludeCondition struct {
	Fields   map[string]string `bson:"fields" json:"fields"`
	Patterns map[string]string `bson:"patterns" json:"patterns"`
}

// ThresholdConfig 阈值规则配置
type ThresholdConfig struct {
	Field      string   `bson:"field" json:"field"`
	Operator   string   `bson:"operator" json:"operator"` // >, <, >=, <=, ==, !=
	Value      float64  `bson:"value" json:"value"`
	TimeWindow uint64   `bson:"time_window" json:"time_window"` // 秒
	GroupBy    []string `bson:"group_by" json:"group_by"`
}

// CorrelationConfig 关联规则配置
type CorrelationConfig struct {
	Events     []string `bson:"events" json:"events"`
	TimeWindow uint64   `bson:"time_window" json:"time_window"` // 秒
	GroupBy    []string `bson:"group_by" json:"group_by"`
	MinCount   int      `bson:"min_count" json:"min_count"`
}

// Action 规则触发后的处置动作
type Action struct {
	Type   string                 `bson:"type" json:"type"`
	Config map[string]interface{} `bson:"config" json:"config"`
}

// DashboardStats 仪表盘统计信息模型
type DashboardStats struct {
	ID             interface{} `bson:"_id,omitempty" json:"id"`
	StatDate       time.Time   `bson:"stat_date" json:"stat_date"` // 需要手动设置为当天日期
	TotalAgents    int64       `bson:"total_agents" json:"total_agents"`
	OnlineAgents   int64       `bson:"online_agents" json:"online_agents"`
	OnlineRate     float64     `bson:"online_rate" json:"online_rate"`
	TodayAlerts    int64       `bson:"today_alerts" json:"today_alerts"`
	PendingAlerts  int64       `bson:"pending_alerts" json:"pending_alerts"`
	TotalAlerts    int64       `bson:"total_alerts" json:"total_alerts"`
	CriticalAlerts int64       `bson:"critical_alerts" json:"critical_alerts"`
	HighAlerts     int64       `bson:"high_alerts" json:"high_alerts"`
	MediumAlerts   int64       `bson:"medium_alerts" json:"medium_alerts"`
	LowAlerts      int64       `bson:"low_alerts" json:"low_alerts"`
	UpdatedAt      time.Time   `bson:"updated_at" json:"updated_at"`
	CreatedAt      time.Time   `bson:"created_at" json:"created_at"`
}

// ToObjectID 将 interface{} 转换为 ObjectID
func ToObjectID(id interface{}) (primitive.ObjectID, error) {
	switch v := id.(type) {
	case primitive.ObjectID:
		return v, nil
	case string:
		return primitive.ObjectIDFromHex(v)
	default:
		return primitive.NilObjectID, nil
	}
}
