//go:build linux
// +build linux

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"artemis_agent/shared"

	"github.com/s3rj1k/go-fanotify/fanotify"
	"golang.org/x/sys/unix"
)

// 超时配置
const (
	HashCalculateTimeout = 10 * time.Second // 文件哈希计算超时
	MaxHashFileSize      = 50 * 1024 * 1024 // 最大哈希文件大小 50MB
	ProcessInfoTimeout   = 5 * time.Second  // 进程信息获取超时
	EventHandleTimeout   = 30 * time.Second // 单个事件处理超时
)

// ProcessInfo 进程详细信息
type ProcessInfo struct {
	PID        int      `json:"pid"`
	Name       string   `json:"name"`
	Cmdline    []string `json:"cmdline"`
	Exe        string   `json:"exe"`
	Cwd        string   `json:"cwd"`
	UID        int      `json:"uid"`
	GID        int      `json:"gid"`
	User       string   `json:"user"`
	PPID       int      `json:"ppid"`
	ParentName string   `json:"parent_name"`
}

// FileMonitorWorker 文件监控Worker (使用 fanotify)
type FileMonitorWorker struct {
	ctx        context.Context
	cancel     context.CancelFunc
	notify     *fanotify.NotifyFD
	healthy    atomic.Bool
	eventCount atomic.Uint64
	startTime  time.Time
	mu         sync.RWMutex
	callback   shared.DataCallback

	// 事件缓冲队列
	eventQueue   []*shared.MonitorData
	queueMutex   sync.Mutex
	maxQueueSize int

	// 监控的挂载点
	mountPoints []string

	//  新增：统计信息
	sendSuccessCount atomic.Uint64
	sendErrorCount   atomic.Uint64
}

func NewFileMonitorWorker() *FileMonitorWorker {
	w := &FileMonitorWorker{
		startTime:    time.Now(),
		eventQueue:   make([]*shared.MonitorData, 0, 1000),
		maxQueueSize: 1000,
		// fanotify 监控整个挂载点，不需要像 fsnotify 那样逐个目录添加
		mountPoints: []string{
			"/", // 监控根文件系统
		},
	}
	w.healthy.Store(true)
	return w
}

func (w *FileMonitorWorker) Name() string {
	return "file_monitor"
}

func (w *FileMonitorWorker) Start(ctx context.Context, callback shared.DataCallback) error {
	// 确保 log 输出到 stderr，以便被主进程捕获
	log.SetOutput(os.Stderr)

	log.Println("[FileMonitor] Starting with fanotify...")

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.callback = callback

	//  新增：检查 callback 状态
	if w.callback == nil {
		log.Println("[FileMonitor]  Callback parameter is nil")
		log.Println("[FileMonitor] This is normal in RPC mode - real callback via broker")
	} else {
		log.Println("[FileMonitor] Callback parameter received")
	}

	// 启动父进程监控
	go shared.WatchParentProcess(w.ctx)

	//  新增：启动健康监控
	go w.monitorHealth()

	// 初始化 fanotify（使用最基础的标志以获得最大兼容性）
	// FAN_CLASS_NOTIF: 只通知，不阻塞
	var err error
	w.notify, err = fanotify.Initialize(
		unix.FAN_CLASS_NOTIF,
		unix.O_RDONLY,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize fanotify: %v", err)
	}

	// 添加监控标记
	successCount := 0
	for _, mountPoint := range w.mountPoints {
		if err := w.addWatchMountPoint(mountPoint); err != nil {
			log.Printf("[FileMonitor] Warning: failed to watch %s: %v", mountPoint, err)
			// 对于根文件系统，尝试监控重要子目录
			if mountPoint == "/" {
				log.Printf("[FileMonitor] Falling back to monitoring important directories...")
				importantDirs := []string{"/etc", "/root", "/home", "/var/log", "/opt", "/usr/local"}
				for _, dir := range importantDirs {
					if _, err := os.Stat(dir); err == nil {
						if err := w.addWatchMountPoint(dir); err == nil {
							successCount++
						}
					}
				}
			}
		} else {
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to watch any paths with fanotify")
	}

	// 启动事件处理循环
	go w.eventLoop()

	log.Printf("[FileMonitor] Started successfully with fanotify, watching %d mount points", len(w.mountPoints))
	return nil
}

// 新增：健康监控
func (w *FileMonitorWorker) monitorHealth() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	callbackNilWarnings := 0

	for {
		select {
		case <-w.ctx.Done():
			log.Println("[FileMonitor] Health monitor stopped")
			return
		case <-ticker.C:
			// 检查 callback 状态
			if w.callback == nil {
				callbackNilWarnings++
				if callbackNilWarnings <= 3 {
					log.Printf("[FileMonitor]   Callback is NIL (warning #%d)", callbackNilWarnings)
					log.Printf("[FileMonitor] Events queued but not sent: %d", len(w.eventQueue))
				}
			} else {
				if callbackNilWarnings > 0 {
					log.Printf("[FileMonitor]  Callback became available (after %d warnings)", callbackNilWarnings)
					callbackNilWarnings = 0
				}
			}

			// 打印统计信息
			total := w.eventCount.Load()
			success := w.sendSuccessCount.Load()
			errors := w.sendErrorCount.Load()
			queued := uint64(len(w.eventQueue))

			log.Printf("[FileMonitor] Stats - Total events: %d, Sent: %d, Errors: %d, Queued: %d",
				total, success, errors, queued)

			// 如果队列积压过多，发出警告
			if queued > 500 {
				log.Printf("[FileMonitor]   Queue size is high (%d), possible send problem", queued)
			}
		}
	}
}

