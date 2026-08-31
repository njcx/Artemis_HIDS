// ============================================
// 告警输出处理
// ============================================

use anyhow::Result;
use async_trait::async_trait;
use std::sync::Mutex;
use std::fs::OpenOptions;
use std::io::Write;
use rdkafka::config::ClientConfig;
use rdkafka::producer::{FutureProducer, FutureRecord};
use log::info;

use super::alert::Alert;

#[async_trait]
pub trait OutputHandler: Send + Sync {
    async fn send(&self, alert: &Alert) -> Result<()>;
}

// ============================================
// 标准输出
// ============================================

pub struct StdoutOutput;

impl StdoutOutput {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl OutputHandler for StdoutOutput {
    async fn send(&self, alert: &Alert) -> Result<()> {
        let json = serde_json::to_string_pretty(alert)?;
        println!("ALERT:\n{}", json);
        Ok(())
    }
}

// ============================================
// 文件输出
// ============================================

pub struct FileOutput {
    path: String,
    file: Mutex<std::fs::File>,
}

impl FileOutput {
    pub fn new(path: &str) -> Result<Self> {
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(path)?;
        
        Ok(Self {
            path: path.to_string(),
            file: Mutex::new(file),
        })
    }
}

#[async_trait]
impl OutputHandler for FileOutput {
    async fn send(&self, alert: &Alert) -> Result<()> {
        let json = serde_json::to_string(alert)?;
        let mut file = self.file.lock().unwrap();
        writeln!(file, "{}", json)?;
        file.flush()?;
        Ok(())
    }
}

// ============================================
// Kafka 输出
// ============================================

pub struct KafkaOutput {
    producer: FutureProducer,
    topic: String,
}

impl KafkaOutput {
    pub async fn new(config: serde_json::Value) -> Result<Self> {
        let brokers = config["brokers"]
            .as_array()
            .and_then(|arr| {
                Some(arr.iter()
                    .filter_map(|v| v.as_str())
                    .collect::<Vec<_>>()
                    .join(","))
            })
            .unwrap_or_else(|| "localhost:9092".to_string());

        let topic = config["topic"]
            .as_str()
            .unwrap_or("hids-alerts")
            .to_string();

        let mut client_config = ClientConfig::new();
        client_config
            .set("bootstrap.servers", &brokers)
            .set("message.timeout.ms", "5000")
            .set("queue.buffering.max.messages", "100000")
            .set("queue.buffering.max.kbytes", "1048576")
            .set("batch.num.messages", "10000");

        // SASL 配置
        if let Some(sasl) = config["sasl"].as_object() {
            if sasl.get("enabled").and_then(|v| v.as_bool()).unwrap_or(false) {
                if let (Some(username), Some(password), Some(mechanism)) = (
                    sasl.get("username").and_then(|v| v.as_str()),
                    sasl.get("password").and_then(|v| v.as_str()),
                    sasl.get("mechanism").and_then(|v| v.as_str()),
                ) {
                    if !username.is_empty() {
                        client_config
                            .set("security.protocol", "SASL_PLAINTEXT")
                            .set("sasl.mechanism", mechanism)
                            .set("sasl.username", username)
                            .set("sasl.password", password);
                        
                        info!("Kafka 生产者 SASL 认证已启用");
                    }
                }
            }
        }

        let producer: FutureProducer = client_config.create()?;

        Ok(Self { producer, topic })
    }
}

#[async_trait]
impl OutputHandler for KafkaOutput {
    async fn send(&self, alert: &Alert) -> Result<()> {
        let payload = serde_json::to_string(alert)?;
        
        let record = FutureRecord::to(&self.topic)
            .key(&alert.alert_id)
            .payload(&payload);

        self.producer
            .send(record, std::time::Duration::from_secs(0))
            .await
            .map_err(|(e, _)| anyhow::anyhow!("发送到 Kafka 失败: {}", e))?;

        Ok(())
    }
}
