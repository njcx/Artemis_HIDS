package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"artemis_agent/shared"

	rpmdb "github.com/knqyf263/go-rpmdb/pkg"
)

// 超时配置
const (
	RpmCommandTimeout    = 1200 * time.Second // rpm 命令超时
	RpmDBTimeoutSQLite   = 1200 * time.Second // SQLite 格式 rpmdb 读取超时
	RpmDBTimeoutBerkeley = 1200 * time.Second // BerkeleyDB 格式 rpmdb 读取超时
	MaxBerkeleyDBSize    = 200 * 1024 * 1024  // BerkeleyDB 最大允许大小 (200MB)
)

// PackageCollector 软件包采集器
type PackageCollector struct {
	pkgManager   string // debian, redhat
	mu           sync.Mutex
	cachedPkgs   []map[string]string // 缓存的包列表
	cacheTime    time.Time           // 缓存时间
	cacheTTL     time.Duration       // 缓存有效期
	rpmdbRunning bool                // 标记 rpmdb 是否正在运行
	rpmdbMu      sync.Mutex          // rpmdb 操作锁
}

// NewPackageCollector 创建软件包采集器
func NewPackageCollector() *PackageCollector {
	pc := &PackageCollector{
		cacheTTL: 30 * time.Minute,
	}
	pc.pkgManager = pc.detectPackageManager()
	log.Printf("[package_collector] Detected package manager: %s", pc.pkgManager)
	return pc
}

// CollectLoop 定期采集软件包信息
func (pc *PackageCollector) CollectLoop(ctx context.Context, sendEvent func(*shared.MonitorData), mu *sync.Mutex) {
	// 每 1 小时采集一次
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	log.Printf("[package_collector] collectLoop started, will collect every 1 hour")

	// 启动后延迟 30 秒采集第一次，避免与其他采集器竞争启动资源
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
		log.Printf("[package_collector] Starting first collection...")
		if mu != nil {
			mu.Lock()
		}
		pc.collectPackages(sendEvent)
		if mu != nil {
			mu.Unlock()
		}
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[package_collector] collectLoop stopped")
			return
		case <-ticker.C:
			if mu != nil {
				mu.Lock()
			}
			pc.collectPackages(sendEvent)
			if mu != nil {
				mu.Unlock()
			}
		}
	}
}

// detectPackageManager 检测系统使用的包管理器
func (pc *PackageCollector) detectPackageManager() string {
	if runtime.GOOS != "linux" {
		return "unsupported"
	}

	// 检查常见的包管理器二进制
	managers := []struct {
		name    string
		command string
	}{
		{"debian", "dpkg"},
		{"redhat", "rpm"},
		{"arch", "pacman"},
	}
	for _, mgr := range managers {
		if _, err := exec.LookPath(mgr.command); err == nil {
			return mgr.name
		}
	}

	return "unknown"
}

// collectPackages 采集已安装的软件包
func (pc *PackageCollector) collectPackages(sendEvent func(*shared.MonitorData)) {
	log.Printf("[package_collector] collectPackages started")

	// 加锁防止并发执行
	if !pc.mu.TryLock() {
		log.Printf("[package_collector] Collection already in progress, skipping")
		return
	}
	defer pc.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[package_collector] panic: %v", r)
		}
	}()

	if pc.pkgManager == "unsupported" || pc.pkgManager == "unknown" {
		log.Printf("[package_collector] Unsupported platform or unknown package manager, skipping")
		return
	}

	log.Printf("[package_collector] Getting installed packages...")
	packages, err := pc.getInstalledPackages()
	if err != nil {
		log.Printf("[package_collector] 获取软件包列表失败: %v", err)
		return
	}
	log.Printf("[package_collector] Got %d packages", len(packages))

	// 转换为 []interface{} 以便 structpb 处理
	packagesList := make([]interface{}, 0, len(packages))
	for _, pkg := range packages {
		pkgMap := make(map[string]interface{})
		for k, v := range pkg {
			pkgMap[k] = v
		}
		packagesList = append(packagesList, pkgMap)
	}

	dataMap := map[string]interface{}{
		"timestamp":       time.Now().Unix(),
		"package_manager": pc.pkgManager,
		"package_count":   len(packages),
		"packages":        packagesList,
	}

	event := &shared.MonitorData{
		WorkerName: "package_monitor", // 使用独立的 WorkerName
		EventType:  "package_inventory",
		Timestamp:  time.Now(),
		Data:       dataMap,
	}

	sendEvent(event)
}

