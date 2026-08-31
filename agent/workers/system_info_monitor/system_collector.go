package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"artemis_agent/shared"

	"github.com/elastic/go-sysinfo"
	"github.com/jaypipes/ghw"
)

// 磁盘信息采集超时
const DiskInfoTimeout = 120 * time.Second

// SystemCollector 系统信息采集器
type SystemCollector struct {
}

// NewSystemCollector 创建系统信息采集器
func NewSystemCollector() *SystemCollector {
	return &SystemCollector{}
}

// CollectLoop 定期采集系统信息
func (sc *SystemCollector) CollectLoop(ctx context.Context, sendEvent func(*shared.MonitorData), mu *sync.Mutex) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Printf("[system_collector] collectLoop started, will collect every 1 minute")

	time.Sleep(5 * time.Second)

	if mu != nil {
		mu.Lock()
	}
	sc.collectSystemInfo(sendEvent)
	if mu != nil {
		mu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if mu != nil {
				mu.Lock()
			}
			sc.collectSystemInfo(sendEvent)
			if mu != nil {
				mu.Unlock()
			}
		}
	}
}

// collectSystemInfo 采集系统信息（使用 go-sysinfo）
func (sc *SystemCollector) collectSystemInfo(sendEvent func(*shared.MonitorData)) {
	log.Printf("[system_collector] Collecting system information...")

	defer func() {
		if r := recover(); r != nil {
			errorEvent := &shared.MonitorData{
				WorkerName: "system_monitor",
				Timestamp:  time.Now(),
				EventType:  "error",
				Data: map[string]interface{}{
					"error": fmt.Sprintf("panic: %v", r),
				},
			}
			sendEvent(errorEvent)
		}
	}()

	dataMap := map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}

	// 获取主机信息
	if host, err := sysinfo.Host(); err == nil {
		info := host.Info()
		dataMap["hostname"] = info.Hostname
		dataMap["architecture"] = info.Architecture
		dataMap["os_type"] = info.OS.Type
		dataMap["os_family"] = info.OS.Family
		dataMap["os_platform"] = info.OS.Platform
		dataMap["os_version"] = info.OS.Version
		dataMap["kernel_version"] = info.KernelVersion
		dataMap["timezone"] = info.Timezone
		dataMap["timezone_offset"] = info.TimezoneOffsetSec

		// uptime 以天为单位，保留一位小数
		uptimeDays := time.Since(info.BootTime).Hours() / 24
		dataMap["uptime_days"] = fmt.Sprintf("%.1f", uptimeDays)

		// CPU 信息
		dataMap["cpu_model"] = sc.getCPUModel()

		// 获取CPU详细信息（物理CPU数、核心数等）
		if cpuInfo := sc.getCPUInfo(); len(cpuInfo) > 0 {
			for k, v := range cpuInfo {
				dataMap[k] = v
			}
		}

		if loadAvg := sc.getLoadAverage(); len(loadAvg) > 0 {
			dataMap["load_average_1min"] = loadAvg["1min"]
			dataMap["load_average_5min"] = loadAvg["5min"]
			dataMap["load_average_15min"] = loadAvg["15min"]
			// 计算 CPU 负载百分比（load / cores * 100）
			if load1, ok := loadAvg["1min"].(float64); ok {
				cpuLoadPercent := (load1 / float64(runtime.NumCPU())) * 100
				if cpuLoadPercent > 100 {
					cpuLoadPercent = 100
				}
				dataMap["cpu_load_percent"] = cpuLoadPercent
			}
		}

		// 内存信息（以 GB 为单位，保留一位小数）
		if memory, err := host.Memory(); err == nil {
			// 更准确的内存使用计算：Total - Available
			actualUsed := memory.Total - memory.Available

			memoryTotalGB := float64(memory.Total) / 1024 / 1024 / 1024
			memoryAvailableGB := float64(memory.Available) / 1024 / 1024 / 1024
			memoryUsedGB := float64(actualUsed) / 1024 / 1024 / 1024
			memoryFreeGB := float64(memory.Free) / 1024 / 1024 / 1024

			dataMap["memory_total_gb"] = fmt.Sprintf("%.1f", memoryTotalGB)
			dataMap["memory_available_gb"] = fmt.Sprintf("%.1f", memoryAvailableGB)
			dataMap["memory_used_gb"] = fmt.Sprintf("%.1f", memoryUsedGB)
			dataMap["memory_free_gb"] = fmt.Sprintf("%.1f", memoryFreeGB)
			dataMap["memory_percent"] = fmt.Sprintf("%.1f", float64(actualUsed)/float64(memory.Total)*100)

			// Swap 信息（以 GB 为单位，保留一位小数）
			swapTotalGB := float64(memory.VirtualTotal) / 1024 / 1024 / 1024
			swapUsedGB := float64(memory.VirtualUsed) / 1024 / 1024 / 1024
			swapFreeGB := float64(memory.VirtualFree) / 1024 / 1024 / 1024

			dataMap["swap_total_gb"] = fmt.Sprintf("%.1f", swapTotalGB)
			dataMap["swap_used_gb"] = fmt.Sprintf("%.1f", swapUsedGB)
			dataMap["swap_free_gb"] = fmt.Sprintf("%.1f", swapFreeGB)
		}
	}

	// 网络接口和IP地址信息（简化版，避免阻塞）
	if ips := sc.getLocalIPs(); len(ips) > 0 {
		dataMap["local_ips"] = ips
		dataMap["ip_count"] = len(ips)
	}

	// 磁盘信息（以 GB 为单位，保留一位小数）
	if diskInfo := sc.getDiskInfo(); len(diskInfo) > 0 {
		for k, v := range diskInfo {
			dataMap[k] = v
		}
	}

	// Runtime 统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	dataMap["go_version"] = runtime.Version()
	dataMap["num_goroutine"] = runtime.NumGoroutine()
	dataMap["go_alloc_mb"] = memStats.Alloc / 1024 / 1024
	dataMap["go_sys_mb"] = memStats.Sys / 1024 / 1024
	dataMap["num_gc"] = memStats.NumGC

	event := &shared.MonitorData{
		WorkerName: "system_monitor", // 使用独立的 WorkerName
		Timestamp:  time.Now(),
		EventType:  "system_info",
		Data:       dataMap,
	}

	sendEvent(event)
}

