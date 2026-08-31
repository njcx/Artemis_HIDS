//go:build linux

package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
)

// CGroupManager 使用 containerd/cgroups 库的资源管理器（Linux）
type CGroupManager struct {
	mode cgroups.CGMode
	// 用于心跳的 CPU 百分比历史数据（30秒间隔）
	lastCPUUsage map[string]float64 // 上次 CPU 累计使用时间（秒）
	lastCPUTime  map[string]int64   // 上次采样的时间戳（纳秒）
	mu           sync.RWMutex       // 保护历史数据的锁

	// 用于日志的 CPU 百分比历史数据（5分钟间隔）
	logLastCPUUsage map[string]float64
	logLastCPUTime  map[string]int64
	logMu           sync.RWMutex
}

func NewCGroupManager() *CGroupManager {
	mode := cgroups.Mode()
	log.Printf("Detected cgroup mode: %v", mode)
	return &CGroupManager{
		mode:            mode,
		lastCPUUsage:    make(map[string]float64),
		lastCPUTime:     make(map[string]int64),
		logLastCPUUsage: make(map[string]float64),
		logLastCPUTime:  make(map[string]int64),
	}
}

// CreateAndLimit 创建CGroup并限制进程资源（自动适配 v1/v2）
func (c *CGroupManager) CreateAndLimit(name string, pid int, limit ResourceLimit) error {
	// 转换资源限制为 specs 格式
	memoryBytes := int64(limit.MemoryMB * 1024 * 1024)
	cpuQuota := int64(limit.CPUPercent * 1000) // CPU 配额（微秒）
	cpuPeriod := uint64(100000)                // 100ms

	resources := &specs.LinuxResources{
		Memory: &specs.LinuxMemory{
			Limit: &memoryBytes,
		},
		CPU: &specs.LinuxCPU{
			Quota:  &cpuQuota,
			Period: &cpuPeriod,
		},
	}

	switch c.mode {
	case cgroups.Legacy:
		return c.createV1(name, pid, resources, limit)
	case cgroups.Hybrid, cgroups.Unified:
		return c.createV2(name, pid, resources, limit)
	default:
		return fmt.Errorf("unsupported cgroup mode: %v", c.mode)
	}
}

// createV1 使用 cgroup v1 API (containerd/cgroups)
func (c *CGroupManager) createV1(name string, pid int, resources *specs.LinuxResources, limit ResourceLimit) error {
	// 创建 cgroup v1 控制器
	cg, err := cgroup1.New(
		cgroup1.StaticPath("/hids/"+name),
		resources,
	)
	if err != nil {
		return fmt.Errorf("create cgroup v1: %v", err)
	}

	// 添加进程到 cgroup
	if err := cg.Add(cgroup1.Process{Pid: pid}); err != nil {
		return fmt.Errorf("add process to cgroup v1: %v", err)
	}

	log.Printf("CGroup v1 [%s] created for PID %d: Memory=%dMB, CPU=%d%%",
		name, pid, limit.MemoryMB, limit.CPUPercent)
	return nil
}

// createV2 使用 cgroup v2 API (containerd/cgroups)
func (c *CGroupManager) createV2(name string, pid int, resources *specs.LinuxResources, limit ResourceLimit) error {
	// 转换 specs.LinuxResources 到 cgroup2.Resources
	cgroupResources := cgroup2.ToResources(resources)

	// 创建 cgroup v2 管理器
	mgr, err := cgroup2.NewManager(
		"/sys/fs/cgroup",
		"/hids/"+name,
		cgroupResources,
	)
	if err != nil {
		return fmt.Errorf("create cgroup v2 manager: %v", err)
	}

	// 添加进程到 cgroup
	if err := mgr.AddProc(uint64(pid)); err != nil {
		return fmt.Errorf("add process to cgroup v2: %v", err)
	}

	log.Printf("CGroup v2 [%s] created for PID %d: Memory=%dMB, CPU=%d%%",
		name, pid, limit.MemoryMB, limit.CPUPercent)
	return nil
}

// GetResourceUsage 获取进程资源使用情况（用于心跳，30秒间隔）
// 返回: 内存使用(MB), CPU百分比(0-100), 错误
func (c *CGroupManager) GetResourceUsage(name string) (memMB int64, cpuPercent float64, err error) {
	switch c.mode {
	case cgroups.Legacy:
		return c.getUsageV1(name, false)
	case cgroups.Hybrid, cgroups.Unified:
		return c.getUsageV2(name, false)
	default:
		return 0, 0, fmt.Errorf("unsupported cgroup mode: %v", c.mode)
	}
}

// GetResourceUsageForLog 获取进程资源使用情况（用于日志，5分钟间隔）
// 返回: 内存使用(MB), CPU百分比(0-100), 错误
func (c *CGroupManager) GetResourceUsageForLog(name string) (memMB int64, cpuPercent float64, err error) {
	switch c.mode {
	case cgroups.Legacy:
		return c.getUsageV1(name, true)
	case cgroups.Hybrid, cgroups.Unified:
		return c.getUsageV2(name, true)
	default:
		return 0, 0, fmt.Errorf("unsupported cgroup mode: %v", c.mode)
	}
}

