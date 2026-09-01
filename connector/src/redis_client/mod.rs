// ============================================
// Redis 客户端模块 - Agent 心跳和状态管理
// ============================================

use anyhow::{Context, Result};
use log::info;
use redis::{aio::ConnectionManager, AsyncCommands};
use serde_json::{json, Value};

use crate::config::RedisConfig;

pub struct RedisClient {
    manager: ConnectionManager,
    config: RedisConfig,
}

impl RedisClient {
    pub async fn new(config: RedisConfig) -> Result<Self> {
        let client = redis::Client::open(config.url.as_str())
            .context("创建 Redis 客户端失败")?;

        let manager = ConnectionManager::new(client)
            .await
            .context("创建 Redis 连接管理器失败")?;

        info!("✓ Redis 客户端已创建: url={}", config.url);

        Ok(Self { manager, config })
    }

    /// 更新 Agent 心跳状态
    /// 
    /// 数据结构：
    /// - hids:agent:registered - Sorted Set: 所有注册过的 agent，score=注册时间（首次出现时间）
    /// - hids:agent:status:{agent_id} - Hash: agent详细状态
    ///   - status: "online" | "offline"
    ///   - last_update: 最后更新时间戳
    ///   - hostname: 主机名
    ///   - data: 原始心跳数据（JSON字符串）
    /// - hids:agent:stats:online - String: 在线总量
    /// - hids:agent:stats:offline - String: 离线总量
    pub async fn update_agent_status(&self, agent_id: &str, data: &Value) -> Result<()> {
        let mut conn = self.manager.clone();
        let now = chrono::Utc::now().timestamp();
        
        // Key 定义
        let registered_key = "hids:agent:registered";
        let status_key = format!("hids:agent:status:{}", agent_id);
        
        // 检查是否是首次注册
        let score_result: Result<Option<f64>, _> = conn.zscore(registered_key, agent_id).await;
        let is_new = score_result.is_err() || score_result.unwrap().is_none();
        
        if is_new {
            // 首次注册：添加到注册集合（注册时间不可变）
            conn.zadd::<_, _, _, ()>(registered_key, agent_id, now as f64)
                .await
                .context("添加到注册集合失败")?;
            info!("新 Agent 注册: {}", agent_id);
        }
        
        // 提取心跳数据中的字段
        let hostname = data["hostname"].as_str().unwrap_or("unknown");
        let raw_data = serde_json::to_string(data)?;
        
        // 更新 Agent 状态信息（Hash）
        conn.hset_multiple::<_, _, _, ()>(
            &status_key,
            &[
                ("status", "online"),
                ("last_update", &now.to_string()),
                ("hostname", hostname),
                ("data", &raw_data),
            ],
        )
        .await
        .context("更新 agent 状态失败")?;
        
        // 更新统计信息（通过定期任务或每次心跳后检查）
        self.update_agent_stats().await?;
        
        Ok(())
    }

    /// 更新 Agent 统计信息（扫描所有 agent，更新在线/离线状态和计数）
    pub async fn update_agent_stats(&self) -> Result<()> {
        let mut conn = self.manager.clone();
        let now = chrono::Utc::now().timestamp();
        let offline_threshold = now - 300; // 5分钟
        
        // 获取所有已注册的 agent
        let all_agents: Vec<String> = conn
            .zrange("hids:agent:registered", 0, -1)
            .await
            .context("获取注册列表失败")?;
        
        let mut online_count = 0;
        let mut offline_count = 0;
        
        // 检查每个 agent 的状态
        for agent_id in all_agents {
            let status_key = format!("hids:agent:status:{}", agent_id);
            
            // 获取最后更新时间
            let last_update: Option<String> = conn.hget(&status_key, "last_update").await.ok();
            
            match last_update {
                Some(timestamp_str) => {
                    if let Ok(last_ts) = timestamp_str.parse::<i64>() {
                        let is_online = last_ts > offline_threshold;
                        let new_status = if is_online { "online" } else { "offline" };
                        
                        // 更新状态字段
                        conn.hset::<_, _, _, ()>(&status_key, "status", new_status)
                            .await
                            .ok();
                        
                        if is_online {
                            online_count += 1;
                        } else {
                            offline_count += 1;
                        }
                    } else {
                        offline_count += 1;
                    }
                }
                None => {
                    // 从未更新过，标记为离线
                    conn.hset::<_, _, _, ()>(&status_key, "status", "offline")
                        .await
                        .ok();
                    offline_count += 1;
                }
            }
        }
        
        // 更新统计计数
        conn.set::<_, _, ()>("hids:agent:stats:online", online_count)
            .await
            .context("更新在线统计失败")?;
        
        conn.set::<_, _, ()>("hids:agent:stats:offline", offline_count)
            .await
            .context("更新离线统计失败")?;
        
        Ok(())
    }

