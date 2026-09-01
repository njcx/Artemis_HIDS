package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"

	// "os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
	"unsafe"

	"artemis_agent/shared"
)

// SecurityLogMonitor 安全日志监控器
type SecurityLogMonitor struct {
	ctx          context.Context
	cancel       context.CancelFunc
	callback     shared.DataCallback
	eventQueue   []*shared.MonitorData
	queueMutex   sync.Mutex
	maxQueueSize int

	// 日志文件追踪
	logFiles    map[string]*LogFileTracker
	logFilesMux sync.RWMutex
}

// LogFileTracker 日志文件追踪器
type LogFileTracker struct {
	Path     string
	File     *os.File
	Reader   *bufio.Reader
	Offset   int64
	LastMod  time.Time
	Cancel   context.CancelFunc
	FirstRun bool // 首次启动标记，用于跳过历史日志
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	EventType string    `json:"event_type"` // ssh_login_failed, ssh_login_success, sudo_command, etc.
	Severity  int       `json:"severity"`   // 1-5
	Timestamp time.Time `json:"timestamp"`
	SourceIP  string    `json:"source_ip"`
	User      string    `json:"user"`
	Message   string    `json:"message"`
	LogFile   string    `json:"log_file"`
	RawLine   string    `json:"raw_line"`
}

// utmp 结构体定义（Linux x86_64）
// 参考: /usr/include/bits/utmp.h
const (
	UT_LINESIZE = 32
	UT_NAMESIZE = 32
	UT_HOSTSIZE = 256
)

type exitStatus struct {
	Termination int16
	Exit        int16
}

type timeval struct {
	Sec  int32
	Usec int32
}

// utmpEntry 必须与系统的 struct utmp 完全匹配
type utmpEntry struct {
	Type    int16   // short
	_       [2]byte // padding to align pid
	Pid     int32   // pid_t (int32 on x86_64)
	Line    [UT_LINESIZE]byte
	Id      [4]byte
	User    [UT_NAMESIZE]byte
	Host    [UT_HOSTSIZE]byte
	Exit    exitStatus
	Session int32 // long on x86_64 is int64, but utmp uses int32
	Time    timeval
	AddrV6  [4]int32 // int32_t[4]
	_       [20]byte // reserved
}

const (
	EMPTY         = 0
	RUN_LVL       = 1
	BOOT_TIME     = 2
	NEW_TIME      = 3
	OLD_TIME      = 4
	INIT_PROCESS  = 5
	LOGIN_PROCESS = 6
	USER_PROCESS  = 7
	DEAD_PROCESS  = 8
	ACCOUNTING    = 9
)

// Start 启动采集
func (slm *SecurityLogMonitor) Start(ctx context.Context, callback shared.DataCallback) error {
	// 确保 log 输出到 stderr，以便被主进程捕获
	log.SetOutput(os.Stderr)

	slm.ctx, slm.cancel = context.WithCancel(ctx)
	slm.callback = callback
	slm.maxQueueSize = 1000
	slm.eventQueue = make([]*shared.MonitorData, 0, slm.maxQueueSize)
	slm.logFiles = make(map[string]*LogFileTracker)

	hostname, _ := os.Hostname()
	log.Printf("[security_log_monitor] Starting on %s", hostname)

	// 启动父进程监控
	go shared.WatchParentProcess(slm.ctx)

	// 启动日志监控
	go slm.monitorLogs()

	return nil
}

// Stop 停止采集
func (slm *SecurityLogMonitor) Stop() error {
	if slm.cancel != nil {
		slm.cancel()
	}

	slm.logFilesMux.Lock()
	defer slm.logFilesMux.Unlock()

	for _, tracker := range slm.logFiles {
		if tracker.Cancel != nil {
			tracker.Cancel()
		}
		if tracker.File != nil {
			tracker.File.Close()
		}
	}

	return nil
}

// GetEvents 获取待发送的事件
func (slm *SecurityLogMonitor) GetEvents() ([]*shared.MonitorData, error) {
	slm.queueMutex.Lock()
	defer slm.queueMutex.Unlock()

	events := make([]*shared.MonitorData, len(slm.eventQueue))
	copy(events, slm.eventQueue)
	slm.eventQueue = slm.eventQueue[:0]

	return events, nil
}

