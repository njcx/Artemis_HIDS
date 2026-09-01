package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestRuleCRUDAgainstLiveMongo 直接调用 rules.go 里的 handler（跳过登录/验证码），
// 对接一个真实运行中的 MongoDB 实例，验证规则的创建/查询/更新/启停/删除全链路，
// 并确认落库文档字段名与 rule-enginx_rust 的 Rule schema 完全一致。
//
// 通过环境变量 VERIFY_MONGO_URI 指定测试用 MongoDB 地址时才会运行，默认跳过，
// 不影响正常的 `go test ./...`。
func TestRuleCRUDAgainstLiveMongo(t *testing.T) {
	uri := os.Getenv("VERIFY_MONGO_URI")
	if uri == "" {
		t.Skip("VERIFY_MONGO_URI 未设置，跳过真实 Mongo 集成验证")
	}

	db, err := InitDatabase(&DatabaseConfig{
		Host:     os.Getenv("VERIFY_MONGO_HOST"),
		Port:     27117,
		Database: "artemis_hids_live_verify",
	})
	if err != nil {
		t.Fatalf("连接 MongoDB 失败: %v", err)
	}
	defer db.Collection("rules").Drop(context.Background())

	if err := CreateIndexes(db); err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}

	gin.SetMode(gin.TestMode)
	s := &APIServer{db: db, router: gin.New()}
	s.router.GET("/rules", s.listRules)
	s.router.GET("/rules/:id", s.getRule)
	s.router.POST("/rules", s.createRule)
	s.router.PUT("/rules/:id", s.updateRule)
	s.router.DELETE("/rules/:id", s.deleteRule)

	ruleJSON := `{
		"id": "audit_live_verify01",
		"name": "实时验证规则",
		"description": "端到端验证 manager -> Mongo -> rule-enginx_rust",
		"severity": "high",
		"enabled": true,
		"type": "pattern",
		"tags": ["verify"],
		"conditions": {
			"event_type": "process_create",
			"fields": {},
			"patterns": {"cmdline": "curl.*\\|.*sh"}
		},
		"actions": [{"type": "alert", "config": {"level": "high"}}],
		"author": "live-test"
	}`

	// 1. 创建
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(ruleJSON))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("创建规则失败: status=%d body=%s", w.Code, w.Body.String())
	}

	// 2. 直接查 Mongo 原始文档，确认字段名与 rule-enginx_rust 期望的一致
	var raw bson.M
	if err := db.Collection("rules").FindOne(context.Background(), bson.M{"id": "audit_live_verify01"}).Decode(&raw); err != nil {
		t.Fatalf("从 Mongo 读取规则文档失败: %v", err)
	}
	for _, key := range []string{"id", "name", "severity", "enabled", "type", "conditions", "actions", "author", "created_at", "updated_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("Mongo 文档缺少字段 %q，实际字段: %v", key, raw)
		}
	}
	if _, ok := raw["rule_type"]; ok {
		t.Errorf("Mongo 文档不应存在 rule_type 字段（应为 type）")
	}
	if createdAt, ok := raw["created_at"].(primitive.DateTime); !ok {
		t.Errorf("created_at 应为 BSON 原生日期类型（primitive.DateTime），实际类型: %T", raw["created_at"])
	} else if createdAt.Time().IsZero() {
		t.Errorf("created_at 不应为零值")
	}

	// 3. 列表查询能查到
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/rules?type=pattern", nil)
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "audit_live_verify01") {
		t.Fatalf("列表查询未包含新建规则: status=%d body=%s", w.Code, w.Body.String())
	}

	// 4. 更新（模拟前端启停开关）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/rules/audit_live_verify01", strings.NewReader(`{"enabled": false, "name": "实时验证规则", "type": "pattern", "severity": "high"}`))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("更新规则失败: status=%d body=%s", w.Code, w.Body.String())
	}
	var afterUpdate bson.M
	if err := db.Collection("rules").FindOne(context.Background(), bson.M{"id": "audit_live_verify01"}).Decode(&afterUpdate); err != nil {
		t.Fatalf("更新后重新读取失败: %v", err)
	}
	if enabled, _ := afterUpdate["enabled"].(bool); enabled {
		t.Errorf("更新后 enabled 应为 false，实际为 %v", afterUpdate["enabled"])
	}

	// 5. 删除
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/rules/audit_live_verify01", nil)
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("删除规则失败: status=%d body=%s", w.Code, w.Body.String())
	}

	count, err := db.Collection("rules").CountDocuments(context.Background(), bson.M{"id": "audit_live_verify01"})
	if err != nil {
		t.Fatalf("确认删除结果失败: %v", err)
	}
	if count != 0 {
		t.Errorf("删除后规则文档应不存在，实际还有 %d 条", count)
	}
}
