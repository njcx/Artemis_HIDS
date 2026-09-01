package protocol

import (
	"encoding/gob"
	"time"
)

func init() {
	gob.Register(RegisterRequest{})
	gob.Register(RegisterResponse{})
	gob.Register(Heartbeat{})
	gob.Register(Command{})
	gob.Register(Event{})
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	gob.Register([]map[string]interface{}{})
	gob.Register(map[string]string{})
	gob.Register([]string{})
}

// MessageType 消息类型
type MessageType uint8

const (
	MsgRegisterRequest MessageType = iota + 1
	MsgRegisterResponse
	MsgHeartbeat
	MsgCommand
	MsgEvent
)

// RegisterRequest 注册请求
type RegisterRequest struct {
	Name    string
	Pid     int
	Version string
	Token   string
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	Success           bool
	Message           string
	HeartbeatInterval time.Duration
}

// Heartbeat 心跳
type Heartbeat struct {
	WorkerName string
	Timestamp  int64
	Status     string
	CpuUsage   float64
	MemUsage   uint64
}

// Command 指令
type Command struct {
	ID      string
	Type    string
	Payload interface{} // 可以是 map[string]interface{} 或其他注册过的类型
}

// Event 事件上报
type Event struct {
	WorkerName string
	Type       string
	Timestamp  int64
	Data       interface{} // 任意数据
}
