// ============================================
// Kafka 消费者实现 - 高性能版本
// 支持多线程并行处理，充分利用 CPU
// ============================================

use anyhow::{Context, Result};
use rdkafka::config::ClientConfig;
use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::message::Message;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use log::{info, warn, error};
use tokio_util::sync::CancellationToken;

use crate::config::ConsumerConfig;
use crate::engine::{Event, RuleEngine};
use crate::alert::AlertHandler;

/// 默认 worker 数量（处理线程数）
const DEFAULT_WORKERS: usize = 8;
/// 默认 channel 缓冲区大小
const DEFAULT_CHANNEL_SIZE: usize = 10000;
/// 批量提交的消息数量
const COMMIT_BATCH_SIZE: u64 = 1000;

pub struct KafkaConsumer {
    consumer: StreamConsumer,
    engine: Arc<RuleEngine>,
    alert_handler: Arc<AlertHandler>,
    cancel_token: CancellationToken,
    num_workers: usize,
    channel_size: usize,
}

impl KafkaConsumer {
    /// 解析 topic 配置，返回要订阅的 topic 列表
    fn resolve_topics(config: &ConsumerConfig) -> Result<Vec<String>> {
        // 优先使用 topics 配置
        if let Some(topics) = &config.topics {
            if !topics.is_empty() {
                return Ok(topics.clone());
            }
        }
        
        // 使用 topic 配置
        if let Some(topic) = &config.topic {
            // 如果是通配符模式（如 hids_events-*），展开为具体的 topics
            if topic.contains('*') {
                let prefix = topic.trim_end_matches('*');
                // 标准的 worker topics
                let workers = vec![
                    "file_monitor",
                    "exec_monitor",
                    "network_monitor",
                    "security_log_monitor",
                ];
                let topics: Vec<String> = workers
                    .iter()
                    .map(|w| format!("{}{}", prefix, w))
                    .collect();
                return Ok(topics);
            } else {
                // 单个 topic
                return Ok(vec![topic.clone()]);
            }
        }
        
        Err(anyhow::anyhow!("配置中必须指定 topic 或 topics 字段"))
    }

    pub fn new(
        config: ConsumerConfig,
        engine: Arc<RuleEngine>,
        alert_handler: Arc<AlertHandler>,
    ) -> Result<Self> {
        let mut client_config = ClientConfig::new();
        
        // 高性能 Kafka 配置
        let offset_reset = &config.auto_offset_reset;
        client_config
            .set("group.id", &config.group_id)
            .set("bootstrap.servers", config.brokers.join(","))
            // 自动提交配置
            .set("enable.auto.commit", "true")
            .set("auto.commit.interval.ms", "5000")  // 5秒提交一次
            // 会话配置
            .set("session.timeout.ms", "30000")
            .set("heartbeat.interval.ms", "3000")
            .set("enable.partition.eof", "false")
            .set("auto.offset.reset", offset_reset)
            // 高性能配置 - 增加预取和批量大小
            .set("fetch.min.bytes", "1")  // 最小获取字节数
            .set("fetch.max.bytes", "52428800")  // 50MB 最大获取
            .set("max.partition.fetch.bytes", "10485760")  // 10MB 每分区
            .set("fetch.wait.max.ms", "100")  // 最大等待时间 100ms
            .set("queued.min.messages", "100000")  // 预取队列最小消息数
            .set("queued.max.messages.kbytes", "1048576");  // 1GB 预取队列大小
        
        let offset_desc = if offset_reset == "earliest" {
            "从头消费所有消息"
        } else {
            "只消费新消息"
        };
        
        // 获取 worker 数量配置
        let num_workers = config.num_workers.unwrap_or(DEFAULT_WORKERS);
        let channel_size = config.channel_size.unwrap_or(DEFAULT_CHANNEL_SIZE);
        
        info!("Kafka consumer 高性能配置:");
        info!("  - auto.offset.reset={} ({})", offset_reset, offset_desc);
        info!("  - 处理线程数: {}", num_workers);
        info!("  - Channel 缓冲区: {}", channel_size);
        info!("  - 预取队列: 100000 消息 / 1GB");

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

        let consumer: StreamConsumer = client_config
            .create()
            .context("创建 Kafka 消费者失败")?;

        // 确定要订阅的 topics
        let topics_to_subscribe = Self::resolve_topics(&config)?;
        
        // 订阅 topics
        let topic_refs: Vec<&str> = topics_to_subscribe.iter().map(|s| s.as_str()).collect();
        consumer
            .subscribe(&topic_refs)
            .context("订阅 topics 失败")?;

        info!("Kafka 消费者已创建: topics={:?}, group={}", topics_to_subscribe, config.group_id);

        Ok(Self {
            consumer,
            engine,
            alert_handler,
            cancel_token: CancellationToken::new(),
            num_workers,
            channel_size,
        })
    }

    /// 获取取消令牌的克隆，用于外部触发关闭
    pub fn get_cancel_token(&self) -> CancellationToken {
        self.cancel_token.clone()
    }