func (w *FileMonitorWorker) addWatchMountPoint(mountPoint string) error {
	// 检查挂载点是否存在
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		return fmt.Errorf("mount point does not exist: %s", mountPoint)
	}

	// 只监控文件写入关闭事件（最关键且权限要求最低）
	eventMask := uint64(unix.FAN_CLOSE_WRITE)

	// 尝试多种标记策略（按优先级降级）
	markStrategies := []struct {
		flags uint
		desc  string
	}{
		// 策略1: FAN_MARK_MOUNT - 监控整个挂载点
		{unix.FAN_MARK_ADD | unix.FAN_MARK_MOUNT, "FAN_MARK_MOUNT"},
		// 策略2: 普通标记 - 只监控指定目录
		{unix.FAN_MARK_ADD, "FAN_MARK_ADD"},
	}

	var lastErr error
	for _, strategy := range markStrategies {
		err := w.notify.Mark(
			strategy.flags,
			eventMask,
			unix.AT_FDCWD,
			mountPoint,
		)
		if err == nil {
			log.Printf("[FileMonitor] Successfully watching %s using %s", mountPoint, strategy.desc)
			return nil
		}
		lastErr = err
		log.Printf("[FileMonitor] Failed to mark with %s: %v, trying next strategy...", strategy.desc, err)
	}

	return fmt.Errorf("all marking strategies failed: %v", lastErr)
}

