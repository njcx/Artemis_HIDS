package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"artemis_agent/shared"

	"os/user"
)

// 命令执行超时配置
const (
	LanguageCommandTimeout = 120 * time.Second // 语言运行时命令超时
	ProcessScanTimeoutLang = 5 * time.Minute   // 进程扫描超时
)

// LanguageCollector 语言运行时采集器
type LanguageCollector struct {
	javaVersionCache sync.Map // map[string]string (exePath -> version)
}

// JavaProcessInfo Java 进程信息
type JavaProcessInfo struct {
	PID         string   `json:"pid"`
	User        string   `json:"user"`
	CommandLine string   `json:"command_line"`
	JarFiles    []string `json:"jar_files"`
	MainClass   string   `json:"main_class"`
	JavaVersion string   `json:"java_version"`
}

// NewLanguageCollector 创建语言运行时采集器
func NewLanguageCollector() *LanguageCollector {
	return &LanguageCollector{}
}

// CollectLoop 定期采集语言运行时信息
func (lc *LanguageCollector) CollectLoop(ctx context.Context, sendEvent func(*shared.MonitorData), mu *sync.Mutex) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Printf("[language_collector] collectLoop started, will collect after 5 seconds")

	// 启动后延迟5秒采集第一次
	time.Sleep(5 * time.Second)
	log.Printf("[language_collector] Starting first collection...")

	if mu != nil {
		mu.Lock()
	}
	lc.collectRuntimes(sendEvent)
	if mu != nil {
		mu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[language_collector] collectLoop context cancelled, exiting")
			return
		case <-ticker.C:
			if mu != nil {
				mu.Lock()
			}
			lc.collectRuntimes(sendEvent)
			if mu != nil {
				mu.Unlock()
			}
		}
	}
}

// collectRuntimes 采集 Java 和 Python 运行时信息
func (lc *LanguageCollector) collectRuntimes(sendEvent func(*shared.MonitorData)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[language_collector] panic: %v", r)
		}
	}()

	log.Printf("[language_collector] collectRuntimes called")

	// 采集 Java 进程
	log.Printf("[language_collector] Starting Java process collection...")
	javaProcesses := lc.getJavaProcesses()
	log.Printf("[language_collector] Found %d Java processes", len(javaProcesses))

	// 转换为 []interface{} 以便 structpb 处理
	javaList := make([]interface{}, 0, len(javaProcesses))
	for _, jp := range javaProcesses {
		javaList = append(javaList, map[string]interface{}{
			"pid":          jp.PID,
			"user":         jp.User,
			"command_line": jp.CommandLine,
			"jar_files":    jp.JarFiles,
			"main_class":   jp.MainClass,
			"java_version": jp.JavaVersion,
		})
	}

	dataMap := map[string]interface{}{
		"timestamp":          time.Now().Unix(),
		"java_process_count": len(javaProcesses),
		"java_processes":     javaList,
	}

	event := &shared.MonitorData{
		WorkerName: "language_monitor", // 使用独立的 WorkerName
		EventType:  "language_runtime_inventory",
		Timestamp:  time.Now(),
		Data:       dataMap,
	}

	sendEvent(event)
}

