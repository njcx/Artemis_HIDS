// ============================================
// Agent 配置监听器 - 处理心跳响应中的命令
// ============================================
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	pb "artemis_agent/pb"
)

// CommandExecutor 命令执行器
type CommandExecutor struct {
	master *AgentMaster
	client pb.MonitorServiceClient
}

func NewCommandExecutor(master *AgentMaster, client pb.MonitorServiceClient) *CommandExecutor {
	return &CommandExecutor{
		master: master,
		client: client,
	}
}

// ProcessCommands 处理心跳响应中的命令
func (e *CommandExecutor) ProcessCommands(commands []*pb.Command) {
	for _, cmd := range commands {
		log.Printf("Executing command: id=%s type=%s", cmd.CommandId, cmd.CommandType)

		success := true
		message := "executed"

		switch cmd.CommandType {
		case "stop_worker":
			workerName := cmd.Payload["worker_name"]
			if err := e.stopWorker(workerName); err != nil {
				success = false
				message = err.Error()
			}

		case "start_worker":
			workerName := cmd.Payload["worker_name"]
			if err := e.startWorker(workerName); err != nil {
				success = false
				message = err.Error()
			}

		case "update_agent":
			version := cmd.Payload["version"]
			downloadURL := cmd.Payload["download_url"]
			checksum := cmd.Payload["checksum"]
			if err := e.updateAgent(version, downloadURL, checksum); err != nil {
				success = false
				message = err.Error()
			}

		case "update_worker":
			workerName := cmd.Payload["worker_name"]
			version := cmd.Payload["version"]
			downloadURL := cmd.Payload["download_url"]
			checksum := cmd.Payload["checksum"]
			if err := e.updateWorker(workerName, version, downloadURL, checksum); err != nil {
				success = false
				message = err.Error()
			}

		case "set_config":
			if err := e.setConfig(cmd.Payload); err != nil {
				success = false
				message = err.Error()
			}

		case "uninstall_agent":
			// uninstallAgent 会自己先回传一次"已收到，即将卸载"的结果，再异步自毁，
			// 所以这里不走下面统一的 reportResult，直接继续处理下一条命令即可。
			e.uninstallAgent(cmd)
			continue

		default:
			success = false
			message = "unknown command type"
		}

		// 记录执行结果
		log.Printf("Command %s executed: success=%v, message=%s", cmd.CommandId, success, message)

		// 回传执行结果给 server
		e.reportResult(cmd, success, message)
	}
}

// reportResult 上报命令执行结果
func (e *CommandExecutor) reportResult(cmd *pb.Command, success bool, message string) {
	if e.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := e.client.ReportCommandResult(ctx, &pb.CommandResult{
		AgentId:   e.master.serverClient.agentID,
		CommandId: cmd.CommandId,
		Success:   success,
		Message:   message,
		Timestamp: time.Now().UnixNano(),
	})
	if err != nil {
		log.Printf("Report command result failed: %v", err)
	}
}

// stopWorker 停止 Worker
func (e *CommandExecutor) stopWorker(workerName string) error {
	log.Printf("Stopping worker: %s", workerName)

	e.master.workersMux.Lock()
	instance, ok := e.master.workers[workerName]
	if !ok {
		e.master.workersMux.Unlock()
		return fmt.Errorf("worker %s not found", workerName)
	}
	e.master.workersMux.Unlock()

	// 停止 gomw Worker
	if err := e.master.gomwMaster.StopWorker(workerName); err != nil {
		log.Printf("gomw stop worker error: %v", err)
	}

	// 清理 CGroup
	e.master.cgroupMgr.Remove(instance.cgroupName)

	// 从 workers map 中移除
	e.master.workersMux.Lock()
	delete(e.master.workers, workerName)
	e.master.workersMux.Unlock()

	return nil
}

// startWorker 启动 Worker
func (e *CommandExecutor) startWorker(workerName string) error {
	log.Printf("Starting worker: %s", workerName)

	// 查找 worker 配置
	var workerConfig *WorkerConfig
	for i := range e.master.config.Workers {
		if e.master.config.Workers[i].Name == workerName {
			workerConfig = &e.master.config.Workers[i]
			break
		}
	}

	if workerConfig == nil {
		return fmt.Errorf("worker config for %s not found", workerName)
	}

	// 检查是否已经在运行
	e.master.workersMux.RLock()
	_, exists := e.master.workers[workerName]
	e.master.workersMux.RUnlock()

	if exists {
		return fmt.Errorf("worker %s is already running", workerName)
	}

	// 启动 worker
	return e.master.startWorker(*workerConfig)
}

