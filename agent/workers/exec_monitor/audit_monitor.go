//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"artemis_agent/shared"

	"github.com/elastic/go-libaudit/v2"
	"github.com/elastic/go-libaudit/v2/auparse"
	"github.com/elastic/go-libaudit/v2/rule"
)

type AuditMonitor struct {
	ctx             context.Context
	callback        shared.DataCallback
	client          *libaudit.AuditClient
	pendingEvents   map[string]*ExecEvent
	eventsMu        sync.Mutex
	eventCount      *atomic.Uint64
	suspiciousCount *atomic.Uint64
	lastEventTime   atomic.Int64 // 最后接收事件的时间戳
	stopCleanup     chan struct{}
	eventQueue      chan *ExecEvent // 异步事件处理队列
	wg              sync.WaitGroup  // 等待所有 goroutine 退出
}

const (
	MaxPendingEvents = 10000 // pendingEvents 最大容量
	EventQueueSize   = 5000  // 事件队列大小（增大以减少丢弃）
)

type ExecEvent struct {
	Syscall       string
	PID           int
	UID           int
	GID           int
	User          string
	Command       string
	Args          []string
	CWD           string
	Exe           string
	PPID          int
	ParentComm    string
	ParentExe     string
	ParentCmdline []string
	Timestamp     time.Time
}

func NewAuditMonitor(ctx context.Context, callback shared.DataCallback, eventCount *atomic.Uint64, suspiciousCount *atomic.Uint64) (*AuditMonitor, error) {
	// 检测并停止冲突的 audit 服务
	if err := stopConflictingAuditServices(); err != nil {
		log.Printf("[AuditMonitor] Warning: failed to stop conflicting audit services: %v", err)
		log.Println("[AuditMonitor] Continuing anyway, may encounter issues...")
	}

	client, err := libaudit.NewAuditClient(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit client: %v", err)
	}

	m := &AuditMonitor{
		ctx:             ctx,
		callback:        callback,
		client:          client,
		pendingEvents:   make(map[string]*ExecEvent),
		eventCount:      eventCount,
		suspiciousCount: suspiciousCount,
		stopCleanup:     make(chan struct{}),
		eventQueue:      make(chan *ExecEvent, EventQueueSize),
	}
	m.lastEventTime.Store(time.Now().Unix())
	return m, nil
}

func (m *AuditMonitor) Start() error {
	// 配置 audit 规则
	if err := m.setupAuditRules(); err != nil {
		return err
	}

	// 启动异步事件处理 workers (多个 worker 并发处理)
	for i := 0; i < 4; i++ {
		m.wg.Add(1)
		go m.processEventWorker()
	}

	m.wg.Add(1)
	go m.receiveEvents()
	m.wg.Add(1)
	go m.cleanupStaleEvents()
	m.wg.Add(1)
	go m.monitorHealth()
	return nil
}

func (m *AuditMonitor) Stop() {
	if m.stopCleanup != nil {
		close(m.stopCleanup)
	}
	if m.client != nil {
		m.client.Close()
	}
	if m.eventQueue != nil {
		close(m.eventQueue)
	}
	// 等待所有 goroutine 退出
	m.wg.Wait()
	log.Println("[AuditMonitor] All goroutines stopped")
}

