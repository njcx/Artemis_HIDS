// ============================================
// gRPC 服务实现
// ============================================

use tonic::{Request, Response, Status};
use log::{info, warn, error};
use std::sync::Arc;

use crate::kafka::KafkaProducer;
use crate::redis::RedisClient;
use crate::stats::ServerStats;

// 包含生成的 protobuf 代码
pub mod hids {
    tonic::include_proto!("hids");
}

pub use hids::monitor_service_server;
pub use hids::*;

pub struct MonitorServiceImpl {
    kafka_producer: Arc<KafkaProducer>,
    redis_client: Option<Arc<RedisClient>>,
    stats: Arc<ServerStats>,
}

impl MonitorServiceImpl {
    pub fn new(
        kafka_producer: Arc<KafkaProducer>,
        redis_client: Option<Arc<RedisClient>>,
        stats: Arc<ServerStats>,
    ) -> Self {
        Self {
            kafka_producer,
            redis_client,
            stats,
        }
    }
}

#[tonic::async_trait]
impl monitor_service_server::MonitorService for MonitorServiceImpl {
    async fn send_batch(
        &self,
        request: Request<BatchRequest>,
    ) -> Result<Response<BatchResponse>, Status> {
        self.stats.increment_requests();
        
        let req = request.into_inner();
        self.stats.update_agent(req.agent_id.clone());

        info!(
            "Received batch from agent {} (hostname: {}): {} events",
            req.agent_id,
            req.hostname,
            req.events.len()
        );

        // 验证请求
        if req.agent_id.is_empty() {
            self.stats.increment_failures();
            return Err(Status::invalid_argument("agent_id is required"));
        }

        if req.events.is_empty() {
            return Ok(Response::new(BatchResponse {
                success: true,
                message: "no events to process".to_string(),
                accepted_count: 0,
            }));
        }

        // 写入 Kafka
        let event_count = req.events.len() as i64;
        match self.kafka_producer.write_batch(&req.agent_id, req.events).await {
            Ok(_) => {
                self.stats.increment_events(event_count);
                Ok(Response::new(BatchResponse {
                    success: true,
                    message: "batch processed successfully".to_string(),
                    accepted_count: event_count as i32,
                }))
            }
            Err(e) => {
                self.stats.increment_failures();
                error!("Failed to write batch to Kafka: {}", e);
                Err(Status::internal(format!("failed to write to kafka: {}", e)))
            }
        }
    }

    async fn send_heartbeat(
        &self,
        request: Request<HeartbeatRequest>,
    ) -> Result<Response<HeartbeatResponse>, Status> {
        self.stats.increment_heartbeats();
        
        let req = request.into_inner();
        self.stats.update_agent(req.agent_id.clone());

        info!(
            "Received heartbeat from agent {} (hostname: {})",
            req.agent_id, req.hostname
        );

        // 写入 Kafka
        if let Err(e) = self
            .kafka_producer
            .write_heartbeat(&req.agent_id, &req.hostname, req.timestamp, req.status)
            .await
        {
            error!("Failed to write heartbeat to Kafka: {}", e);
            return Err(Status::internal(format!("failed to write heartbeat: {}", e)));
        }

        // 从 Redis 获取待执行的命令
        let mut commands = Vec::new();
        if let Some(ref redis_client) = self.redis_client {
            match redis_client.get_commands(&req.agent_id, 10).await {
                Ok(cmd_list) => {
                    for cmd_data in cmd_list {
                        match serde_json::from_str::<serde_json::Value>(&cmd_data) {
                            Ok(cmd_json) => {
                                let command = Command {
                                    command_id: cmd_json["command_id"].as_str().unwrap_or("").to_string(),
                                    command_type: cmd_json["command_type"].as_str().unwrap_or("").to_string(),
                                    payload: cmd_json["payload"]
                                        .as_object()
                                        .map(|obj| {
                                            obj.iter()
                                                .map(|(k, v)| (k.clone(), v.as_str().unwrap_or("").to_string()))
                                                .collect()
                                        })
                                        .unwrap_or_default(),
                                    timestamp: cmd_json["timestamp"].as_i64().unwrap_or(0),
                                };
                                commands.push(command);
                            }
                            Err(e) => {
                                warn!("Failed to parse command JSON: {}", e);
                            }
                        }
                    }
                }
                Err(e) => {
                    warn!("Failed to get commands from Redis: {}", e);
                }
            }
        }

        Ok(Response::new(HeartbeatResponse {
            success: true,
            message: "heartbeat received".to_string(),
            commands,
        }))
    }

    async fn report_command_result(
        &self,
        request: Request<CommandResult>,
    ) -> Result<Response<CommandResultAck>, Status> {
        let result = request.into_inner();

        info!(
            "Received command result from agent {}: command_id={}, success={}",
            result.agent_id, result.command_id, result.success
        );

        // 保存到 Redis
        if let Some(ref redis_client) = self.redis_client {
            if let Err(e) = redis_client
                .save_command_result(
                    &result.agent_id,
                    &result.command_id,
                    result.success,
                    &result.message,
                )
                .await
            {
                warn!("Failed to save command result to Redis: {}", e);
            }
        }

        Ok(Response::new(CommandResultAck { received: true }))
    }
}
