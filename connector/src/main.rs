// ============================================
// HIDS Connector - Kafka to OpenObserve & Redis
// ============================================

mod config;
mod kafka;
mod openobserve;
mod redis_client;

use anyhow::Result;
use log::info;
use std::sync::Arc;
use std::time::Duration;
use tokio::signal;
use tokio_util::sync::CancellationToken;

#[tokio::main]
async fn main() -> Result<()> {
    // 初始化日志
    env_logger::init();

    info!(" HIDS Connector 启动中...");

    // 加载配置
    let config = config::Config::load("config.yaml").await?;
    info!("配置文件加载成功");

    // 初始化 OpenObserve 客户端
    let openobserve = Arc::new(
        openobserve::OpenObserveClient::new(config.openobserve.clone()).await?
    );
    info!("✓ OpenObserve 客户端已初始化");

    // 初始化 Redis 客户端
    let redis_client = Arc::new(
        redis_client::RedisClient::new(config.redis.clone()).await?
    );
    info!("✓ Redis 客户端已初始化");

    // 创建退出令牌
    let shutdown_token = CancellationToken::new();
    
    // 为每个 topic 启动消费者
    let mut handles = Vec::new();
    
    for topic_config in &config.kafka.topics {
        let openobserve_clone = Arc::clone(&openobserve);
        let redis_clone = Arc::clone(&redis_client);
        let kafka_config = config.kafka.clone();
        let topic_config_clone = topic_config.clone();
        let token_clone = shutdown_token.clone();
        
        let handle = tokio::spawn(async move {
            kafka::consume_topic(
                kafka_config,
                topic_config_clone,
                openobserve_clone,
                redis_clone,
                token_clone,
            )
            .await
        });
        
        handles.push(handle);
    }

    info!("✓ 所有消费者已启动");
    info!("等待数据... (按 Ctrl+C 退出)");

    // 等待 Ctrl+C
    signal::ctrl_c().await?;
    info!("收到退出信号，正在关闭...");
    
    // 发送退出信号
    shutdown_token.cancel();

    // 等待所有任务完成（最多等待 5 秒）
    let timeout = Duration::from_secs(5);
    for handle in handles {
        match tokio::time::timeout(timeout, handle).await {
            Ok(_) => {},
            Err(_) => info!("任务超时，强制退出"),
        }
    }

    info!("HIDS Connector 已退出");
    Ok(())
}
