package syncer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mongo_syncer/config"
	"mongo_syncer/database"
	oo "mongo_syncer/openobserve"
)

// Syncer OpenObserve 到 MongoDB 定时轮询同步器
type Syncer struct {
	config  *config.Config
	mongodb *database.MongoDB
	oo      *oo.Client

	// cursors 记录流水型数据（进程/告警）上次轮询到的最大 _timestamp（微秒），用于增量拉取
	cursors   map[string]int64
	cursorsMu sync.Mutex
}

// NewSyncer 创建同步器
func NewSyncer(cfg *config.Config, mongodb *database.MongoDB) *Syncer {
	client := oo.NewClient(cfg.OpenObserve.URL, cfg.OpenObserve.Org, cfg.OpenObserve.Username, cfg.OpenObserve.Password)

	return &Syncer{
		config:  cfg,
		mongodb: mongodb,
		oo:      client,
		cursors: make(map[string]int64),
	}
}

// Start 启动轮询循环
func (s *Syncer) Start(ctx context.Context) {
	log.Println("[INFO] 启动 OpenObserve 到 MongoDB 轮询同步器")

	interval := time.Duration(s.config.Sync.PollInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动立即跑一轮，之后按轮询间隔重复
	s.pollOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] 收到退出信号，停止轮询")
			return
		case <-ticker.C:
			s.pollOnce(ctx)
		}
	}
}

// pollOnce 执行一轮完整的同步
func (s *Syncer) pollOnce(ctx context.Context) {
	log.Println("[INFO] 开始一轮同步")

	s.syncSystemInfo(ctx)
	s.syncAlerts(ctx)
	s.syncPorts(ctx)
	s.syncPackages(ctx)
	s.syncContainers(ctx)
	s.syncProcesses(ctx)

	log.Println("[INFO] 本轮同步完成")
}

// timeRange 返回一个宽泛的时间范围（微秒），用于快照型查询（不依赖游标，只取最新）
func (s *Syncer) timeRange() (int64, int64) {
	end := time.Now().UnixMicro()
	start := time.Now().Add(-24 * time.Hour).UnixMicro()
	return start, end
}

// latestPerAgent 对快照型 stream，先查每个 agent_id 的最大 _timestamp，
// 再针对每个 (agent_id, maxTs) 精确查出那一条完整记录。
// 1台agent量级下这个"两步查询"策略开销可忽略；OpenObserve 的 SQL 方言对子查询支持不确定，
// 用这种显式两步的写法更稳妥。
func (s *Syncer) latestPerAgent(ctx context.Context, stream string) ([]map[string]interface{}, error) {
	start, end := s.timeRange()

	groupSQL := fmt.Sprintf("SELECT agent_id, MAX(_timestamp) as max_ts FROM %s GROUP BY agent_id", stream)
	groups, err := s.oo.Search(ctx, groupSQL, start, end)
	if err != nil {
		return nil, fmt.Errorf("查询 %s 最新时间戳失败: %w", stream, err)
	}

	results := make([]map[string]interface{}, 0, len(groups))
	for _, g := range groups {
		agentID := oo.ToString(g["agent_id"])
		maxTs := oo.ToInt64(g["max_ts"])
		if agentID == "" || maxTs == 0 {
			continue
		}

		detailSQL := fmt.Sprintf(
			"SELECT * FROM %s WHERE agent_id = '%s' AND _timestamp = %d LIMIT 1",
			stream, escapeSQLString(agentID), maxTs,
		)
		hits, err := s.oo.Search(ctx, detailSQL, start, end)
		if err != nil {
			log.Printf("[ERROR] 查询 %s 详情失败 (agent_id=%s): %v", stream, agentID, err)
			continue
		}
		if len(hits) > 0 {
			results = append(results, hits[0])
		}
	}

	return results, nil
}

// escapeSQLString 转义 SQL 字符串里的单引号，防止 agent_id 里出现特殊字符破坏查询语句
func escapeSQLString(s string) string {
	result := ""
	for _, r := range s {
		if r == '\'' {
			result += "''"
		} else {
			result += string(r)
		}
	}
	return result
}

// syncSystemInfo 同步系统信息（快照型：hids_system_info -> agents collection）
func (s *Syncer) syncSystemInfo(ctx context.Context) {
	hits, err := s.latestPerAgent(ctx, "hids_system_info")
	if err != nil {
		log.Printf("[ERROR] 同步系统信息失败: %v", err)
		return
	}

	successCount, errorCount := 0, 0
	for _, hit := range hits {
		if err := s.mongodb.UpsertSystemInfo(ctx, hit); err != nil {
			log.Printf("[ERROR] upsert 系统信息失败: %v", err)
			errorCount++
		} else {
			successCount++
		}
	}
	if len(hits) > 0 {
		log.Printf("[INFO] 系统信息同步: 成功=%d, 失败=%d", successCount, errorCount)
	}
}

// syncAlerts 同步告警（流水型：hids_alerts -> agent_security_events collection，不去重）
func (s *Syncer) syncAlerts(ctx context.Context) {
	hits := s.fetchIncremental(ctx, "hids_alerts")
	if len(hits) == 0 {
		return
	}

	if err := s.mongodb.BatchInsertAlerts(ctx, hits); err != nil {
		log.Printf("[ERROR] 批量插入告警失败: %v", err)
		return
	}
	log.Printf("[INFO] 告警同步: %d 条", len(hits))
}

