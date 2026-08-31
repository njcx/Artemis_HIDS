// ============================================
// 配置管理模块
// ============================================

use anyhow::Result;
use serde::{Deserialize, Serialize};
use std::fs;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    pub consumer: ConsumerConfig,
    pub alert: AlertConfig,
    pub rules: RulesConfig,
    pub logging: LoggingConfig,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ConsumerConfig {
    pub brokers: Vec<String>,
    /// 支持三种配置方式：
    /// 1. topic: "hids_events" - 单个topic（兼容旧配置）
    /// 2. topic: "hids_events-*" - topic前缀模式
    /// 3. topics: ["hids_events-file_monitor", "hids_events-process_monitor"] - 多个topic
    #[serde(default)]
    pub topic: Option<String>,
    #[serde(default)]
    pub topics: Option<Vec<String>>,
    pub group_id: String,
    #[serde(default)]
    pub min_bytes: i32,
    #[serde(default)]
    pub max_bytes: i32,
    #[serde(default)]
    pub sasl: Option<SaslConfig>,
    /// 消费起始位置：
    /// - "latest" (默认): 只消费新消息
    /// - "earliest": 从头开始消费所有消息
    #[serde(default = "default_auto_offset_reset")]
    pub auto_offset_reset: String,
    /// 处理线程数（默认 8）
    /// 建议设置为 CPU 核心数的 1-2 倍
    #[serde(default)]
    pub num_workers: Option<usize>,
    /// Channel 缓冲区大小（默认 10000）
    /// 用于消费者和处理器之间的消息缓冲
    #[serde(default)]
    pub channel_size: Option<usize>,
}

fn default_auto_offset_reset() -> String {
    "latest".to_string()
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct SaslConfig {
    pub enabled: bool,
    pub mechanism: String, // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
    pub username: String,
    pub password: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct AlertConfig {
    pub outputs: Vec<AlertOutput>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct AlertOutput {
    #[serde(rename = "type")]
    pub output_type: String, // stdout, file, kafka
    pub enabled: bool,
    pub config: serde_json::Value,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct RulesConfig {
    pub directory: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct LoggingConfig {
    pub level: String,
    #[serde(default)]
    pub file: Option<String>,
}

impl Config {
    pub fn from_file(path: &str) -> Result<Self> {
        let content = fs::read_to_string(path)?;
        let config: Config = serde_yaml::from_str(&content)?;
        Ok(config)
    }
}
