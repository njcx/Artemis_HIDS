#!/bin/bash

echo "启动 Rule-Enginx (Rust 版本)"

# 检查二进制文件是否存在
if [ ! -f "target/release/rule-enginx" ]; then
    echo "可执行文件不存在，请先运行 ./build.sh"
    exit 1
fi

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "配置文件 config.yaml 不存在"
    exit 1
fi

# 启动
echo "使用配置文件: config.yaml"
echo "规则目录: ./rules"
echo ""

# 设置日志级别
export RUST_LOG=info

# 运行
./target/release/rule-enginx --config config.yaml