// syncPorts 同步端口快照（快照型：hids_port_monitor -> agent_ports collection）
func (s *Syncer) syncPorts(ctx context.Context) {
	hits, err := s.latestPerAgent(ctx, "hids_port_monitor")
	if err != nil {
		log.Printf("[ERROR] 同步端口信息失败: %v", err)
		return
	}

	for _, hit := range hits {
		agentID := oo.ToString(hit["agent_id"])
		if agentID == "" {
			continue
		}
		ports := oo.ExtractJSONArray(hit["data_ports"])
		if err := s.mongodb.ReplacePorts(ctx, agentID, ports); err != nil {
			log.Printf("[ERROR] 替换端口快照失败 (agent_id=%s): %v", agentID, err)
			continue
		}
		log.Printf("[INFO] 端口快照同步: agent_id=%s, count=%d", agentID, len(ports))
	}
}

// syncPackages 同步软件包快照（快照型：hids_package_info -> agent_software collection）
func (s *Syncer) syncPackages(ctx context.Context) {
	hits, err := s.latestPerAgent(ctx, "hids_package_info")
	if err != nil {
		log.Printf("[ERROR] 同步软件包信息失败: %v", err)
		return
	}

	for _, hit := range hits {
		agentID := oo.ToString(hit["agent_id"])
		if agentID == "" {
			continue
		}
		packages := oo.ExtractJSONArray(hit["data_packages"])
		packageManager := oo.ToString(hit["data_package_manager"])
		if err := s.mongodb.ReplaceSoftware(ctx, agentID, packageManager, packages); err != nil {
			log.Printf("[ERROR] 替换软件快照失败 (agent_id=%s): %v", agentID, err)
			continue
		}
		log.Printf("[INFO] 软件包快照同步: agent_id=%s, count=%d", agentID, len(packages))
	}
}

// syncContainers 同步容器快照（快照型：hids_container_info -> agent_containers collection）
func (s *Syncer) syncContainers(ctx context.Context) {
	hits, err := s.latestPerAgent(ctx, "hids_container_info")
	if err != nil {
		log.Printf("[ERROR] 同步容器信息失败: %v", err)
		return
	}

	for _, hit := range hits {
		agentID := oo.ToString(hit["agent_id"])
		if agentID == "" {
			continue
		}
		containers := oo.ExtractJSONArray(hit["data_containers"])
		if err := s.mongodb.ReplaceContainers(ctx, agentID, containers); err != nil {
			log.Printf("[ERROR] 替换容器快照失败 (agent_id=%s): %v", agentID, err)
			continue
		}
		log.Printf("[INFO] 容器快照同步: agent_id=%s, count=%d", agentID, len(containers))
	}
}

// syncProcesses 同步进程信息（流水型：hids_process_monitor -> agent_processes collection，按 (agent_id,pid) upsert）
func (s *Syncer) syncProcesses(ctx context.Context) {
	hits := s.fetchIncremental(ctx, "hids_process_monitor")
	if len(hits) == 0 {
		return
	}

	successCount, errorCount := 0, 0
	for _, hit := range hits {
		agentID := oo.ToString(hit["agent_id"])
		if agentID == "" {
			continue
		}
		if err := s.mongodb.UpsertProcess(ctx, agentID, hit); err != nil {
			errorCount++
			continue
		}
		successCount++
	}
	log.Printf("[INFO] 进程同步: 成功=%d, 失败=%d", successCount, errorCount)
}

// fetchIncremental 按内存游标增量拉取流水型 stream 的新记录，拉取后更新游标
func (s *Syncer) fetchIncremental(ctx context.Context, stream string) []map[string]interface{} {
	s.cursorsMu.Lock()
	lastTs := s.cursors[stream]
	s.cursorsMu.Unlock()

	now := time.Now().UnixMicro()
	if lastTs == 0 {
		lastTs = time.Now().Add(-time.Duration(s.config.Sync.LookbackSecs) * time.Second).UnixMicro()
	}

	sql := fmt.Sprintf("SELECT * FROM %s WHERE _timestamp > %d ORDER BY _timestamp ASC LIMIT 5000", stream, lastTs)
	hits, err := s.oo.Search(ctx, sql, lastTs, now)
	if err != nil {
		log.Printf("[ERROR] 增量查询 %s 失败: %v", stream, err)
		return nil
	}

	if len(hits) == 0 {
		return nil
	}

	maxTs := lastTs
	for _, hit := range hits {
		ts := oo.ToInt64(hit["_timestamp"])
		if ts > maxTs {
			maxTs = ts
		}
	}

	s.cursorsMu.Lock()
	s.cursors[stream] = maxTs
	s.cursorsMu.Unlock()

	return hits
}

// Close 关闭同步器（当前无额外资源需要释放，保留接口兼容 main.go 调用）
func (s *Syncer) Close() error {
	log.Println("[INFO] 关闭同步器")
	return nil
}