// GetMetrics 返回性能指标
func (slm *SecurityLogMonitor) GetMetrics() (map[string]interface{}, error) {
	slm.queueMutex.Lock()
	queueLen := len(slm.eventQueue)
	slm.queueMutex.Unlock()

	slm.logFilesMux.RLock()
	fileCount := len(slm.logFiles)
	slm.logFilesMux.RUnlock()

	return map[string]interface{}{
		"queue_length":    queueLen,
		"max_queue":       slm.maxQueueSize,
		"monitored_files": fileCount,
	}, nil
}

// Health 健康检查
func (slm *SecurityLogMonitor) Health() bool {
	return slm.ctx != nil && slm.ctx.Err() == nil
}

// Name 返回 worker 名称
func (slm *SecurityLogMonitor) Name() string {
	return "security_log_monitor"
}

// monitorLogs 监控日志文件
func (slm *SecurityLogMonitor) monitorLogs() {
	log.Printf("[security_log_monitor] monitorLogs started, detecting log files...")

	// 定义要监控的日志文件
	logFiles := slm.detectLogFiles()

	log.Printf("[security_log_monitor] Found %d log files to monitor", len(logFiles))

	// 为每个日志文件启动监控
	for _, logPath := range logFiles {
		go slm.tailLogFile(logPath)
	}

	// 启动二进制日志监控
	go slm.monitorBinaryLogs()

	// 等待上下文取消
	<-slm.ctx.Done()
}

// detectLogFiles 检测存在的日志文件
func (slm *SecurityLogMonitor) detectLogFiles() []string {
	candidateFiles := []string{
		"/var/log/auth.log", // Debian/Ubuntu 认证日志
		"/var/log/secure",   // RedHat/CentOS 认证日志
		"/var/log/syslog",   // 系统日志
		"/var/log/messages", // 系统消息
		// "/var/log/fail2ban.log",    // Fail2ban
		// "/var/log/audit/audit.log", // 审计日志
		// "/var/log/kern.log",        // 内核日志
		"/var/log/cron", // Cron 日志
	}

	var existingFiles []string
	for _, path := range candidateFiles {
		if _, err := os.Stat(path); err == nil {
			existingFiles = append(existingFiles, path)
			log.Printf("[security_log_monitor] Will monitor: %s", path)
		}
	}

	return existingFiles
}

// tailLogFile 追踪日志文件
func (slm *SecurityLogMonitor) tailLogFile(logPath string) {
	ctx, cancel := context.WithCancel(slm.ctx)
	defer cancel()

	tracker := &LogFileTracker{
		Path:     logPath,
		Cancel:   cancel,
		FirstRun: true,
	}

	slm.logFilesMux.Lock()
	slm.logFiles[logPath] = tracker
	slm.logFilesMux.Unlock()

	for {
		select {
		case <-ctx.Done():
			if tracker.File != nil {
				tracker.File.Close()
			}
			return
		default:
		}

		// 打开文件（仅首次或文件被关闭时）
		if tracker.File == nil {
			file, err := os.Open(logPath)
			if err != nil {
				log.Printf("[security_log_monitor] Failed to open %s: %v", logPath, err)
				time.Sleep(10 * time.Second)
				continue
			}

			// 首次启动时跳到文件末尾，之后从上次 offset 继续
			if tracker.FirstRun {
				fileInfo, _ := file.Stat()
				tracker.Offset = fileInfo.Size()
				tracker.FirstRun = false
				log.Printf("[security_log_monitor] First run for %s, seeking to end (offset=%d)", logPath, tracker.Offset)
			}
			file.Seek(tracker.Offset, io.SeekStart)

			tracker.File = file
			tracker.Reader = bufio.NewReader(file)
		}

		// 读取新行
		for {
			select {
			case <-ctx.Done():
				tracker.File.Close()
				return
			default:
			}

			line, err := tracker.Reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// 到达文件末尾，等待新内容（不关闭文件，类似 tail -f）
					time.Sleep(500 * time.Millisecond)

					// 检查文件是否被轮转（inode 变化）
					fileInfo, statErr := tracker.File.Stat()
					if statErr != nil {
						// 文件可能被删除，关闭并重新打开
						tracker.File.Close()
						tracker.File = nil
						tracker.Reader = nil
						break
					}

					// 检查文件是否被截断（日志轮转）
					if fileInfo.Size() < tracker.Offset {
						log.Printf("[security_log_monitor] File %s truncated, reopening", logPath)
						tracker.File.Close()
						tracker.File = nil
						tracker.Reader = nil
						tracker.Offset = 0
						break
					}

					continue // 继续等待新内容
				}
				log.Printf("[security_log_monitor] Read error %s: %v", logPath, err)
				tracker.File.Close()
				tracker.File = nil
				tracker.Reader = nil
				time.Sleep(5 * time.Second)
				break
			}

			tracker.Offset += int64(len(line))

			// 解析日志行
			slm.parseLine(logPath, strings.TrimSpace(line))
		}
	}
}

