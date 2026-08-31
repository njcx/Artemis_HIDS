#!/bin/bash
# ============================================
# 启动多个测试客户端进行压测
# ============================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 参数
NUM_CLIENTS=${1:-5}
SERVER_ADDR=${2:-"localhost:50051"}
BATCH_INTERVAL=${3:-5}
BATCH_SIZE=${4:-10}

echo "Starting $NUM_CLIENTS test clients..."
echo "Server: $SERVER_ADDR"
echo "Batch Interval: ${BATCH_INTERVAL}s"
echo "Batch Size: $BATCH_SIZE events"

# 创建日志目录
mkdir -p logs

# 清理旧日志
rm -f logs/client-*.log

# 启动客户端
for i in $(seq 1 $NUM_CLIENTS); do
  AGENT_ID=$(printf "test-agent-%03d" $i)
  HOSTNAME=$(printf "test-host-%03d" $i)

  ./test-client \
    -server "$SERVER_ADDR" \
    -agent-id "$AGENT_ID" \
    -hostname "$HOSTNAME" \
    -batch "$BATCH_INTERVAL" \
    -batch-size "$BATCH_SIZE" \
    > "logs/client-$i.log" 2>&1 &

  PID=$!
  echo "Started client $i (PID: $PID, Agent: $AGENT_ID)"

  # 避免同时启动太多
  sleep 0.5
done

echo ""
echo "All clients started!"
echo "Logs: logs/client-*.log"
echo ""
echo "To stop all clients:"
echo "pkill -f test-client"
echo ""
echo "To view logs:"
echo "tail -f logs/client-1.log"
