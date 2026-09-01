// ============================================
// 规则匹配器
// ============================================

use regex::Regex;
use evalexpr::{ContextWithMutableVariables, HashMapContext};

use super::types::Rule;
use crate::engine::Event;

pub struct RuleMatcher;

impl RuleMatcher {
    /// 检查事件是否匹配规则
    pub fn matches(rule: &Rule, event: &Event) -> bool {
        let cond = &rule.conditions;

        // 1. 检查 worker_name
        if let Some(ref worker_name) = cond.worker_name {
            if worker_name != &event.worker_name {
                log::trace!("规则 {} worker_name 不匹配: 期望={}, 实际={}",
                    rule.id, worker_name, event.worker_name);
                return false;
            }
        }

        // 2. 检查 event_type
        if let Some(ref event_type) = cond.event_type {
            if event_type != &event.event_type {
                log::trace!("规则 {} event_type 不匹配: 期望={}, 实际={}",
                    rule.id, event_type, event.event_type);
                return false;
            }
        }

        // 3. 检查字段精确匹配
        for (field, value) in &cond.fields {
            match event.get_data_string(field) {
                Some(event_value) if event_value == *value => continue,
                Some(event_value) => {
                    log::trace!("规则 {} 字段 {} 不匹配: 期望={}, 实际={}",
                        rule.id, field, value, event_value);
                    return false;
                }
                None => {
                    log::trace!("规则 {} 字段 {} 不存在", rule.id, field);
                    return false;
                }
            }
        }

        // 4. 检查正则匹配
        for (field, pattern) in &cond.patterns {
            match event.get_data_string(field) {
                Some(event_value) => {
                    if let Ok(re) = Regex::new(pattern) {
                        if !re.is_match(&event_value) {
                            log::debug!("规则 {} 正则不匹配: 字段={}, 值={}, 模式={}",
                                rule.id, field, event_value, pattern);
                            return false;
                        }
                        log::debug!("规则 {} 正则匹配成功: 字段={}, 值={}",
                            rule.id, field, event_value);
                    } else {
                        log::warn!("无效的正则表达式: {}", pattern);
                        return false;
                    }
                }
                None => {
                    log::debug!("规则 {} 字段 {} 不存在于事件数据中, data={:?}",
                        rule.id, field, event.data);
                    return false;
                }
            }
        }

        // 5. 检查表达式（如果有）
        if let Some(ref expr) = cond.expression {
            if !Self::eval_expression(expr, event) {
                return false;
            }
        }

        // 6. 检查排除条件
        if let Some(ref exclude) = cond.exclude {
            if Self::matches_exclude(exclude, event) {
                return false;
            }
        }

        true
    }

    pub fn eval_expression(expr: &str, event: &Event) -> bool {
        // 构建上下文
        let mut context: HashMapContext = HashMapContext::new();
        
        // 添加基本字段
        let _ = context.set_value("worker_name".to_string(), event.worker_name.clone().into());
        let _ = context.set_value("event_type".to_string(), event.event_type.clone().into());
        let _ = context.set_value("agent_id".to_string(), event.agent_id.clone().into());
        
        // 添加数据字段（转换为字符串）
        if let serde_json::Value::Object(map) = &event.data {
            for (key, value) in map {
                let value_str = match value {
                    serde_json::Value::String(s) => s.clone(),
                    serde_json::Value::Number(n) => n.to_string(),
                    serde_json::Value::Bool(b) => b.to_string(),
                    _ => continue,
                };
                let _ = context.set_value(key.clone(), value_str.into());
            }
        }

        // 求值表达式
        match evalexpr::eval_boolean_with_context(expr, &context) {
            Ok(result) => result,
            Err(e) => {
                log::warn!("表达式求值失败: {} - {}", expr, e);
                false
            }
        }
    }

    fn matches_exclude(exclude: &super::types::ExcludeCondition, event: &Event) -> bool {
        // 检查排除字段
        for (field, value) in &exclude.fields {
            if let Some(event_value) = event.get_data_string(field) {
                if event_value == *value {
                    return true; // 匹配排除条件
                }
            }
        }

        // 检查排除模式
        for (field, pattern) in &exclude.patterns {
            if let Some(event_value) = event.get_data_string(field) {
                if let Ok(re) = Regex::new(pattern) {
                    if re.is_match(&event_value) {
                        return true; // 匹配排除条件
                    }
                }
            }
        }

        false
    }
}
