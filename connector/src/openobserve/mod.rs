// ============================================
// OpenObserve 客户端模块
// ============================================

use anyhow::{Context, Result};
use base64::{Engine as _, engine::general_purpose};
use log::info;
use reqwest::Client;
use serde_json::Value;

use crate::config::OpenObserveConfig;

pub struct OpenObserveClient {
    client: Client,
    config: OpenObserveConfig,
    auth_header: String,
}

impl OpenObserveClient {
    pub async fn new(config: OpenObserveConfig) -> Result<Self> {
        let client = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()
            .context("创建 HTTP 客户端失败")?;

        // 构建 Basic Auth
        let auth = format!("{}:{}", config.username, config.password);
        let auth_header = format!("Basic {}", general_purpose::STANDARD.encode(auth));

        info!(" OpenObserve 客户端已创建: url={}", config.url);

        Ok(Self {
            client,
            config,
            auth_header,
        })
    }

    pub async fn write_batch(&self, index: &str, data: &[Value]) -> Result<()> {
        if data.is_empty() {
            return Ok(());
        }

        let url = format!(
            "{}/api/{}/{}/_json",
            self.config.url, self.config.org, index
        );

        // OpenObserve 接受 JSON 数组格式
        let response = self
            .client
            .post(&url)
            .header("Authorization", &self.auth_header)
            .header("Content-Type", "application/json")
            .json(&data)
            .send()
            .await
            .context("发送请求到 OpenObserve 失败")?;

        if !response.status().is_success() {
            let status = response.status();
            let text = response.text().await.unwrap_or_default();
            anyhow::bail!("OpenObserve 返回错误: {} - {}", status, text);
        }

        Ok(())
    }
}