func (w *FileMonitorWorker) eventLoop() {
	log.Printf("[FileMonitor] eventLoop started, monitoring file system events with fanotify...")

	// 跳过自己的 PID，避免监控自身
	selfPID := os.Getpid()

	for {
		select {
		case <-w.ctx.Done():
			if w.notify != nil && w.notify.File != nil {
				w.notify.File.Close()
			}
			return

		default:
			// 获取 fanotify 事件（非阻塞，设置超时）
			event, err := w.notify.GetEvent(selfPID)
			if err != nil {
				// 检查是否是 context 取消导致的错误
				if w.ctx.Err() != nil {
					return
				}
				log.Printf("[FileMonitor] GetEvent error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if event != nil {
				w.handleFanotifyEvent(event)
				// 必须关闭事件的文件描述符，防止泄漏
				event.Close()
			}
		}
	}
}

func (w *FileMonitorWorker) handleFanotifyEvent(event *fanotify.EventMetadata) {
	w.eventCount.Add(1)

	// 获取文件路径
	path, err := event.GetPath()
	if err != nil {
		log.Printf("[FileMonitor] Failed to get path for FD %d: %v", event.Fd, err)
		return
	}

	// 过滤不感兴趣的路径
	if w.shouldSkipPath(path) {
		return
	}

	// 确定事件类型
	eventType := w.getFanotifyEventType(event)
	if eventType == "" {
		return
	}

	// 计算严重性
	severity := w.calculateFileSeverity(path, eventType)

	// 获取进程详细信息
	pid := event.GetPID()
	processInfo := w.getProcessInfo(pid)

	// 检查是否需要计算文件哈希
	fileHash := ""
	if w.shouldCalculateHash(path) {
		if hash, err := w.calculateFileHash(path); err == nil {
			fileHash = hash
		}
	}

	data := &shared.MonitorData{
		WorkerName: w.Name(),
		Timestamp:  time.Now(),
		EventType:  eventType,
		Severity:   severity,
		Data: map[string]interface{}{
			"path":         path,
			"operation":    eventType,
			"filename":     filepath.Base(path),
			"directory":    filepath.Dir(path),
			"file_hash":    fileHash,
			"pid":          processInfo.PID,
			"process_name": processInfo.Name,
			"cmdline":      processInfo.Cmdline,
			"exe":          processInfo.Exe,
			"cwd":          processInfo.Cwd,
			"uid":          processInfo.UID,
			"gid":          processInfo.GID,
			"user":         processInfo.User,
			"ppid":         processInfo.PPID,
			"parent_name":  processInfo.ParentName,
		},
	}

	if severity >= 3 {
		log.Printf("[FileMonitor] High severity event: %s %s (severity: %d, pid: %d, process: %s, user: %s)",
			eventType, path, severity, pid, processInfo.Name, processInfo.User)
	}

	// 主动推送事件到 Master
	if w.callback != nil {
		if err := w.callback.Send(data); err != nil {
			//  改进：更详细的错误日志
			w.sendErrorCount.Add(1)
			log.Printf("[FileMonitor]  Failed to send event #%d: %v", w.eventCount.Load(), err)

			// 发送失败，添加到队列作为备份
			w.queueMutex.Lock()
			if len(w.eventQueue) < w.maxQueueSize {
				w.eventQueue = append(w.eventQueue, data)
			} else {
				log.Printf("[FileMonitor]   Queue full, dropping oldest event")
			}
			w.queueMutex.Unlock()
		} else {
			//  新增：成功计数
			w.sendSuccessCount.Add(1)

			// 只在关键事件时打印日志，避免日志过多
			if severity >= 3 || w.sendSuccessCount.Load()%100 == 0 {
				log.Printf("[FileMonitor]  Event sent (#%d, total success: %d)",
					w.eventCount.Load(), w.sendSuccessCount.Load())
			}
		}
	} else {
		//  改进：callback 为 nil 时的处理
		log.Printf("[FileMonitor]  Callback is nil, queuing event (total queued: %d)", len(w.eventQueue)+1)

		w.queueMutex.Lock()
		if len(w.eventQueue) < w.maxQueueSize {
			w.eventQueue = append(w.eventQueue, data)
		} else {
			// 队列满了，丢弃最旧的事件
			w.eventQueue = append(w.eventQueue[1:], data)
		}
		w.queueMutex.Unlock()
	}
}

// shouldSkipPath 判断是否应该跳过该路径
func (w *FileMonitorWorker) shouldSkipPath(path string) bool {
	// 跳过 /proc, /sys, /dev 等虚拟文件系统
	skipPrefixes := []string{
		"/proc/",
		"/sys/",
		"/dev/",
		"/run/",
	}

	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// 跳过一些高频但不重要的文件
	skipPatterns := []string{
		".swp",
		".tmp",
		"~",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// getFanotifyEventType 将 fanotify 事件掩码转换为事件类型字符串
func (w *FileMonitorWorker) getFanotifyEventType(event *fanotify.EventMetadata) string {
	// 我们只监控 FAN_CLOSE_WRITE 事件
	if event.MatchMask(unix.FAN_CLOSE_WRITE) {
		return "file_modified"
	}
	return ""
}

// getProcessInfo 获取进程的详细信息（带超时保护）
func (w *FileMonitorWorker) getProcessInfo(pid int) ProcessInfo {
	info := ProcessInfo{
		PID:        pid,
		Name:       "unknown",
		Cmdline:    []string{},
		Exe:        "",
		Cwd:        "",
		UID:        -1,
		GID:        -1,
		User:       "unknown",
		PPID:       -1,
		ParentName: "unknown",
	}

	if pid <= 0 {
		return info
	}

	// 使用带超时的 goroutine 获取进程信息
	resultChan := make(chan ProcessInfo, 1)

	go func() {
		result := w.getProcessInfoInternal(pid)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		return result
	case <-time.After(ProcessInfoTimeout):
		log.Printf("[FileMonitor] getProcessInfo timeout for PID %d after %v", pid, ProcessInfoTimeout)
		return info
	}
}

// getProcessInfoInternal 内部实现，获取进程详细信息
func (w *FileMonitorWorker) getProcessInfoInternal(pid int) ProcessInfo {
	info := ProcessInfo{
		PID:        pid,
		Name:       "unknown",
		Cmdline:    []string{},
		Exe:        "",
		Cwd:        "",
		UID:        -1,
		GID:        -1,
		User:       "unknown",
		PPID:       -1,
		ParentName: "unknown",
	}

	procPath := fmt.Sprintf("/proc/%d", pid)

	// 检查进程目录是否存在
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		return info
	}

	// 获取进程名称
	if commBytes, err := ioutil.ReadFile(filepath.Join(procPath, "comm")); err == nil {
		info.Name = strings.TrimSpace(string(commBytes))
	}

	// 获取命令行
	if cmdlineBytes, err := ioutil.ReadFile(filepath.Join(procPath, "cmdline")); err == nil {
		// cmdline 使用 NULL 分隔参数
		parts := strings.Split(string(cmdlineBytes), "\x00")
		for _, part := range parts {
			if part != "" {
				info.Cmdline = append(info.Cmdline, part)
			}
		}
	}

	// 获取可执行文件路径
	if exePath, err := os.Readlink(filepath.Join(procPath, "exe")); err == nil {
		info.Exe = exePath
	}

	// 获取当前工作目录
	if cwdPath, err := os.Readlink(filepath.Join(procPath, "cwd")); err == nil {
		info.Cwd = cwdPath
	}

	// 获取 UID/GID 和 PPID
	if statusFile, err := os.Open(filepath.Join(procPath, "status")); err == nil {
		defer statusFile.Close()
		scanner := bufio.NewScanner(statusFile)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if uid, err := strconv.Atoi(fields[1]); err == nil {
						info.UID = uid
						// 获取用户名
						if u, err := user.LookupId(fields[1]); err == nil {
							info.User = u.Username
						}
					}
				}
			} else if strings.HasPrefix(line, "Gid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if gid, err := strconv.Atoi(fields[1]); err == nil {
						info.GID = gid
					}
				}
			} else if strings.HasPrefix(line, "PPid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if ppid, err := strconv.Atoi(fields[1]); err == nil {
						info.PPID = ppid
					}
				}
			}
		}
	}

	// 获取父进程名称
	if info.PPID > 0 {
		parentCommPath := fmt.Sprintf("/proc/%d/comm", info.PPID)
		if commBytes, err := ioutil.ReadFile(parentCommPath); err == nil {
			info.ParentName = strings.TrimSpace(string(commBytes))
		}
	}

	return info
}

