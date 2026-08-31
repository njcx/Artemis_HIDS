#!/bin/bash
set -e

cd "$(dirname "$0")/.."

ENV_FILE=".env"

detect_lan_ip() {
    local ip
    ip=$(ip route get 1.1.1.1 2>/dev/null | awk '{print $7; exit}')
    if [ -z "$ip" ]; then
        ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    fi
    echo "$ip"
    return 0
}

if [ -f "$ENV_FILE" ]; then
    if grep -q "^LAN_IP=" "$ENV_FILE"; then
        echo ".env 已存在，跳过随机密码/IP 生成（复用现有配置）"
        exit 0
    fi
    echo ".env 已存在，但缺少 LAN_IP，补充写入..."
    LAN_IP=$(detect_lan_ip)
    echo "检测到内网 IP: ${LAN_IP:-未检测到}"

    echo "LAN_IP=${LAN_IP}" >> "$ENV_FILE"

    echo ""
    echo "已补充写入 .env："
    echo "内网 IP: ${LAN_IP}（用于生成 agent 安装包里的 server_addr）"
    exit 0
fi

MONGO_ROOT_PASS=$(openssl rand -hex 16)
REDIS_PASS=$(openssl rand -hex 16)
OPENOBSERVE_ROOT_PASS="$(openssl rand -hex 12)Aa1!"
KAFKA_SASL_PASS=$(openssl rand -hex 16)
MANAGER_ADMIN_PASSWORD="$(openssl rand -hex 12)Aa1!"

LAN_IP=$(detect_lan_ip)
echo "检测到内网 IP: ${LAN_IP:-未检测到}"

cat > "$ENV_FILE" <<EOF
MONGO_ROOT_USERNAME=mongoadmin
MONGO_ROOT_PASS=${MONGO_ROOT_PASS}
REDIS_PASS=${REDIS_PASS}
OPENOBSERVE_ROOT_EMAIL=root@example.com
OPENOBSERVE_ROOT_PASS=${OPENOBSERVE_ROOT_PASS}
KAFKA_SASL_USERNAME=artemis
KAFKA_SASL_PASS=${KAFKA_SASL_PASS}
MANAGER_ADMIN_PASSWORD=${MANAGER_ADMIN_PASSWORD}
LAN_IP=${LAN_IP}
EOF

chmod 600 "$ENV_FILE"

echo ""
echo "首次初始化，已生成随机密码到 .env："
echo ""
echo "MongoDB    : mongoadmin / ${MONGO_ROOT_PASS}"
echo "Redis      : (no user) / ${REDIS_PASS}"
echo "OpenObserve: root@example.com / ${OPENOBSERVE_ROOT_PASS}"
echo "Kafka      : artemis / ${KAFKA_SASL_PASS}"
echo "Manager    : admin / ${MANAGER_ADMIN_PASSWORD}"
echo ""
echo "内网 IP: ${LAN_IP}（用于生成 agent 安装包里的 server_addr）"
echo ""
echo "密码仅保存在 .env 文件中（未纳入版本控制），请妥善保管。"
