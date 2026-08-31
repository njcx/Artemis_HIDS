//go:build !linux

package main

import (
	"fmt"
	"log"
)

// CGroupManager Stub 实现（非 Linux 系统）
type CGroupManager struct{}

func NewCGroupManager() *CGroupManager {
	log.Printf("CGroup not supported on this platform (macOS/Windows)")
	return &CGroupManager{}
}

func (c *CGroupManager) CreateAndLimit(name string, pid int, limit ResourceLimit) error {
	log.Printf("Warning: CGroup not available on this platform, skipping resource limits for %s", name)
	return nil
}

func (c *CGroupManager) GetResourceUsage(name string) (memMB int64, cpuPercent float64, err error) {
	return 0, 0, fmt.Errorf("cgroup not supported on this platform")
}

func (c *CGroupManager) GetResourceUsageForLog(name string) (memMB int64, cpuPercent float64, err error) {
	return 0, 0, fmt.Errorf("cgroup not supported on this platform")
}

func (c *CGroupManager) Remove(name string) error {
	return nil
}
