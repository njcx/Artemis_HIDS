#!/bin/bash

# Kafka 到 MongoDB 同步工具构建脚本

set -e

echo "=========================================="
echo "构建 Kafka 到 MongoDB 同步工具"
echo "=========================================="

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go 环境"
    exit 1
fi

echo "Go 版本: $(go version)"

# 下载依赖
echo "下载依赖..."
go mod tidy
go mod download

# 构建
echo "构建程序..."
go build -o mongo_syncer main.go

# 检查构建结果
if [ -f "mongo_syncer" ]; then
    echo "构建成功: mongo_syncer"
    echo ""
    echo "使用方法:"
    echo "./mongo_syncer -config config.yaml"
else
    echo "构建失败"
    exit 1
fi

echo "=========================================="
