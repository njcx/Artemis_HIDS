//go:build linux

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"artemis_agent/shared"

	"github.com/elastic/go-libaudit/v2"
)

// ExecMonitorWorker 进程执行监控 Worker (自动降级：audit -> netlink)
type ExecMonitorWorker struct {
	ctx             context.Context
	cancel          context.CancelFunc
	healthy         atomic.Bool
	eventCount      atomic.Uint64
	suspiciousCount atomic.Uint64
	startTime       time.Time
	mu              sync.RWMutex
	callback        shared.DataCallback

	// 监控模式
	mode string // "audit" 或 "netlink"

	// Audit 相关
	auditMonitor *AuditMonitor

	// Netlink 相关
	netlinkMonitor *NetlinkMonitor
}

func NewExecMonitorWorker() *ExecMonitorWorker {
	w := &ExecMonitorWorker{
		startTime: time.Now(),
	}
	w.healthy.Store(true)
	return w
}

func (w *ExecMonitorWorker) Name() string {
	return "exec_monitor"
}

func (w *ExecMonitorWorker) Start(ctx context.Context, callback shared.DataCallback) error {
	log.SetOutput(os.Stderr)
	log.Println("[ExecMonitor] Starting...")

	w.callback = callback
	w.ctx, w.cancel = context.WithCancel(ctx)

	if os.Geteuid() != 0 {
		return fmt.Errorf("exec_monitor requires root privileges")
	}

	// 尝试使用 Audit (优先级高)
	log.Println("[ExecMonitor] Attempting to use audit subsystem...")
	auditMonitor, err := NewAuditMonitor(w.ctx, callback, &w.eventCount, &w.suspiciousCount)
	if err != nil {
		log.Printf("[ExecMonitor] Audit initialization failed: %v", err)
		log.Println("[ExecMonitor] Falling back to netlink-connector...")

		// 降级到 Netlink
		netlinkMonitor, err := NewNetlinkMonitor(w.ctx, callback, &w.eventCount, &w.suspiciousCount)
		if err != nil {
			return fmt.Errorf("both audit and netlink failed: audit=%v, netlink=%v", err, err)
		}

		w.netlinkMonitor = netlinkMonitor
		w.mode = "netlink"

		if err := w.netlinkMonitor.Start(); err != nil {
			return fmt.Errorf("failed to start netlink monitor: %v", err)
		}

		log.Println("[ExecMonitor] Started successfully using NETLINK-CONNECTOR")
	} else {
		w.auditMonitor = auditMonitor
		w.mode = "audit"

		if err := w.auditMonitor.Start(); err != nil {
			log.Printf("[ExecMonitor] Audit start failed: %v, falling back to netlink...", err)

			// 降级到 Netlink
			netlinkMonitor, err := NewNetlinkMonitor(w.ctx, callback, &w.eventCount, &w.suspiciousCount)
			if err != nil {
				return fmt.Errorf("both audit and netlink failed: audit=%v, netlink=%v", err, err)
			}

			w.netlinkMonitor = netlinkMonitor
			w.mode = "netlink"

			if err := w.netlinkMonitor.Start(); err != nil {
				return fmt.Errorf("failed to start netlink monitor: %v", err)
			}

			log.Println("[ExecMonitor] Started successfully using NETLINK-CONNECTOR (fallback)")
		} else {
			log.Println("[ExecMonitor] Started successfully using AUDIT")
		}
	}

	go shared.WatchParentProcess(w.ctx)

	log.Printf("[ExecMonitor] Running in %s mode", w.mode)
	return nil
}

func (w *ExecMonitorWorker) Stop() error {
	log.Println("[ExecMonitor] Stopping...")

	if w.cancel != nil {
		w.cancel()
	}

	if w.auditMonitor != nil {
		w.auditMonitor.Stop()
	}

	if w.netlinkMonitor != nil {
		w.netlinkMonitor.Stop()
	}

	w.healthy.Store(false)
	log.Println("[ExecMonitor] Stopped")
	return nil
}

func (w *ExecMonitorWorker) HealthCheck() (bool, string) {
	if !w.healthy.Load() {
		return false, "worker not healthy"
	}

	uptime := time.Since(w.startTime)
	eventCount := w.eventCount.Load()
	suspiciousCount := w.suspiciousCount.Load()

	status := fmt.Sprintf("mode=%s, uptime=%s, events=%d, suspicious=%d",
		w.mode, uptime.Round(time.Second), eventCount, suspiciousCount)

	return true, status
}

// GetEvents 实现 MonitorWorker 接口的必需方法（用于兼容性）
func (w *ExecMonitorWorker) GetEvents() ([]*shared.MonitorData, error) {
	return nil, nil
}

// GetMetrics 获取性能指标
func (w *ExecMonitorWorker) GetMetrics() (map[string]interface{}, error) {
	return map[string]interface{}{
		"mode":             w.mode,
		"event_count":      w.eventCount.Load(),
		"suspicious_count": w.suspiciousCount.Load(),
		"uptime":           time.Since(w.startTime).Seconds(),
	}, nil
}

// Health 健康检查
func (w *ExecMonitorWorker) Health() bool {
	return w.healthy.Load()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--standalone" {
		shared.StartStandalone(NewExecMonitorWorker())
		return
	}
	shared.StartGomwWorker("exec_monitor", "1.0.0", NewExecMonitorWorker())
}

// tryAuditClient 尝试创建 audit 客户端
func tryAuditClient() (*libaudit.AuditClient, error) {
	client, err := libaudit.NewAuditClient(nil)
	if err != nil {
		return nil, err
	}

	// 测试是否能设置 enabled
	status, err := client.GetStatus()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to get audit status: %v", err)
	}

	// 检查是否有权限启用 audit
	if status.Enabled == 0 {
		if err := client.SetEnabled(true, libaudit.NoWait); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to enable audit: %v", err)
		}
	}

	return client, nil
}
