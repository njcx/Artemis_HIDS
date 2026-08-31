#!/bin/bash
# Agent 配置管理示例脚本

API_BASE="http://localhost:8080/api/v1/config"
AGENT_ID="agent-001"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}===========================================${NC}"
echo -e "${GREEN}  HIDS Agent 配置管理工具${NC}"
echo -e "${GREEN}===========================================${NC}"
echo ""

# 1. 获取 agent 配置
echo -e "${YELLOW}1. 获取 Agent 配置${NC}"
curl -s "${API_BASE}/agents/${AGENT_ID}" | jq '.'
echo ""

# 2. 停止 file_monitor worker
echo -e "${YELLOW}2. 停止 file_monitor Worker${NC}"
curl -s -X POST "${API_BASE}/agents/${AGENT_ID}/workers/file_monitor/stop" | jq '.'
echo ""

# 3. 启动 file_monitor worker
echo -e "${YELLOW}3. 启动 file_monitor Worker${NC}"
curl -s -X POST "${API_BASE}/agents/${AGENT_ID}/workers/file_monitor/start" | jq '.'
echo ""

# 4. 更新 agent
echo -e "${YELLOW}4. 更新 Agent 到新版本${NC}"
curl -s -X POST "${API_BASE}/agents/${AGENT_ID}/update" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.1.0",
    "download_url": "https://releases.example.com/agent-1.1.0.tar.gz",
    "checksum": "sha256:abc123..."
  }' | jq '.'
echo ""

# 5. 更新 network_monitor worker
echo -e "${YELLOW}5. 更新 network_monitor Worker${NC}"
curl -s -X POST "${API_BASE}/agents/${AGENT_ID}/workers/network_monitor/update" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.2.0",
    "download_url": "https://releases.example.com/network_monitor-1.2.0.so",
    "checksum": "sha256:def456..."
  }' | jq '.'
echo ""

# 6. 批量停止所有 agent 的 file_monitor
echo -e "${YELLOW}6. 批量操作 - 停止多个 Agent 的 file_monitor${NC}"
curl -s -X POST "${API_BASE}/batch" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_ids": ["agent-001", "agent-002", "agent-003"],
    "command_type": "stop_worker",
    "payload": {
      "worker_name": "file_monitor"
    }
  }' | jq '.'
echo ""

# 7. 更新完整配置
echo -e "${YELLOW}7. 更新完整 Agent 配置${NC}"
curl -s -X PUT "${API_BASE}/agents/${AGENT_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.0.0",
    "workers": {
      "file_monitor": {
        "enabled": true,
        "config": {
          "watch_paths": ["/etc", "/var/log"],
          "interval": 5
        }
      },
      "network_monitor": {
        "enabled": true,
        "config": {
          "capture_ports": [22, 80, 443]
        }
      },
      "process_monitor": {
        "enabled": false,
        "config": {}
      }
    }
  }' | jq '.'
echo ""

echo -e "${GREEN}===========================================${NC}"
echo -e "${GREEN}  测试完成！${NC}"
echo -e "${GREEN}===========================================${NC}"
