package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagedDatabaseCreateAndScale(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/managed-databases/" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"detail":"ok","uuid":"md1","name":"db","engine":"valkey","version":"7.2.11","status":"provisioning"}`))
			return
		}
		_, _ = w.Write([]byte(`{"detail":"ok"}`))
	})
	ctx := context.Background()

	replicas := 2
	md, err := c.ManagedDatabases.Create(ctx, &CreateManagedDatabaseRequest{
		ProjectID: 1, Name: "db", Engine: "valkey", Version: "7.2.11", PlanUUID: "p1", Replicas: &replicas,
	})
	if err != nil || md.UUID != "md1" || md.Status != "provisioning" {
		t.Fatalf("got %+v, %v", md, err)
	}
	body := (*reqs)[0].Body
	if _, ok := body["topology"]; ok {
		t.Errorf("unset topology must be omitted: %v", body)
	}
	if _, ok := body["backup"]; ok {
		t.Errorf("unset backup must be omitted: %v", body)
	}

	plan := "p2"
	if err := c.ManagedDatabases.Scale(ctx, "md1", &ScaleManagedDatabaseRequest{PlanUUID: &plan}); err != nil {
		t.Fatal(err)
	}
	scale := (*reqs)[1]
	if scale.Path != "/managed-databases/md1/scale" || len(scale.Body) != 1 || scale.Body["plan_uuid"] != "p2" {
		t.Fatalf("scale must send exactly one field: %+v", scale)
	}

	if err := c.ManagedDatabases.SetProtection(ctx, "md1", true); err != nil {
		t.Fatal(err)
	}
	if p := (*reqs)[2]; p.Path != "/managed-databases/md1/protection" || p.Body["enabled"] != true {
		t.Fatalf("got %+v", p)
	}
}

func TestManagedDatabaseUserPasswordIsOptional(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"u1","username":"app","password":"generated-secret","status":"pending"}`))
	})
	user, err := c.ManagedDatabases.CreateUser(context.Background(), "md1", "app", "")
	if err != nil || user.Password != "generated-secret" {
		t.Fatalf("got %+v, %v", user, err)
	}
	if _, ok := (*reqs)[0].Body["password"]; ok {
		t.Fatalf("an empty password must let the API generate one: %v", (*reqs)[0].Body)
	}
}

func TestManagedDatabaseGetDatabaseNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"uuid":"d1","name":"app","status":"active"}]`))
	})
	_, err := c.ManagedDatabases.GetDatabase(context.Background(), "md1", "missing")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsNotFound() {
		t.Fatalf("expected a 404 APIError, got %v", err)
	}
}

func TestAlertPathsUseTrailingSlash(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	ctx := context.Background()
	if _, err := c.Alerts.ListChannels(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Alerts.ListRules(ctx, 882); err != nil {
		t.Fatal(err)
	}
	// Without the slash /triggers/notificators is routed to /triggers/{id} and answers 404.
	if p := (*reqs)[0].Path; p != "/triggers/notificators/" {
		t.Errorf("channels path %q", p)
	}
	if r := (*reqs)[1]; r.Path != "/triggers/" || r.Query != "project_id=882" {
		t.Errorf("rules request %+v", r)
	}
}

func TestDDoSFirewallRuleIDIsLookedUp(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"detail":"Rule created successfully"}`))
			return
		}
		_, _ = w.Write([]byte(`{"rules":[{"id":7,"network":"203.0.113.5","protocol":17,"dst_port":53,"action":0},
			{"id":9,"network":"203.0.113.5","protocol":6,"dst_port":443,"action":50,"action_label":"TLS Validation (TCP)"}],"total":2}`))
	})
	rule, err := c.DDoS.CreateFirewallRule(context.Background(), &CreateDDoSFirewallRuleRequest{
		Network: "203.0.113.5", Protocol: 6, DstPort: 443, Action: 50,
	})
	if err != nil || rule.ID != 9 {
		t.Fatalf("got %+v, %v", rule, err)
	}
	if r := (*reqs)[1]; r.Method != http.MethodGet || r.Path != "/ddos-mitigation/firewall-rules/203.0.113.5" {
		t.Fatalf("lookup request %+v", r)
	}
}

func TestDDoSPrefixListCreateFindsItsUUID(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"detail":"Prefix list created successfully"}`))
			return
		}
		_, _ = w.Write([]byte(`{"prefix_lists":[{"uuid":"g1","name":"scanners","is_global":true},
			{"uuid":"o1","name":"scanners","is_global":false}],"total":2}`))
	})
	list, err := c.DDoS.CreatePrefixList(context.Background(), "scanners", "")
	if err != nil || list.UUID != "o1" {
		t.Fatalf("must pick the organization's list, not the global one: %+v, %v", list, err)
	}
}