    /// 获取 Agent 详细状态
    pub async fn get_agent_status(&self, agent_id: &str) -> Result<Option<Value>> {
        let mut conn = self.manager.clone();
        let status_key = format!("hids:agent:status:{}", agent_id);

        let data: Option<std::collections::HashMap<String, String>> = 
            conn.hgetall(&status_key).await?;

        match data {
            Some(hash) if !hash.is_empty() => {
                // 转换为 JSON
                let mut json_obj = serde_json::Map::new();
                for (k, v) in hash {
                    if k == "data" {
                        // data 字段是 JSON 字符串，需要解析
                        if let Ok(parsed) = serde_json::from_str::<Value>(&v) {
                            json_obj.insert(k, parsed);
                        } else {
                            json_obj.insert(k, Value::String(v));
                        }
                    } else if k == "last_update" {
                        // 数字字段
                        if let Ok(num) = v.parse::<i64>() {
                            json_obj.insert(k, Value::Number(num.into()));
                        } else {
                            json_obj.insert(k, Value::String(v));
                        }
                    } else {
                        json_obj.insert(k, Value::String(v));
                    }
                }
                Ok(Some(Value::Object(json_obj)))
            }
            _ => Ok(None),
        }
    }

    /// 获取所有 Agent 列表（包含状态）
    pub async fn get_all_agents(&self) -> Result<Vec<Value>> {
        let mut conn = self.manager.clone();

        // 获取所有已注册的 agent（按注册时间排序）
        let agents_with_scores: Vec<(String, f64)> = conn
            .zrange_withscores("hids:agent:registered", 0, -1)
            .await
            .context("获取注册列表失败")?;

        let mut result = Vec::new();
        
        for (agent_id, register_time) in agents_with_scores {
            let status_key = format!("hids:agent:status:{}", agent_id);
            
            // 获取状态信息
            let hash: Option<std::collections::HashMap<String, String>> = 
                conn.hgetall(&status_key).await.ok();
            
            let mut agent_info = serde_json::Map::new();
            agent_info.insert("agent_id".to_string(), Value::String(agent_id));
            agent_info.insert("register_time".to_string(), Value::Number((register_time as i64).into()));
            
            if let Some(status_data) = hash {
                for (k, v) in status_data {
                    if k == "last_update" {
                        if let Ok(num) = v.parse::<i64>() {
                            agent_info.insert(k, Value::Number(num.into()));
                        }
                    } else if k == "data" {
                        // 跳过原始数据，避免冗余
                        continue;
                    } else {
                        agent_info.insert(k, Value::String(v));
                    }
                }
            } else {
                agent_info.insert("status".to_string(), Value::String("offline".to_string()));
            }
            
            result.push(Value::Object(agent_info));
        }

        Ok(result)
    }

    /// 获取统计信息
    pub async fn get_stats(&self) -> Result<Value> {
        let mut conn = self.manager.clone();
        
        let online: i64 = conn.get("hids:agent:stats:online").await.unwrap_or(0);
        let offline: i64 = conn.get("hids:agent:stats:offline").await.unwrap_or(0);
        let total = online + offline;
        
        Ok(json!({
            "total": total,
            "online": online,
            "offline": offline
        }))
    }
}
