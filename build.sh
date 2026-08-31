#!/bin/bash
set -e

cd "$(dirname "$0")"

./docker/init-env.sh
echo ""

LAN_IP=$(grep "^LAN_IP=" .env | cut -d= -f2-)

./docker/gen-certs.sh
echo ""

SERVICES=(server_rust connector rule-enginx mongo_syncer manager frontend)
NOCACHE=()
if [ "$1" == "--no-cache" ]; then
    NOCACHE=(--no-cache)
    echo "构建 agent-builder 镜像（不使用缓存，供 frontend 提取二进制）..."
else
    echo "构建 agent-builder 镜像（供 frontend 提取二进制）..."
fi
docker-compose build "${NOCACHE[@]}" --build-arg PUBLIC_IP="${LAN_IP}" agent-builder

if [ "$1" == "--no-cache" ]; then
    echo "构建所有服务镜像（不使用缓存）..."
    docker-compose build --no-cache "${SERVICES[@]}"
else
    echo "构建所有服务镜像..."
    docker-compose build "${SERVICES[@]}"
fi

echo ""
echo "构建完成！"
echo ""
docker-compose images agent-builder "${SERVICES[@]}"
echo ""
echo "启动全部服务: docker-compose up -d"
echo "查看日志:     docker-compose logs -f <service>"
echo "停止服务:     docker-compose down"
