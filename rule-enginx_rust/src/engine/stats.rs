// ============================================
// 引擎统计信息
// ============================================

use std::sync::atomic::{AtomicU64, Ordering};
use chrono::{DateTime, Utc};

pub struct EngineStats {
    pub processed_events: AtomicU64,
    pub matched_events: AtomicU64,
    pub start_time: DateTime<Utc>,
}

impl EngineStats {
    pub fn new() -> Self {
        Self {
            processed_events: AtomicU64::new(0),
            matched_events: AtomicU64::new(0),
            start_time: Utc::now(),
        }
    }

    pub fn increment_processed(&self) {
        self.processed_events.fetch_add(1, Ordering::Relaxed);
    }

    pub fn increment_matched(&self) {
        self.matched_events.fetch_add(1, Ordering::Relaxed);
    }

    pub fn get_processed(&self) -> u64 {
        self.processed_events.load(Ordering::Relaxed)
    }

    pub fn get_matched(&self) -> u64 {
        self.matched_events.load(Ordering::Relaxed)
    }
}