    /// 高性能启动 - 多线程并行处理
    pub async fn start(&mut self) -> Result<()> {
        use tokio::time::{Duration, interval};

        // 创建 flume channel (MPMC - 多生产者多消费者)
        let (tx, rx) = flume::bounded::<Vec<u8>>(self.channel_size);
        
        // 统计计数器
        let consumed_count = Arc::new(AtomicU64::new(0));
        let processed_count = Arc::new(AtomicU64::new(0));
        
        // 启动多个 worker 处理线程
        let mut worker_handles = Vec::new();
        for worker_id in 0..self.num_workers {
            let engine = self.engine.clone();
            let alert_handler = self.alert_handler.clone();
            let cancel_token = self.cancel_token.clone();
            let processed = processed_count.clone();
            let rx = rx.clone();
            
            let handle = tokio::spawn(async move {
                Self::worker_loop(worker_id, rx, engine, alert_handler, cancel_token, processed).await;
            });
            worker_handles.push(handle);
        }
        
        info!("启动 {} 个处理 worker", self.num_workers);

        // 统计定时器
        let mut stats_interval = interval(Duration::from_secs(60));
        let mut last_consumed = 0u64;
        let mut last_processed = 0u64;

        info!("Kafka 消费者开始运行 (高性能模式)...");

        loop {
            tokio::select! {
                biased;  // 优先处理取消信号
                
                // 检查取消信号
                _ = self.cancel_token.cancelled() => {
                    info!("收到取消信号，停止消费...");
                    break;
                }

                // 消费消息
                message_result = self.consumer.recv() => {
                    match message_result {
                        Ok(message) => {
                            if let Some(payload) = message.payload() {
                                // 发送到 channel，由 worker 处理
                                if tx.send_async(payload.to_vec()).await.is_err() {
                                    warn!("Channel 已关闭，停止消费");
                                    break;
                                }
                                
                                let count = consumed_count.fetch_add(1, Ordering::Relaxed);
                                
                                // 批量提交 offset
                                if count % COMMIT_BATCH_SIZE == 0 {
                                    if let Err(e) = self.consumer.commit_message(&message, rdkafka::consumer::CommitMode::Async) {
                                        warn!("提交偏移量失败: {}", e);
                                    }
                                }
                            }
                        }
                        Err(e) => {
                            error!("接收消息失败: {}", e);
                            tokio::time::sleep(Duration::from_millis(10)).await;
                        }
                    }
                }

                // 定期输出统计
                _ = stats_interval.tick() => {
                    let current_consumed = consumed_count.load(Ordering::Relaxed);
                    let current_processed = processed_count.load(Ordering::Relaxed);
                    let stats = self.engine.get_stats();
                    
                    let consume_rate = current_consumed - last_consumed;
                    let process_rate = current_processed - last_processed;
                    let pending = current_consumed.saturating_sub(current_processed);
                    
                    info!("统计: 消费={}/min, 处理={}/min, 待处理={}, 总处理={}, 匹配={}", 
                        consume_rate,
                        process_rate,
                        pending,
                        stats.get_processed(),
                        stats.get_matched()
                    );
                    
                    last_consumed = current_consumed;
                    last_processed = current_processed;
                }
            }
        }

        // 关闭 channel，通知 worker 退出
        drop(tx);
        
        // 等待所有 worker 完成
        for handle in worker_handles {
            let _ = handle.await;
        }

        info!("Kafka 消费者已停止");
        Ok(())
    }

    /// Worker 处理循环
    async fn worker_loop(
        worker_id: usize,
        rx: flume::Receiver<Vec<u8>>,
        engine: Arc<RuleEngine>,
        alert_handler: Arc<AlertHandler>,
        cancel_token: CancellationToken,
        processed_count: Arc<AtomicU64>,
    ) {
        info!("Worker {} 启动", worker_id);
        
        loop {
            tokio::select! {
                biased;
                
                _ = cancel_token.cancelled() => {
                    info!("Worker {} 收到取消信号", worker_id);
                    break;
                }
                
                msg = rx.recv_async() => {
                    match msg {
                        Ok(payload) => {
                            // 解析事件
                            match serde_json::from_slice::<Event>(&payload) {
                                Ok(event) => {
                                    // 规则匹配
                                    let results = engine.process_event(&event);
                                    
                                    // 处理匹配结果
                                    for result in results {
                                        if let Err(e) = alert_handler.handle_match(result).await {
                                            error!("Worker {} 处理告警失败: {}", worker_id, e);
                                        }
                                    }
                                    
                                    processed_count.fetch_add(1, Ordering::Relaxed);
                                }
                                Err(e) => {
                                    warn!("Worker {} 解析事件失败: {}", worker_id, e);
                                }
                            }
                        }
                        Err(_) => {
                            info!("Worker {} Channel 已关闭", worker_id);
                            break;
                        }
                    }
                }
            }
        }
        
        info!("Worker {} 退出", worker_id);
    }
}