// shouldCalculateHash 判断是否需要计算文件的哈希
func (w *FileMonitorWorker) shouldCalculateHash(path string) bool {
	// 获取文件信息
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	// 跳过目录
	if fileInfo.IsDir() {
		return false
	}

	// 跳过过大的文件（使用常量 MaxHashFileSize）
	if fileInfo.Size() > MaxHashFileSize {
		return false
	}

	// 检查是否是可执行文件
	mode := fileInfo.Mode()
	if mode&0111 != 0 {
		return true
	}

	// 检查是否是脚本文件
	scriptExtensions := []string{
		".sh",                                       // Shell 脚本
		".php", ".php3", ".php4", ".php5", ".phtml", // PHP
		".jsp", ".jspx", // JSP
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, scriptExt := range scriptExtensions {
		if ext == scriptExt {
			return true
		}
	}

	return false
}

// calculateFileHash 计算文件的 SHA256 哈希值（带超时保护）
func (w *FileMonitorWorker) calculateFileHash(path string) (string, error) {
	// 检查文件大小
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fileInfo.Size() > MaxHashFileSize {
		return "", fmt.Errorf("file too large: %d bytes", fileInfo.Size())
	}

	// 使用带超时的 goroutine 计算哈希
	type hashResult struct {
		hash string
		err  error
	}
	resultChan := make(chan hashResult, 1)

	go func() {
		file, err := os.Open(path)
		if err != nil {
			resultChan <- hashResult{err: err}
			return
		}
		defer file.Close()

		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			resultChan <- hashResult{err: err}
			return
		}

		resultChan <- hashResult{hash: hex.EncodeToString(hash.Sum(nil))}
	}()

	select {
	case result := <-resultChan:
		return result.hash, result.err
	case <-time.After(HashCalculateTimeout):
		return "", fmt.Errorf("hash calculation timeout after %v", HashCalculateTimeout)
	}
}

