#!/bin/bash
# ============================================
# 测试示例脚本
# ============================================

echo "HIDS Test Client - Usage Examples"
echo ""

# 1. 基本测试
echo "1. Basic test with default settings:"
echo "   ./test-client"
echo ""

# 2. 快速测试
echo "2. Fast test (1 second batch interval, 5 events per batch):"
echo "   ./test-client -batch 1 -batch-size 5"
echo ""

# 3. 高负载测试
echo "3. High load test (50 events per second):"
echo "   ./test-client -batch 1 -batch-size 50"
echo ""

# 4. 只测试特定类型
echo "4. Test only file events:"
echo "   ./test-client -process=false -network=false"
echo ""

# 5. 自定义 Agent 信息
echo "5. Custom agent info:"
echo "   ./test-client -agent-id my-agent -hostname my-host"
echo ""

# 6. 连接远程服务器
echo "6. Connect to remote server:"
echo "   ./test-client -server 192.168.1.100:50051"
echo ""

# 7. 查看帮助
echo "7. Show help:"
echo "   ./test-client -h"
echo ""

# 8. 多客户端压测
echo "8. Multiple clients stress test:"
echo "   ./run_multiple.sh 10    # Start 10 clients"
echo ""
