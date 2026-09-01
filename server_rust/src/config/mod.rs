// ============================================
// 配置管理模块
// ============================================

use anyhow::Result;
use serde::{Deserialize, Serialize};
use std::fs;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    pub server: ServerConfig,
    pub kafka: KafkaConfig,
    pub redis: RedisConfig,
    pub logging: LoggingConfig,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
    pub max_concurrent_streams: u32,
    pub max_receive_message_size: usize,
    #[serde(default)]
    pub tls: Option<TlsConfig>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct TlsConfig {
    pub enabled: bool,
    pub cert_file: String,
    pub key_file: String,
    #[serde(default)]
    pub ca_file: Option<String>,
    #[serde(default)]
    pub client_auth: bool,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct KafkaConfig {
    pub brokers: Vec<String>,
    pub topic: String,
    #[serde(default)]
    pub compression: Option<String>,
    #[serde(default)]
    pub batch_size: Option<usize>,
    #[serde(default)]
    pub batch_timeout: Option<String>,
    #[serde(default)]
    pub max_retries: Option<u32>,
    #[serde(default)]
    pub buffer_memory: Option<usize>,
    #[serde(default)]
    pub max_in_flight: Option<u32>,
    #[serde(default = "default_async")]
    pub r#async: bool,
    #[serde(default)]
    pub sasl: Option<SaslConfig>,
}

fn default_async() -> bool {
    true
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct SaslConfig {
    pub enabled: bool,
    pub mechanism: String,
    pub username: String,
    pub password: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct RedisConfig {
    pub host: String,
    pub port: u16,
    #[serde(default)]
    pub password: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct LoggingConfig {
    pub level: String,
    #[serde(default)]
    pub file: Option<String>,
    #[serde(default)]
    pub max_size: Option<usize>,
    #[serde(default)]
    pub max_backups: Option<usize>,
    #[serde(default)]
    pub max_age: Option<usize>,
}

impl Config {
    pub fn from_file(path: &str) -> Result<Self> {
        let content = fs::read_to_string(path)?;
        let config: Config = serde_yaml::from_str(&content)?;
        Ok(config)
    }
}
