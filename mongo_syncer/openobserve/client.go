// ============================================
// OpenObserve 查询客户端
// ============================================
package openobserve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client OpenObserve HTTP 查询客户端
type Client struct {
	baseURL    string
	org        string
	username   string
	Password   string
	httpClient *http.Client
}

// NewClient 创建 OpenObserve 查询客户端
func NewClient(baseURL, org, username, password string) *Client {
	return &Client{
		baseURL:    baseURL,
		org:        org,
		username:   username,
		Password:   password,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type searchRequest struct {
	Query searchQuery `json:"query"`
}

type searchQuery struct {
	SQL       string `json:"sql"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

type searchResponse struct {
	Hits    []map[string]interface{} `json:"hits"`
	Message string                   `json:"message"`
}

// Search 执行一次 SQL 查询，返回原始 hits
func (c *Client) Search(ctx context.Context, sql string, startUs, endUs int64) ([]map[string]interface{}, error) {
	reqBody := searchRequest{
		Query: searchQuery{SQL: sql, StartTime: startUs, EndTime: endUs},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化查询请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/api/%s/_search?type=logs", c.baseURL, c.org)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.SetBasicAuth(c.username, c.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 OpenObserve 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenObserve 返回错误状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var result searchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return result.Hits, nil
}

// ExtractJSONArray 反序列化 OpenObserve 里被字符串化的 JSON 数组字段
// （原 Kafka JSON 中的嵌套数组，摄入 OpenObserve 后变成字符串值）
func ExtractJSONArray(raw interface{}) []map[string]interface{} {
	str, ok := raw.(string)
	if !ok || str == "" {
		return nil
	}

	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(str), &arr); err != nil {
		return nil
	}
	return arr
}

// ToInt64 把 OpenObserve 统一返回的 float64 数字字段转为 int64
func ToInt64(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}

// ToFloat64 提取 float64 数字字段
func ToFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

// ToString 提取字符串字段，非字符串类型返回空字符串
func ToString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// ToBool 提取布尔字段
func ToBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}
