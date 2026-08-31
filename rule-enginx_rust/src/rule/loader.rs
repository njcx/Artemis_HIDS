// ============================================
// 规则加载器
// ============================================

use anyhow::{Context, Result};
use std::fs;
use std::path::Path;

use super::types::Rule;

pub struct RuleLoader;

impl RuleLoader {
    pub fn load_from_directory(dir: &str) -> Result<Vec<Rule>> {
        let path = Path::new(dir);
        if !path.exists() {
            return Err(anyhow::anyhow!("规则目录不存在: {}", dir));
        }

        let mut rules = Vec::new();
        let entries = fs::read_dir(path)
            .with_context(|| format!("读取规则目录失败: {}", dir))?;

        for entry in entries {
            let entry = entry?;
            let path = entry.path();
            
            // 只处理 .yaml 和 .yml 文件
            if path.is_file() {
                if let Some(ext) = path.extension() {
                    if ext == "yaml" || ext == "yml" {
                        match Self::load_rule_file(&path) {
                            Ok(rule) => {
                                if rule.enabled {
                                    log::info!("加载规则: {} ({})", rule.id, rule.name);
                                    rules.push(rule);
                                } else {
                                    log::debug!("跳过禁用规则: {}", rule.id);
                                }
                            }
                            Err(e) => {
                                log::warn!("加载规则失败 {:?}: {}", path, e);
                            }
                        }
                    }
                }
            }
        }

        if rules.is_empty() {
            log::warn!("⚠ 未加载任何规则");
        }

        Ok(rules)
    }

    fn load_rule_file(path: &Path) -> Result<Rule> {
        let content = fs::read_to_string(path)
            .with_context(|| format!("读取规则文件失败: {:?}", path))?;
        
        let rule: Rule = serde_yaml::from_str(&content)
            .with_context(|| format!("解析规则文件失败: {:?}", path))?;
        
        Ok(rule)
    }
}