// getJavaProcesses 获取所有活跃的 Java 进程（使用高性能方法）
func (lc *LanguageCollector) getJavaProcesses() []JavaProcessInfo {
	var processes []JavaProcessInfo
	start := time.Now()

	// 直接读取 /proc 目录，不使用 gopsutil.Processes()，减少 I/O
	entries, err := os.ReadDir("/proc")
	if err != nil {
		log.Printf("[language_collector] Failed to read /proc: %v", err)
		return processes
	}

	log.Printf("[language_collector] Total entries to scan: %d", len(entries))

	for _, entry := range entries {
		// 检查超时
		if time.Since(start) > ProcessScanTimeoutLang {
			log.Printf("[language_collector] Process scan timeout, stopping")
			break
		}

		if !entry.IsDir() {
			continue
		}

		// 解析 PID
		pidStr := entry.Name()
		if _, err := strconv.Atoi(pidStr); err != nil {
			continue // 不是进程目录
		}

		// 1. 优先读取 Cmdline，这是识别 Java 进程最高效的方法
		// 绝大多数 Java 进程的 Cmdline 中都包含 "java"
		cmdlinePath := fmt.Sprintf("/proc/%s/cmdline", pidStr)
		cmdlineBytes, err := os.ReadFile(cmdlinePath)
		if err != nil || len(cmdlineBytes) == 0 {
			continue
		}

		// 处理 \0 分隔符
		cmdlineParts := bytes.Split(cmdlineBytes, []byte{0})
		var cmdlineArgs []string
		for _, part := range cmdlineParts {
			if len(part) > 0 {
				cmdlineArgs = append(cmdlineArgs, string(part))
			}
		}
		cmdline := strings.Join(cmdlineArgs, " ")
		cmdLower := strings.ToLower(cmdline)

		// 判断是否为 Java 进程
		isJava := false

		// 方式1: 检查命令行
		if strings.Contains(cmdLower, "/java ") ||
			strings.Contains(cmdLower, "/java\t") ||
			strings.HasPrefix(cmdLower, "java ") ||
			strings.Contains(cmdLower, " -jar ") ||
			strings.Contains(cmdLower, "-djava.") ||
			strings.Contains(cmdLower, "-xmx") ||
			strings.Contains(cmdLower, "-xms") {
			isJava = true
		}

		// 方式2: 如果命令行没识别出来，再尝试读取 Exe (readlink)
		// 这种情况比较少见，但有些封装的 java 程序可能 cmdline 很短
		var exe string
		if !isJava {
			exe, _ = os.Readlink(fmt.Sprintf("/proc/%s/exe", pidStr))
			if strings.Contains(strings.ToLower(exe), "java") ||
				strings.HasSuffix(exe, "/java") ||
				strings.HasSuffix(exe, "/javaw") {
				isJava = true
			}
		}

		if !isJava {
			continue
		}

		// 方式3: Name (通常不需要，因为 stat Name 截断严重且不包含路径)

		// 获取用户名 (读 status)
		// 简单起见，我们重用 ProcessCollector 的逻辑，或者为了不引入依赖，我们简单解析 status
		// 这里如果不获取用户名，也可以，或者用 "unknown"
		// 为了完整性，我们尝试获取
		username := lc.getUsername(pidStr)

		// 提取 jar 文件
		jarFiles := lc.extractJarFilesFromFd(pidStr)
		if len(jarFiles) == 0 {
			jarFiles = lc.extractJarFiles(cmdline)
		}

		// 提取主类
		mainClass := lc.extractMainClass(cmdline)

		// 获取 Java 版本 (优先使用 exe 路径查找版本，避免 exec)
		if exe == "" {
			exe, _ = os.Readlink(fmt.Sprintf("/proc/%s/exe", pidStr))
		}
		javaVersion := lc.getJavaVersionCached(exe, pidStr)

		log.Printf("[language_collector] Found Java process: PID=%s, MainClass=%s", pidStr, mainClass)

		processes = append(processes, JavaProcessInfo{
			PID:         pidStr,
			User:        username,
			CommandLine: cmdline,
			JarFiles:    jarFiles,
			MainClass:   mainClass,
			JavaVersion: javaVersion,
		})
	}

	return processes
}

// 用户名缓存
var langUserCache = make(map[string]string)
var langUserCacheMu sync.RWMutex

// getUsername 获取进程用户名
func (lc *LanguageCollector) getUsername(pid string) string {
	// 1. 读取 /proc/[pid]/status 获取 Uid
	statusPath := fmt.Sprintf("/proc/%s/status", pid)
	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		return "unknown"
	}

	uid := "0"
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

	// 2. 查询 Uid 对应的用户名 (带缓存)
	langUserCacheMu.RLock()
	if u, ok := langUserCache[uid]; ok {
		langUserCacheMu.RUnlock()
		return u
	}
	langUserCacheMu.RUnlock()

	u, err := user.LookupId(uid)
	name := uid
	if err == nil {
		name = u.Username
	}

	langUserCacheMu.Lock()
	langUserCache[uid] = name
	langUserCacheMu.Unlock()
	return name
}

