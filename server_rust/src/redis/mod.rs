// ============================================
// Redis 客户端模块
// ============================================

use anyhow::{Context, Result};
use log::info;
use redis::aio::ConnectionManager;
use redis::AsyncCommands;

use crate::config::RedisConfig;

pub struct RedisClient {
    conn: ConnectionManager,
}

impl RedisClient {
    pub async fn new(config: RedisConfig) -> Result<Self> {
        let redis_url = if config.password.is_empty() {
            format!("redis://{}:{}", config.host, config.port)
        } else {
            format!("redis://:{}@{}:{}", config.password, config.host, config.port)
        };

        let client = redis::Client::open(redis_url)
            .context("创建 Redis 客户端失败")?;
        
        let conn = ConnectionManager::new(client).await
            .context("连接 Redis 失败")?;

        info!("Redis 客户端已连接: {}:{}", config.host, config.port);

        Ok(Self { conn })
    }

    pub async fn get_commands(&self, agent_id: &str, limit: isize) -> Result<Vec<String>> {
        let key = format!("agent:{}:commands", agent_id);
        let mut conn = self.conn.clone();
        
        // 从 Redis 列表中获取命令
        let commands: Vec<String> = conn.lrange(&key, 0, limit - 1).await
            .context("从 Redis 获取命令失败")?;
        
        // 删除已获取的命令
        if !commands.is_empty() {
            let _: () = conn.ltrim(&key, commands.len() as isize, -1).await
                .context("删除已获取的命令失败")?;
        }
        
        Ok(commands)
    }

    pub async fn save_command_result(
        &self,
        agent_id: &str,
        command_id: &str,
        success: bool,
        message: &str,
    ) -> Result<()> {
        let key = format!("agent:{}:command_results", agent_id);
        let value = serde_json::json!({
            "command_id": command_id,
            "success": success,
            "message": message,
            "timestamp": chrono::Utc::now().timestamp(),
        });

        let mut conn = self.conn.clone();
        let _: () = conn.rpush(&key, value.to_string()).await
            .context("保存命令结果失败")?;
        
        // 设置过期时间 24 小时
        let _: () = conn.expire(&key, 86400).await
            .context("设置过期时间失败")?;
        
        Ok(())
    }
}
