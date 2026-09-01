package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"artemis_agent/shared"

	"github.com/shirou/gopsutil/v3/host"
)

// ProcessScanTimeout 进程扫描超时
const ProcessScanTimeout = 5 * time.Minute

// ProcessCollector 进程采集器
type ProcessCollector struct {
	systemBootTime uint64
	clockTicks     float64
	pageSize       uint64
}

// ProcessInfo 进程详细信息
type ProcessInfo struct {
	PID        int32
	Name       string
	Cmdline    []string
	Username   string
	CreateTime int64
	Exe        string
	Cwd        string
	PPID       int32
	Status     string
	MemoryMB   float64
	CPUPercent float64
	NumThreads int32
	// NumFDs     int32
	// NumConns   int
}

// NewProcessCollector 创建进程采集器
func NewProcessCollector() *ProcessCollector {
	pc := &ProcessCollector{
		clockTicks: 100,  // 默认 100 Hz，稍后会尝试获取
		pageSize:   4096, // 默认 4KB
	}

	// 获取系统启动时间，用于计算进程启动时间
	if bootTime, err := host.BootTime(); err == nil {
		pc.systemBootTime = bootTime
	}

	// 尝试获取 SC_CLK_TCK
	// 在 Go 中直接获取比较麻烦，通常 Linux 是 100
	// 简单起见，我们假设 100，误差影响 CPU% 的绝对值，但不影响相对大小

	return pc
}

// CollectLoop 定期采集进程信息
func (pc *ProcessCollector) CollectLoop(ctx context.Context, sendEvent func(*shared.MonitorData), mu *sync.Mutex) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Printf("[process_collector] collectLoop started, will collect every 5 minutes")

	time.Sleep(10 * time.Second)
	log.Printf("[process_collector] Starting first collection...")

	if mu != nil {
		mu.Lock()
	}
	pc.scanProcesses(sendEvent)
	if mu != nil {
		mu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[process_collector] collectLoop context cancelled, exiting")
			return
		case <-ticker.C:
			if mu != nil {
				mu.Lock()
			}
			pc.scanProcesses(sendEvent)
			if mu != nil {
				mu.Unlock()
			}
		}
	}
}

// scanProcesses 扫描所有进程
func (pc *ProcessCollector) scanProcesses(sendEvent func(*shared.MonitorData)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[process_collector] panic: %v", r)
		}
	}()

	start := time.Now()

	// 直接读取 /proc 目录获取 PID 列表
	entries, err := os.ReadDir("/proc")
	if err != nil {
		log.Printf("[process_collector] Failed to read /proc: %v", err)
		return
	}

	log.Printf("[process_collector] Starting scan (entries in /proc: %d)", len(entries))

	var reportedCount int
	var skippedCount int

	for _, entry := range entries {
		// 检查超时
		if time.Since(start) > ProcessScanTimeout {
			log.Printf("[process_collector] Scan timeout, stopping")
			break
		}

		if !entry.IsDir() {
			continue
		}

		// 解析 PID
		pid, err := strconv.ParseInt(entry.Name(), 10, 32)
		if err != nil {
			continue // 不是进程目录
		}

		// 采集信息 (高性能版)
		info := pc.fastCollectProcessInfo(int32(pid))
		if info == nil {
			skippedCount++
			continue
		}

		// 过滤内核线程
		if pc.isKernelThread(info) {
			skippedCount++
			continue
		}

		// 过滤 Agent 自身
		if pc.isAgentProcess(info) {
			skippedCount++
			continue
		}

		// 上报
		pc.reportProcess(info, sendEvent)
		reportedCount++
	}

	log.Printf("[process_collector] Scan completed in %v. Reported: %d, Skipped: %d",
		time.Since(start), reportedCount, skippedCount)
}