// extractJarFilesFromFd 从 /proc/{pid}/fd 读取进程实际打开的 jar 文件
func (lc *LanguageCollector) extractJarFilesFromFd(pid string) []string {
	var jars []string
	seen := make(map[string]bool)

	fdDir := fmt.Sprintf("/proc/%s/fd", pid)

	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return jars
	}

	for _, entry := range entries {
		fdPath := filepath.Join(fdDir, entry.Name())

		target, err := os.Readlink(fdPath)
		if err != nil {
			continue
		}

		if strings.HasSuffix(target, ".jar") {
			if !seen[target] {
				seen[target] = true
				jars = append(jars, target)
			}
		}
	}

	return jars
}

// extractJarFiles 从命令行中提取 jar 文件路径
func (lc *LanguageCollector) extractJarFiles(cmdLine string) []string {
	var jars []string

	jarPattern := regexp.MustCompile(`([^\s]+\.jar)`)
	matches := jarPattern.FindAllString(cmdLine, -1)

	for _, match := range matches {
		jar := strings.TrimSpace(match)
		if jar != "" {
			jars = append(jars, jar)
		}
	}

	return jars
}

// extractMainClass 从命令行中提取主类名
func (lc *LanguageCollector) extractMainClass(cmdLine string) string {
	if idx := strings.Index(cmdLine, "-jar"); idx != -1 {
		parts := strings.Fields(cmdLine[idx+4:])
		if len(parts) > 0 {
			return filepath.Base(parts[0])
		}
	}

	classPattern := regexp.MustCompile(`\b([a-z][a-z0-9_]*\.)+[A-Z][a-zA-Z0-9_]*\b`)
	if match := classPattern.FindString(cmdLine); match != "" {
		return match
	}

	return "unknown"
}

// getJavaVersionCached 带缓存的获取 Java 版本
func (lc *LanguageCollector) getJavaVersionCached(exePath string, pid string) string {
	if exePath != "" {
		if v, ok := lc.javaVersionCache.Load(exePath); ok {
			return v.(string)
		}
	}

	version := lc.getJavaVersion(pid)

	if exePath != "" && version != "unknown" {
		lc.javaVersionCache.Store(exePath, version)
	}
	return version
}

// getJavaVersion 获取指定进程使用的 Java 版本（带超时保护）
func (lc *LanguageCollector) getJavaVersion(pid string) string {
	environPath := fmt.Sprintf("/proc/%s/environ", pid)
	data, err := os.ReadFile(environPath)
	if err != nil {
		return "unknown"
	}

	envVars := strings.Split(string(data), "\x00")
	for _, env := range envVars {
		if strings.HasPrefix(env, "JAVA_HOME=") {
			javaHome := strings.TrimPrefix(env, "JAVA_HOME=")
			javaCmd := filepath.Join(javaHome, "bin", "java")

			ctx, cancel := context.WithTimeout(context.Background(), LanguageCommandTimeout)
			cmd := exec.CommandContext(ctx, javaCmd, "-version")
			output, err := cmd.CombinedOutput()
			cancel()

			if err == nil {
				versionStr := string(output)
				if strings.Contains(versionStr, "version") {
					lines := strings.Split(versionStr, "\n")
					if len(lines) > 0 {
						return strings.TrimSpace(lines[0])
					}
				}
			} else if ctx.Err() == context.DeadlineExceeded {
				log.Printf("[language_collector] java -version timeout for JAVA_HOME=%s", javaHome)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), LanguageCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "java", "-version")
	output, err := cmd.CombinedOutput()
	if err == nil {
		versionStr := string(output)
		if strings.Contains(versionStr, "version") {
			lines := strings.Split(versionStr, "\n")
			if len(lines) > 0 {
				return strings.TrimSpace(lines[0])
			}
		}
	} else if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[language_collector] default java -version timeout")
	}

	return "unknown"
}