// calculateCPUPercent 计算 CPU 百分比
// cpuUsageSec: 当前累计 CPU 使用时间（秒）
// forLog: 是否用于日志（使用独立的历史数据）
// 返回: CPU 百分比 (0-100)
func (c *CGroupManager) calculateCPUPercent(name string, cpuUsageSec float64, forLog bool) float64 {
	now := time.Now().UnixNano()

	var lastUsage float64
	var lastTime int64
	var hasLast bool

	if forLog {
		c.logMu.Lock()
		defer c.logMu.Unlock()

		lastUsage, hasLast = c.logLastCPUUsage[name]
		lastTime = c.logLastCPUTime[name]

		// 更新历史数据
		c.logLastCPUUsage[name] = cpuUsageSec
		c.logLastCPUTime[name] = now
	} else {
		c.mu.Lock()
		defer c.mu.Unlock()

		lastUsage, hasLast = c.lastCPUUsage[name]
		lastTime = c.lastCPUTime[name]

		// 更新历史数据
		c.lastCPUUsage[name] = cpuUsageSec
		c.lastCPUTime[name] = now
	}

	// 如果没有历史数据，返回 0（第一次采样）
	if !hasLast || lastTime == 0 {
		return 0
	}

	// 计算时间间隔（秒）
	timeDeltaSec := float64(now-lastTime) / 1e9
	if timeDeltaSec <= 0 {
		return 0
	}

	// 计算 CPU 使用时间增量
	cpuDelta := cpuUsageSec - lastUsage
	if cpuDelta < 0 {
		// CPU 计数器可能被重置，返回 0
		return 0
	}

	// 计算 CPU 百分比: (CPU时间增量 / 实际时间间隔) * 100
	cpuPercent := (cpuDelta / timeDeltaSec) * 100

	// 限制在合理范围内 (0-100 * CPU核心数，但我们限制单核)
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	if cpuPercent < 0 {
		cpuPercent = 0
	}

	// 保留 2 位小数
	cpuPercent = math.Round(cpuPercent*100) / 100

	return cpuPercent
}

func (c *CGroupManager) getUsageV1(name string, forLog bool) (int64, float64, error) {
	cg, err := cgroup1.Load(cgroup1.StaticPath("/hids/" + name))
	if err != nil {
		return 0, 0, err
	}

	stats, err := cg.Stat(cgroup1.IgnoreNotExist)
	if err != nil {
		return 0, 0, err
	}

	memMB := int64(0)
	if stats.Memory != nil && stats.Memory.Usage != nil {
		memMB = int64(stats.Memory.Usage.Usage / 1024 / 1024)
	}

	cpuUsageSec := float64(0)
	if stats.CPU != nil && stats.CPU.Usage != nil {
		cpuUsageSec = float64(stats.CPU.Usage.Total) / 1e9 // 纳秒转秒
	}

	// 计算真正的 CPU 百分比
	cpuPercent := c.calculateCPUPercent(name, cpuUsageSec, forLog)

	return memMB, cpuPercent, nil
}

func (c *CGroupManager) getUsageV2(name string, forLog bool) (int64, float64, error) {
	mgr, err := cgroup2.Load("/hids/" + name)
	if err != nil {
		return 0, 0, err
	}

	stats, err := mgr.Stat()
	if err != nil {
		return 0, 0, err
	}

	memMB := int64(0)
	if stats.Memory != nil {
		memMB = int64(stats.Memory.Usage / 1024 / 1024)
	}

	cpuUsageSec := float64(0)
	if stats.CPU != nil {
		cpuUsageSec = float64(stats.CPU.UsageUsec) / 1e6 // 微秒转秒
	}

	// 计算真正的 CPU 百分比
	cpuPercent := c.calculateCPUPercent(name, cpuUsageSec, forLog)

	return memMB, cpuPercent, nil
}

// Remove 删除CGroup
func (c *CGroupManager) Remove(name string) error {
	// 清理历史数据
	c.mu.Lock()
	delete(c.lastCPUUsage, name)
	delete(c.lastCPUTime, name)
	c.mu.Unlock()

	c.logMu.Lock()
	delete(c.logLastCPUUsage, name)
	delete(c.logLastCPUTime, name)
	c.logMu.Unlock()

	switch c.mode {
	case cgroups.Legacy:
		cg, err := cgroup1.Load(cgroup1.StaticPath("/hids/" + name))
		if err != nil {
			// 如果 cgroup 不存在，静默忽略
			return nil
		}

		// 先尝试删除，如果失败（因为还有进程），静默忽略
		_ = cg.Delete()
		return nil

	case cgroups.Hybrid, cgroups.Unified:
		mgr, err := cgroup2.Load("/hids/" + name)
		if err != nil {
			// 如果 cgroup 不存在，静默忽略
			return nil
		}

		// 先尝试删除，如果失败（因为还有进程），静默忽略
		_ = mgr.Delete()
		return nil

	default:
		return fmt.Errorf("unsupported cgroup mode: %v", c.mode)
	}
}