// updateAgent 更新 Agent
func (e *CommandExecutor) updateAgent(version, downloadURL, checksum string) error {
	log.Printf("Updating agent to version %s from %s", version, downloadURL)

	// 1. 下载新版本
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("hids-agent-%s.tmp", version))
	if err := downloadFile(downloadURL, tmpFile); err != nil {
		return fmt.Errorf("download failed: %v", err)
	}
	defer os.Remove(tmpFile)

	// 2. 校验 checksum
	if err := verifyChecksum(tmpFile, checksum); err != nil {
		return fmt.Errorf("checksum verification failed: %v", err)
	}

	// 3. 备份当前版本
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %v", err)
	}

	backupPath := exePath + ".backup"
	if err := copyFile(exePath, backupPath); err != nil {
		log.Printf("Warning: failed to backup: %v", err)
	}

	// 4. 替换旧版本
	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("chmod failed: %v", err)
	}

	if err := os.Rename(tmpFile, exePath); err != nil {
		return fmt.Errorf("replace binary failed: %v", err)
	}

	log.Printf("Agent updated to version %s, restarting...", version)

	// 5. 重启（发送信号给自己）
	go func() {
		time.Sleep(1 * time.Second)
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	return nil
}

// uninstallAgent 远程卸载：先回传一次"已收到，即将卸载"的结果，再异步执行
// systemctl stop/disable + 删除安装目录，最后自行退出。
// 安装目录从当前可执行文件路径推导，而不是硬编码 /usr/local/artemis-agent，
// 避免与 install.sh 的约定脱节。
func (e *CommandExecutor) uninstallAgent(cmd *pb.Command) {
	log.Printf("Uninstalling agent (command_id=%s)...", cmd.CommandId)

	e.reportResult(cmd, true, "uninstall command received, agent will self-destruct shortly")

	go func() {
		time.Sleep(2 * time.Second)

		if err := exec.Command("systemctl", "stop", "artemis-agent").Run(); err != nil {
			log.Printf("Warning: systemctl stop failed: %v", err)
		}
		if err := exec.Command("systemctl", "disable", "artemis-agent").Run(); err != nil {
			log.Printf("Warning: systemctl disable failed: %v", err)
		}
		if err := os.Remove("/etc/systemd/system/artemis-agent.service"); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: remove service file failed: %v", err)
		}
		if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
			log.Printf("Warning: systemctl daemon-reload failed: %v", err)
		}

		exePath, err := os.Executable()
		if err != nil {
			log.Printf("Warning: get executable path failed: %v, cannot remove install dir", err)
		} else {
			installDir := filepath.Dir(exePath)
			log.Printf("Removing install directory: %s", installDir)
			if err := os.RemoveAll(installDir); err != nil {
				log.Printf("Warning: remove install dir failed: %v", err)
			}
		}

		log.Println("Agent uninstalled, exiting")
		os.Exit(0)
	}()
}

// updateWorker 更新 Worker 插件
func (e *CommandExecutor) updateWorker(workerName, version, downloadURL, checksum string) error {
	log.Printf("Updating worker %s to version %s from %s", workerName, version, downloadURL)

	// 查找 worker 配置
	var workerConfig *WorkerConfig
	for i := range e.master.config.Workers {
		if e.master.config.Workers[i].Name == workerName {
			workerConfig = &e.master.config.Workers[i]
			break
		}
	}

	if workerConfig == nil {
		return fmt.Errorf("worker config for %s not found", workerName)
	}

	// 1. 下载新版本插件
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.tmp", workerName, version))
	if err := downloadFile(downloadURL, tmpFile); err != nil {
		return fmt.Errorf("download failed: %v", err)
	}
	defer os.Remove(tmpFile)

	// 2. 校验 checksum
	if err := verifyChecksum(tmpFile, checksum); err != nil {
		return fmt.Errorf("checksum verification failed: %v", err)
	}

	// 3. 停止旧 worker
	if err := e.stopWorker(workerName); err != nil {
		log.Printf("Warning: failed to stop worker: %v", err)
	}

	// 4. 备份和替换插件文件
	backupPath := workerConfig.BinaryPath + ".backup"
	if err := copyFile(workerConfig.BinaryPath, backupPath); err != nil {
		log.Printf("Warning: failed to backup: %v", err)
	}

	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("chmod failed: %v", err)
	}

	if err := os.Rename(tmpFile, workerConfig.BinaryPath); err != nil {
		return fmt.Errorf("replace binary failed: %v", err)
	}

	// 5. 启动新 worker
	time.Sleep(1 * time.Second) // 等待文件系统同步
	return e.startWorker(workerName)
}

