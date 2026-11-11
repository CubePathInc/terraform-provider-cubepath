package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const (
	fakeBMNumber = "123"
	fakeBMUUID   = "0f8e7d6c-5b4a-4392-8171-6f5e4d3c2b1a"
	fakeProject  = 7
	fakeNetwork  = 5
	fakeLB       = "11111111-1111-4111-8111-111111111111"
	fakeListener = "22222222-2222-4222-8222-222222222222"
	fakeChannel  = "33333333-3333-4333-8333-333333333333"
)

// fakeCubepathAPI is an in-memory CubePath API for the baremetal reference tests. With
// withUUID it answers like the post-cutover API (a uuid string added on baremetal objects),
// without it like the API of today. In both shapes the id of a baremetal stays the number
// and the echo fields (route next hop, BGP peer, load balancer target, alert target) give
// the number back.
type fakeCubepathAPI struct {
	mu       sync.Mutex
	withUUID bool
	// state per kind
	baremetal bool
	hostname  string
	routes    []map[string]any
	peers     []map[string]any
	targets   []map[string]any
	rules     map[string]map[string]any
	requests  []string
	counter   int
}

func newFakeCubepathAPI(t *testing.T, withUUID bool) (*fakeCubepathAPI, *httptest.Server) {
	t.Helper()
	f := &fakeCubepathAPI{withUUID: withUUID, rules: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

// Requests returns "METHOD path" of every request received so far.
func (f *fakeCubepathAPI) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *fakeCubepathAPI) baremetalJSON() map[string]any {
	bm := map[string]any{
		"id": 123, "hostname": f.hostname, "label": f.hostname, "status": "active", "user": "root",
		"os":                map[string]any{"id": 1, "name": "debian-12", "version": "12"},
		"location":          map[string]any{"id": 1, "location_name": "us-mia-1", "description": "Miami"},
		"baremetal_model":   map[string]any{"id": 1, "model_name": "c1.metal.plus", "cpu": "Xeon", "ram": 64, "storage_type": "NVMe", "disk_count": 2, "disk_size": "2 x 1TB"},
		"floating_ips":      []any{map[string]any{"id": 1, "address": "203.0.113.10", "type": "IPv4"}, map[string]any{"id": 2, "address": "2001:db8::10", "type": "IPv6"}},
		"monitoring_enable": false, "protected": false, "ssh_keys": []any{},
		"created_at": "2026-11-01T10:00:00Z",
	}
	if f.withUUID {
		bm["uuid"] = fakeBMUUID
	}
	return bm
}

func (f *fakeCubepathAPI) next(prefix string) string {
	f.counter++
	return fmt.Sprintf("%s-%d", prefix, f.counter)
}

func (f *fakeCubepathAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	var body map[string]any
	if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	write := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	p := r.URL.Path
	switch {
	case r.Method == http.MethodGet && p == "/projects/":
		bms := []any{}
		if f.baremetal {
			bms = append(bms, f.baremetalJSON())
		}
		write(200, []any{map[string]any{
			"project": map[string]any{"id": fakeProject, "name": "p", "description": "", "created_at": "2026-10-01T10:00:00Z"},
			"vps":     []any{}, "networks": []any{}, "baremetals": bms,
		}})
	case r.Method == http.MethodPost && p == fmt.Sprintf("/baremetal/deploy/%d", fakeProject):
		f.baremetal = true
		f.hostname, _ = body["hostname"].(string)
		write(200, map[string]any{"detail": "ok", "task_id": "t1"})
	case strings.HasPrefix(p, "/baremetal/") && strings.HasSuffix(p, "/monitoring"):
		write(200, map[string]any{"detail": "ok"})
	// routes
	case p == fmt.Sprintf("/networks/%d/routes", fakeNetwork) && r.Method == http.MethodGet:
		write(200, f.routes)
	case p == fmt.Sprintf("/networks/%d/routes", fakeNetwork) && r.Method == http.MethodPost:
		route := map[string]any{"id": f.next("route"), "network_id": fakeNetwork, "destination": body["destination"],
			"next_hop_type": body["next_hop_type"], "next_hop_target": body["next_hop_target"],
			"resolved_next_hop_ip": "10.0.0.2", "description": "", "created_at": "2026-11-01T10:00:00Z"}
		f.routes = append(f.routes, route)
		write(201, route)
	case strings.HasPrefix(p, fmt.Sprintf("/networks/%d/routes/", fakeNetwork)) && r.Method == http.MethodDelete:
		f.routes = nil
		write(200, map[string]any{"detail": "ok"})
	case strings.HasPrefix(p, fmt.Sprintf("/networks/%d/bgp-peers/", fakeNetwork)) && r.Method == http.MethodDelete:
		f.peers = nil
		write(200, map[string]any{"detail": "ok"})
	case strings.HasPrefix(p, "/loadbalancer/") && strings.Contains(p, "/targets/") && r.Method == http.MethodDelete:
		f.targets = nil
		write(200, map[string]any{"detail": "ok"})
	case strings.HasPrefix(p, "/triggers/") && r.Method == http.MethodDelete:
		delete(f.rules, strings.TrimPrefix(p, "/triggers/"))
		write(200, map[string]any{"detail": "ok"})
	// BGP peers
	case p == fmt.Sprintf("/networks/%d/bgp-peers", fakeNetwork) && r.Method == http.MethodGet:
		write(200, f.peers)
	case p == fmt.Sprintf("/networks/%d/bgp-peers", fakeNetwork) && r.Method == http.MethodPost:
		id := f.next("peer")
		f.peers = append(f.peers, map[string]any{"id": id, "network_id": fakeNetwork, "peer_type": body["peer_type"],
			"peer_target": body["peer_target"], "remote_asn": body["remote_asn"], "max_prefix": 100, "description": nil,
			"enabled": true, "resolved_peer_ip": "10.0.0.2", "last_state": "Established"})
		write(201, map[string]any{"peer_id": id})
	// load balancer
	case p == "/loadbalancer/" && r.Method == http.MethodGet:
		write(200, []any{map[string]any{
			"uuid": fakeLB, "name": "lb", "status": "active", "project_id": fakeProject,
			"listeners": []any{map[string]any{"uuid": fakeListener, "name": "http", "protocol": "tcp", "source_port": 80,
				"target_port": 80, "enabled": true, "targets": f.targets}},
		}})
	case p == fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets", fakeLB, fakeListener) && r.Method == http.MethodPost:
		t := map[string]any{"uuid": f.next("target"), "target_type": body["target_type"], "target_uuid": body["target_uuid"],
			"target_name": "web", "target_ip": "10.0.0.2", "port": 0, "weight": 100, "enabled": true, "health_status": "healthy"}
		f.targets = append(f.targets, t)
		write(201, t)
	// alert rules
	case p == "/triggers/" && r.Method == http.MethodPost:
		id := f.next("rule")
		rule := map[string]any{"id": id, "project_id": body["project_id"], "name": body["name"], "description": nil,
			"target_type": body["target_type"], "target_id": body["target_id"], "target_name": nil,
			"metric_type": body["metric_type"], "operator": body["operator"], "threshold": body["threshold"],
			"duration_seconds": body["duration_seconds"], "cooldown_seconds": body["cooldown_seconds"],
			"status": "enabled", "last_triggered_at": nil, "actions": body["actions"]}
		f.rules[id] = rule
		write(201, rule)
	case strings.HasPrefix(p, "/triggers/") && r.Method == http.MethodGet:
		if rule, ok := f.rules[strings.TrimPrefix(p, "/triggers/")]; ok {
			write(200, rule)
			return
		}
		write(404, map[string]any{"detail": "Not Found"})
	default:
		write(404, map[string]any{"detail": "Not Found: " + r.Method + " " + p})
	}
}

// seedOldState creates the objects that the state fixture (written by the provider before
// this change) refers to, as the API of either shape holds them.
func (f *fakeCubepathAPI) seedOldState() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.baremetal, f.hostname = true, "web1"
	f.routes = []map[string]any{{"id": "route-2", "network_id": fakeNetwork, "destination": "192.168.10.0/24",
		"next_hop_type": "baremetal", "next_hop_target": fakeBMNumber, "resolved_next_hop_ip": "10.0.0.2",
		"description": "", "created_at": "2026-11-01T10:00:00Z"}}
	f.peers = []map[string]any{{"id": "peer-1", "network_id": fakeNetwork, "peer_type": "baremetal",
		"peer_target": fakeBMNumber, "remote_asn": 65001, "max_prefix": 100, "description": nil, "enabled": true,
		"resolved_peer_ip": "10.0.0.2", "last_state": "Established"}}
	f.targets = []map[string]any{{"uuid": "target-4", "target_type": "baremetal", "target_uuid": fakeBMNumber,
		"target_name": "web", "target_ip": "10.0.0.2", "port": 0, "weight": 100, "enabled": true, "health_status": "healthy"}}
	f.rules["rule-3"] = map[string]any{"id": "rule-3", "project_id": fakeProject, "name": "bm-network", "description": nil,
		"target_type": "baremetal", "target_id": fakeBMNumber, "target_name": nil, "metric_type": "network_in",
		"operator": "gt", "threshold": 80, "duration_seconds": 300, "cooldown_seconds": 600, "status": "enabled",
		"last_triggered_at": nil, "actions": []any{map[string]any{"action_type": "notify", "notificator_id": fakeChannel, "order": 0, "enabled": true}}}
}
