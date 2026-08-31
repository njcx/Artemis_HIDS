// ============================================
// 规则引擎实现 - 高性能版本
// 支持规则索引和预编译正则
// ============================================

use dashmap::DashMap;
use regex::Regex;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use crate::rule::{Rule, RuleMatcher, RuleType};
use super::event::Event;
use super::stats::EngineStats;

pub struct MatchResult {
    pub rule: Rule,
    pub event: Event,
    pub matched_at: i64,
    pub context: HashMap<String, String>,
}

/// 预编译的规则，包含编译后的正则表达式
struct CompiledRule {
    rule: Rule,
    /// 预编译的 patterns 正则表达式
    compiled_patterns: HashMap<String, Regex>,
    /// 预编译的 exclude patterns 正则表达式
    compiled_exclude_patterns: HashMap<String, Regex>,
}

impl CompiledRule {
    fn new(rule: Rule) -> Self {
        let mut compiled_patterns = HashMap::new();
        let mut compiled_exclude_patterns = HashMap::new();
        
        // 预编译 patterns
        for (field, pattern) in &rule.conditions.patterns {
            if let Ok(re) = Regex::new(pattern) {
                compiled_patterns.insert(field.clone(), re);
            }
        }
        
        // 预编译 exclude patterns
        if let Some(exclude) = &rule.conditions.exclude {
            for (field, pattern) in &exclude.patterns {
                if let Ok(re) = Regex::new(pattern) {
                    compiled_exclude_patterns.insert(field.clone(), re);
                }
            }
        }
        
        Self {
            rule,
            compiled_patterns,
            compiled_exclude_patterns,
        }
    }
}

pub struct RuleEngine {
    /// 所有规则（预编译）
    rules: Vec<CompiledRule>,
    /// 按 worker_name 索引的规则
    rules_by_worker: HashMap<String, Vec<usize>>,
    /// 按 event_type 索引的规则
    rules_by_event_type: HashMap<String, Vec<usize>>,
    /// 通用规则（没有指定 worker_name 或 event_type）
    generic_rules: Vec<usize>,
    /// 统计
    stats: Arc<EngineStats>,
    /// 阈值统计缓存
    threshold_cache: Arc<DashMap<String, ThresholdCounter>>,
    /// 关联事件缓存
    correlation_cache: Arc<DashMap<String, Vec<Event>>>,
}

struct ThresholdCounter {
    count: u64,
    first_seen: u64,
    last_seen: u64,
}

impl RuleEngine {
    pub fn new(rules: Vec<Rule>) -> Self {
        let stats = Arc::new(EngineStats::new());
        
        // 预编译所有规则
        let compiled_rules: Vec<CompiledRule> = rules
            .into_iter()
            .filter(|r| r.enabled)
            .map(CompiledRule::new)
            .collect();
        
        // 构建索引
        let mut rules_by_worker: HashMap<String, Vec<usize>> = HashMap::new();
        let mut rules_by_event_type: HashMap<String, Vec<usize>> = HashMap::new();
        let mut generic_rules: Vec<usize> = Vec::new();
        
        for (idx, compiled) in compiled_rules.iter().enumerate() {
            let rule = &compiled.rule;
            let has_worker = rule.conditions.worker_name.is_some();
            let has_event_type = rule.conditions.event_type.is_some();
            
            if let Some(ref worker_name) = rule.conditions.worker_name {
                rules_by_worker
                    .entry(worker_name.clone())
                    .or_default()
                    .push(idx);
            }
            
            if let Some(ref event_type) = rule.conditions.event_type {
                rules_by_event_type
                    .entry(event_type.clone())
                    .or_default()
                    .push(idx);
            }
            
            // 如果没有指定 worker_name 和 event_type，则为通用规则
            if !has_worker && !has_event_type {
                generic_rules.push(idx);
            }
        }
        
        log::info!("规则引擎初始化完成:");
        log::info!("  - 总规则数: {}", compiled_rules.len());
        log::info!("  - Worker 索引: {} 个分类", rules_by_worker.len());
        log::info!("  - EventType 索引: {} 个分类", rules_by_event_type.len());
        log::info!("  - 通用规则: {} 条", generic_rules.len());
        
        // 启动清理任务
        let threshold_cache = Arc::new(DashMap::new());
        let correlation_cache = Arc::new(DashMap::new());
        
        let threshold_cache_clone = threshold_cache.clone();
        let correlation_cache_clone = correlation_cache.clone();
        
        tokio::spawn(async move {
            Self::cleanup_task(threshold_cache_clone, correlation_cache_clone).await;
        });

        Self {
            rules: compiled_rules,
            rules_by_worker,
            rules_by_event_type,
            generic_rules,
            stats,
            threshold_cache,
            correlation_cache,
        }
    }