func (m *AuditMonitor) setupAuditRules() error {
	// 设置 PID
	if err := m.client.SetPID(libaudit.WaitForReply); err != nil {
		return fmt.Errorf("failed to set PID: %v", err)
	}

	// 设置 audit 为启用状态
	if err := m.client.SetEnabled(true, libaudit.WaitForReply); err != nil {
		return fmt.Errorf("failed to enable audit: %v", err)
	}

	// 增加缓冲区大小（默认 420，增加到 8192）- 使用 WaitForReply 保持序列号同步
	if err := m.client.SetBacklogLimit(8192, libaudit.WaitForReply); err != nil {
		log.Printf("[AuditMonitor] Warning: failed to set backlog limit: %v", err)
	}

	// 设置速率限制（0 = 无限制）- 使用 WaitForReply 保持序列号同步
	if err := m.client.SetRateLimit(0, libaudit.WaitForReply); err != nil {
		log.Printf("[AuditMonitor] Warning: failed to set rate limit: %v", err)
	}

	// 删除所有现有规则
	deleted, err := m.client.DeleteRules()
	if err != nil {
		log.Printf("[AuditMonitor] Warning: failed to delete existing rules: %v", err)
	} else {
		log.Printf("[AuditMonitor] Deleted %d existing rules", deleted)
	}

	// 添加 execve 监控规则
	execveRule := &rule.SyscallRule{
		Type:     rule.AppendSyscallRuleType,
		List:     "exit",
		Action:   "always",
		Syscalls: []string{"execve", "execveat"},
		Keys:     []string{"exec"},
	}

	execveRuleData, err := rule.Build(execveRule)
	if err != nil {
		return fmt.Errorf("failed to build execve rule: %v", err)
	}

	if err := m.client.AddRule(execveRuleData); err != nil {
		return fmt.Errorf("failed to add execve rule: %v", err)
	}

	log.Println("[AuditMonitor] Audit rules configured successfully")
	return nil
}

// cleanupStaleEvents 定期清理过期事件并上报，防止内存泄漏
func (m *AuditMonitor) cleanupStaleEvents() {
	defer m.wg.Done()

	// 使用更短的间隔（200ms）确保事件及时处理
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.stopCleanup:
			return
		case <-ticker.C:
			m.eventsMu.Lock()
			now := time.Now()
			var toReport []*ExecEvent
			var toDelete []string

			// 收集需要上报和删除的事件
			for msgID, event := range m.pendingEvents {
				age := now.Sub(event.Timestamp)

				// 超过 150ms 就上报（确保收集到 CWD 事件）
				if age > 150*time.Millisecond {
					toReport = append(toReport, event)
					toDelete = append(toDelete, msgID)
				}
			}

			// 删除已处理的事件
			for _, msgID := range toDelete {
				delete(m.pendingEvents, msgID)
			}

			pendingCount := len(m.pendingEvents)
			m.eventsMu.Unlock()

			// 异步上报事件（不阻塞清理循环）
			for _, event := range toReport {
				m.enqueueEvent(event)
			}

			// 定期日志
			if len(toReport) > 0 && pendingCount > 100 {
				log.Printf("[AuditMonitor] Reported %d events, %d pending", len(toReport), pendingCount)
			}

			// 警告：pending events 过多
			if pendingCount > 5000 {
				log.Printf("[AuditMonitor] WARNING: High pending events: %d", pendingCount)
			}
		}
	}
}

// monitorHealth 监控 receiveEvents 是否正常运行
func (m *AuditMonitor) monitorHealth() {
	defer m.wg.Done()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.stopCleanup:
			return
		case <-ticker.C:
			lastEventTime := time.Unix(m.lastEventTime.Load(), 0)
			timeSinceLastEvent := time.Since(lastEventTime)

			// 如果超过 5 分钟没有接收到任何事件，警告
			if timeSinceLastEvent > 5*time.Minute {
				log.Printf("[AuditMonitor] WARNING: No events received for %v (last event at: %s)",
					timeSinceLastEvent.Round(time.Second), lastEventTime.Format("2006-01-02 15:04:05"))
			}
		}
	}
}

// ReceiveTimeout 单次 Receive 调用的超时时间
const ReceiveTimeout = 30 * time.Second

