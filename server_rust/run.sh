#!/bin/bash

# HIDS Server 启动脚本

CONFIG_FILE="${1:-config.yaml}"
RUST_LOG="${RUST_LOG:-info}"

echo "启动 HIDS Server (Rust 版本)"
echo "配置文件: $CONFIG_FILE"
echo "日志级别: $RUST_LOG"
echo ""

export RUST_LOG

if [ ! -f "target/release/hids-server" ]; then
    echo "可执行文件不存在，请先运行 ./build.sh"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo "配置文件不存在: $CONFIG_FILE"
    exit 1
fi

exec ./target/release/hids-server --config "$CONFIG_FILE"