// fastCollectProcessInfo 高性能采集单个进程信息
// 核心思想：只读取必要的文件，减少 I/O
func (pc *ProcessCollector) fastCollectProcessInfo(pid int32) *ProcessInfo {
	// 1. 读取 /proc/[pid]/stat (一次 I/O 获取 PID, PPID, Name, State, Times, Threads, RSS)
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	statBytes, err := os.ReadFile(statPath)
	if err != nil {
		return nil // 进程可能已退出
	}

	// 解析 stat
	// 格式: pid (comm) state ppid ...
	// 注意 comm 可能包含空格和括号，需要特殊处理
	statStr := string(statBytes)
	lastRParen := strings.LastIndex(statStr, ")")
	if lastRParen == -1 || lastRParen+2 >= len(statStr) {
		return nil
	}

	// 提取 Name (去掉括号)
	name := statStr[strings.Index(statStr, "(")+1 : lastRParen]

	// 解析剩余字段
	fields := strings.Fields(statStr[lastRParen+2:])
	// fields[0] 是 state (因为原字符串是 ") state ppid...")
	// 索引映射:
	// stat[0]=pid, stat[1]=comm, stat[2]=state, stat[3]=ppid
	// 我们的 fields 是从 state 开始的，所以:
	// fields[0] = state
	// fields[1] = ppid
	// fields[11] = utime (14 - 3)
	// fields[12] = stime (15 - 3)
	// fields[17] = num_threads (20 - 3)
	// fields[19] = starttime (22 - 3)
	// fields[21] = rss (24 - 3)

	if len(fields) < 22 {
		return nil
	}

	state := fields[0]
	ppid, _ := strconv.ParseInt(fields[1], 10, 32)
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)
	numThreads, _ := strconv.ParseInt(fields[17], 10, 32)
	starttime, _ := strconv.ParseUint(fields[19], 10, 64)
	rssPages, _ := strconv.ParseUint(fields[21], 10, 64)

	// 计算 CreateTime
	createTime := int64(pc.systemBootTime) + int64(starttime/uint64(pc.clockTicks))

	// 计算 CPU 使用率 (自启动以来的平均值)
	totalTime := float64(utime+stime) / pc.clockTicks
	seconds := time.Now().Unix() - createTime
	cpuPercent := 0.0
	if seconds > 0 {
		cpuPercent = (totalTime / float64(seconds)) * 100
	}

	// 计算 MemoryMB
	memoryMB := float64(rssPages*pc.pageSize) / 1024 / 1024

	// 2. 读取 /proc/[pid]/cmdline (一次 I/O)
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
	cmdlineBytes, err := os.ReadFile(cmdlinePath)
	var cmdline []string
	if err == nil {
		// cmdline 以 \0 分隔
		parts := bytes.Split(cmdlineBytes, []byte{0})
		for _, p := range parts {
			if len(p) > 0 {
				cmdline = append(cmdline, string(p))
			}
		}
	}

	// 3. 读取 /proc/[pid]/status 获取 Uid (一次 I/O)
	// 这一步比较耗时，如果追求极致性能可以只读 status 里的 Uid 行
	// 或者直接读 /proc/[pid]/loginuid (如果是登录用户)
	// 这里我们简化：读 status
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	statusBytes, _ := os.ReadFile(statusPath)
	uid := "0"
	if statusBytes != nil {
		lines := strings.Split(string(statusBytes), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Uid:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					uid = parts[1]
				}
				break
			}
		}
	}
	// 转换 Uid 为 Username (可能涉及 /etc/passwd I/O, 可以加缓存)
	username := pc.lookupUser(uid)

	// 4. 读取 Exe 和 Cwd (readlink, 两次 I/O)
	exe, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	cwd, _ := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))

	// 5. 连接数和 FDs (如果不需要精确值，可以跳过，这非常耗时)
	// 原版代码调用了 numFDs 和 NumConns，这需要遍历 /proc/[pid]/fd
	// 在 1% CPU 下，这绝对是性能杀手！
	// 我们在这里做个取舍：默认不采集 FDs 和 Conns，或者仅对非频繁采集
	// 为了解决用户的问题，我们**暂时设为 0**，或者仅采集 FDs 数量（ReadDir /proc/pid/fd 的长度）
	// numFDs := 0
	// fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	// if err == nil {
	// 	numFDs = len(fds)
	// }
	// 注释掉是为了最大化性能。ReadDir fd 也是很重的。

	return &ProcessInfo{
		PID:        pid,
		Name:       name,
		Cmdline:    cmdline,
		Username:   username,
		CreateTime: createTime,
		Exe:        exe,
		Cwd:        cwd,
		PPID:       int32(ppid),
		Status:     state,
		MemoryMB:   memoryMB,
		CPUPercent: cpuPercent,
		NumThreads: int32(numThreads),
		// NumFDs:     int32(numFDs),
		// NumConns:   0,
	}
}