// getInstalledPackages 获取已安装的软件包列表
func (pc *PackageCollector) getInstalledPackages() ([]map[string]string, error) {
	switch pc.pkgManager {
	case "debian":
		return pc.getDebianPackages()
	case "redhat":
		return pc.getRpmPackages()
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", pc.pkgManager)
	}
}

// getDebianPackages 获取 apt/dpkg 安装的软件包
func (pc *PackageCollector) getDebianPackages() ([]map[string]string, error) {
	statusFile := "/var/lib/dpkg/status"
	file, err := os.Open(statusFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %v", statusFile, err)
	}
	defer file.Close()

	var packages []map[string]string
	scanner := bufio.NewScanner(file)

	currentPkg := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if name, ok := currentPkg["Package"]; ok && name != "" {
				status := currentPkg["Status"]
				if strings.Contains(status, "install ok installed") {
					packages = append(packages, map[string]string{
						"name":         name,
						"version":      currentPkg["Version"],
						"architecture": currentPkg["Architecture"],
					})
				}
			}
			currentPkg = make(map[string]string)
			continue
		}

		if strings.HasPrefix(line, "Package: ") {
			currentPkg["Package"] = strings.TrimPrefix(line, "Package: ")
		} else if strings.HasPrefix(line, "Version: ") {
			currentPkg["Version"] = strings.TrimPrefix(line, "Version: ")
		} else if strings.HasPrefix(line, "Architecture: ") {
			currentPkg["Architecture"] = strings.TrimPrefix(line, "Architecture: ")
		} else if strings.HasPrefix(line, "Status: ") {
			currentPkg["Status"] = strings.TrimPrefix(line, "Status: ")
		}
	}

	// 处理最后一个包
	if name, ok := currentPkg["Package"]; ok && name != "" {
		status := currentPkg["Status"]
		if strings.Contains(status, "install ok installed") {
			packages = append(packages, map[string]string{
				"name":         name,
				"version":      currentPkg["Version"],
				"architecture": currentPkg["Architecture"],
			})
		}
	}

	return packages, nil
}

// getRpmPackages 获取 rpm/yum/dnf 安装的软件包
func (pc *PackageCollector) getRpmPackages() ([]map[string]string, error) {
	// 检查缓存是否有效
	if pc.cachedPkgs != nil && time.Since(pc.cacheTime) < pc.cacheTTL {
		log.Printf("[package_collector] getRpmPackages: using cached data (%d packages, age: %v)",
			len(pc.cachedPkgs), time.Since(pc.cacheTime))
		return pc.cachedPkgs, nil
	}

	// 检查是否有 rpmdb 操作正在进行
	pc.rpmdbMu.Lock()
	if pc.rpmdbRunning {
		pc.rpmdbMu.Unlock()
		log.Printf("[package_collector] getRpmPackages: rpmdb operation in progress, using rpm command...")
		return pc.getRpmPackagesFromCommand()
	}
	pc.rpmdbMu.Unlock()

	log.Printf("[package_collector] getRpmPackages: trying rpmdb first...")

	// 优先尝试 rpmdb
	packages, err := pc.getRpmPackagesFromDB()
	if err != nil {
		log.Printf("[package_collector] getRpmPackages: rpmdb failed (%v), falling back to rpm command...", err)
		packages, err = pc.getRpmPackagesFromCommand()
		if err != nil {
			return nil, fmt.Errorf("failed to get rpm packages: %v", err)
		}
	}

	// 更新缓存
	pc.cachedPkgs = packages
	pc.cacheTime = time.Now()

	log.Printf("[package_collector] Successfully read %d packages", len(packages))
	return packages, nil
}

// detectRpmDBFormat 检测 RPM 数据库格式
func (pc *PackageCollector) detectRpmDBFormat(dbFile string) string {
	baseName := filepath.Base(dbFile)
	if baseName == "rpmdb.sqlite" {
		return "sqlite"
	}
	if baseName == "Packages.db" {
		return "ndb"
	}

	f, err := os.Open(dbFile)
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	header := make([]byte, 16)
	n, err := f.Read(header)
	if err != nil || n < 16 {
		return "unknown"
	}

	if string(header[:15]) == "SQLite format 3" {
		return "sqlite"
	}

	if (header[0] == 0x00 && header[1] == 0x05 && header[2] == 0x31 && header[3] == 0x62) ||
		(header[0] == 0x62 && header[1] == 0x31 && header[2] == 0x05 && header[3] == 0x00) ||
		(header[0] == 0x00 && header[1] == 0x06 && header[2] == 0x15 && header[3] == 0x61) ||
		(header[0] == 0x61 && header[1] == 0x15 && header[2] == 0x06 && header[3] == 0x00) {
		return "bdb"
	}

	return "unknown"
}

