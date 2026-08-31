#!/bin/bash
# Artemis HIDS Agent 卸载脚本

set -e

INSTALL_DIR="/usr/local/artemis-agent"
SERVICE_NAME="artemis-agent"

echo "======================================"
echo "Artemis HIDS Agent 卸载程序"
echo "======================================"
echo ""

# 检查 root 权限
if [ "$(id -u)" != "0" ]; then
    echo "错误: 此脚本需要 root 权限运行"
    echo "请使用: sudo $0"
    exit 1
fi

# 停止服务
echo "停止服务..."
systemctl stop $SERVICE_NAME 2>/dev/null || true

# 禁用服务
echo "禁用服务..."
systemctl disable $SERVICE_NAME 2>/dev/null || true

# 删除 systemd service 文件
echo "删除 systemd service..."
rm -f /etc/systemd/system/${SERVICE_NAME}.service
systemctl daemon-reload

# 删除安装目录
echo "删除安装目录: $INSTALL_DIR"
echo ""

rm -rf $INSTALL_DIR

echo ""
echo "======================================"
echo "卸载完成！"
echo "======================================"
echo ""