// setConfig 设置配置。支持两种方式：
//  1. payload["config_json"] 存在时，反序列化成完整 *Config 整体替换（manager 的
//     updateAgentConfig 走这条路径，因为 gRPC payload 是 map<string,string>，无法直接传嵌套对象）。
//  2. 否则按零散字段（server_addr/report_interval/data_buff_size）增量更新，兼容旧命令来源。
func (e *CommandExecutor) setConfig(payload map[string]string) error {
	log.Printf("Setting config: %+v", payload)

	if len(payload) == 0 {
		return fmt.Errorf("empty config payload")
	}

	if configJSON, ok := payload["config_json"]; ok {
		var newConfig Config
		if err := json.Unmarshal([]byte(configJSON), &newConfig); err != nil {
			return fmt.Errorf("invalid config_json: %v", err)
		}
		*e.master.config = newConfig
		log.Printf("Configuration replaced wholesale from config_json")

		if err := e.saveConfig(); err != nil {
			return fmt.Errorf("save config failed: %v", err)
		}
		log.Println("Configuration updated successfully")
		return nil
	}

	// 2. 更新内存中的配置
	for key, value := range payload {
		switch key {
		case "server_addr":
			e.master.config.ServerAddr = value
			log.Printf("Updated server_addr to: %s", value)
		case "report_interval":
			var intervalSec int
			if _, err := fmt.Sscanf(value, "%d", &intervalSec); err != nil {
				return fmt.Errorf("invalid report_interval: %v", err)
			}
			e.master.config.ReportInterval = intervalSec
			log.Printf("Updated report_interval to: %d seconds", intervalSec)
		case "data_buff_size":
			var size int
			if _, err := fmt.Sscanf(value, "%d", &size); err != nil {
				return fmt.Errorf("invalid data_buff_size: %v", err)
			}
			e.master.config.DataBuffSize = size
			log.Printf("Updated data_buff_size to: %d", size)
		default:
			log.Printf("Unknown config key: %s", key)
		}
	}

	// 3. 保存配置到文件
	if err := e.saveConfig(); err != nil {
		return fmt.Errorf("save config failed: %v", err)
	}

	log.Println("Configuration updated successfully")
	return nil
}

// saveConfig 保存配置到文件（使用启动时传入的真实配置路径，而不是硬编码相对路径）
func (e *CommandExecutor) saveConfig() error {
	configPath := e.master.configPath
	if configPath == "" {
		configPath = "config.json"
	}

	// 序列化配置
	data, err := json.MarshalIndent(e.master.config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %v", err)
	}

	log.Printf("Configuration saved to %s", configPath)
	return nil
}

// downloadFile 下载文件
func downloadFile(url, filepath string) error {
	log.Printf("Downloading from %s to %s", url, filepath)

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// 发送请求
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("http get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// 创建文件
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("create file: %v", err)
	}
	defer out.Close()

	// 下载
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("copy data: %v", err)
	}

	log.Printf("Downloaded %d bytes", written)
	return nil
}

// verifyChecksum 验证文件 checksum
func verifyChecksum(filepath, expectedChecksum string) error {
	log.Printf("Verifying checksum for %s", filepath)

	// 读取文件
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("read file: %v", err)
	}

	// 计算 SHA256
	hash := sha256.Sum256(data)
	actualChecksum := hex.EncodeToString(hash[:])

	// 比较
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			expectedChecksum, actualChecksum)
	}

	log.Printf("Checksum verified: %s", actualChecksum[:16]+"...")
	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
