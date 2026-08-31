//go:build !linux

package main

import (
	"context"
	"fmt"

	"artemis_agent/shared"
)

type ExecMonitorWorker struct{}

func NewExecMonitorWorker() *ExecMonitorWorker {
	return &ExecMonitorWorker{}
}

func (w *ExecMonitorWorker) Name() string {
	return "exec_monitor"
}

func (w *ExecMonitorWorker) Start(ctx context.Context, callback shared.DataCallback) error {
	return fmt.Errorf("exec_monitor is only supported on Linux")
}

func (w *ExecMonitorWorker) Stop() error {
	return nil
}

func (w *ExecMonitorWorker) GetEvents() ([]*shared.MonitorData, error) {
	return nil, fmt.Errorf("not supported on this platform")
}

func (w *ExecMonitorWorker) GetMetrics() (map[string]interface{}, error) {
	return map[string]interface{}{
		"supported": false,
		"platform":  "non-linux",
	}, nil
}

func (w *ExecMonitorWorker) Health() bool {
	return false
}

func (w *ExecMonitorWorker) HealthCheck() (bool, string) {
	return false, "not supported on this platform"
}

func main() {
	shared.StartGomwWorker("exec_monitor", "1.0.0", NewExecMonitorWorker())
}