// getLoadAverage 获取系统负载平均值
func (sc *SystemCollector) getLoadAverage() map[string]interface{} {
	result := make(map[string]interface{})

	if runtime.GOOS == "linux" {
		// Linux: 从 /proc/loadavg 读取
		file, err := os.Open("/proc/loadavg")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			if scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) >= 3 {
					if load1, err := strconv.ParseFloat(fields[0], 64); err == nil {
						result["1min"] = load1
					}
					if load5, err := strconv.ParseFloat(fields[1], 64); err == nil {
						result["5min"] = load5
					}
					if load15, err := strconv.ParseFloat(fields[2], 64); err == nil {
						result["15min"] = load15
					}
				}
			}
		}
		return result
	}

	// macOS 和其他系统
	if host, err := sysinfo.Host(); err == nil {
		info := host.Info()

		if runtime.GOOS == "darwin" {
			// macOS: 使用简单的估算，基于运行时间和内存压力
			if memory, err := host.Memory(); err == nil {
				memPressure := float64(memory.Used) / float64(memory.Total)
				estimatedLoad := memPressure * float64(runtime.NumCPU())
				result["1min"] = estimatedLoad
				result["5min"] = estimatedLoad * 0.9
				result["15min"] = estimatedLoad * 0.8
			}
		}

		// 使用 info 避免未使用变量警告
		_ = info
	}

	return result
}

// getLocalIPs 获取本机IP地址（过滤虚拟网络接口）
func (sc *SystemCollector) getLocalIPs() []string {
	var ips []string

	// 使用 net.Interfaces() 获取所有接口，并过滤虚拟网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		// 跳过 Docker 和虚拟网络接口
		if sc.isDockerInterface(iface.Name) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}

	return ips
}