// parseLine 解析日志行
func (slm *SecurityLogMonitor) parseLine(logPath, line string) {
	if line == "" {
		return
	}

	// 检测各种安全事件
	events := slm.detectSecurityEvents(logPath, line)

	for _, event := range events {
		slm.sendEvent(event)
	}
}

// detectSecurityEvents 检测安全事件
func (slm *SecurityLogMonitor) detectSecurityEvents(logPath, line string) []*SecurityEvent {
	var events []*SecurityEvent

	// SSH 登录失败
	if event := slm.detectSSHLoginFailed(logPath, line); event != nil {
		events = append(events, event)
	}

	// SSH 登录成功
	if event := slm.detectSSHLoginSuccess(logPath, line); event != nil {
		events = append(events, event)
	}

	// Sudo 命令执行
	if event := slm.detectSudoCommand(logPath, line); event != nil {
		events = append(events, event)
	}

	// 用户添加/删除
	if event := slm.detectUserChange(logPath, line); event != nil {
		events = append(events, event)
	}

	// 认证失败
	if event := slm.detectAuthFailure(logPath, line); event != nil {
		events = append(events, event)
	}

	// 权限变更
	if event := slm.detectPermissionChange(logPath, line); event != nil {
		events = append(events, event)
	}

	// SELinux/AppArmor 拒绝
	if event := slm.detectSELinuxDenial(logPath, line); event != nil {
		events = append(events, event)
	}

	return events
}

// detectSSHLoginFailed SSH 登录失败
func (slm *SecurityLogMonitor) detectSSHLoginFailed(logPath, line string) *SecurityEvent {
	patterns := []string{
		`Failed password for (.*?) from ([\d\.]+) port`,
		`Failed password for invalid user (.*?) from ([\d\.]+)`,
		`authentication failure.*user=(.*?)`,
		`Connection closed by authenticating user (.*?) ([\d\.]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			user := ""
			ip := ""
			if len(matches) > 1 {
				user = matches[1]
			}
			if len(matches) > 2 {
				ip = matches[2]
			}

			return &SecurityEvent{
				EventType: "ssh_login_failed",
				Severity:  3,
				Timestamp: time.Now(),
				SourceIP:  ip,
				User:      user,
				Message:   "SSH login attempt failed",
				LogFile:   logPath,
				RawLine:   line,
			}
		}
	}

	return nil
}

// detectSSHLoginSuccess SSH 登录成功
func (slm *SecurityLogMonitor) detectSSHLoginSuccess(logPath, line string) *SecurityEvent {
	patterns := []string{
		`Accepted (publickey|password|keyboard-interactive) for (.*?) from ([\d\.]+)`,
		`pam_unix\(sshd:session\): session opened for user (.*?)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			user := ""
			ip := ""

			if strings.Contains(pattern, "Accepted") {
				if len(matches) > 2 {
					user = matches[2]
				}
				if len(matches) > 3 {
					ip = matches[3]
				}
			} else {
				if len(matches) > 1 {
					user = matches[1]
				}
			}

			return &SecurityEvent{
				EventType: "ssh_login_success",
				Severity:  2,
				Timestamp: time.Now(),
				SourceIP:  ip,
				User:      user,
				Message:   "SSH login successful",
				LogFile:   logPath,
				RawLine:   line,
			}
		}
	}

	return nil
}