// 用户名缓存
var userCache = make(map[string]string)
var userCacheMu sync.RWMutex

func (pc *ProcessCollector) lookupUser(uid string) string {
	userCacheMu.RLock()
	if u, ok := userCache[uid]; ok {
		userCacheMu.RUnlock()
		return u
	}
	userCacheMu.RUnlock()

	u, err := user.LookupId(uid)
	name := uid
	if err == nil {
		name = u.Username
	}

	userCacheMu.Lock()
	userCache[uid] = name
	userCacheMu.Unlock()
	return name
}

// isKernelThread 判断是否为内核线程
func (pc *ProcessCollector) isKernelThread(p *ProcessInfo) bool {
	// 1. PPID 为 2 (kthreadd) 的通常是内核线程
	if p.PPID == 2 {
		return true
	}

	// 2. 进程名在方括号中，且没有 cmdline
	if len(p.Name) > 2 && p.Name[0] == '[' && p.Name[len(p.Name)-1] == ']' {
		if len(p.Cmdline) == 0 {
			return true
		}
	}

	return false
}

// isAgentProcess 判断是否为 Agent 自身进程
// 检查 Name、Exe 和 Cmdline 中是否包含 artemis-agent
func (pc *ProcessCollector) isAgentProcess(p *ProcessInfo) bool {

	// 检查命令行
	cmdlineStr := strings.Join(p.Cmdline, " ")
	if strings.Contains(cmdlineStr, "artemis-agent") || strings.Contains(cmdlineStr, "artemis_agent") {
		return true
	}

	return false
}

// reportProcess 上报进程信息
func (pc *ProcessCollector) reportProcess(info *ProcessInfo, sendEvent func(*shared.MonitorData)) {
	severity := pc.calculateSeverity(info)

	// 转换 cmdline 为 []interface{}
	cmdlineInterface := make([]interface{}, len(info.Cmdline))
	for i, v := range info.Cmdline {
		cmdlineInterface[i] = v
	}

	dataMap := map[string]interface{}{
		"pid":         info.PID,
		"ppid":        info.PPID,
		"name":        info.Name,
		"args":        cmdlineInterface,
		"cmdline":     strings.Join(info.Cmdline, " "),
		"exe":         info.Exe,
		"cwd":         info.Cwd,
		"username":    info.Username,
		"create_time": info.CreateTime,
		"status":      info.Status,
		"memory_mb":   info.MemoryMB,
		"cpu_percent": info.CPUPercent,
		"num_threads": info.NumThreads,
		// "num_fds":     info.NumFDs,
		// "num_conns":   info.NumConns,
	}

	event := &shared.MonitorData{
		WorkerName: "process_monitor",
		Timestamp:  time.Now(),
		EventType:  "process_snapshot",
		Severity:   severity,
		Data:       dataMap,
	}

	sendEvent(event)
}

// calculateSeverity 计算严重性
func (pc *ProcessCollector) calculateSeverity(info *ProcessInfo) int {
	severity := 1

	// 可疑进程名
	suspiciousNames := []string{
		"nc", "netcat", "ncat",
		"python", "perl", "ruby", "php",
		"wget", "curl", "bash", "sh",
	}
	for _, suspicious := range suspiciousNames {
		if info.Name == suspicious {
			severity = 3
			break
		}
	}

	// 命令行包含反弹 shell 特征
	cmdStr := strings.Join(info.Cmdline, " ")
	if strings.Contains(cmdStr, "/dev/tcp") || strings.Contains(cmdStr, "bash -i") {
		severity = 5
	}

	return severity
}