// isDockerInterface 检查是否为 Docker 或虚拟网络接口
func (sc *SystemCollector) isDockerInterface(name string) bool {
	// 常见 Docker 或虚拟接口前缀
	dockerPatterns := []string{
		"docker",
		"veth",
		"br-",   // Docker Compose 创建的 bridge
		"lo",    // 回环
		"virbr", // libvirt
		"lxc",
		"tun",
		"tap",
	}
	for _, p := range dockerPatterns {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// getCPUModel 获取CPU型号
func (sc *SystemCollector) getCPUModel() string {
	if runtime.GOOS == "linux" {
		// Linux: 从 /proc/cpuinfo 读取
		file, err := os.Open("/proc/cpuinfo")
		if err != nil {
			return "unknown"
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// macOS 和其他系统
	return "unknown"
}

// getCPUInfo 获取CPU详细信息（物理CPU数、每个CPU核心数等）
func (sc *SystemCollector) getCPUInfo() map[string]interface{} {
	result := make(map[string]interface{})

	if runtime.GOOS != "linux" {
		return result
	}

	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return result
	}
	defer file.Close()

	// 用于记录物理CPU ID
	physicalIDs := make(map[string]bool)
	coreIDs := make(map[string]bool)
	var coresPerCPU int
	var siblings int

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "physical id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				physicalIDs[strings.TrimSpace(parts[1])] = true
			}
		}

		if strings.HasPrefix(line, "core id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				coreIDs[strings.TrimSpace(parts[1])] = true
			}
		}

		if strings.HasPrefix(line, "cpu cores") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				if cores, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					coresPerCPU = cores
				}
			}
		}

		if strings.HasPrefix(line, "siblings") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				if sib, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					siblings = sib
				}
			}
		}
	}

	// 物理CPU数量
	physicalCPUCount := len(physicalIDs)
	if physicalCPUCount > 0 {
		result["cpu_physical_count"] = physicalCPUCount
	}

	// 每个物理CPU的核心数
	if coresPerCPU > 0 {
		result["cpu_cores_per_cpu"] = coresPerCPU
	}

	// 总物理核心数
	if physicalCPUCount > 0 && coresPerCPU > 0 {
		result["cpu_physical_cores"] = physicalCPUCount * coresPerCPU
	}

	// 逻辑核心数（包含超线程）
	result["cpu_logical_cores"] = runtime.NumCPU()

	// 是否启用超线程
	if siblings > 0 && coresPerCPU > 0 {
		result["cpu_threads_per_core"] = siblings / coresPerCPU
		result["cpu_hyperthreading"] = siblings > coresPerCPU
	}

	return result
}

// getDiskInfo 获取磁盘信息（使用 ghw，带超时保护）
func (sc *SystemCollector) getDiskInfo() map[string]interface{} {
	result := make(map[string]interface{})

	// 使用带超时的磁盘信息采集，防止 NFS/慢速存储阻塞
	done := make(chan *ghw.BlockInfo, 1)
	go func() {
		block, err := ghw.Block()
		if err != nil {
			log.Printf("[system_collector] Failed to get block storage info: %v", err)
			done <- nil
			return
		}
		done <- block
	}()

	var block *ghw.BlockInfo
	select {
	case block = <-done:
		if block == nil {
			return result
		}
	case <-time.After(DiskInfoTimeout):
		log.Printf("[system_collector] WARNING: Disk info collection timeout after %v", DiskInfoTimeout)
		return result
	}

	var totalSize uint64
	var physicalDiskCount int
	var disksInfo []map[string]interface{}

	// 遍历所有磁盘，收集详细信息
	for _, disk := range block.Disks {
		// 跳过环回设备、光驱和可移动设备
		if disk.IsRemovable || strings.HasPrefix(disk.Name, "loop") || strings.HasPrefix(disk.Name, "sr") {
			continue
		}

		physicalDiskCount++
		totalSize += disk.SizeBytes

		diskInfo := map[string]interface{}{
			"name":       disk.Name,
			"size_gb":    fmt.Sprintf("%.1f", float64(disk.SizeBytes)/1024/1024/1024),
			"model":      disk.Model,
			"vendor":     disk.Vendor,
			"serial":     disk.SerialNumber,
			"partitions": len(disk.Partitions),
		}

		// 获取该磁盘所有分区的使用情况
		var partitionsInfo []map[string]interface{}
		var diskUsedTotal uint64
		var diskFreeTotal uint64

		for _, part := range disk.Partitions {
			partInfo := map[string]interface{}{
				"name":        part.Name,
				"size_gb":     fmt.Sprintf("%.1f", float64(part.SizeBytes)/1024/1024/1024),
				"mount_point": part.MountPoint,
				"filesystem":  part.Type,
			}

			// 如果分区已挂载，获取使用情况
			if part.MountPoint != "" {
				used, free, percent := sc.getPartitionUsage(part.MountPoint)
				if used > 0 || free > 0 {
					partInfo["used_gb"] = fmt.Sprintf("%.1f", used)
					partInfo["free_gb"] = fmt.Sprintf("%.1f", free)
					partInfo["percent"] = fmt.Sprintf("%.1f", percent)

					diskUsedTotal += uint64(used * 1024 * 1024 * 1024)
					diskFreeTotal += uint64(free * 1024 * 1024 * 1024)
				}
			}

			partitionsInfo = append(partitionsInfo, partInfo)
		}

		diskInfo["partition_details"] = partitionsInfo

		// 计算整个磁盘的使用情况（所有分区总和）
		if diskUsedTotal > 0 || diskFreeTotal > 0 {
			totalDiskSpace := diskUsedTotal + diskFreeTotal
			diskInfo["used_gb"] = fmt.Sprintf("%.1f", float64(diskUsedTotal)/1024/1024/1024)
			diskInfo["free_gb"] = fmt.Sprintf("%.1f", float64(diskFreeTotal)/1024/1024/1024)
			if totalDiskSpace > 0 {
				diskInfo["percent"] = fmt.Sprintf("%.1f", float64(diskUsedTotal)/float64(totalDiskSpace)*100)
			}
		}

		disksInfo = append(disksInfo, diskInfo)
	}

	// 计算总体使用情况（所有挂载分区）
	totalUsed, totalFree, totalPercent := sc.getAllPartitionsUsage()

	result["disk_count"] = physicalDiskCount
	result["disk_total_gb"] = fmt.Sprintf("%.1f", float64(totalSize)/1024/1024/1024)
	result["disk_used_gb"] = fmt.Sprintf("%.1f", totalUsed)
	result["disk_free_gb"] = fmt.Sprintf("%.1f", totalFree)
	result["disk_percent"] = fmt.Sprintf("%.1f", totalPercent)
	result["disks"] = disksInfo

	return result
}

