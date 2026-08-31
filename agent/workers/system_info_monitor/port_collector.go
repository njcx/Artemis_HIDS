package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"artemis_agent/shared"

	"github.com/cakturk/go-netstat/netstat"
)

// PortCollector 端口监听采集器
type PortCollector struct {
}

// ListeningPort 监听端口信息 (保持与原始 port_monitor 一致)
type ListeningPort struct {
	Protocol    string `json:"protocol"`     // tcp/udp/tcp6/udp6
	LocalAddr   string `json:"local_addr"`   // 监听地址
	LocalPort   string `json:"local_port"`   // 监听端口
	PID         string `json:"pid"`          // 进程ID
	ProcessName string `json:"process_name"` // 进程名称
	CommandLine string `json:"command_line"` // 完整命令行
	User        string `json:"user"`         // 运行用户
}

// NewPortCollector 创建端口采集器
func NewPortCollector() *PortCollector {
	return &PortCollector{}
}

// CollectLoop 定期采集端口信息
func (pc *PortCollector) CollectLoop(ctx context.Context, sendEvent func(*shared.MonitorData), mu *sync.Mutex) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Printf("[port_collector] collectLoop started, will collect after 5 seconds")

	// 启动后延迟5秒采集第一次
	time.Sleep(5 * time.Second)
	log.Printf("[port_collector] Starting first collection...")

	if mu != nil {
		mu.Lock()
	}
	pc.collectPorts(sendEvent)
	if mu != nil {
		mu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[port_collector] collectLoop context cancelled, exiting")
			return
		case <-ticker.C:
			if mu != nil {
				mu.Lock()
			}
			pc.collectPorts(sendEvent)
			if mu != nil {
				mu.Unlock()
			}
		}
	}
}

// collectPorts 采集端口监听信息
func (pc *PortCollector) collectPorts(sendEvent func(*shared.MonitorData)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[port_collector] panic: %v", r)
		}
	}()

	ports := pc.getListeningPorts()

	if len(ports) == 0 {
		return
	}

	log.Printf("[port_collector] Found %d listening ports", len(ports))

	// 转换为 []interface{} 以便 structpb 处理
	portsList := make([]interface{}, 0, len(ports))
	for _, port := range ports {
		portsList = append(portsList, map[string]interface{}{
			"protocol":     port.Protocol,
			"local_addr":   port.LocalAddr,
			"local_port":   port.LocalPort,
			"pid":          port.PID,
			"process_name": port.ProcessName,
			"command_line": port.CommandLine,
			"user":         port.User,
		})
	}

	dataMap := map[string]interface{}{
		"timestamp":  time.Now().Unix(),
		"port_count": len(ports),
		"ports":      portsList,
	}

	event := &shared.MonitorData{
		WorkerName: "port_monitor", // 使用独立的 WorkerName
		EventType:  "listening_ports_inventory",
		Timestamp:  time.Now(),
		Data:       dataMap,
	}

	sendEvent(event)
}

// getListeningPorts 获取所有监听端口
func (pc *PortCollector) getListeningPorts() []ListeningPort {
	var ports []ListeningPort

	// TCP Socks
	tcpSocks, err := netstat.TCPSocks(netstat.NoopFilter)
	if err == nil {
		for _, sock := range tcpSocks {
			// sock.State == 10 (TCP_LISTEN)
			if sock.State == 10 {
				ports = append(ports, pc.convertSockToPort(sock, "tcp"))
			}
		}
	} else {
		log.Printf("[port_collector] Failed to get TCP socks: %v", err)
	}

	// TCP6 Socks
	tcp6Socks, err := netstat.TCP6Socks(netstat.NoopFilter)
	if err == nil {
		for _, sock := range tcp6Socks {
			if sock.State == 10 {
				ports = append(ports, pc.convertSockToPort(sock, "tcp6"))
			}
		}
	}

	// UDP Socks
	udpSocks, err := netstat.UDPSocks(netstat.NoopFilter)
	if err == nil {
		for _, sock := range udpSocks {
			ports = append(ports, pc.convertSockToPort(sock, "udp"))
		}
	}

	// UDP6 Socks
	udp6Socks, err := netstat.UDP6Socks(netstat.NoopFilter)
	if err == nil {
		for _, sock := range udp6Socks {
			ports = append(ports, pc.convertSockToPort(sock, "udp6"))
		}
	}

	// 过滤常见系统服务端口
	return pc.filterPorts(ports)
}

func (pc *PortCollector) convertSockToPort(sock netstat.SockTabEntry, protocol string) ListeningPort {
	pid := "-"
	pname := "-"
	cmdline := "-"
	user := pc.getUsername(fmt.Sprintf("%d", sock.UID))

	if sock.Process != nil {
		pid = strconv.Itoa(sock.Process.Pid)
		pname = sock.Process.Name
		cmdline = pc.getProcessCmdline(pid)
	}

	return ListeningPort{
		Protocol:    protocol,
		LocalAddr:   sock.LocalAddr.IP.String(),
		LocalPort:   fmt.Sprintf("%d", sock.LocalAddr.Port),
		PID:         pid,
		ProcessName: pname,
		CommandLine: cmdline,
		User:        user,
	}
}

// filterPorts 过滤常见系统服务端口
func (pc *PortCollector) filterPorts(ports []ListeningPort) []ListeningPort {
	var filtered []ListeningPort

	// 定义常见系统服务（可以选择性过滤）
	commonServices := map[string]bool{
		"22":   true, // SSH
		"53":   true, // DNS
		"631":  true, // CUPS
		"5353": true, // mDNS
	}

	// 定义常见系统进程（可以选择性过滤）
	systemProcesses := map[string]bool{
		"systemd-resolved": true,
		"avahi-daemon":     true,
		"cupsd":            true,
	}

	for _, port := range ports {
		// 过滤规则：
		// 1. 跳过 127.0.0.1 和 ::1 的端口（本地回环）
		if port.LocalAddr == "127.0.0.1" || port.LocalAddr == "::1" || port.LocalAddr == "127.0.0.53" {
			continue
		}

		// 2. 保留所有非常见端口（可疑端口）
		// 3. 或保留所有 0.0.0.0/:: 监听的端口（对外暴露）
		isCommon := commonServices[port.LocalPort] && systemProcesses[port.ProcessName]
		isExternal := port.LocalAddr == "0.0.0.0" || port.LocalAddr == "::"

		// 如果是常见服务且不对外，跳过
		if isCommon && !isExternal {
			continue
		}

		filtered = append(filtered, port)
	}

	return filtered
}

// getUsername 根据 UID 获取用户名
func (pc *PortCollector) getUsername(uid string) string {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return uid
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && fields[2] == uid {
			return fields[0]
		}
	}

	return uid
}

// getProcessCmdline 获取进程完整命令行
func (pc *PortCollector) getProcessCmdline(pid string) string {
	if pid == "-" || pid == "" {
		return "-"
	}

	cmdlineFile := fmt.Sprintf("/proc/%s/cmdline", pid)
	data, err := os.ReadFile(cmdlineFile)
	if err != nil {
		return "-"
	}

	cmdline := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.TrimSpace(cmdline)
}
