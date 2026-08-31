// ============================================
// 规则类型定义
// ============================================

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Rule {
    pub id: String,
    pub name: String,
    pub description: String,
    pub severity: Severity,
    pub enabled: bool,
    #[serde(rename = "type")]
    pub rule_type: RuleType,
    #[serde(default)]
    pub tags: Vec<String>,
    pub conditions: RuleConditions,
    #[serde(default)]
    pub threshold: Option<ThresholdConfig>,
    #[serde(default)]
    pub correlation: Option<CorrelationConfig>,
    pub actions: Vec<Action>,
    pub author: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum Severity {
    Critical,
    High,
    Medium,
    Low,
    Info,
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum RuleType {
    Pattern,
    Threshold,
    Correlation,
    Statistical,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct RuleConditions {
    #[serde(default)]
    pub worker_name: Option<String>,
    #[serde(default)]
    pub event_type: Option<String>,
    #[serde(default)]
    pub fields: HashMap<String, String>,
    #[serde(default)]
    pub patterns: HashMap<String, String>,
    #[serde(default)]
    pub expression: Option<String>,
    #[serde(default)]
    pub exclude: Option<ExcludeCondition>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ExcludeCondition {
    #[serde(default)]
    pub fields: HashMap<String, String>,
    #[serde(default)]
    pub patterns: HashMap<String, String>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ThresholdConfig {
    pub field: String,
    pub operator: String, // >, <, >=, <=, ==, !=
    pub value: f64,
    pub time_window: u64, // 秒
    #[serde(default)]
    pub group_by: Vec<String>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct CorrelationConfig {
    pub events: Vec<String>,
    pub time_window: u64, // 秒
    pub group_by: Vec<String>,
    pub min_count: usize,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Action {
    #[serde(rename = "type")]
    pub action_type: String,
    #[serde(default)]
    pub config: HashMap<String, serde_json::Value>,
}
