package main

import (
	"context"
	"log"
	"os"
	"sync"

	"artemis_agent/shared"
)

// SystemInfoMonitor 系统信息综合监控器
// 合并 port_monitor, system_monitor, package_monitor, language_monitor, container_monitor, process_monitor
// 每个采集器上报时使用各自的 WorkerName，数据会写入不同的 Kafka topic
type SystemInfoMonitor struct {
	ctx          context.Context
	cancel       context.CancelFunc
	callback     shared.DataCallback
	eventQueue   []*shared.MonitorData
	queueMutex   sync.Mutex
	collectionMu sync.Mutex // 全局采集锁，防止多个采集器同时消耗 CPU
	maxQueueSize int

	// 各采集器
	portCollector      *PortCollector
	systemCollector    *SystemCollector
	packageCollector   *PackageCollector
	languageCollector  *LanguageCollector
	containerCollector *ContainerCollector
	processCollector   *ProcessCollector
}

// Start 启动监控
func (sim *SystemInfoMonitor) Start(ctx context.Context, callback shared.DataCallback) error {
	// 确保 log 输出到 stderr，以便被主进程捕获
	log.SetOutput(os.Stderr)

	sim.ctx, sim.cancel = context.WithCancel(ctx)
	sim.callback = callback
	sim.maxQueueSize = 200
	sim.eventQueue = make([]*shared.MonitorData, 0, sim.maxQueueSize)

	hostname, _ := os.Hostname()
	log.Printf("[system_info_monitor] Starting on %s", hostname)

	// 启动父进程监控
	go shared.WatchParentProcess(sim.ctx)

	// 初始化各采集器
	sim.portCollector = NewPortCollector()
	sim.systemCollector = NewSystemCollector()
	sim.packageCollector = NewPackageCollector()
	sim.languageCollector = NewLanguageCollector()
	sim.containerCollector = NewContainerCollector()
	sim.processCollector = NewProcessCollector()

	// 启动各采集器的采集循环（带 panic 恢复）
	go sim.safeCollectLoop("PortCollector", sim.portCollector.CollectLoop, &sim.collectionMu)
	go sim.safeCollectLoop("SystemCollector", sim.systemCollector.CollectLoop, &sim.collectionMu)
	go sim.safeCollectLoop("PackageCollector", sim.packageCollector.CollectLoop, &sim.collectionMu)
	go sim.safeCollectLoop("LanguageCollector", sim.languageCollector.CollectLoop, &sim.collectionMu)
	go sim.safeCollectLoop("ContainerCollector", sim.containerCollector.CollectLoop, &sim.collectionMu)
	go sim.safeCollectLoop("ProcessCollector", sim.processCollector.CollectLoop, &sim.collectionMu)

	log.Printf("[system_info_monitor] All collectors started")
	return nil
}

// safeCollectLoop 安全地运行采集循环，捕获 panic
func (sim *SystemInfoMonitor) safeCollectLoop(name string, collectFunc func(context.Context, func(*shared.MonitorData), *sync.Mutex), mu *sync.Mutex) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[system_info_monitor] %s panic recovered: %v", name, r)
		}
	}()

	log.Printf("[system_info_monitor] Starting %s...", name)
	collectFunc(sim.ctx, sim.sendEvent, mu)
	log.Printf("[system_info_monitor] %s stopped", name)
}

// sendEvent 统一的事件发送方法
func (sim *SystemInfoMonitor) sendEvent(event *shared.MonitorData) {
	if sim.callback != nil {
		log.Printf("[system_info_monitor] Sending event: WorkerName=%s, EventType=%s", event.WorkerName, event.EventType)
		if err := sim.callback.Send(event); err != nil {
			log.Printf("[system_info_monitor] callback.Send failed: %v, adding to queue", err)
			sim.addToQueue(event)
		} else {
			log.Printf("[system_info_monitor] Event sent successfully: WorkerName=%s", event.WorkerName)
		}
	} else {
		log.Printf("[system_info_monitor] callback is nil, adding to queue")
		sim.addToQueue(event)
	}
}

// addToQueue 添加事件到队列
func (sim *SystemInfoMonitor) addToQueue(event *shared.MonitorData) {
	sim.queueMutex.Lock()
	defer sim.queueMutex.Unlock()

	if len(sim.eventQueue) >= sim.maxQueueSize {
		sim.eventQueue = sim.eventQueue[1:]
	}
	sim.eventQueue = append(sim.eventQueue, event)
}

// Stop 停止监控
func (sim *SystemInfoMonitor) Stop() error {
	log.Println("[system_info_monitor] Stopping...")
	if sim.cancel != nil {
		sim.cancel()
	}
	return nil
}

// GetEvents 获取待发送的事件
func (sim *SystemInfoMonitor) GetEvents() ([]*shared.MonitorData, error) {
	sim.queueMutex.Lock()
	defer sim.queueMutex.Unlock()

	events := make([]*shared.MonitorData, len(sim.eventQueue))
	copy(events, sim.eventQueue)
	sim.eventQueue = sim.eventQueue[:0]

	return events, nil
}

// Name 返回 worker 名称
func (sim *SystemInfoMonitor) Name() string {
	return "system_info_monitor"
}

// GetMetrics 返回性能指标
func (sim *SystemInfoMonitor) GetMetrics() (map[string]interface{}, error) {
	sim.queueMutex.Lock()
	queueLen := len(sim.eventQueue)
	sim.queueMutex.Unlock()

	return map[string]interface{}{
		"queue_length": queueLen,
		"max_queue":    sim.maxQueueSize,
	}, nil
}

// Health 健康检查
func (sim *SystemInfoMonitor) Health() bool {
	return sim.ctx != nil && sim.ctx.Err() == nil
}

// 主函数
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--standalone" {
		shared.StartStandalone(&SystemInfoMonitor{})
		return
	}
	shared.StartGomwWorker("system_info_monitor", "1.0.0", &SystemInfoMonitor{})
}
