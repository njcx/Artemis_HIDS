#!/bin/bash

# ============================================
# HIDS Connector 编译脚本
# ============================================

set -e

echo "编译 HIDS Connector..."
echo "================================"

# 编译 Release 版本
cargo build --release

echo ""
echo "================================"
echo "编译完成！"
echo ""
echo "生成的文件:"
ls -lh target/release/artemis-connector
echo ""
echo "使用方法:"
echo "  1. 配置文件: config.yaml"
echo "  2. 启动: RUST_LOG=info ./target/release/artemis-connector"
echo ""
