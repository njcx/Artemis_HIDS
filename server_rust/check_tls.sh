#!/bin/bash
# ============================================
# 验证 Rust Server TLS 配置
# ============================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "========================================"
echo "  Rust HIDS Server TLS Configuration Check"
echo "========================================"
echo ""

# 1. 检查证书文件
echo "1. Checking certificate files..."
CERT_DIR="../server/certs"
REQUIRED_CERTS=(
    "$CERT_DIR/ca-cert.pem"
    "$CERT_DIR/server-cert.pem"
    "$CERT_DIR/server-key.pem"
)

MISSING=0
for cert in "${REQUIRED_CERTS[@]}"; do
    if [ -f "$cert" ]; then
        echo "   [OK] $cert"
    else
        echo "   [MISSING] $cert (missing)"
        MISSING=1
    fi
done

if [ $MISSING -eq 1 ]; then
    echo ""
    echo "Some certificates are missing!"
    echo "Run: cd ../server && ./generate_certs.sh"
    exit 1
fi

echo ""

# 2. 检查配置文件
echo "2. Checking config.yaml..."
if [ -f "config.yaml" ]; then
    echo "   [OK] config.yaml exists"
    
    if grep -q "enabled: true" config.yaml; then
        echo "   [OK] TLS is enabled"
        
        if grep -q "client_auth: true" config.yaml; then
            echo "   [INFO] Mutual TLS (two-way) configured"
        else
            echo "   [INFO] One-way TLS configured"
        fi
    else
        echo "   [WARNING] TLS is disabled"
    fi
else
    echo "   [MISSING] config.yaml not found"
    exit 1
fi

echo ""

# 3. 检查编译
echo "3. Checking binary..."
if [ -f "target/release/hids-server" ]; then
    echo "   [OK] Release binary exists"
elif [ -f "target/debug/hids-server" ]; then
    echo "   [OK] Debug binary exists"
else
    echo "   [WARNING] Binary not found (run: cargo build)"
fi

echo ""
echo "========================================"
echo "  Rust Server TLS Configuration OK"
echo "========================================"
echo ""
echo "Next steps:"
echo "  1. Start server:  ./start_with_tls.sh"
echo "  2. Start client:  cd ../test/test_grpc_client && ./start_with_tls.sh"
echo ""
