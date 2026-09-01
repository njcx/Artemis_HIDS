package transport

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"sync"

	"artemis_agent/pkg/gomw/protocol"
)

// Transport 封装 Socket 连接，提供基于消息的读写
type Transport struct {
	conn net.Conn
	mu   sync.Mutex // 保护写操作
}

// NewTransport 创建 Transport
func NewTransport(conn net.Conn) *Transport {
	return &Transport{
		conn: conn,
	}
}

// Close 关闭连接
func (t *Transport) Close() error {
	return t.conn.Close()
}

// WriteMessage 发送消息
// 格式: [Length(4 bytes)] [Type(1 byte)] [Payload(Gob encoded)]
func (t *Transport) WriteMessage(msgType protocol.MessageType, payload interface{}) error {
	// 1. 编码 Payload
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("gob encode error: %v", err)
	}
	payloadBytes := buf.Bytes()

	// 2. 计算长度 (Type 1 byte + Payload bytes)
	length := uint32(1 + len(payloadBytes))

	// 3. 构建数据包
	// 4 bytes length + 1 byte type + payload
	packet := make([]byte, 4+length)
	binary.BigEndian.PutUint32(packet[0:4], length)
	packet[4] = byte(msgType)
	copy(packet[5:], payloadBytes)

	// 4. 发送 (加锁防止并发写导致数据错乱)
	t.mu.Lock()
	defer t.mu.Unlock()

	// Set write deadline if needed
	// t.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	_, err := t.conn.Write(packet)
	return err
}

// ReadMessage 读取消息
func (t *Transport) ReadMessage() (protocol.MessageType, interface{}, error) {
	// 1. 读取长度头 (4 bytes)
	header := make([]byte, 4)
	if _, err := io.ReadFull(t.conn, header); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(header)

	// 2. 读取 Body (Type + Payload)
	body := make([]byte, length)
	if _, err := io.ReadFull(t.conn, body); err != nil {
		return 0, nil, err
	}

	msgType := protocol.MessageType(body[0])
	payloadBytes := body[1:]

	// 3. 解码 Payload
	var payload interface{}
	buf := bytes.NewReader(payloadBytes)
	dec := gob.NewDecoder(buf)

	// 根据消息类型创建对应的结构体指针
	switch msgType {
	case protocol.MsgRegisterRequest:
		payload = &protocol.RegisterRequest{}
	case protocol.MsgRegisterResponse:
		payload = &protocol.RegisterResponse{}
	case protocol.MsgHeartbeat:
		payload = &protocol.Heartbeat{}
	case protocol.MsgCommand:
		payload = &protocol.Command{}
	case protocol.MsgEvent:
		payload = &protocol.Event{}
	default:
		return 0, nil, fmt.Errorf("unknown message type: %d", msgType)
	}

	if err := dec.Decode(payload); err != nil {
		return 0, nil, fmt.Errorf("gob decode error: %v", err)
	}

	return msgType, payload, nil
}
