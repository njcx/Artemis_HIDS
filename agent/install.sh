#!/bin/bash
# Artemis HIDS Agent 安装脚本

set -e

INSTALL_DIR="/usr/local/artemis-agent"
SERVICE_FILE="artemis-agent.service"
BINARY_NAME="artemis_agent"

echo "======================================"
echo "Artemis HIDS Agent 安装程序"
echo "======================================"
echo ""

# 检查 root 权限
if [ "$(id -u)" != "0" ]; then
    echo "错误: 此脚本需要 root 权限运行"
    echo "请使用: sudo $0"
    exit 1
fi

# 检查二进制文件是否存在
if [ ! -f "$BINARY_NAME" ]; then
    echo "错误: 找不到 $BINARY_NAME"
    echo "请先编译: ./build.sh"
    exit 1
fi

# 创建安装目录
echo "创建安装目录: $INSTALL_DIR"
mkdir -p $INSTALL_DIR
mkdir -p $INSTALL_DIR/workers
mkdir -p $INSTALL_DIR/certs
# mkdir -p $INSTALL_DIR/logs

# 复制主程序
echo "复制主程序..."
cp $BINARY_NAME $INSTALL_DIR/
chmod +x $INSTALL_DIR/$BINARY_NAME

# 复制 workers (只复制 workers/ 目录下的 ELF 可执行文件)
echo "复制 workers..."
for worker_file in workers/*; do
    # 跳过目录，只处理文件
    if [ -f "$worker_file" ]; then
        worker_name=$(basename "$worker_file")
        # 检查文件是否是 ELF 可执行文件
        if file "$worker_file" | grep -q "ELF"; then
            cp "$worker_file" $INSTALL_DIR/workers/$worker_name
            chmod +x $INSTALL_DIR/workers/$worker_name
            echo "  - $worker_name (ELF)"
        fi
    fi
done

# 复制配置文件（如果不存在）
if [ ! -f "$INSTALL_DIR/config.json" ]; then
    echo "复制配置文件..."
    if [ -f "config.json" ]; then
        cp config.json $INSTALL_DIR/config.json
        echo "  已复制配置文件到 $INSTALL_DIR/config.json"
    fi
fi

# 复制证书文件（如果存在）
if [ -d "certs" ]; then
    echo "复制证书文件..."
    cp -r certs/* $INSTALL_DIR/certs/ 2>/dev/null || true
fi

# 安装 systemd service
echo "安装 systemd service..."
cp $SERVICE_FILE /etc/systemd/system/
systemctl daemon-reload
systemctl start artemis-agent
systemctl enable artemis-agent


echo ""
echo "======================================"
echo "安装完成！已启动，并设置为开机自启"
echo "======================================"
echo ""
echo "安装位置: $INSTALL_DIR"


echo ""
echo "下一步可选操作："
echo "1. 编辑配置文件:"
echo "   vim $INSTALL_DIR/config.json"
echo ""
echo "2. 重启动服务:"
echo "   systemctl restart artemis-agent"
echo ""
echo "3. 查看状态:"
echo "   systemctl status artemis-agent"
echo ""
echo "4. 查看日志:"
echo "   tail -f /var/log/artemis-agent/*.log"
echo ""