// detectSudoCommand Sudo 命令执行
func (slm *SecurityLogMonitor) detectSudoCommand(logPath, line string) *SecurityEvent {
	pattern := `sudo:.*?USER=(.*?) ; COMMAND=(.*)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(line)

	if matches != nil {
		user := ""
		command := ""
		if len(matches) > 1 {
			user = matches[1]
		}
		if len(matches) > 2 {
			command = matches[2]
		}

		severity := 2
		// 敏感命令提高严重级别
		if strings.Contains(command, "passwd") ||
			strings.Contains(command, "useradd") ||
			strings.Contains(command, "userdel") ||
			strings.Contains(command, "usermod") ||
			strings.Contains(command, "groupadd") {
			severity = 4
		}

		return &SecurityEvent{
			EventType: "sudo_command",
			Severity:  severity,
			Timestamp: time.Now(),
			User:      user,
			Message:   fmt.Sprintf("Sudo command executed: %s", command),
			LogFile:   logPath,
			RawLine:   line,
		}
	}

	return nil
}

// detectUserChange 用户添加/删除
func (slm *SecurityLogMonitor) detectUserChange(logPath, line string) *SecurityEvent {
	patterns := map[string]string{
		"user_added":    `useradd.*?name=(.*?)`,
		"user_deleted":  `userdel.*?name=(.*?)`,
		"user_modified": `usermod.*?name=(.*?)`,
		"group_added":   `groupadd.*?group=(.*?)`,
	}

	for eventType, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			target := ""
			if len(matches) > 1 {
				target = matches[1]
			}

			return &SecurityEvent{
				EventType: eventType,
				Severity:  4,
				Timestamp: time.Now(),
				User:      target,
				Message:   fmt.Sprintf("User/Group change: %s", eventType),
				LogFile:   logPath,
				RawLine:   line,
			}
		}
	}

	return nil
}

// detectAuthFailure 认证失败
func (slm *SecurityLogMonitor) detectAuthFailure(logPath, line string) *SecurityEvent {
	keywords := []string{
		"authentication failure",
		"auth failure",
		"FAILED LOGIN",
		"Invalid user",
		"User not known",
	}

	for _, keyword := range keywords {
		if strings.Contains(line, keyword) {
			// 提取用户名和 IP
			userRe := regexp.MustCompile(`user[=:]?\s*(\w+)`)
			ipRe := regexp.MustCompile(`from ([\d\.]+)`)

			user := ""
			ip := ""

			if userMatches := userRe.FindStringSubmatch(line); userMatches != nil {
				user = userMatches[1]
			}
			if ipMatches := ipRe.FindStringSubmatch(line); ipMatches != nil {
				ip = ipMatches[1]
			}

			return &SecurityEvent{
				EventType: "auth_failure",
				Severity:  3,
				Timestamp: time.Now(),
				SourceIP:  ip,
				User:      user,
				Message:   "Authentication failure detected",
				LogFile:   logPath,
				RawLine:   line,
			}
		}
	}

	return nil
}

// detectPermissionChange 权限变更
func (slm *SecurityLogMonitor) detectPermissionChange(logPath, line string) *SecurityEvent {
	keywords := []string{
		"chmod",
		"chown",
		"chgrp",
		"setfacl",
	}

	for _, keyword := range keywords {
		if strings.Contains(line, keyword) {
			return &SecurityEvent{
				EventType: "permission_change",
				Severity:  2,
				Timestamp: time.Now(),
				Message:   "File permission change detected",
				LogFile:   logPath,
				RawLine:   line,
			}
		}
	}

	return nil
}

// detectSELinuxDenial SELinux/AppArmor 拒绝
func (slm *SecurityLogMonitor) detectSELinuxDenial(logPath, line string) *SecurityEvent {
	keywords := []string{
		"avc: denied",
		"apparmor=\"DENIED\"",
		"audit: type=1400",
	}

	for _, keyword := range keywords {
		if strings.Contains(line, keyword) {
			return &SecurityEvent{
				EventType: "selinux_denial",
				Severity:  3,
				Timestamp: time.Now(),
				Message:   "SELinux/AppArmor access denial",
				LogFile:   logPath,
				RawLine:   line,
			}
		}
	}

	return nil
}

// sendEvent 发送事件
func (slm *SecurityLogMonitor) sendEvent(event *SecurityEvent) {
	// 直接将事件字段放入 dataMap，不使用 JSON 字符串
	dataMap := map[string]interface{}{
		"timestamp":  event.Timestamp.Unix(),
		"event_type": event.EventType,
		"severity":   event.Severity,
		"source_ip":  event.SourceIP,
		"user":       event.User,
		"message":    event.Message,
		"log_file":   event.LogFile,
		"raw_line":   event.RawLine,
	}

	monitorData := &shared.MonitorData{
		WorkerName: "security_log_monitor",
		EventType:  "security_log_event",
		Timestamp:  event.Timestamp,
		Severity:   event.Severity,
		Data:       dataMap,
	}

	if slm.callback != nil {
		if err := slm.callback.Send(monitorData); err != nil {
			log.Printf("[security_log_monitor] callback.Send 失败: %v", err)
			slm.addToQueue(monitorData)
		}
	} else {
		slm.addToQueue(monitorData)
	}
}

// addToQueue 添加事件到队列
func (slm *SecurityLogMonitor) addToQueue(event *shared.MonitorData) {
	slm.queueMutex.Lock()
	defer slm.queueMutex.Unlock()

	if len(slm.eventQueue) >= slm.maxQueueSize {
		slm.eventQueue = slm.eventQueue[1:]
	}
	slm.eventQueue = append(slm.eventQueue, event)
}

// monitorBinaryLogs 监控二进制日志文件（直接读取 btmp/wtmp，类似 tail -f）
func (slm *SecurityLogMonitor) monitorBinaryLogs() {
	log.Println("[security_log_monitor] Starting binary log monitoring (direct file reading)")

	// 监控 btmp（失败登录）- 追加模式
	go slm.tailBinaryLog("/var/log/btmp", "login_failed")

	// 监控 wtmp（登录历史）- 追加模式
	go slm.tailBinaryLog("/var/log/wtmp", "login_history")

	<-slm.ctx.Done()
}

// tailBinaryLog 实时监控 utmp 格式的二进制日志文件
func (slm *SecurityLogMonitor) tailBinaryLog(filePath string, logType string) {
	// 等待文件存在
	for {
		if _, err := os.Stat(filePath); err == nil {
			break
		}
		select {
		case <-slm.ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}

	entrySize := int64(unsafe.Sizeof(utmpEntry{}))
	log.Printf("[security_log_monitor] Starting to monitor %s (utmp entry size: %d bytes)", filePath, entrySize)

	var offset int64 = 0
	firstRun := true

	for {
		select {
		case <-slm.ctx.Done():
			return
		default:
		}

		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("[security_log_monitor] Failed to open %s: %v", filePath, err)
			time.Sleep(10 * time.Second)
			continue
		}

		// 首次运行跳到文件末尾
		if firstRun {
			fileInfo, _ := file.Stat()
			offset = fileInfo.Size()
			firstRun = false
			log.Printf("[security_log_monitor] First run for %s, seeking to end (offset=%d)", filePath, offset)
		}

		file.Seek(offset, io.SeekStart)

		for {
			select {
			case <-slm.ctx.Done():
				file.Close()
				return
			default:
			}

			entry := utmpEntry{}
			err := binary.Read(file, binary.LittleEndian, &entry)
			if err != nil {
				if err == io.EOF {
					file.Close()
					// 等待新内容
					time.Sleep(1 * time.Second)
					break
				}
				log.Printf("[security_log_monitor] Read error %s: %v", filePath, err)
				file.Close()
				time.Sleep(5 * time.Second)
				break
			}

			offset += entrySize

			// 解析 utmp 条目
			slm.parseUtmpEntry(&entry, filePath, logType)
		}
	}
}

// parseUtmpEntry 解析 utmp 条目
func (slm *SecurityLogMonitor) parseUtmpEntry(entry *utmpEntry, filePath string, logType string) {
	// 提取字符串字段（去除 null 字节）
	user := strings.TrimRight(string(entry.User[:]), "\x00")
	host := strings.TrimRight(string(entry.Host[:]), "\x00")
	line := strings.TrimRight(string(entry.Line[:]), "\x00")

	// 调试日志：显示收到的条目（包括原始字节前几位）
	if entry.Type != EMPTY && (user != "" || host != "" || line != "") {
		log.Printf("[security_log_monitor] Binary log entry: file=%s type=%d(%s) user='%s' host='%s' line='%s' pid=%d time=%d",
			filePath, entry.Type, getUtmpTypeName(entry.Type), user, host, line, entry.Pid, entry.Time.Sec)
	}

	// 跳过空记录
	if user == "" && host == "" {
		return
	}

	timestamp := time.Unix(int64(entry.Time.Sec), 0)

	var event *SecurityEvent

	switch logType {
	case "login_failed":
		// btmp 记录的都是失败的登录尝试
		if entry.Type == USER_PROCESS || entry.Type == LOGIN_PROCESS {
			event = &SecurityEvent{
				EventType: "ssh_login_failed",
				Severity:  4,
				Timestamp: timestamp,
				SourceIP:  host,
				User:      user,
				Message:   fmt.Sprintf("Failed login attempt from %s on %s", host, line),
				LogFile:   filePath,
				RawLine:   fmt.Sprintf("user=%s host=%s line=%s pid=%d", user, host, line, entry.Pid),
			}
		}

	case "login_history":
		// wtmp 记录登录历史
		if entry.Type == USER_PROCESS {
			event = &SecurityEvent{
				EventType: "ssh_login_success",
				Severity:  2,
				Timestamp: timestamp,
				SourceIP:  host,
				User:      user,
				Message:   fmt.Sprintf("User logged in from %s on %s", host, line),
				LogFile:   filePath,
				RawLine:   fmt.Sprintf("user=%s host=%s line=%s pid=%d", user, host, line, entry.Pid),
			}
		} else if entry.Type == DEAD_PROCESS {
			event = &SecurityEvent{
				EventType: "user_logout",
				Severity:  1,
				Timestamp: timestamp,
				SourceIP:  host,
				User:      user,
				Message:   fmt.Sprintf("User logged out on %s", line),
				LogFile:   filePath,
				RawLine:   fmt.Sprintf("user=%s host=%s line=%s pid=%d", user, host, line, entry.Pid),
			}
		}
	}

	if event != nil {
		log.Printf("[security_log_monitor] Sending binary log event: type=%s user=%s ip=%s",
			event.EventType, event.User, event.SourceIP)
		slm.sendEvent(event)
	}
}

// getUtmpTypeName 获取 utmp 类型名称
func getUtmpTypeName(t int16) string {
	switch t {
	case EMPTY:
		return "EMPTY"
	case RUN_LVL:
		return "RUN_LVL"
	case BOOT_TIME:
		return "BOOT_TIME"
	case NEW_TIME:
		return "NEW_TIME"
	case OLD_TIME:
		return "OLD_TIME"
	case INIT_PROCESS:
		return "INIT_PROCESS"
	case LOGIN_PROCESS:
		return "LOGIN_PROCESS"
	case USER_PROCESS:
		return "USER_PROCESS"
	case DEAD_PROCESS:
		return "DEAD_PROCESS"
	case ACCOUNTING:
		return "ACCOUNTING"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", t)
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--standalone" {
		shared.StartStandalone(&SecurityLogMonitor{})
		return
	}
	shared.StartGomwWorker("security_log_monitor", "1.0.0", &SecurityLogMonitor{})
}
