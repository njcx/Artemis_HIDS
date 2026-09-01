// ============================================
// 规则加载器（MongoDB）
// ============================================

use anyhow::{Context, Result};
use futures_util::stream::TryStreamExt;
use mongodb::bson::doc;
use mongodb::options::IndexOptions;
use mongodb::{Collection, IndexModel};

use super::types::Rule;

pub struct RuleLoader;

impl RuleLoader {
    pub async fn load_from_mongo(collection: &Collection<Rule>) -> Result<Vec<Rule>> {
        let filter = doc! { "enabled": true };
        let mut cursor = collection
            .find(filter)
            .await
            .context("查询 MongoDB 规则集合失败")?;

        let mut rules = Vec::new();
        while let Some(rule) = cursor.try_next().await.context("读取规则文档失败")? {
            log::info!("加载规则: {} ({})", rule.id, rule.name);
            rules.push(rule);
        }

        if rules.is_empty() {
            log::warn!("⚠ 未从 MongoDB 加载任何规则");
        }

        Ok(rules)
    }

    pub async fn ensure_indexes(collection: &Collection<Rule>) -> Result<()> {
        let id_index = IndexModel::builder()
            .keys(doc! { "id": 1 })
            .options(IndexOptions::builder().unique(true).build())
            .build();
        let enabled_index = IndexModel::builder()
            .keys(doc! { "enabled": 1 })
            .build();

        collection
            .create_index(id_index)
            .await
            .context("创建 id 唯一索引失败")?;
        collection
            .create_index(enabled_index)
            .await
            .context("创建 enabled 索引失败")?;

        Ok(())
    }
}