func TestDDoSProfileAssignmentsSendEmptyLists(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"detail":"ok"}`))
	})
	ctx := context.Background()
	if err := c.DDoS.SetProfileCountries(ctx, "203.0.113.5", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.DDoS.SetProfileASNs(ctx, "203.0.113.5", nil); err != nil {
		t.Fatal(err)
	}
	if v, ok := (*reqs)[0].Body["iso_codes"].([]interface{}); !ok || len(v) != 0 {
		t.Errorf("clearing countries must send an empty list: %v", (*reqs)[0].Body)
	}
	if v, ok := (*reqs)[1].Body["asns"].([]interface{}); !ok || len(v) != 0 {
		t.Errorf("clearing ASNs must send an empty list: %v", (*reqs)[1].Body)
	}
}

func TestVPSSSHKeysBodyIsARawArray(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		raw = string(body)
		_, _ = w.Write([]byte(`{"detail":"ok"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient("token", srv.URL, WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.VPS.AddSSHKeys(context.Background(), 5, []int{12, 47}); err != nil {
		t.Fatal(err)
	}
	if raw != "[12,47]" {
		t.Fatalf("body %q, want a raw JSON array", raw)
	}
}

func TestMoveEndpoints(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"detail":"ok"}`))
	})
	ctx := context.Background()
	calls := []func() error{
		func() error { return c.VPS.MoveProject(ctx, 1, 9) },
		func() error { return c.Baremetal.MoveProject(ctx, 2, 9) },
		func() error { return c.Networks.MoveProject(ctx, 3, 9) },
		func() error { return c.LoadBalancer.MoveProject(ctx, "lb", 9) },
		func() error { return c.NATGateway.MoveProject(ctx, "ng", 9) },
		func() error { return c.Kubernetes.MoveProject(ctx, "k8s", 9) },
		func() error { return c.DNS.MoveZone(ctx, "z", 9) },
		func() error { return c.CDN.MoveZone(ctx, "c", 9) },
		func() error { return c.AvailabilityGroups.MoveProject(ctx, "ag", 9) },
	}
	for _, call := range calls {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"/vps/1/move-project", "/baremetal/2/move-project", "/networks/3/move-project",
		"/loadbalancer/lb/move-project", "/nat-gateway/ng/move-to-project", "/kubernetes/k8s/move",
		"/dns/zones/z/move-project", "/cdn/zones/c/move-project", "/vps/availability-groups/ag/move-project",
	}
	for i, path := range want {
		r := (*reqs)[i]
		if r.Method != http.MethodPost || r.Path != path || r.Body["project_id"] != float64(9) {
			t.Errorf("call %d: got %s %s %v, want POST %s", i, r.Method, r.Path, r.Body, path)
		}
	}
}

func TestLBHealthCheckDecoding(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"uuid":"lb1","listeners":[
			{"uuid":"l1","health_check":null},
			{"uuid":"l2","health_check":{"protocol":"tcp","path":"/","port":8080,"interval_seconds":10,"timeout_seconds":2,
			"healthy_threshold":2,"unhealthy_threshold":3,"expected_codes":"200-399","http_method":"GET"}}]}]`))
	})
	ctx := context.Background()
	hc, err := c.LoadBalancer.GetHealthCheck(ctx, "lb1", "l1")
	if err != nil || hc != nil {
		t.Fatalf("no health check: got %+v, %v", hc, err)
	}
	hc, err = c.LoadBalancer.GetHealthCheck(ctx, "lb1", "l2")
	if err != nil || hc == nil || hc.Protocol != "tcp" || hc.Port == nil || *hc.Port != 8080 {
		t.Fatalf("got %+v, %v", hc, err)
	}
	_, err = c.LoadBalancer.GetHealthCheck(ctx, "lb1", "missing")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsNotFound() {
		t.Fatalf("expected a 404 APIError, got %v", err)
	}
}
