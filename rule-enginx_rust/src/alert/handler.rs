// ============================================
// 告警处理器
// ============================================

use anyhow::Result;
use log::{info, error};

use crate::config::AlertConfig;
use crate::engine::MatchResult;
use super::alert::Alert;
use super::outputs::{OutputHandler, StdoutOutput, FileOutput, KafkaOutput};

pub struct AlertHandler {
    outputs: Vec<Box<dyn OutputHandler>>,
}

impl AlertHandler {
    pub async fn new(config: AlertConfig) -> Result<Self> {
        let mut outputs: Vec<Box<dyn OutputHandler>> = Vec::new();

        for output_config in config.outputs {
            if !output_config.enabled {
                continue;
            }

            match output_config.output_type.as_str() {
                "stdout" => {
                    outputs.push(Box::new(StdoutOutput::new()));
                    info!("启用 stdout 输出");
                }
                "file" => {
                    let path = output_config.config["path"].as_str()
                        .unwrap_or("./alerts.log");
                    outputs.push(Box::new(FileOutput::new(path)?));
                    info!("启用 file 输出: {}", path);
                }
                "kafka" => {
                    outputs.push(Box::new(KafkaOutput::new(output_config.config).await?));
                    info!("启用 kafka 输出");
                }
                _ => {
                    log::warn!("未知的输出类型: {}", output_config.output_type);
                }
            }
        }

        Ok(Self { outputs })
    }

    pub async fn handle_match(&self, result: MatchResult) -> Result<()> {
        // 构建告警
        let alert = Alert::new(
            result.rule.id.clone(),
            result.rule.name.clone(),
            result.rule.severity.clone(),
            result.rule.description.clone(),
            result.event.agent_id.clone(),
            result.event.worker_name.clone(),
            result.event.event_type.clone(),
            result.event.timestamp,
            serde_json::to_value(&result.event.data)?,
            serde_json::to_value(&result.context)?,
            result.rule.tags.clone(),
        );

        // 发送到所有输出
        for output in &self.outputs {
            if let Err(e) = output.send(&alert).await {
                error!("输出告警失败: {}", e);
            }
        }

        Ok(())
    }
}
