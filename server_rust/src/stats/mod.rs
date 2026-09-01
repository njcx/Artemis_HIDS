// ============================================
// 统计信息模块
// ============================================

use dashmap::DashMap;
use std::sync::atomic::{AtomicI64, Ordering};
use std::time::Duration;
use chrono::{DateTime, Utc};
use log::info;

pub struct ServerStats {
    total_requests: AtomicI64,
    total_events: AtomicI64,
    total_heartbeats: AtomicI64,
    failed_requests: AtomicI64,
    connected_agents: DashMap<String, DateTime<Utc>>,
}

impl ServerStats {
    pub fn new() -> Self {
        Self {
            total_requests: AtomicI64::new(0),
            total_events: AtomicI64::new(0),
            total_heartbeats: AtomicI64::new(0),
            failed_requests: AtomicI64::new(0),
            connected_agents: DashMap::new(),
        }
    }

    pub fn increment_requests(&self) {
        self.total_requests.fetch_add(1, Ordering::Relaxed);
    }

    pub fn increment_events(&self, count: i64) {
        self.total_events.fetch_add(count, Ordering::Relaxed);
    }

    pub fn increment_heartbeats(&self) {
        self.total_heartbeats.fetch_add(1, Ordering::Relaxed);
    }

    pub fn increment_failures(&self) {
        self.failed_requests.fetch_add(1, Ordering::Relaxed);
    }

    pub fn update_agent(&self, agent_id: String) {
        self.connected_agents.insert(agent_id, Utc::now());
    }

    pub fn get_stats(&self) -> serde_json::Value {
        serde_json::json!({
            "total_requests": self.total_requests.load(Ordering::Relaxed),
            "total_events": self.total_events.load(Ordering::Relaxed),
            "total_heartbeats": self.total_heartbeats.load(Ordering::Relaxed),
            "failed_requests": self.failed_requests.load(Ordering::Relaxed),
            "connected_agents": self.connected_agents.len(),
        })
    }

    pub async fn print_stats_loop(&self) {
        let mut interval = tokio::time::interval(Duration::from_secs(60));
        
        loop {
            interval.tick().await;
            let stats = self.get_stats();
            info!("统计: requests={}, events={}, heartbeats={}, agents={}, failures={}", 
                stats["total_requests"],
                stats["total_events"],
                stats["total_heartbeats"],
                stats["connected_agents"],
                stats["failed_requests"]
            );
        }
    }
}
