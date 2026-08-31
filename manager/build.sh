#!/bin/bash
# 构建脚本

set -e

echo "构建 Manager API..."
cd cmd/api
go build -o ../../bin/manager-api main.go

# echo "构建 Manager Consumer..."
# cd ../consumer
# go build -o ../../bin/manager-consumer main.go

# echo "构建 Manager Syncer..."
# cd ../syncer
# go build -o ../../bin/manager-syncer main.go

cd ../..

echo "构建完成！"
echo "可执行文件位于 bin/ 目录"
ls -lh bin/
