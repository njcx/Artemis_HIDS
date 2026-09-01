#!/bin/bash
# Artemis HIDS Agent 远程一键安装脚本
# 用法: curl -sSL http://<host>:8081/install.sh | ARTEMIS_SERVER=http://<host>:8081 bash

set -e

ARTEMIS_SERVER="${ARTEMIS_SERVER:-http://localhost:8081}"
TARBALL_URL="${ARTEMIS_SERVER%/}/artemis-agent.tar.gz"

echo "======================================"
echo "Artemis HIDS Agent 远程安装程序"
echo "======================================"
echo ""

if [ "$(id -u)" != "0" ]; then
    echo "错误: 此脚本需要 root 权限运行"
    echo "请使用: sudo -E bash -c \"curl -sSL ... | ARTEMIS_SERVER=... bash\""
    exit 1
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "下载安装包: $TARBALL_URL"
curl -fsSL "$TARBALL_URL" -o "$WORK_DIR/artemis-agent.tar.gz"

echo "解压安装包..."
tar -xzf "$WORK_DIR/artemis-agent.tar.gz" -C "$WORK_DIR"

cd "$WORK_DIR"
exec bash install.sh
