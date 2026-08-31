// ============================================
// 告警数据结构
// ============================================

use serde::{Deserialize, Serialize};
use chrono::{DateTime, Utc};

use crate::rule::Severity;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Alert {
    pub alert_id: String,
    pub rule_id: String,
    pub rule_name: String,
    pub severity: Severity,
    pub description: String,
    pub agent_id: String,
    pub worker_name: String,
    pub event_type: String,
    pub timestamp: i64,
    pub created_at: DateTime<Utc>,
    pub data: serde_json::Value,
    pub context: serde_json::Value,
    #[serde(default)]
    pub tags: Vec<String>,
}

impl Alert {
    pub fn new(
        rule_id: String,
        rule_name: String,
        severity: Severity,
        description: String,
        agent_id: String,
        worker_name: String,
        event_type: String,
        timestamp: i64,
        data: serde_json::Value,
        context: serde_json::Value,
        tags: Vec<String>,
    ) -> Self {
        use uuid::Uuid;
        
        Self {
            alert_id: Uuid::new_v4().to_string(),
            rule_id,
            rule_name,
            severity,
            description,
            agent_id,
            worker_name,
            event_type,
            timestamp,
            created_at: Utc::now(),
            data,
            context,
            tags,
        }
    }
}