func (m *AuditMonitor) receiveEvents() {
	defer m.wg.Done()

	errorCount := 0
	lastErrorTime := time.Now()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		// 使用带超时的 Receive，防止在老内核上永久阻塞
		rawEvent, err := m.receiveWithTimeout()
		if err != nil {
			if m.ctx.Err() != nil {
				return
			}

			// 超时不算错误，只是没有事件
			if err == errReceiveTimeout {
				continue
			}

			// 错误频率控制：1 秒内多次错误只打印一次
			now := time.Now()
			if now.Sub(lastErrorTime) > time.Second {
				log.Printf("[AuditMonitor] Error receiving event: %v (suppressing further errors for 1s)", err)
				lastErrorTime = now
				errorCount = 0
			} else {
				errorCount++
			}

			// 如果缓冲区满，微等待后重试
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// 成功接收，重置错误计数
		if errorCount > 0 {
			log.Printf("[AuditMonitor] Recovered after %d errors", errorCount)
			errorCount = 0
		}

		// 更新最后事件时间
		m.lastEventTime.Store(time.Now().Unix())

		m.handleRawEvent(rawEvent)
	}
}

// errReceiveTimeout 表示 Receive 超时
var errReceiveTimeout = fmt.Errorf("receive timeout")

// receiveWithTimeout 带超时的 Receive 调用
// 在老内核（如 CentOS 7 的 3.10）上，Receive 可能永久阻塞
func (m *AuditMonitor) receiveWithTimeout() (*libaudit.RawAuditMessage, error) {
	type receiveResult struct {
		event *libaudit.RawAuditMessage
		err   error
	}

	resultChan := make(chan receiveResult, 1)

	go func() {
		event, err := m.client.Receive(false)
		resultChan <- receiveResult{event: event, err: err}
	}()

	select {
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	case result := <-resultChan:
		return result.event, result.err
	case <-time.After(ReceiveTimeout):
		// 超时，但 goroutine 仍在运行
		// 这是一个已知的问题：goroutine 会泄漏，但比永久阻塞好
		log.Printf("[AuditMonitor] Receive timeout after %v, possible kernel issue", ReceiveTimeout)
		return nil, errReceiveTimeout
	}
}

func (m *AuditMonitor) handleRawEvent(rawEvent *libaudit.RawAuditMessage) {
	// 处理 SYSCALL (1300), CWD (1307), EXECVE (1309) 事件
	switch rawEvent.Type {
	case auparse.AUDIT_SYSCALL: // 1300
		m.handleSyscallEvent(rawEvent)
	case auparse.AUDIT_CWD: // 1307
		m.handleCwdEvent(rawEvent)
	case auparse.AUDIT_EXECVE: // 1309
		m.handleExecveEvent(rawEvent)
	}
}

func (m *AuditMonitor) extractMessageID(data []byte) string {
	// 格式: audit(1234567890.123:456)
	str := string(data)
	start := strings.Index(str, "audit(")
	if start == -1 {
		return ""
	}
	end := strings.Index(str[start:], ")")
	if end == -1 {
		return ""
	}
	return str[start+6 : start+end]
}

func (m *AuditMonitor) handleSyscallEvent(rawEvent *libaudit.RawAuditMessage) {
	msg, err := auparse.Parse(rawEvent.Type, string(rawEvent.Data))
	if err != nil {
		return
	}

	data, err := msg.Data()
	if err != nil {
		return
	}

	// 只处理 execve/execveat
	syscallName := data["syscall"]
	if syscallName != "execve" && syscallName != "execveat" && syscallName != "59" && syscallName != "322" {
		return
	}

	msgID := m.extractMessageID(rawEvent.Data)
	if msgID == "" {
		return
	}

	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()

	// 检查 pendingEvents 容量，防止无限增长
	if len(m.pendingEvents) >= MaxPendingEvents {
		log.Printf("[AuditMonitor] WARNING: pendingEvents at capacity (%d), dropping oldest events", MaxPendingEvents)
		// 删除最旧的 10% 事件
		count := 0
		for id := range m.pendingEvents {
			delete(m.pendingEvents, id)
			count++
			if count >= MaxPendingEvents/10 {
				break
			}
		}
	}

	event, exists := m.pendingEvents[msgID]
	if !exists {
		event = &ExecEvent{
			Timestamp: time.Now(),
		}
		m.pendingEvents[msgID] = event
	} else {
		// 事件已存在（可能由 CWD 事件创建），更新时间戳
		event.Timestamp = time.Now()
	}

	// 填充 SYSCALL 信息
	event.Syscall = syscallName
	if pid, err := strconv.Atoi(data["pid"]); err == nil {
		event.PID = pid
	}
	if uid, err := strconv.Atoi(data["uid"]); err == nil {
		event.UID = uid
	}
	if gid, err := strconv.Atoi(data["gid"]); err == nil {
		event.GID = gid
	}
	if ppid, err := strconv.Atoi(data["ppid"]); err == nil {
		event.PPID = ppid
	}
	event.Exe = data["exe"]
	event.Command = data["comm"]

	// 不再启动单独的 goroutine，由 cleanupStaleEvents 统一处理超时上报
}

