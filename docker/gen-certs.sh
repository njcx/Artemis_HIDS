#!/bin/bash
set -e

cd "$(dirname "$0")/.."

SERVER_CERT_DIR="server_rust/certs"
AGENT_CERT_DIR="agent/certs"
ENV_FILE=".env"

REQUIRED_FILES=(
    "$SERVER_CERT_DIR/ca-cert.pem"
    "$SERVER_CERT_DIR/server-cert.pem"
    "$SERVER_CERT_DIR/server-key.pem"
    "$SERVER_CERT_DIR/client-cert.pem"
    "$SERVER_CERT_DIR/client-key.pem"
)

ALL_EXIST=1
for f in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$f" ]; then
        ALL_EXIST=0
        break
    fi
done

if [ "$ALL_EXIST" -eq 1 ]; then
    echo "TLS 证书已存在，跳过生成（复用现有证书）"
    exit 0
fi

echo "TLS 证书缺失，正在生成..."

LAN_IP=$(grep "^LAN_IP=" "$ENV_FILE" 2>/dev/null | cut -d= -f2-)

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

SUBJECT_PREFIX="/C=CN/ST=Beijing/L=Beijing/O=Artemis/OU=HIDS"

openssl req -x509 -newkey rsa:4096 -days 3650 -nodes \
    -keyout "$WORK_DIR/ca-key.pem" -out "$WORK_DIR/ca-cert.pem" \
    -subj "${SUBJECT_PREFIX}/CN=Artemis-HIDS-CA" >/dev/null 2>&1

cat > "$WORK_DIR/server-ext.cnf" <<EOF
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.local
DNS.3 = hids-server
DNS.4 = server
DNS.5 = artemis-hids-server
DNS.6 = *.artemis.local
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
IP.3 = ::1
EOF

if [ -n "$LAN_IP" ]; then
    echo "IP.4 = ${LAN_IP}" >> "$WORK_DIR/server-ext.cnf"
fi

openssl req -newkey rsa:4096 -nodes \
    -keyout "$WORK_DIR/server-key.pem" -out "$WORK_DIR/server-req.pem" \
    -subj "${SUBJECT_PREFIX}/CN=localhost" >/dev/null 2>&1

openssl x509 -req -days 3650 \
    -in "$WORK_DIR/server-req.pem" \
    -CA "$WORK_DIR/ca-cert.pem" -CAkey "$WORK_DIR/ca-key.pem" -CAcreateserial \
    -out "$WORK_DIR/server-cert.pem" \
    -extfile "$WORK_DIR/server-ext.cnf" >/dev/null 2>&1

openssl req -newkey rsa:4096 -nodes \
    -keyout "$WORK_DIR/client-key.pem" -out "$WORK_DIR/client-req.pem" \
    -subj "${SUBJECT_PREFIX}/CN=client" >/dev/null 2>&1

openssl x509 -req -days 3650 \
    -in "$WORK_DIR/client-req.pem" \
    -CA "$WORK_DIR/ca-cert.pem" -CAkey "$WORK_DIR/ca-key.pem" -CAcreateserial \
    -out "$WORK_DIR/client-cert.pem" >/dev/null 2>&1

mkdir -p "$SERVER_CERT_DIR" "$AGENT_CERT_DIR"

cp "$WORK_DIR/ca-key.pem" "$WORK_DIR/ca-cert.pem" \
   "$WORK_DIR/server-req.pem" "$WORK_DIR/server-cert.pem" "$WORK_DIR/server-key.pem" \
   "$WORK_DIR/client-req.pem" "$WORK_DIR/client-cert.pem" "$WORK_DIR/client-key.pem" \
   "$WORK_DIR/ca-cert.srl" \
   "$SERVER_CERT_DIR/"
cp "$WORK_DIR/server-ext.cnf" "$SERVER_CERT_DIR/server-ext.cnf"

cp "$WORK_DIR/ca-cert.pem" "$WORK_DIR/client-cert.pem" "$WORK_DIR/client-key.pem" \
   "$AGENT_CERT_DIR/"

chmod 600 "$SERVER_CERT_DIR"/*-key.pem "$AGENT_CERT_DIR"/*-key.pem
chmod 644 "$SERVER_CERT_DIR"/*-cert.pem "$AGENT_CERT_DIR"/*-cert.pem

echo ""
echo "已生成新的 TLS 证书链："
echo "CA         : $SERVER_CERT_DIR/ca-cert.pem"
echo "Server     : $SERVER_CERT_DIR/server-cert.pem（SAN 包含内网 IP: ${LAN_IP:-未检测到}）"
echo "Client     : $SERVER_CERT_DIR/client-cert.pem"
echo "Agent 证书 : $AGENT_CERT_DIR/（ca-cert.pem / client-cert.pem / client-key.pem 已同步）"
echo ""
