package client

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCDNOriginHealthFieldsRoundTrip(t *testing.T) {
	var o CDNOrigin
	body := `{"uuid":"o1","health_check_enabled":true,"health_check_path":"/ping","health_status":"unhealthy",
		"health_check_type":"http","health_check_method":"HEAD","health_check_expected_status":"200 301",
		"health_check_interval":30,"health_check_threshold":5,
		"health_detail":"connection refused","health_changed_at":"2026-11-25T10:00:00Z"}`
	if err := json.Unmarshal([]byte(body), &o); err != nil {
		t.Fatal(err)
	}
	if o.HealthCheckType != "http" || o.HealthCheckMethod != "HEAD" || o.HealthCheckExpectedStatus != "200 301" ||
		o.HealthCheckInterval != 30 || o.HealthCheckThreshold != 5 {
		t.Fatalf("check fields: %+v", o)
	}
	if o.HealthDetail == nil || *o.HealthDetail != "connection refused" || o.HealthChangedAt == nil || *o.HealthChangedAt != "2026-11-25T10:00:00Z" {
		t.Fatalf("read only fields: %+v", o)
	}

	var up CDNOrigin
	if err := json.Unmarshal([]byte(`{"uuid":"o2","health_detail":null,"health_changed_at":null}`), &up); err != nil {
		t.Fatal(err)
	}
	if up.HealthDetail != nil || up.HealthChangedAt != nil {
		t.Fatalf("null should stay nil: %+v", up)
	}
}

func TestCDNOriginRequestsOmitUnsetHealthFields(t *testing.T) {
	b, _ := json.Marshal(&CreateCDNOriginRequest{Name: "a"})
	if strings.Contains(string(b), "health_check_type") || strings.Contains(string(b), "health_check_interval") {
		t.Fatalf("create sent unset fields: %s", b)
	}
	b, _ = json.Marshal(&UpdateCDNOriginRequest{})
	if string(b) != "{}" {
		t.Fatalf("empty update: %s", b)
	}
	typ, n := "http", 20
	b, _ = json.Marshal(&UpdateCDNOriginRequest{HealthCheckType: &typ, HealthCheckInterval: &n})
	if string(b) != `{"health_check_type":"http","health_check_interval":20}` {
		t.Fatalf("update: %s", b)
	}
}
