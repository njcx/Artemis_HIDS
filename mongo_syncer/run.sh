#!/bin/bash

# Kafka 到 MongoDB 同步工具运行脚本

set -e

CONFIG_FILE="${1:-config.yaml}"

echo "=========================================="
echo "启动 Kafka 到 MongoDB 同步工具"
echo "=========================================="

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 配置文件不存在: $CONFIG_FILE"
    echo ""
    echo "请先创建配置文件:"
    echo "cp config.example.yaml config.yaml"
    echo "vim config.yaml"
    exit 1
fi

echo "使用配置文件: $CONFIG_FILE"

# 检查程序是否存在
if [ ! -f "mongo_syncer" ]; then
    echo "程序不存在，开始构建..."
    ./build.sh
fi

# 运行程序
echo "启动同步器..."
./mongo_syncer -config "$CONFIG_FILE"
