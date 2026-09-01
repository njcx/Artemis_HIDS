#!/bin/bash

# 生成 protobuf 代码

set -e

echo "生成 Agent protobuf 代码..."

 export PATH=$PATH:$(go env GOPATH)/bin
# 创建输出目录
mkdir -p pb

# 生成 Go 代码
protoc --proto_path=proto \
       --go_out=pb --go_opt=paths=source_relative \
       --go-grpc_out=pb --go-grpc_opt=paths=source_relative \
       proto/monitor.proto

echo "Protobuf 代码生成完成"
echo "输出目录: pb/"
