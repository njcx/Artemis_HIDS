#!/bin/bash

# ============================================
# Artemis HIDS Connector 启动脚本
# ============================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 工作目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Artemis HIDS Connector 启动${NC}"
echo -e "${BLUE}========================================${NC}"

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo -e "${RED} 配置文件不存在: config.yaml${NC}"
    exit 1
fi
echo -e "${GREEN} 配置文件已找到${NC}"

# 检查二进制文件
BINARY="target/release/artemis-connector"
if [ ! -f "$BINARY" ]; then
    echo -e "${YELLOW} 二进制文件不存在，开始编译...${NC}"
    bash build.sh
fi
echo -e "${GREEN} 二进制文件已就绪${NC}"

# 设置日志级别（可通过环境变量覆盖）
export RUST_LOG=${RUST_LOG:-info}

# 设置日志格式
export RUST_LOG_STYLE=${RUST_LOG_STYLE:-always}

# 启动 Connector
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN} 启动 Connector...${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "${YELLOW}配置文件: config.yaml${NC}"
echo -e "${YELLOW}日志级别: $RUST_LOG${NC}"
echo -e "${BLUE}========================================${NC}"

# 执行二进制文件
exec "$BINARY" --config config.yaml
