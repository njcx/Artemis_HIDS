package shared

import (
	"context"

	"artemis_agent/pkg/gomw"
	"artemis_agent/pkg/gomw/protocol"

	"github.com/sirupsen/logrus"
)

// StartGomwWorker 启动基于 gomw 的 Worker
func StartGomwWorker(name, version string, impl MonitorWorker) {
	logger := logrus.New()

	// 创建 gomw Worker
	worker, err := gomw.NewWorker(gomw.WorkerConfig{
		Name:    name,
		Version: version,
		Logger:  logger,
	})
	if err != nil {
		logger.Fatalf("Failed to create worker: %v", err)
	}

	// 创建回调适配器
	callback := &GomwCallback{worker: worker}

	// 在后台启动业务逻辑
	go func() {
		if err := impl.Start(context.Background(), callback); err != nil {
			logger.Fatalf("Failed to start worker implementation: %v", err)
		}
	}()

	// 启动 gomw (阻塞)
	adapter := &GomwHandlerAdapter{Impl: impl}
	if err := worker.Start(adapter); err != nil {
		logger.Fatalf("Worker stopped: %v", err)
	}
}

// GomwHandlerAdapter 适配 gomw 的 WorkerHandler
type GomwHandlerAdapter struct {
	Impl MonitorWorker
}

func (a *GomwHandlerAdapter) HandleCommand(cmd *protocol.Command) error {
	// TODO: 实现命令分发
	return nil
}

// GomwCallback 适配 DataCallback
type GomwCallback struct {
	worker *gomw.Worker
}

func (c *GomwCallback) Send(data *MonitorData) error {
	err := c.worker.ReportEventWithName(data.WorkerName, data.EventType, data.Data)
	if err != nil {
		logrus.Errorf("[GomwCallback] Failed to send event: WorkerName=%s, EventType=%s, Error=%v",
			data.WorkerName, data.EventType, err)
	}
	return err
}

// StandaloneCallback 用于 standalone 模式的简单回调
type StandaloneCallback struct {
	Logger *logrus.Logger
}

func (c *StandaloneCallback) Send(data *MonitorData) error {
	c.Logger.Infof("[Standalone] Event: %s, Severity: %d, Data: %+v",
		data.EventType, data.Severity, data.Data)
	return nil
}

// StartStandalone 启动 standalone 模式
func StartStandalone(impl MonitorWorker) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Info("Running in standalone mode")

	ctx := context.Background()
	callback := &StandaloneCallback{Logger: logger}

	if err := impl.Start(ctx, callback); err != nil {
		logger.Fatalf("Failed to start worker: %v", err)
	}

	// 阻塞主线程
	select {}
}
