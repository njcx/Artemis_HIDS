// ============================================
// 配置管理模块
// ============================================

use anyhow::Result;
use serde::{Deserialize, Serialize};
use std::fs;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    pub kafka: KafkaConfig,
    pub openobserve: OpenObserveConfig,
    pub redis: RedisConfig,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct KafkaConfig {
    pub brokers: Vec<String>,
    pub group_id: String,
    pub topics: Vec<TopicConfig>,
    #[serde(default)]
    pub sasl: Option<SaslConfig>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct TopicConfig {
    pub name: String,
    pub index: String,  // OpenObserve 索引名称
    #[serde(default)]
    pub is_heartbeat: bool,  // 是否为心跳 topic
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct SaslConfig {
    pub enabled: bool,
    pub mechanism: String,
    pub username: String,
    pub password: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct OpenObserveConfig {
    pub url: String,
    pub org: String,
    pub username: String,
    pub password: String,
    #[serde(default = "default_batch_size")]
    pub batch_size: usize,
    #[serde(default = "default_flush_interval")]
    pub flush_interval_secs: u64,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct RedisConfig {
    pub url: String,
    #[serde(default = "default_key_prefix")]
    pub key_prefix: String,
    #[serde(default = "default_ttl")]
    pub ttl_secs: usize,
}

fn default_batch_size() -> usize {
    100
}

fn default_flush_interval() -> u64 {
    5
}

fn default_key_prefix() -> String {
    "hids:agent:".to_string()
}

fn default_ttl() -> usize {
    300
}

impl Config {
    pub async fn load(path: &str) -> Result<Self> {
        let content = fs::read_to_string(path)?;
        let config: Config = serde_yaml::from_str(&content)?;
        Ok(config)
    }
}
