#!/bin/bash

# ============================================
# Artemis HIDS Manager 快速启动脚本
# ============================================

set -e

echo "========================================"
echo "Artemis HIDS Manager 启动脚本"
echo "========================================"


# 检查配置文件
echo ""
echo "检查配置文件..."
if [ ! -f "config.yaml" ]; then
    echo "config.yaml 不存在，从示例复制..."
    cp config.example.yaml config.yaml
    echo "请编辑 config.yaml 并填写正确的数据库密码"
    exit 1
fi

echo "配置文件存在"


# 启动服务
echo ""
echo "启动 Manager API Server..."
echo ""
echo "========================================"
echo "默认管理员账号:"
echo "用户名: admin"
echo "密码: admin123"
echo "========================================"
echo ""
echo "访问地址: http://localhost:8080"
echo ""

./bin/manager-api ./config.yaml

