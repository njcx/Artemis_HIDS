//go:build linux

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"artemis_agent/shared"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

const (
	NETLINK_CONNECTOR    = 11
	CN_IDX_PROC          = 0x1
	CN_VAL_PROC          = 0x1
	PROC_EVENT_EXEC      = 0x00000002
	PROC_CN_MCAST_LISTEN = 1
)

type NetlinkMonitor struct {
	ctx             context.Context
	callback        shared.DataCallback
	nlConn          *netlink.Conn
	eventCount      *atomic.Uint64
	suspiciousCount *atomic.Uint64
	lastEventTime   atomic.Int64 // 最后接收事件的时间戳
	stopMonitor     chan struct{}
	eventQueue      chan *ExecEvent // 异步事件处理队列
	wg              sync.WaitGroup  // 等待所有 goroutine 退出
}

// NetlinkReceiveTimeout netlink 接收超时时间
const NetlinkReceiveTimeout = 30 * time.Second

func NewNetlinkMonitor(ctx context.Context, callback shared.DataCallback, eventCount *atomic.Uint64, suspiciousCount *atomic.Uint64) (*NetlinkMonitor, error) {
	conn, err := netlink.Dial(unix.NETLINK_CONNECTOR, &netlink.Config{
		Groups: CN_IDX_PROC,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dial netlink: %v", err)
	}

	// 设置读取超时，防止在老内核上永久阻塞
	if err := conn.SetReadDeadline(time.Now().Add(NetlinkReceiveTimeout)); err != nil {
		log.Printf("[NetlinkMonitor] Warning: failed to set read deadline: %v", err)
	}

	m := &NetlinkMonitor{
		ctx:             ctx,
		callback:        callback,
		nlConn:          conn,
		eventCount:      eventCount,
		suspiciousCount: suspiciousCount,
		stopMonitor:     make(chan struct{}),
		eventQueue:      make(chan *ExecEvent, 1000),
	}
	m.lastEventTime.Store(time.Now().Unix())
	return m, nil
}

func (m *NetlinkMonitor) Start() error {
	if err := m.subscribeProcEvents(); err != nil {
		return err
	}

	// 启动异步事件处理 workers
	for i := 0; i < 4; i++ {
		m.wg.Add(1)
		go m.processEventWorker()
	}

	m.wg.Add(1)
	go m.receiveEvents()
	m.wg.Add(1)
	go m.monitorHealth()
	return nil
}

func (m *NetlinkMonitor) Stop() {
	if m.stopMonitor != nil {
		close(m.stopMonitor)
	}
	if m.nlConn != nil {
		m.nlConn.Close()
	}
	if m.eventQueue != nil {
		close(m.eventQueue)
	}
	m.wg.Wait()
	log.Println("[NetlinkMonitor] All goroutines stopped")
}

func (m *NetlinkMonitor) subscribeProcEvents() error {
	msg := make([]byte, 20)

	// cn_msg header
	binary.LittleEndian.PutUint32(msg[0:4], CN_IDX_PROC)
	binary.LittleEndian.PutUint32(msg[4:8], CN_VAL_PROC)
	binary.LittleEndian.PutUint32(msg[8:12], 0)  // seq
	binary.LittleEndian.PutUint32(msg[12:16], 0) // ack
	binary.LittleEndian.PutUint16(msg[16:18], 4) // len
	binary.LittleEndian.PutUint16(msg[18:20], 0) // flags

	// op (PROC_CN_MCAST_LISTEN)
	msg = append(msg, byte(PROC_CN_MCAST_LISTEN), 0, 0, 0)

	if _, err := m.nlConn.Send(netlink.Message{Data: msg}); err != nil {
		return fmt.Errorf("failed to subscribe: %v", err)
	}

	log.Println("[NetlinkMonitor] Subscribed to process events")
	return nil
}

// monitorHealth 监控 receiveEvents 是否正常运行
func (m *NetlinkMonitor) monitorHealth() {
	defer m.wg.Done()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.stopMonitor:
			return
		case <-ticker.C:
			lastEventTime := time.Unix(m.lastEventTime.Load(), 0)
			timeSinceLastEvent := time.Since(lastEventTime)

			// 如果超过 5 分钟没有接收到任何事件，警告
			if timeSinceLastEvent > 5*time.Minute {
				log.Printf("[NetlinkMonitor] WARNING: No events received for %v (last event at: %s)",
					timeSinceLastEvent.Round(time.Second), lastEventTime.Format("2006-01-02 15:04:05"))
			}
		}
	}
}