func (m *AuditMonitor) handleCwdEvent(rawEvent *libaudit.RawAuditMessage) {
	msg, err := auparse.Parse(rawEvent.Type, string(rawEvent.Data))
	if err != nil {
		return
	}

	data, err := msg.Data()
	if err != nil {
		return
	}

	msgID := m.extractMessageID(rawEvent.Data)
	if msgID == "" {
		return
	}

	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()

	if event, exists := m.pendingEvents[msgID]; exists {
		// 事件已存在，直接设置 CWD
		event.CWD = data["cwd"]
	} else {
		// SYSCALL 事件还没到，先创建占位事件
		m.pendingEvents[msgID] = &ExecEvent{
			CWD:       data["cwd"],
			Timestamp: time.Now(),
		}
	}
}

func (m *AuditMonitor) handleExecveEvent(rawEvent *libaudit.RawAuditMessage) {
	msg, err := auparse.Parse(rawEvent.Type, string(rawEvent.Data))
	if err != nil {
		return
	}

	data, err := msg.Data()
	if err != nil {
		return
	}

	msgID := m.extractMessageID(rawEvent.Data)
	if msgID == "" {
		return
	}

	// 提取参数
	var args []string
	for i := 0; i < 100; i++ { // 最多 100 个参数
		key := fmt.Sprintf("a%d", i)
		if arg, ok := data[key]; ok {
			args = append(args, decodeAuditString(arg))
		} else {
			break
		}
	}

	m.eventsMu.Lock()
	if event, exists := m.pendingEvents[msgID]; exists {
		event.Args = args
		// 不要立即删除和上报，等待 timeout 以便收集 CWD 事件
		m.eventsMu.Unlock()
	} else {
		m.eventsMu.Unlock()
	}
}

// processEventWorker 异步处理事件，避免 callback 阻塞主循环
func (m *AuditMonitor) processEventWorker() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event, ok := <-m.eventQueue:
			if !ok {
				return
			}
			// 执行实际的事件处理和回调
			m.processEvent(event)
		}
	}
}

// 将事件放入队列异步处理
func (m *AuditMonitor) enqueueEvent(event *ExecEvent) {
	select {
	case m.eventQueue <- event:
		// 成功入队
	case <-time.After(100 * time.Millisecond):
		// 队列满，记录警告但不阻塞
		log.Printf("[AuditMonitor] WARNING: Event queue full, dropping event: cmd=%s", event.Command)
	}
}

func decodeAuditString(s string) string {
	// 去除引号
	s = strings.Trim(s, "\"")

	// 尝试解码
	decoded, err := strconv.Unquote(`"` + s + `"`)
	if err == nil {
		return decoded
	}

	return s
}

