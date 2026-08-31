#!/bin/bash
# ============================================
# 构建测试客户端
# ============================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building HIDS Test gRPC Client..."

# 下载依赖
echo "Downloading dependencies..."
go mod tidy

# 构建
echo "Compiling..."
go build -o test-client -ldflags="-s -w" main.go

echo "Build successful!"
echo "Run with: ./test-client -h"
