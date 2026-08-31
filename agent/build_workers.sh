#!/bin/bash
set -e

echo "编译 HIDS Agent Workers (静态编译，跨平台兼容)..."
echo "================================"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 设置静态编译参数
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

echo "编译配置: CGO_ENABLED=0 GOOS=linux GOARCH=amd64"
echo ""

# 编译 file_monitor (从主模块目录构建，才能正确找到 shared 包)
echo "编译 file_monitor..."
go build -ldflags "-s -w" -o workers/file ./workers/file_monitor/
if [ -f "workers/file" ]; then
    echo "[OK] file_monitor 编译完成: $(ls -lh workers/file | awk '{print $5}')"
else
    echo "[FAILED] file_monitor 编译失败"
    exit 1
fi

# 编译 network_monitor
echo ""
echo "编译 network_monitor..."
go build -ldflags "-s -w" -o workers/network ./workers/network_monitor/
if [ -f "workers/network" ]; then
    echo "[OK] network_monitor 编译完成: $(ls -lh workers/network | awk '{print $5}')"
else
    echo "[FAILED] network_monitor 编译失败"
    exit 1
fi

# 编译 security_log_monitor
echo ""
echo "编译 security_log_monitor..."
go build -ldflags "-s -w" -o workers/log ./workers/security_log_monitor/
if [ -f "workers/log" ]; then
    echo "[OK] security_log_monitor 编译完成: $(ls -lh workers/log | awk '{print $5}')"
else
    echo "[FAILED] security_log_monitor 编译失败"
    exit 1
fi

# 编译 exec_monitor (自动降级：audit -> netlink)
echo ""
echo "编译 exec_monitor..."
go build -ldflags "-s -w" -o workers/exec ./workers/exec_monitor/
if [ -f "workers/exec" ]; then
    echo "[OK] exec_monitor 编译完成: $(ls -lh workers/exec | awk '{print $5}')"
else
    echo "[FAILED] exec_monitor 编译失败"
    exit 1
fi


# 编译 system_info_monitor
echo ""
echo "编译 system_info_monitor..."
go build -ldflags "-s -w" -o workers/system ./workers/system_info_monitor/
if [ -f "workers/system" ]; then
    echo "[OK] system_info_monitor 编译完成: $(ls -lh workers/system | awk '{print $5}')"
else
    echo "[FAILED] system_info_monitor 编译失败"
    exit 1
fi


# ============================================
# 新拆分的 Workers (原 system_info_monitor)
# ============================================

# # 编译 port_monitor (端口监听采集)
# echo ""
# echo "编译 port_monitor..."
# go build -ldflags "-s -w" -o workers/port ./workers/port_monitor/
# if [ -f "workers/port" ]; then
#     echo "[OK] port_monitor 编译完成: $(ls -lh workers/port | awk '{print $5}')"
# else
#     echo "[FAILED] port_monitor 编译失败"
#     exit 1
# fi

# # 编译 system_monitor (系统信息采集)
# echo ""
# echo "编译 system_monitor..."
# go build -ldflags "-s -w" -o workers/system ./workers/system_monitor/
# if [ -f "workers/system" ]; then
#     echo "[OK] system_monitor 编译完成: $(ls -lh workers/system | awk '{print $5}')"
# else
#     echo "[FAILED] system_monitor 编译失败"
#     exit 1
# fi

# # 编译 package_monitor (软件包采集)
# echo ""
# echo "编译 package_monitor..."
# go build -ldflags "-s -w" -o workers/package ./workers/package_monitor/
# if [ -f "workers/package" ]; then
#     echo "[OK] package_monitor 编译完成: $(ls -lh workers/package | awk '{print $5}')"
# else
#     echo "[FAILED] package_monitor 编译失败"
#     exit 1
# fi

# # 编译 language_monitor (语言运行时采集)
# echo ""
# echo "编译 language_monitor..."
# go build -ldflags "-s -w" -o workers/language ./workers/language_monitor/
# if [ -f "workers/language" ]; then
#     echo "[OK] language_monitor 编译完成: $(ls -lh workers/language | awk '{print $5}')"
# else
#     echo "[FAILED] language_monitor 编译失败"
#     exit 1
# fi

# # 编译 container_monitor (容器采集)
# echo ""
# echo "编译 container_monitor..."
# go build -ldflags "-s -w" -o workers/container ./workers/container_monitor/
# if [ -f "workers/container" ]; then
#     echo "[OK] container_monitor 编译完成: $(ls -lh workers/container | awk '{print $5}')"
# else
#     echo "[FAILED] container_monitor 编译失败"
#     exit 1
# fi

# # 编译 process_monitor (进程快照采集)
# echo ""
# echo "编译 process_monitor..."
# go build -ldflags "-s -w" -o workers/process ./workers/process_monitor/
# if [ -f "workers/process" ]; then
#     echo "[OK] process_monitor 编译完成: $(ls -lh workers/process | awk '{print $5}')"
# else
#     echo "[FAILED] process_monitor 编译失败"
#     exit 1
# fi


echo ""
echo "================================"
echo "所有 Workers 编译完成!"
echo ""

echo "================================"

echo ""
echo "测试单个 Worker (独立模式):"
echo "  ./workers/file --standalone"
echo "  ./workers/network --standalone"
echo "  ./workers/log --standalone"
echo "  ./workers/exec --standalone"
# echo "  ./workers/port --standalone"
echo "  ./workers/system --standalone"
# echo "  ./workers/package --standalone"
# echo "  ./workers/language --standalone"
# echo "  ./workers/container --standalone"
# echo "  ./workers/process --standalone"
echo ""