func (m *AuditMonitor) processEvent(event *ExecEvent) {
	// 过滤：关键字段为空则丢弃
	if event.Command == "" || event.Exe == "" || event.PID == 0 {
		return
	}

	// 更新计数器
	if m.eventCount != nil {
		m.eventCount.Add(1)
	}

	// 读取父进程信息
	event.ParentComm, event.ParentExe, event.ParentCmdline = readParentProcessInfo(event.PPID)

	// 读取用户名
	event.User = getUsernameFromUID(event.UID)

	// 计算威胁等级
	severity := calculateSeverity(event)

	if severity >= 3 {
		if m.suspiciousCount != nil {
			m.suspiciousCount.Add(1)
		}
	}

	// 计算可执行文件哈希
	exeHash := calculateFileHash(event.Exe)

	data := &shared.MonitorData{
		WorkerName: "exec_monitor",
		Timestamp:  event.Timestamp,
		EventType:  "syscall_exec",
		Severity:   severity,
		Data: map[string]interface{}{
			"monitor_mode":   "audit",
			"syscall":        event.Syscall,
			"pid":            event.PID,
			"ppid":           event.PPID,
			"uid":            event.UID,
			"gid":            event.GID,
			"user":           event.User,
			"command":        event.Command,
			"args":           event.Args,
			"cmdline":        strings.Join(event.Args, " "),
			"exe":            event.Exe,
			"exe_hash":       exeHash,
			"cwd":            event.CWD,
			"parent_comm":    event.ParentComm,
			"parent_exe":     event.ParentExe,
			"parent_cmdline": event.ParentCmdline,
		},
	}

	if m.callback != nil {
		m.callback.Send(data)
	}
}

func getUsernameFromUID(uid int) string {
	content, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}

	targetUID := fmt.Sprintf(":%d:", uid)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, targetUID) {
			parts := strings.Split(line, ":")
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}

func readParentProcessInfo(ppid int) (comm string, exe string, cmdline []string) {
	if ppid <= 0 {
		return
	}

	procPath := fmt.Sprintf("/proc/%d", ppid)

	// 首先检查进程是否存在
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		// 父进程已退出，这是正常情况（短生命周期进程）
		return
	}

	// 读取 comm - 最可靠的字段
	commPath := fmt.Sprintf("%s/comm", procPath)
	if data, err := os.ReadFile(commPath); err == nil {
		comm = strings.TrimSpace(string(data))
	}

	// 读取 exe - 可能因权限问题失败
	exePath := fmt.Sprintf("%s/exe", procPath)
	if link, err := os.Readlink(exePath); err == nil {
		exe = link
	} else {
		// 如果 readlink 失败，尝试从 /proc/{pid}/stat 获取
		// 某些情况下 exe 可能显示为 (deleted) 或无法访问
		if comm != "" {
			// 尝试在常见路径中查找可执行文件
			commonPaths := []string{"/usr/bin/", "/bin/", "/usr/sbin/", "/sbin/", "/usr/local/bin/"}
			for _, path := range commonPaths {
				fullPath := path + comm
				if _, statErr := os.Stat(fullPath); statErr == nil {
					exe = fullPath
					break
				}
			}
		}
	}

	// 读取 cmdline
	cmdlinePath := fmt.Sprintf("%s/cmdline", procPath)
	if data, err := os.ReadFile(cmdlinePath); err == nil && len(data) > 0 {
		cmdline = strings.Split(string(data), "\x00")
		if len(cmdline) > 0 && cmdline[len(cmdline)-1] == "" {
			cmdline = cmdline[:len(cmdline)-1]
		}
	}

	return
}

// 文件哈希计算的限制
const (
	MaxHashFileSize      = 50 * 1024 * 1024 // 最大 50MB，超过则跳过
	HashCalculateTimeout = 10 * time.Second // 哈希计算超时时间
)

func calculateFileHash(filePath string) string {
	if filePath == "" {
		return ""
	}

	// 检查文件大小，跳过大文件
	info, err := os.Stat(filePath)
	if err != nil {
		return ""
	}
	if info.Size() > MaxHashFileSize {
		return "skipped:file_too_large"
	}

	// 使用带超时的哈希计算
	done := make(chan string, 1)
	go func() {
		file, err := os.Open(filePath)
		if err != nil {
			done <- ""
			return
		}
		defer file.Close()

		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			done <- ""
			return
		}
		done <- fmt.Sprintf("%x", hash.Sum(nil))
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(HashCalculateTimeout):
		return "skipped:timeout"
	}
}