// getRpmPackagesFromDB 使用 go-rpmdb 直接读取数据库
func (pc *PackageCollector) getRpmPackagesFromDB() ([]map[string]string, error) {
	log.Printf("[package_collector] getRpmPackagesFromDB: starting...")

	pc.rpmdbMu.Lock()
	if pc.rpmdbRunning {
		pc.rpmdbMu.Unlock()
		return nil, fmt.Errorf("rpmdb operation already in progress")
	}
	pc.rpmdbRunning = true
	pc.rpmdbMu.Unlock()

	defer func() {
		pc.rpmdbMu.Lock()
		pc.rpmdbRunning = false
		pc.rpmdbMu.Unlock()
	}()

	dbDirs := []string{"/var/lib/rpm", "/usr/lib/sysimage/rpm"}
	dbFiles := []string{"rpmdb.sqlite", "Packages.db", "Packages"}

	var dbFile string
	var dbSize int64
	var dbFormat string

	for _, dir := range dbDirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		for _, file := range dbFiles {
			filePath := filepath.Join(dir, file)
			if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
				format := pc.detectRpmDBFormat(filePath)

				if format == "bdb" && fi.Size() > MaxBerkeleyDBSize {
					continue
				}

				dbFile = filePath
				dbSize = fi.Size()
				dbFormat = format
				break
			}
		}
		if dbFile != "" {
			break
		}
	}

	if dbFile == "" {
		return nil, fmt.Errorf("no suitable rpmdb file found in %v", dbDirs)
	}

	var timeout time.Duration
	switch dbFormat {
	case "sqlite":
		timeout = RpmDBTimeoutSQLite
	case "bdb":
		timeout = RpmDBTimeoutBerkeley
	default:
		timeout = RpmDBTimeoutSQLite
	}

	type rpmdbResult struct {
		packages []map[string]string
		err      error
	}
	resultChan := make(chan rpmdbResult, 1)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		log.Printf("[package_collector] getRpmPackagesFromDB: opening db file: %s (size: %d, format: %s)",
			dbFile, dbSize, dbFormat)
		db, err := rpmdb.Open(dbFile)
		if err != nil {
			select {
			case resultChan <- rpmdbResult{nil, fmt.Errorf("failed to open rpmdb %s: %v", dbFile, err)}:
			case <-ctx.Done():
			}
			return
		}
		defer db.Close()

		select {
		case <-ctx.Done():
			return
		default:
		}

		pkgList, err := db.ListPackages()
		if err != nil {
			select {
			case resultChan <- rpmdbResult{nil, fmt.Errorf("failed to list packages from rpmdb: %v", err)}:
			case <-ctx.Done():
			}
			return
		}

		var packages []map[string]string
		for _, pkg := range pkgList {
			packages = append(packages, map[string]string{
				"name":         pkg.Name,
				"version":      fmt.Sprintf("%s-%s", pkg.Version, pkg.Release),
				"architecture": pkg.Arch,
			})
		}

		select {
		case resultChan <- rpmdbResult{packages, nil}:
		case <-ctx.Done():
		}
	}()

	select {
	case result := <-resultChan:
		return result.packages, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("rpmdb read timeout after %v (format: %s)", timeout, dbFormat)
	}
}

// getRpmPackagesFromCommand 使用 rpm 命令获取软件包列表
func (pc *PackageCollector) getRpmPackagesFromCommand() ([]map[string]string, error) {
	rpmPath, err := exec.LookPath("rpm")
	if err != nil {
		return nil, fmt.Errorf("rpm command not found: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), RpmCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, rpmPath, "-qa", "--queryformat", "%{NAME}\t%{VERSION}-%{RELEASE}\t%{ARCH}\n")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("rpm command timeout after %v", RpmCommandTimeout)
		}
		return nil, fmt.Errorf("rpm command failed: %v", err)
	}

	var packages []map[string]string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) >= 3 {
			packages = append(packages, map[string]string{
				"name":         fields[0],
				"version":      fields[1],
				"architecture": fields[2],
			})
		} else if len(fields) >= 1 {
			packages = append(packages, map[string]string{
				"name":         fields[0],
				"version":      "",
				"architecture": "",
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse rpm output: %v", err)
	}

	return packages, nil
}