    /// 处理事件 - 使用索引加速
    pub fn process_event(&self, event: &Event) -> Vec<MatchResult> {
        self.stats.increment_processed();
        let mut results = Vec::new();
        
        // 收集需要检查的规则索引（去重）
        let mut rule_indices: Vec<usize> = Vec::new();
        
        // 1. 添加按 worker_name 匹配的规则
        if let Some(indices) = self.rules_by_worker.get(&event.worker_name) {
            rule_indices.extend(indices);
        }
        
        // 2. 添加按 event_type 匹配的规则
        if let Some(indices) = self.rules_by_event_type.get(&event.event_type) {
            for idx in indices {
                if !rule_indices.contains(idx) {
                    rule_indices.push(*idx);
                }
            }
        }
        
        // 3. 添加通用规则
        for idx in &self.generic_rules {
            if !rule_indices.contains(idx) {
                rule_indices.push(*idx);
            }
        }
        
        // 检查每个候选规则
        for idx in rule_indices {
            let compiled = &self.rules[idx];
            
            let matched = match compiled.rule.rule_type {
                RuleType::Pattern => self.match_pattern_compiled(compiled, event),
                RuleType::Threshold => self.match_threshold_compiled(compiled, event),
                RuleType::Correlation => self.match_correlation_compiled(compiled, event),
                RuleType::Statistical => self.match_threshold_compiled(compiled, event),
            };

            if let Some(result) = matched {
                self.stats.increment_matched();
                log::info!("规则匹配成功: rule_id={}, rule_name={}", 
                    compiled.rule.id, compiled.rule.name);
                results.push(result);
            }
        }

        results
    }

    /// 使用预编译正则的模式匹配
    fn match_pattern_compiled(&self, compiled: &CompiledRule, event: &Event) -> Option<MatchResult> {
        let rule = &compiled.rule;
        let cond = &rule.conditions;

        // 1. 检查 worker_name
        if let Some(ref worker_name) = cond.worker_name {
            if worker_name != &event.worker_name {
                return None;
            }
        }

        // 2. 检查 event_type
        if let Some(ref event_type) = cond.event_type {
            if event_type != &event.event_type {
                return None;
            }
        }

        // 3. 检查字段精确匹配
        for (field, value) in &cond.fields {
            match event.get_data_string(field) {
                Some(event_value) if event_value == *value => continue,
                _ => return None,
            }
        }

        // 4. 使用预编译的正则匹配
        for (field, re) in &compiled.compiled_patterns {
            match event.get_data_string(field) {
                Some(event_value) => {
                    if !re.is_match(&event_value) {
                        return None;
                    }
                }
                None => return None,
            }
        }

        // 5. 检查表达式（如果有）
        if let Some(ref expr) = cond.expression {
            if !RuleMatcher::eval_expression(expr, event) {
                return None;
            }
        }

        // 6. 检查排除条件
        if let Some(ref exclude) = cond.exclude {
            // 检查排除字段
            for (field, value) in &exclude.fields {
                if let Some(event_value) = event.get_data_string(field) {
                    if event_value == *value {
                        return None;
                    }
                }
            }
            
            // 使用预编译的排除正则
            for (field, re) in &compiled.compiled_exclude_patterns {
                if let Some(event_value) = event.get_data_string(field) {
                    if re.is_match(&event_value) {
                        return None;
                    }
                }
            }
        }

        Some(MatchResult {
            rule: rule.clone(),
            event: event.clone(),
            matched_at: current_timestamp(),
            context: HashMap::new(),
        })
    }