func (w *FileMonitorWorker) calculateFileSeverity(path string, eventType string) int {
	// 系统关键文件 - 高危
	criticalFiles := []string{
		"/etc/passwd", "/etc/shadow", "/etc/sudoers",
		"/etc/ssh/sshd_config", "/etc/hosts",
		"/root/.ssh/authorized_keys",
	}

	for _, critical := range criticalFiles {
		if path == critical {
			return 5
		}
	}

	// /etc 目录下的配置文件 - 中等
	if filepath.Dir(path) == "/etc" {
		return 3
	}

	// 日志文件删除 - 可疑
	if eventType == "file_deleted" && filepath.Dir(path) == "/var/log" {
		return 4
	}

	// .ssh 目录 - 中等
	if filepath.Base(filepath.Dir(path)) == ".ssh" {
		return 3
	}

	// 临时目录 - 信息级
	if filepath.Dir(path) == "/tmp" {
		return 1
	}

	return 2
}

func (w *FileMonitorWorker) Stop() error {
	log.Println("[FileMonitor] Stopping...")
	if w.cancel != nil {
		w.cancel()
	}
	if w.notify != nil && w.notify.File != nil {
		w.notify.File.Close()
	}
	return nil
}

func (w *FileMonitorWorker) GetEvents() ([]*shared.MonitorData, error) {
	w.queueMutex.Lock()
	defer w.queueMutex.Unlock()

	// 返回所有事件并清空队列
	events := make([]*shared.MonitorData, len(w.eventQueue))
	copy(events, w.eventQueue)
	w.eventQueue = w.eventQueue[:0] // 清空队列

	return events, nil
}

func (w *FileMonitorWorker) GetMetrics() (map[string]interface{}, error) {
	uptime := time.Since(w.startTime)
	eventCount := w.eventCount.Load()

	return map[string]interface{}{
		"uptime_seconds":     uptime.Seconds(),
		"event_count":        eventCount,
		"send_success_count": w.sendSuccessCount.Load(),
		"send_error_count":   w.sendErrorCount.Load(),
		"queue_size":         len(w.eventQueue),
		"mount_points":       len(w.mountPoints),
		"healthy":            w.healthy.Load(),
	}, nil
}

func (w *FileMonitorWorker) Health() bool {
	return w.healthy.Load()
}

func (w *FileMonitorWorker) HealthCheck() (bool, string) {
	healthy := w.healthy.Load()
	if !healthy {
		return false, "worker unhealthy"
	}

	uptime := time.Since(w.startTime)
	eventCount := w.eventCount.Load()

	return true, fmt.Sprintf("uptime=%s events=%d mount_points=%d",
		uptime.Round(time.Second), eventCount, len(w.mountPoints))
}

// 实现 gomw 接口
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--standalone" {
		shared.StartStandalone(NewFileMonitorWorker())
		return
	}
	shared.StartGomwWorker("file_monitor", "1.0.0", NewFileMonitorWorker())
}
