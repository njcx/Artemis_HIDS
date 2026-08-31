#!/bin/bash

# HIDS Agent 完整构建脚本 (静态编译，跨平台兼容)

set -e

echo "构建 HIDS Agent 及 Workers (静态编译)"
echo "================================"
echo ""

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "Go 未安装"
    exit 1
fi

echo "Go version: $(go version)"
echo ""

# 设置静态编译参数
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
echo "编译配置: CGO_ENABLED=0 GOOS=linux GOARCH=amd64"
echo ""

# 1. 生成 protobuf 代码
echo "步骤 1: 生成 Protobuf 代码"
echo "--------------------------------"
if [ -f "proto/monitor.proto" ]; then
    export PATH=$PATH:$(go env GOPATH)/bin
    
    if ! command -v protoc &> /dev/null; then
        echo "Warning: protoc 未安装，跳过 protobuf 生成"
    else
        mkdir -p pb
        protoc \
          --go_out=pb --go_opt=paths=source_relative \
          --go-grpc_out=pb --go-grpc_opt=paths=source_relative \
          --plugin=protoc-gen-go=$(go env GOPATH)/bin/protoc-gen-go \
          --plugin=protoc-gen-go-grpc=$(go env GOPATH)/bin/protoc-gen-go-grpc \
          --proto_path=proto \
          proto/monitor.proto
        
        if [ -f "pb/monitor.pb.go" ]; then
            echo "Protobuf 代码生成成功"
        else
            echo "Warning: Protobuf 代码可能未生成，但继续构建"
        fi
    fi
else
    echo "Warning: proto/monitor.proto 不存在，跳过"
fi
echo ""

# 2. 整理依赖
echo "步骤 2: 整理依赖"
echo "--------------------------------"
go mod tidy
echo "依赖整理完成"
echo ""

# 3. 编译主程序
echo "步骤 3: 编译 Agent 主程序"
echo "--------------------------------"
go build -ldflags "-s -w" -o artemis_agent
if [ -f "artemis_agent" ]; then
    echo "Agent 主程序编译完成: artemis_agent"
    ls -lh artemis_agent
else
    echo "Agent 主程序编译失败"
    exit 1
fi
echo ""

