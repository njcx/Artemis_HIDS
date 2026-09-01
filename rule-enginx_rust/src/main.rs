// ============================================
// HIDS 规则引擎主程序 (Rust 版本)
// 高性能事件处理和规则匹配
// ============================================

mod config;
mod engine;
mod kafka;
mod rule;
mod alert;
mod utils;

use anyhow::{Context, Result};
use clap::Parser;
use log::{error, info};
use mongodb::bson::doc;
use mongodb::options::ClientOptions;
use mongodb::Client;
use std::sync::Arc;
use std::time::Duration;
use tokio::signal;

use crate::config::Config;
use crate::engine::RuleEngine;
use crate::kafka::KafkaConsumer;
use crate::alert::AlertHandler;
use crate::rule::Rule;

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// 配置文件路径
    #[arg(short, long, default_value = "config.yaml")]
    config: String,
}

#[tokio::main]
async fn main() -> Result<()> {
    // 初始化日志
    env_logger::Builder::from_default_env()
        .filter_level(log::LevelFilter::Info)
        .init();

    let args = Args::parse();
    info!("启动 HIDS 规则引擎 (Rust 版本)");

    // 加载配置
    info!("加载配置文件: {}", args.config);
    let config = Config::from_file(&args.config)?;

    // 连接 MongoDB
    info!("连接 MongoDB: {}", config.mongodb.database);
    let mut client_options = tokio::time::timeout(
        Duration::from_secs(10),
        ClientOptions::parse(&config.mongodb.uri),
    )
    .await
    .context("连接 MongoDB 超时")?
    .context("解析 MongoDB URI 失败")?;
    client_options.max_pool_size = Some(config.mongodb.max_pool_size);
    client_options.min_pool_size = Some(config.mongodb.min_pool_size);
    client_options.max_idle_time = Some(Duration::from_secs(config.mongodb.max_conn_idle_time));

    let mongo_client = Client::with_options(client_options).context("创建 MongoDB 客户端失败")?;
    tokio::time::timeout(
        Duration::from_secs(10),
        mongo_client
            .database(&config.mongodb.database)
            .run_command(doc! {"ping": 1}),
    )
    .await
    .context("Ping MongoDB 超时")?
    .context("Ping MongoDB 失败")?;

    let rules_collection: mongodb::Collection<Rule> = mongo_client
        .database(&config.mongodb.database)
        .collection(&config.mongodb.collection);

    // 创建索引（幂等）
    rule::RuleLoader::ensure_indexes(&rules_collection).await?;

    // 加载规则
    info!("从 MongoDB 加载规则: {}.{}", config.mongodb.database, config.mongodb.collection);
    let rules = rule::RuleLoader::load_from_mongo(&rules_collection).await?;
    info!("加载 {} 条规则", rules.len());

    // 创建规则引擎
    let engine = Arc::new(RuleEngine::new(rules));

    // 创建告警处理器
    let alert_handler = Arc::new(AlertHandler::new(config.alert.clone()).await?);

    // 创建 Kafka 消费者
    let mut consumer = KafkaConsumer::new(
        config.consumer.clone(),
        engine.clone(),
        alert_handler.clone(),
    )?;

    // 启动规则热更新轮询任务
    let poll_engine = engine.clone();
    let poll_collection = rules_collection.clone();
    let poll_interval_secs = config.mongodb.poll_interval_secs;
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_secs(poll_interval_secs));
        interval.tick().await; // 首次立即 tick，跳过（已经加载过一次）
        loop {
            interval.tick().await;
            match rule::RuleLoader::load_from_mongo(&poll_collection).await {
                Ok(rules) => poll_engine.reload(rules),
                Err(e) => error!("规则热更新失败，保留当前规则集: {}", e),
            }
        }
    });

    // 获取取消令牌
    let cancel_token = consumer.get_cancel_token();

    let topic_info = if let Some(topics) = &config.consumer.topics {
        format!("{} topics", topics.len())
    } else if let Some(topic) = &config.consumer.topic {
        format!("topic: {}", topic)
    } else {
        "未知".to_string()
    };
    info!("开始消费 Kafka {}", topic_info);
    
    // 启动消费者
    let consumer_handle = tokio::spawn(async move {
        if let Err(e) = consumer.start().await {
            error!("Kafka 消费者错误: {}", e);
        }
    });

    // 等待中断信号
    match signal::ctrl_c().await {
        Ok(()) => {
            info!("收到中断信号 (Ctrl+C)，正在关闭...");
        }
        Err(err) => {
            error!("监听中断信号失败: {}", err);
        }
    }

    // 触发取消令牌，通知消费者停止
    cancel_token.cancel();
    
    // 等待消费者优雅关闭，最多等待 5 秒
    let shutdown_timeout = tokio::time::Duration::from_secs(5);
    match tokio::time::timeout(shutdown_timeout, consumer_handle).await {
        Ok(_) => {
            info!("消费者已优雅关闭");
        }
        Err(_) => {
            info!("消费者关闭超时，强制终止");
        }
    }
    
    info!("规则引擎已关闭");
    Ok(())
}
