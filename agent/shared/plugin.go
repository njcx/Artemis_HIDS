package shared

import (
	"context"
	"log"
	"os"
	"syscall"
	"time"
)

// MonitorData 监控数据
type MonitorData struct {
	WorkerName string                 `json:"worker_name"`
	Timestamp  time.Time              `json:"timestamp"`
	EventType  string                 `json:"event_type"`
	Severity   int                    `json:"severity"` // 1-5: info, low, medium, high, critical
	Data       map[string]interface{} `json:"data"`
}

// DataCallback 数据回调接口(Master传给Worker用于发送数据)
type DataCallback interface {
	Send(data *MonitorData) error
}

// MonitorWorker 监控Worker接口
type MonitorWorker interface {
	// Name 返回Worker名称
	Name() string

	// Start 启动监控
	Start(ctx context.Context, callback DataCallback) error

	// Stop 停止监控
	Stop() error

	// GetEvents 获取待处理的事件（拉取模式）
	GetEvents() ([]*MonitorData, error)

	// GetMetrics 获取性能指标
	GetMetrics() (map[string]interface{}, error)

	// Health 健康检查
	Health() bool
}

// ============================================
// 父进程监控
// ============================================

// WatchParentProcess 监控父进程，如果父进程死亡则强制退出
// 这确保了即使 master 被 kill -9，worker 也能自动清理
func WatchParentProcess(ctx context.Context) {
	// 可通过环境变量 DISABLE_PARENT_WATCH=1 禁用
	if os.Getenv("DISABLE_PARENT_WATCH") == "1" {
		log.Printf("[Worker] Parent process watching is DISABLED (DISABLE_PARENT_WATCH=1)")
		<-ctx.Done()
		return
	}

	// 等待 5 秒，确保 worker 完全启动并与 master 建立连接
	time.Sleep(5 * time.Second)

	parentPID := os.Getppid()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	log.Printf("[Worker] Watching parent process (PID: %d)", parentPID)

	// 用于强制退出的通道
	forceExit := make(chan struct{})

	// 单独的 goroutine 处理强制退出，避免被其他阻塞的 goroutine 影响
	go func() {
		<-forceExit
		log.Printf("[Worker] Force killing process...")
		// 使用 SIGKILL 强制杀死自己，确保一定退出
		pid := os.Getpid()
		syscall.Kill(pid, syscall.SIGKILL)
	}()

	checkCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Worker] Context cancelled, stop watching parent")
			return
		case <-ticker.C:
			currentPPID := os.Getppid()

			// 只检查 PPID 是否变成 1 (被 init/systemd 接管)
			// 这是最可靠的检测父进程死亡的方法
			if currentPPID == 1 {
				checkCount++
				log.Printf("[Worker] Parent process died (PPID became 1), check count: %d", checkCount)

				// 连续检测到 2 次 PPID=1，确认父进程确实死亡
				if checkCount >= 2 {
					log.Printf("[Worker] Parent confirmed dead, initiating shutdown...")

					// 1. 先尝试优雅退出（给 3 秒时间）
					go func() {
						time.Sleep(3 * time.Second)
						log.Printf("[Worker] Graceful shutdown timeout, forcing exit...")
						close(forceExit)
					}()

					// 2. 立即 exit，大部分情况下会成功
					log.Printf("[Worker] Exiting now...")
					os.Exit(0)
				}
			} else {
				// PPID 不是 1，重置计数器
				if checkCount > 0 {
					log.Printf("[Worker] Parent process recovered (PPID=%d), reset check count", currentPPID)
					checkCount = 0
				}
			}
		}
	}
}