func calculateSeverity(event *ExecEvent) int {
	severity := 1

	// 可疑命令
	suspiciousCommands := []string{
		"nc", "netcat", "ncat",
		"bash", "sh", "dash", "zsh",
		"python", "perl", "ruby", "php",
		"wget", "curl",
		"chmod", "chown",
		"useradd", "usermod", "passwd",
	}

	for _, cmd := range suspiciousCommands {
		if strings.Contains(strings.ToLower(event.Command), cmd) ||
			strings.Contains(strings.ToLower(event.Exe), cmd) {
			severity = 3
			break
		}
	}

	// 可疑父进程
	suspiciousParents := []string{"nginx", "apache", "httpd", "tomcat", "java"}
	shellNames := []string{"bash", "sh", "zsh", "dash"}
	for _, parent := range suspiciousParents {
		if strings.Contains(strings.ToLower(event.ParentComm), parent) {
			for _, shell := range shellNames {
				if strings.Contains(strings.ToLower(event.Command), shell) {
					severity = 5
					break
				}
			}
		}
	}

	// 可疑命令行模式
	if len(event.Args) > 0 {
		argsStr := strings.Join(event.Args, " ")
		suspiciousPatterns := []string{
			"bash -i",
			"/dev/tcp/",
			"/dev/udp/",
			"bash -c",
			"python -c",
			"perl -e",
			"nc -",
			"base64",
			"eval",
		}
		for _, pattern := range suspiciousPatterns {
			if strings.Contains(argsStr, pattern) {
				severity = 4
				break
			}
		}
	}

	// Root 权限加权
	if event.UID == 0 && severity >= 3 {
		severity = 5
	}

	return severity
}

// stopConflictingAuditServices 检测并停止冲突的 audit 服务
func stopConflictingAuditServices() error {
	log.Println("[AuditMonitor] Checking for conflicting audit services...")

	// 检查并停止 auditd 服务
	auditdServices := []string{"auditd", "auditd.service"}
	for _, service := range auditdServices {
		if isServiceRunning(service) {
			log.Printf("[AuditMonitor] Found running service: %s, attempting to stop...", service)
			if err := stopService(service); err != nil {
				log.Printf("[AuditMonitor] Failed to stop %s: %v", service, err)
			} else {
				log.Printf("[AuditMonitor] Successfully stopped %s", service)
				time.Sleep(500 * time.Millisecond) // 等待服务完全停止
			}
		}
	}

	// 查找并终止可能持有 audit 的进程
	auditProcesses := []string{"auditd", "audispd"}
	for _, procName := range auditProcesses {
		if err := killProcessByName(procName); err != nil {
			log.Printf("[AuditMonitor] Failed to kill %s processes: %v", procName, err)
		}
	}

	log.Println("[AuditMonitor] Conflicting audit services check completed")
	return nil
}

// isServiceRunning 检查服务是否在运行
func isServiceRunning(serviceName string) bool {
	// 尝试使用 systemctl 检查
	cmd := exec.Command("systemctl", "is-active", serviceName)
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "active" {
		return true
	}

	// 尝试使用 service 命令检查
	cmd = exec.Command("service", serviceName, "status")
	err = cmd.Run()
	return err == nil
}

// stopService 停止服务
func stopService(serviceName string) error {
	// 尝试使用 systemctl
	cmd := exec.Command("systemctl", "stop", serviceName)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// 尝试使用 service 命令
	cmd = exec.Command("service", serviceName, "stop")
	return cmd.Run()
}

// killProcessByName 根据进程名终止进程
func killProcessByName(processName string) error {
	// 使用 pkill 命令
	cmd := exec.Command("pkill", "-9", processName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// pkill 返回非零表示没有找到进程，这不算错误
		if strings.Contains(string(output), "no process found") ||
			strings.Contains(err.Error(), "exit status 1") {
			log.Printf("[AuditMonitor] No %s processes found", processName)
			return nil
		}
		return fmt.Errorf("pkill %s failed: %v, output: %s", processName, err, string(output))
	}
	log.Printf("[AuditMonitor] Killed %s processes", processName)
	return nil
}
