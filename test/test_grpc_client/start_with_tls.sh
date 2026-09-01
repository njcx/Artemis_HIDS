#!/bin/bash
# ============================================
# 快速启动测试客户端 with TLS
# ============================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Starting HIDS Test Client with TLS..."

# 检查证书是否存在
if [ ! -f "../../server/certs/ca-cert.pem" ]; then
    echo "Error: CA certificate not found at ../../server/certs/ca-cert.pem"
    echo "Please run: cd ../../server && ./generate_certs.sh"
    exit 1
fi

# 检查客户端是否编译
if [ ! -f "test-client" ]; then
    echo "Building test client..."
    go build -o test-client main.go
fi

echo "Certificates found"
echo "Starting client with TLS..."
echo ""

# 启动客户端（单向 TLS）
./test-client \
    -ca-file ../../server/certs/ca-cert.pem \
    "$@"