    /// 阈值匹配（使用预编译规则）
    fn match_threshold_compiled(&self, compiled: &CompiledRule, event: &Event) -> Option<MatchResult> {
        // 先检查基本条件
        if self.match_pattern_compiled(compiled, event).is_none() {
            return None;
        }

        let rule = &compiled.rule;
        let threshold_config = rule.threshold.as_ref()?;
        
        // 构建缓存 key
        let mut key_parts = vec![rule.id.clone()];
        for group_field in &threshold_config.group_by {
            if let Some(value) = event.get_data_string(group_field) {
                key_parts.push(value);
            }
        }
        let cache_key = key_parts.join(":");

        let now = current_timestamp() as u64;
        
        // 更新计数器
        let mut triggered = false;
        self.threshold_cache.entry(cache_key.clone())
            .and_modify(|counter| {
                if now - counter.first_seen > threshold_config.time_window {
                    counter.count = 1;
                    counter.first_seen = now;
                    counter.last_seen = now;
                } else {
                    counter.count += 1;
                    counter.last_seen = now;
                    triggered = Self::check_threshold(
                        counter.count as f64, 
                        &threshold_config.operator, 
                        threshold_config.value
                    );
                }
            })
            .or_insert_with(|| {
                ThresholdCounter {
                    count: 1,
                    first_seen: now,
                    last_seen: now,
                }
            });

        if triggered {
            let mut context = HashMap::new();
            context.insert("threshold_key".to_string(), cache_key);
            
            Some(MatchResult {
                rule: rule.clone(),
                event: event.clone(),
                matched_at: current_timestamp(),
                context,
            })
        } else {
            None
        }
    }

    /// 关联匹配（使用预编译规则）
    fn match_correlation_compiled(&self, compiled: &CompiledRule, event: &Event) -> Option<MatchResult> {
        let rule = &compiled.rule;
        let correlation_config = rule.correlation.as_ref()?;
        
        // 构建缓存 key
        let mut key_parts = vec![rule.id.clone()];
        for group_field in &correlation_config.group_by {
            if let Some(value) = event.get_data_string(group_field) {
                key_parts.push(value);
            }
        }
        let cache_key = key_parts.join(":");

        let now = current_timestamp() as u64;
        
        // 添加事件到缓存
        let mut events = self.correlation_cache.entry(cache_key.clone())
            .or_insert_with(Vec::new);
        
        // 清理过期事件
        events.retain(|e| (now - e.timestamp as u64) <= correlation_config.time_window);
        
        // 添加当前事件
        events.push(event.clone());
        
        // 检查是否满足关联条件
        let event_types: std::collections::HashSet<_> = events.iter()
            .map(|e| e.event_type.as_str())
            .collect();
        
        let required_types: std::collections::HashSet<_> = correlation_config.events.iter()
            .map(|s| s.as_str())
            .collect();
        
        if event_types.is_superset(&required_types) && events.len() >= correlation_config.min_count {
            let mut context = HashMap::new();
            context.insert("correlation_key".to_string(), cache_key);
            context.insert("event_count".to_string(), events.len().to_string());
            
            Some(MatchResult {
                rule: rule.clone(),
                event: event.clone(),
                matched_at: current_timestamp(),
                context,
            })
        } else {
            None
        }
    }

    /// 检查阈值条件
    fn check_threshold(count: f64, operator: &str, value: f64) -> bool {
        match operator {
            ">" => count > value,
            "<" => count < value,
            ">=" => count >= value,
            "<=" => count <= value,
            "==" => (count - value).abs() < f64::EPSILON,
            "!=" => (count - value).abs() >= f64::EPSILON,
            _ => false,
        }
    }

    /// 定期清理过期数据
    async fn cleanup_task(
        threshold_cache: Arc<DashMap<String, ThresholdCounter>>,
        correlation_cache: Arc<DashMap<String, Vec<Event>>>,
    ) {
        let mut interval = tokio::time::interval(tokio::time::Duration::from_secs(60));
        
        loop {
            interval.tick().await;
            let now = current_timestamp() as u64;
            
            // 清理阈值缓存（超过1小时）
            threshold_cache.retain(|_, counter| {
                now - counter.last_seen < 3600
            });
            
            // 清理关联缓存（超过1小时）
            correlation_cache.retain(|_, events| {
                events.iter().any(|e| {
                    let ts = e.timestamp as u64;
                    now - ts < 3600
                })
            });
            
            log::debug!("缓存清理完成: threshold={}, correlation={}", 
                threshold_cache.len(), correlation_cache.len());
        }
    }

    pub fn get_stats(&self) -> Arc<EngineStats> {
        self.stats.clone()
    }
}

fn current_timestamp() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs() as i64
}
