// ============================================
// Kafka 生产者模块
// ============================================

use anyhow::{Context, Result};
use log::{info, error};
use rdkafka::config::ClientConfig;
use rdkafka::producer::{FutureProducer, FutureRecord};
use serde_json::json;

use crate::config::KafkaConfig;

// 将 prost_types::Struct 转换为 serde_json::Value
fn struct_to_json_value(s: &prost_types::Struct) -> serde_json::Value {
    let mut map = serde_json::Map::new();
    for (key, value) in &s.fields {
        map.insert(key.clone(), value_to_json_value(value));
    }
    serde_json::Value::Object(map)
}

// 将 prost_types::Value 转换为 serde_json::Value
fn value_to_json_value(v: &prost_types::Value) -> serde_json::Value {
    match &v.kind {
        Some(prost_types::value::Kind::NullValue(_)) => serde_json::Value::Null,
        Some(prost_types::value::Kind::NumberValue(n)) => {
            serde_json::json!(n)
        }
        Some(prost_types::value::Kind::StringValue(s)) => {
            serde_json::Value::String(s.clone())
        }
        Some(prost_types::value::Kind::BoolValue(b)) => {
            serde_json::Value::Bool(*b)
        }
        Some(prost_types::value::Kind::StructValue(s)) => struct_to_json_value(s),
        Some(prost_types::value::Kind::ListValue(l)) => {
            let arr: Vec<serde_json::Value> = l
                .values
                .iter()
                .map(value_to_json_value)
                .collect();
            serde_json::Value::Array(arr)
        }
        None => serde_json::Value::Null,
    }
}

pub struct KafkaProducer {
    producer: FutureProducer,
    topic_prefix: String,
}

impl KafkaProducer {
    pub async fn new(config: KafkaConfig) -> Result<Self> {
        let mut client_config = ClientConfig::new();
        
        // 基础配置
        client_config
            .set("bootstrap.servers", config.brokers.join(","))
            .set("message.timeout.ms", "5000")
            .set("queue.buffering.max.messages", "1000000")
            .set("queue.buffering.max.kbytes", config.buffer_memory.unwrap_or(536870912).to_string())
            .set("batch.num.messages", config.batch_size.unwrap_or(1000).to_string())
            .set("linger.ms", "10")
            .set("compression.type", config.compression.as_deref().unwrap_or("lz4"))
            .set("max.in.flight.requests.per.connection", config.max_in_flight.unwrap_or(100).to_string())
            .set("message.send.max.retries", "10")
            .set("retry.backoff.ms", "200")
            .set("retry.backoff.max.ms", "10000");

        // SASL 认证配置
        if let Some(sasl) = &config.sasl {
            if sasl.enabled && !sasl.username.is_empty() {
                client_config
                    .set("security.protocol", "SASL_PLAINTEXT")
                    .set("sasl.mechanism", &sasl.mechanism)
                    .set("sasl.username", &sasl.username)
                    .set("sasl.password", &sasl.password);
                
                info!("Kafka SASL 认证已启用: mechanism={}", sasl.mechanism);
            }
        }

        let producer: FutureProducer = client_config
            .create()
            .context("创建 Kafka 生产者失败")?;

        info!("Kafka 生产者已创建: topic_prefix={}", config.topic);

        Ok(Self {
            producer,
            topic_prefix: config.topic,
        })
    }

    pub async fn write_batch(
        &self,
        agent_id: &str,
        events: Vec<crate::grpc::MonitorEvent>,
    ) -> Result<()> {
        for event in events {
            // 将 prost_types::Struct 转换为 serde_json::Value
            let event_data = if let Some(data) = event.data {
                struct_to_json_value(&data)
            } else {
                serde_json::Value::Null
            };

            let payload = json!({
                "agent_id": agent_id,
                "worker_name": event.worker_name,
                "timestamp": event.timestamp,
                "event_type": event.event_type,
                "data": event_data,
            });

            let payload_str = serde_json::to_string(&payload)?;
            
            // 根据 worker_name 确定 topic: {topic_prefix}-{worker_name}
            let topic = format!("{}-{}", self.topic_prefix, event.worker_name);
            
            let record = FutureRecord::to(&topic)
                .key(agent_id)
                .payload(&payload_str);

            if let Err((e, _)) = self.producer
                .send(record, std::time::Duration::from_secs(5))
                .await
            {
                error!("发送事件到 Kafka 失败: {}", e);
                return Err(anyhow::anyhow!("发送到 Kafka 失败: {}", e));
            }
        }

        Ok(())
    }

    pub async fn write_heartbeat(
        &self,
        agent_id: &str,
        hostname: &str,
        timestamp: i64,
        status: Option<prost_types::Struct>,
    ) -> Result<()> {
        // 将 prost_types::Struct 转换为 serde_json::Value
        let status_data = if let Some(s) = status {
            struct_to_json_value(&s)
        } else {
            serde_json::Value::Null
        };

        let payload = json!({
            "agent_id": agent_id,
            "hostname": hostname,
            "worker_name": "heartbeat",
            "timestamp": timestamp,
            "event_type": "heartbeat",
            "data": status_data,
        });

        let payload_str = serde_json::to_string(&payload)?;
        
        // 心跳数据使用 heartbeat topic
        let topic = format!("{}-heartbeat", self.topic_prefix);
        
        let record = FutureRecord::to(&topic)
            .key(agent_id)
            .payload(&payload_str);

        self.producer
            .send(record, std::time::Duration::from_secs(5))
            .await
            .map_err(|(e, _)| anyhow::anyhow!("发送心跳到 Kafka 失败: {}", e))?;

        Ok(())
    }
}
