//go:build !linux
// +build !linux

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"artemis_agent/shared"
)

// FileMonitorWorker 文件监控Worker (macOS/Windows fallback - 不可用)
type FileMonitorWorker struct{}

func NewFileMonitorWorker() *FileMonitorWorker {
	return &FileMonitorWorker{}
}

func (w *FileMonitorWorker) Name() string {
	return "file_monitor"
}

func (w *FileMonitorWorker) Start(ctx context.Context, callback shared.DataCallback) error {
	log.SetOutput(os.Stderr)
	return fmt.Errorf("file_monitor with fanotify is only supported on Linux")
}

func (w *FileMonitorWorker) Stop() error {
	return nil
}

func (w *FileMonitorWorker) GetEvents() ([]*shared.MonitorData, error) {
	return nil, nil
}

func (w *FileMonitorWorker) GetMetrics() (map[string]interface{}, error) {
	return map[string]interface{}{
		"error": "fanotify not supported on this platform",
	}, nil
}

func (w *FileMonitorWorker) Health() bool {
	return false
}

func (w *FileMonitorWorker) HealthCheck() (bool, string) {
	return false, "fanotify only supported on Linux"
}

// 实现 gomw 接口
func main() {
	shared.StartGomwWorker("file_monitor", "1.0.0", NewFileMonitorWorker())
}
