package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 配置结构
type Config struct {
	Kafka   KafkaConfig   `yaml:"kafka"`
	MongoDB MongoDBConfig `yaml:"mongodb"`
	Sync    SyncConfig    `yaml:"sync"`
	Log     LogConfig     `yaml:"log"`
}

// KafkaConfig Kafka 配置
type KafkaConfig struct {
	Brokers []string   `yaml:"brokers"`
	GroupID string     `yaml:"group_id"`
	Topics  []string   `yaml:"topics"`
	SASL    SASLConfig `yaml:"sasl"`
}

// SASLConfig SASL 认证配置
type SASLConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Mechanism string `yaml:"mechanism"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
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
	BatchSize       int  `yaml:"batch_size"`
	FlushInterval   int  `yaml:"flush_interval"`
	SystemInfoDedup bool `yaml:"system_info_dedup"`
	AlertDedup      bool `yaml:"alert_dedup"`
	Workers         int  `yaml:"workers"`
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
	if config.Sync.FlushInterval == 0 {
		config.Sync.FlushInterval = 5
	}
	if config.Sync.Workers == 0 {
		config.Sync.Workers = 4
	}
	if config.MongoDB.MaxPoolSize == 0 {
		config.MongoDB.MaxPoolSize = 100
	}
	if config.MongoDB.MinPoolSize == 0 {
		config.MongoDB.MinPoolSize = 10
	}

	return &config, nil
}
