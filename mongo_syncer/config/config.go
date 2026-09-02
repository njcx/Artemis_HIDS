package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 配置结构
type Config struct {
	OpenObserve OpenObserveConfig `yaml:"openobserve"`
	MongoDB     MongoDBConfig     `yaml:"mongodb"`
	Sync        SyncConfig        `yaml:"sync"`
	Log         LogConfig         `yaml:"log"`
}

// OpenObserveConfig OpenObserve 查询配置
type OpenObserveConfig struct {
	URL      string `yaml:"url"`
	Org      string `yaml:"org"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// MongoDBConfig MongoDB 配置
type MongoDBConfig struct {
	URI             string `yaml:"uri"`
	Database        string `yaml:"database"`
	MaxPoolSize     uint64 `yaml:"max_pool_size"`
	MinPoolSize     uint64 `yaml:"min_pool_size"`
	MaxConnIdleTime int    `yaml:"max_conn_idle_time"`
}

// SyncConfig 同步配置
type SyncConfig struct {
	// PollInterval 轮询 OpenObserve 的间隔（秒）
	PollInterval int `yaml:"poll_interval"`
	// LookbackSecs 首次启动或游标丢失时，流水型数据（进程/告警）回溯查询的时间窗口（秒）
	LookbackSecs int `yaml:"lookback_secs"`
	// BatchSize 批量写入 MongoDB 时的分批大小
	BatchSize int `yaml:"batch_size"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level         string `yaml:"level"`
	File          string `yaml:"file"`
	EnableConsole bool   `yaml:"enable_console"`
}

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if config.Sync.BatchSize == 0 {
		config.Sync.BatchSize = 100
	}
	if config.Sync.PollInterval == 0 {
		config.Sync.PollInterval = 30
	}
	if config.Sync.LookbackSecs == 0 {
		config.Sync.LookbackSecs = 300
	}
	if config.OpenObserve.Org == "" {
		config.OpenObserve.Org = "default"
	}
	if config.MongoDB.MaxPoolSize == 0 {
		config.MongoDB.MaxPoolSize = 100
	}
	if config.MongoDB.MinPoolSize == 0 {
		config.MongoDB.MinPoolSize = 10
	}

	return &config, nil
}
