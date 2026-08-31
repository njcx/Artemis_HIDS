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

use anyhow::Result;
use clap::Parser;
use log::{error, info};
use std::sync::Arc;
use tokio::signal;

use crate::config::Config;
use crate::engine::RuleEngine;
use crate::kafka::KafkaConsumer;
use crate::alert::AlertHandler;

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
    
    // 加载规则
    info!("加载规则目录: {}", config.rules.directory);
    let rules = rule::RuleLoader::load_from_directory(&config.rules.directory)?;
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
