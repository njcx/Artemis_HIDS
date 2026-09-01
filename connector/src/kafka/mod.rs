// ============================================
// Kafka 消费者模块
// ============================================

use anyhow::{Context, Result};
use log::{error, info, warn};
use rdkafka::config::ClientConfig;
use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::message::Message;
use serde_json::Value;
use std::sync::Arc;
use std::time::Duration;
use tokio_util::sync::CancellationToken;

use crate::config::{KafkaConfig, TopicConfig};
use crate::openobserve::OpenObserveClient;
use crate::redis_client::RedisClient;

pub async fn consume_topic(
    kafka_config: KafkaConfig,
    topic_config: TopicConfig,
    openobserve: Arc<OpenObserveClient>,
    redis_client: Arc<RedisClient>,
    shutdown_token: CancellationToken,
) -> Result<()> {
    info!("启动消费者: topic={}, index={}", topic_config.name, topic_config.index);

    // 创建消费者
    let mut client_config = ClientConfig::new();
    client_config
        .set("bootstrap.servers", kafka_config.brokers.join(","))
        .set("group.id", &kafka_config.group_id)
        .set("enable.auto.commit", "true")
        .set("auto.offset.reset", "earliest")
        .set("session.timeout.ms", "6000");

    // SASL 认证
    if let Some(sasl) = &kafka_config.sasl {
        if sasl.enabled {
            client_config
                .set("security.protocol", "SASL_PLAINTEXT")
                .set("sasl.mechanism", &sasl.mechanism)
                .set("sasl.username", &sasl.username)
                .set("sasl.password", &sasl.password);
            info!(" Kafka SASL 认证已启用");
        }
    }

    let consumer: StreamConsumer = client_config
        .create()
        .context("创建 Kafka 消费者失败")?;

    consumer
        .subscribe(&[&topic_config.name])
        .context("订阅 topic 失败")?;

    info!("已订阅 topic: {}", topic_config.name);

    // 批量缓冲
    let mut batch = Vec::new();
    let batch_size = 100;
    let flush_interval = Duration::from_secs(5);
    let mut last_flush = tokio::time::Instant::now();

    loop {
        // 检查是否收到退出信号
        if shutdown_token.is_cancelled() {
            info!("收到退出信号，topic={} 正在退出", topic_config.name);
            
            // 刷新剩余数据
            if !batch.is_empty() {
                if let Err(e) = openobserve.write_batch(&topic_config.index, &batch).await {
                    error!("写入 OpenObserve 失败: {}", e);
                } else {
                    info!("最终写入 {} 条数据到索引: {}", batch.len(), topic_config.index);
                }
            }
            break;
        }

        // 使用超时接收消息（1秒超时，避免阻塞）
        let recv_result = tokio::time::timeout(
            Duration::from_secs(1),
            consumer.recv()
        ).await;

        match recv_result {
            Ok(Ok(message)) => {
                if let Some(payload) = message.payload() {
                    match serde_json::from_slice::<Value>(payload) {
                        Ok(mut data) => {
                            // 添加额外字段
                            if let Some(obj) = data.as_object_mut() {
                                obj.insert("_timestamp".to_string(), Value::Number(
                                    chrono::Utc::now().timestamp_millis().into()
                                ));
                            }

                            // 如果是心跳数据，写入 Redis
                            if topic_config.is_heartbeat {
                                if let Err(e) = process_heartbeat(&data, &redis_client).await {
                                    error!("处理心跳失败: {}", e);
                                }
                            }

                            // 添加到批量缓冲
                            batch.push(data);

                            // 批量写入 OpenObserve
                            if batch.len() >= batch_size || last_flush.elapsed() >= flush_interval {
                                if let Err(e) = openobserve.write_batch(&topic_config.index, &batch).await {
                                    error!("写入 OpenObserve 失败: {}", e);
                                } else {
                                    info!("写入 {} 条数据到索引: {}", batch.len(), topic_config.index);
                                }
                                batch.clear();
                                last_flush = tokio::time::Instant::now();
                            }
                        }
                        Err(e) => {
                            warn!("解析消息失败: {}", e);
                        }
                    }
                }
            }
            Ok(Err(e)) => {
                error!("接收消息失败: {}", e);
                tokio::time::sleep(Duration::from_secs(1)).await;
            }
            Err(_) => {
                // 超时，继续循环（会检查退出信号）
                continue;
            }
        }
    }
    
    Ok(())
}

async fn process_heartbeat(data: &Value, redis_client: &RedisClient) -> Result<()> {
    // 提取 agent_id 和状态信息
    let agent_id = data["agent_id"]
        .as_str()
        .ok_or_else(|| anyhow::anyhow!("缺少 agent_id"))?;

    let hostname = data["hostname"].as_str().unwrap_or("");
    let timestamp = data["timestamp"].as_i64().unwrap_or(0);
    let status_data = data["data"].clone();

    // 构建 Redis 数据
    let redis_data = serde_json::json!({
        "agent_id": agent_id,
        "hostname": hostname,
        "last_heartbeat": timestamp,
        "status": status_data,
        "updated_at": chrono::Utc::now().timestamp(),
    });

    // 写入 Redis
    redis_client.update_agent_status(agent_id, &redis_data).await?;
    
    Ok(())
}
