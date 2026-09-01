#!/bin/bash

echo "构建 Rule-Enginx (Rust 版本)..."

# 清理旧的构建
cargo clean

# 构建 release 版本（优化性能）
cargo build --release

if [ $? -eq 0 ]; then
    echo "构建成功！"
    echo "可执行文件: target/release/rule-enginx"
    echo ""
    echo "运行方式:"
    echo "  ./target/release/rule-enginx --config config.yaml"
    echo ""
    
    # 显示二进制文件大小
    ls -lh target/release/rule-enginx
else
    echo "构建失败"
    exit 1
fi
