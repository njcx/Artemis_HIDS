// ============================================
// 事件定义
// ============================================

use serde::{Deserialize, Serialize, Deserializer};
use std::collections::HashMap;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Event {
    pub agent_id: String,
    pub worker_name: String,
    #[serde(deserialize_with = "deserialize_timestamp")]
    pub timestamp: i64,
    pub event_type: String,
    #[serde(default)]
    pub data: serde_json::Value,
}

// 自定义反序列化函数，支持字符串和数字格式的时间戳
fn deserialize_timestamp<'de, D>(deserializer: D) -> Result<i64, D::Error>
where
    D: Deserializer<'de>,
{
    use serde::de::Error;
    
    let value: serde_json::Value = Deserialize::deserialize(deserializer)?;
    
    match value {
        serde_json::Value::Number(n) => {
            n.as_i64().ok_or_else(|| Error::custom("timestamp 不是有效的 i64"))
        }
        serde_json::Value::String(s) => {
            // 尝试解析字符串格式的时间戳（纳秒）
            s.parse::<i64>()
                .map(|ns| ns / 1_000_000_000) // 转换为秒
                .map_err(|_| Error::custom("无法解析时间戳字符串"))
        }
        _ => Err(Error::custom("timestamp 必须是数字或字符串")),
    }
}

impl Event {
    pub fn new(
        agent_id: String,
        worker_name: String,
        event_type: String,
        data: serde_json::Value,
    ) -> Self {
        Self {
            agent_id,
            worker_name,
            timestamp: chrono::Utc::now().timestamp(),
            event_type,
            data,
        }
    }
    
    /// 从 data 中获取字符串字段
    pub fn get_data_string(&self, key: &str) -> Option<String> {
        self.data.get(key).and_then(|v| match v {
            serde_json::Value::String(s) => Some(s.clone()),
            serde_json::Value::Number(n) => Some(n.to_string()),
            serde_json::Value::Bool(b) => Some(b.to_string()),
            _ => None,
        })
    }
    
    /// 从 data 中获取任意类型的值
    pub fn get_data_value(&self, key: &str) -> Option<&serde_json::Value> {
        self.data.get(key)
    }
}
