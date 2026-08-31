#!/bin/bash
# ============================================
# 快速启动 HIDS Rust Server with TLS
# ============================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Starting HIDS Rust Server with TLS..."

# 检查证书是否存在
if [ ! -f "../server/certs/server-cert.pem" ]; then
    echo "Certificates not found, generating..."
    cd ../server && ./generate_certs.sh && cd - > /dev/null
fi

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "Error: config.yaml not found"
    exit 1
fi

# 检查 TLS 配置
if grep -q "enabled: true" config.yaml; then
    echo "TLS is enabled"
else
    echo "Warning: TLS is disabled in config.yaml"
fi

# 编译（如果需要）
if [ ! -f "target/debug/hids-server" ] && [ ! -f "target/release/hids-server" ]; then
    echo "Building server..."
    cargo build
fi

# 启动 server
echo "Starting Rust server..."
if [ -f "target/release/hids-server" ]; then
    ./target/release/hids-server
else
    ./target/debug/hids-server
fi
