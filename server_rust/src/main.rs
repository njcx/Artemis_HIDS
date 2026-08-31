// ============================================
// HIDS Server 主程序 (Rust 版本)
// 高性能 gRPC 服务器，接收 Agent 数据并写入 Kafka
// ============================================

mod config;
mod grpc;
mod kafka;
mod redis;
mod stats;

use anyhow::Result;
use clap::Parser;
use log::{error, info, warn};
use std::sync::Arc;
use tokio::signal;
use tonic::transport::{Server, Identity, Certificate, ServerTlsConfig};

use crate::config::Config;
use crate::grpc::MonitorServiceImpl;
use crate::kafka::KafkaProducer;
use crate::redis::RedisClient;
use crate::stats::ServerStats;

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// 配置文件路径
    #[arg(short, long, default_value = "config.yaml")]
    config: String,
}

#[tokio::main]
async fn main() -> Result<()> {
    // 初始化 Rustls CryptoProvider (必须在使用 TLS 之前)
    rustls::crypto::ring::default_provider()
        .install_default()
        .expect("Failed to install rustls crypto provider");
    
    // 初始化日志
    env_logger::Builder::from_default_env()
        .filter_level(log::LevelFilter::Info)
        .init();

    let args = Args::parse();
    info!("启动 HIDS Server (Rust 版本)");

    // 加载配置
    info!("加载配置文件: {}", args.config);
    let config = Config::from_file(&args.config)?;
    
    // 创建 Kafka 生产者
    info!("初始化 Kafka 生产者");
    let kafka_producer = Arc::new(KafkaProducer::new(config.kafka.clone()).await?);
    
    // 创建 Redis 客户端
    let redis_client = if !config.redis.host.is_empty() {
        info!("初始化 Redis 客户端");
        Some(Arc::new(RedisClient::new(config.redis.clone()).await?))
    } else {
        info!("⊘ Redis 未配置");
        None
    };
    
    // 创建统计信息
    let stats = Arc::new(ServerStats::new());
    
    // 启动统计信息输出
    let stats_clone = stats.clone();
    tokio::spawn(async move {
        stats_clone.print_stats_loop().await;
    });
    
    // 创建 gRPC 服务
    let service = MonitorServiceImpl::new(
        kafka_producer,
        redis_client,
        stats.clone(),
    );
    
    let addr = format!("{}:{}", config.server.host, config.server.port).parse()?;
    info!("gRPC 服务监听: {}", addr);
    
    // 构建 gRPC 服务器
    let mut server_builder = Server::builder()
        .concurrency_limit_per_connection(config.server.max_concurrent_streams as usize);
    
    // 配置 TLS
    if let Some(tls_config) = &config.server.tls {
        if tls_config.enabled {
            info!("TLS 已启用");
            
            // 加载服务器证书和密钥
            let cert = tokio::fs::read(&tls_config.cert_file).await?;
            let key = tokio::fs::read(&tls_config.key_file).await?;
            let identity = Identity::from_pem(cert, key);
            
            let mut tls = ServerTlsConfig::new().identity(identity);
            
            // 如果需要客户端证书验证（双向TLS）
            if tls_config.client_auth {
                if let Some(ca_file) = &tls_config.ca_file {
                    info!("双向 TLS 已启用（需要客户端证书）");
                    let ca_cert = tokio::fs::read(ca_file).await?;
                    let ca_cert = Certificate::from_pem(ca_cert);
                    tls = tls.client_ca_root(ca_cert);
                } else {
                    warn!("Warning: client_auth 启用但未配置 ca_file，使用单向 TLS");
                }
            } else {
                info!("单向 TLS 已启用（不需要客户端证书）");
            }
            
            server_builder = server_builder.tls_config(tls)?;
        } else {
            warn!("Warning: TLS 已禁用，连接不安全！");
        }
    } else {
        warn!("Warning: 未配置 TLS，连接不安全！");
    }
    
    // 启动 gRPC 服务器
    let server = server_builder
        .add_service(grpc::monitor_service_server::MonitorServiceServer::new(service))
        .serve_with_shutdown(addr, async {
            match signal::ctrl_c().await {
                Ok(()) => {
                    info!("收到中断信号，正在关闭...");
                }
                Err(err) => {
                    error!("监听中断信号失败: {}", err);
                }
            }
        });

    info!("HIDS Server 已启动");
    
    if let Err(e) = server.await {
        error!("服务器错误: {}", e);
    }

    info!("👋 HIDS Server 已关闭");
    Ok(())
}
