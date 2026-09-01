package server

import (
	"encoding/json"
	"testing"
	"time"
)

// TestRuleJSONSchemaMatchesRuleEnginx 验证 Rule 的 JSON 字段名与
// rule-enginx_rust 的 src/rule/types.rs::Rule 完全一致，防止前端/manager
// 写入的文档被 rule-enginx_rust 读取时字段丢失或解析失败。
func TestRuleJSONSchemaMatchesRuleEnginx(t *testing.T) {
	rule := Rule{
		ID:          "audit_test01",
		Name:        "测试规则",
		Description: "用于 schema 校验",
		Severity:    "high",
		Enabled:     true,
		Type:        "pattern",
		Tags:        []string{"test"},
		Conditions: RuleConditions{
			EventType: "process_create",
			Fields:    map[string]string{},
			Patterns:  map[string]string{"cmdline": "curl.*\\|.*sh"},
		},
		Actions: []Action{
			{Type: "alert", Config: map[string]interface{}{"level": "high"}},
		},
		Author:    "tester",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// rule-enginx_rust Rule 的顶层字段名（見 rule-enginx_rust/src/rule/types.rs）
	wantTopLevel := []string{
		"id", "name", "description", "severity", "enabled", "type",
		"tags", "conditions", "actions", "author", "created_at", "updated_at",
	}
	for _, key := range wantTopLevel {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing expected top-level field %q in Rule JSON, got keys: %v", key, keysOf(raw))
		}
	}
	if _, present := raw["rule_type"]; present {
		t.Errorf("field must be named \"type\", not \"rule_type\"")
	}

	actions, ok := raw["actions"].([]interface{})
	if !ok || len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", raw["actions"])
	}
	action, ok := actions[0].(map[string]interface{})
	if !ok {
		t.Fatalf("action is not an object: %v", actions[0])
	}
	if _, present := action["type"]; !present {
		t.Errorf("Action must serialize its type field as \"type\", not \"action_type\"")
	}
	if _, present := action["action_type"]; present {
		t.Errorf("Action field must be named \"type\", not \"action_type\"")
	}
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