func (m *NetlinkMonitor) receiveEvents() {
	defer m.wg.Done()

	errorCount := 0
	lastErrorTime := time.Now()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.stopMonitor:
			return
		default:
		}

		// 重新设置读取超时
		if err := m.nlConn.SetReadDeadline(time.Now().Add(NetlinkReceiveTimeout)); err != nil {
			log.Printf("[NetlinkMonitor] Warning: failed to set read deadline: %v", err)
		}

		msgs, err := m.nlConn.Receive()
		if err != nil {
			if m.ctx.Err() != nil {
				return
			}

			// 检查是否是超时错误
			if isTimeoutError(err) {
				// 超时不是错误，只是没有事件，继续循环
				continue
			}

			// 错误频率控制
			now := time.Now()
			if now.Sub(lastErrorTime) > time.Second {
				log.Printf("[NetlinkMonitor] Error receiving: %v (suppressing further errors for 1s)", err)
				lastErrorTime = now
				errorCount = 0
			} else {
				errorCount++
			}

			time.Sleep(10 * time.Millisecond)
			continue
		}

		// 成功接收，重置错误计数
		if errorCount > 0 {
			log.Printf("[NetlinkMonitor] Recovered after %d errors", errorCount)
			errorCount = 0
		}

		// 更新最后事件时间
		m.lastEventTime.Store(time.Now().Unix())

		for _, msg := range msgs {
			m.handleMessage(msg.Data)
		}
	}
}

// isTimeoutError 检查是否是超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// 检查是否是 net.Error 类型的超时
	if netErr, ok := err.(interface{ Timeout() bool }); ok {
		return netErr.Timeout()
	}
	// 检查错误消息
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "i/o timeout")
}

func (m *NetlinkMonitor) handleMessage(data []byte) {
	if len(data) < 36 {
		return
	}

	// 跳过 cn_msg (20 bytes) + proc_event_header (16 bytes)
	offset := 36

	if len(data) < offset+4 {
		return
	}

	// 读取事件类型
	eventType := binary.LittleEndian.Uint32(data[20:24])
	if eventType != PROC_EVENT_EXEC {
		return
	}

	// 读取 PID
	pid := int(binary.LittleEndian.Uint32(data[offset : offset+4]))

	// 读取进程信息
	event := m.readProcessInfo(pid)
	if event == nil {
		return
	}

	// 异步处理事件，不阻塞接收循环
	m.enqueueEvent(event)
}

func (m *NetlinkMonitor) readProcessInfo(pid int) *ExecEvent {
	event := &ExecEvent{
		PID:       pid,
		Syscall:   "execve",
		Timestamp: time.Now(),
	}

	procPath := fmt.Sprintf("/proc/%d", pid)

	// 首先读取 status 获取 PPID，然后立即读取父进程信息
	// 这样可以在父进程退出前尽可能获取到信息
	var ppid int
	if data, err := os.ReadFile(fmt.Sprintf("%s/status", procPath)); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			switch fields[0] {
			case "Uid:":
				if uid, err := strconv.Atoi(fields[1]); err == nil {
					event.UID = uid
					event.User = getUsernameFromUID(uid)
				}
			case "Gid:":
				if gid, err := strconv.Atoi(fields[1]); err == nil {
					event.GID = gid
				}
			case "PPid:":
				if p, err := strconv.Atoi(fields[1]); err == nil {
					ppid = p
					event.PPID = ppid
				}
			}
		}
	}

	// 立即读取父进程信息（在父进程退出前）
	if ppid > 0 {
		event.ParentComm, event.ParentExe, event.ParentCmdline = readParentProcessInfo(ppid)
	}

	// 读取 cwd
	cwdRead := false
	if link, err := os.Readlink(fmt.Sprintf("%s/cwd", procPath)); err == nil {
		event.CWD = link
		cwdRead = true
	}

	// 读取 comm
	if data, err := os.ReadFile(fmt.Sprintf("%s/comm", procPath)); err == nil {
		event.Command = strings.TrimSpace(string(data))
	}

	// 读取 exe
	if link, err := os.Readlink(fmt.Sprintf("%s/exe", procPath)); err == nil {
		event.Exe = link
	}

	// 读取 cmdline
	if data, err := os.ReadFile(fmt.Sprintf("%s/cmdline", procPath)); err == nil {
		event.Args = strings.Split(string(data), "\x00")
		if len(event.Args) > 0 && event.Args[len(event.Args)-1] == "" {
			event.Args = event.Args[:len(event.Args)-1]
		}
	}

	// 如果第一次 cwd 读取失败，重试一次
	if !cwdRead {
		time.Sleep(5 * time.Millisecond)
		if link, err := os.Readlink(fmt.Sprintf("%s/cwd", procPath)); err == nil {
			event.CWD = link
		}
	}

	return event
}

// processEventWorker 异步处理事件，避免 callback 阻塞主循环
func (m *NetlinkMonitor) processEventWorker() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event, ok := <-m.eventQueue:
			if !ok {
				return
			}
			// 执行实际的事件处理
			m.processEvent(event)
		}
	}
}

// enqueueEvent 将事件放入队列异步处理
func (m *NetlinkMonitor) enqueueEvent(event *ExecEvent) {
	select {
	case m.eventQueue <- event:
		// 成功入队
	case <-time.After(100 * time.Millisecond):
		// 队列满，记录警告但不阻塞
		log.Printf("[NetlinkMonitor] WARNING: Event queue full, dropping event: cmd=%s", event.Command)
	}
}

func (m *NetlinkMonitor) processEvent(event *ExecEvent) {
	// 过滤：关键字段为空则丢弃
	if event.Command == "" || event.Exe == "" || event.PID == 0 {
		return
	}

	// 更新计数器
	if m.eventCount != nil {
		m.eventCount.Add(1)
	}

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
			"monitor_mode":   "netlink-connector",
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