// getPartitionUsage 获取指定分区的使用情况（使用 syscall.Statfs）
func (sc *SystemCollector) getPartitionUsage(mountPoint string) (usedGB, freeGB, percentUsed float64) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return 0, 0, 0
	}

	var stat syscall.Statfs_t
	err := syscall.Statfs(mountPoint, &stat)
	if err != nil {
		log.Printf("[system_collector] Failed to get partition usage for %s: %v", mountPoint, err)
		return 0, 0, 0
	}

	// 计算磁盘大小（字节）
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	availBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	// 转换为 GB
	usedGB = float64(usedBytes) / 1024 / 1024 / 1024
	freeGB = float64(availBytes) / 1024 / 1024 / 1024

	// 计算使用百分比
	if totalBytes > 0 {
		percentUsed = float64(usedBytes) / float64(totalBytes) * 100
	}

	return usedGB, freeGB, percentUsed
}

// getAllPartitionsUsage 获取所有挂载分区的总使用情况
func (sc *SystemCollector) getAllPartitionsUsage() (totalUsedGB, totalFreeGB, totalPercent float64) {
	if runtime.GOOS != "linux" {
		// 非 Linux 系统返回根分区
		return sc.getPartitionUsage("/")
	}

	// 读取所有挂载点
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return 0, 0, 0
	}
	defer file.Close()

	var totalUsedBytes uint64
	var totalTotalBytes uint64
	var totalFreeBytes uint64
	seenMounts := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		mountPoint := fields[1]
		fsType := fields[2]

		// 跳过虚拟文件系统
		if sc.isVirtualFilesystem(fsType) {
			continue
		}

		// 跳过重复挂载点
		if seenMounts[mountPoint] {
			continue
		}
		seenMounts[mountPoint] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountPoint, &stat); err == nil {
			totalBytes := stat.Blocks * uint64(stat.Bsize)
			freeBytes := stat.Bfree * uint64(stat.Bsize)
			usedBytes := totalBytes - freeBytes

			totalTotalBytes += totalBytes
			totalUsedBytes += usedBytes
			totalFreeBytes += stat.Bavail * uint64(stat.Bsize)
		}
	}

	// 转换为 GB
	totalUsedGB = float64(totalUsedBytes) / 1024 / 1024 / 1024
	totalFreeGB = float64(totalFreeBytes) / 1024 / 1024 / 1024

	// 计算总百分比
	if totalTotalBytes > 0 {
		totalPercent = float64(totalUsedBytes) / float64(totalTotalBytes) * 100
	}

	return totalUsedGB, totalFreeGB, totalPercent
}

// isVirtualFilesystem 检查是否为虚拟文件系统
func (sc *SystemCollector) isVirtualFilesystem(fsType string) bool {
	virtualFS := []string{
		"proc", "sysfs", "devtmpfs", "devpts", "tmpfs",
		"cgroup", "cgroup2", "pstore", "bpf", "tracefs",
		"debugfs", "securityfs", "sockfs", "pipefs",
		"ramfs", "hugetlbfs", "rpc_pipefs", "autofs",
	}
	for _, vfs := range virtualFS {
		if fsType == vfs {
			return true
		}
	}
	return false
}
