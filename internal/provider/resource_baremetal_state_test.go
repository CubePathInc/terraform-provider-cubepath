package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// These tests run the real terraform binary against an httptest fake of the CubePath API.
// They are skipped when no terraform binary is available (TF_ACC_TERRAFORM_PATH or PATH).

func requireTerraform(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC_TERRAFORM_PATH") == "" {
		if _, err := exec.LookPath("terraform"); err != nil {
			t.Skip("no terraform binary: set TF_ACC_TERRAFORM_PATH")
		}
	}
	t.Setenv("TF_ACC", "1")
	t.Setenv("TF_ACC_TEMP_DIR", t.TempDir())
}

func testFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"cubepath": providerserver.NewProtocol6WithError(New("test")()),
	}
}

// fakeBaremetalAPI serves one baremetal server in project 7 with the given floating IPs.
type fakeBaremetalAPI struct {
	mu          sync.Mutex
	floatingIPs []any
	withOS      bool
	deployed    bool
	hostname    string
}

func (f *fakeBaremetalAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	write := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/projects/":
		bms := []any{}
		if f.deployed {
			bm := map[string]any{
				"id": 123, "hostname": f.hostname, "label": f.hostname, "status": "active", "user": "root",
				"location":          map[string]any{"id": 1, "location_name": "us-mia-1", "description": "Miami"},
				"baremetal_model":   map[string]any{"id": 1, "model_name": "c1.metal.plus", "cpu": "Xeon", "ram": 64, "storage_type": "NVMe", "disk_count": 2, "disk_size": "2 x 1TB"},
				"floating_ips":      f.floatingIPs,
				"monitoring_enable": false, "protected": false, "ssh_keys": []any{},
				"created_at": "2026-11-01T10:00:00Z",
			}
			if f.withOS {
				bm["os"] = map[string]any{"id": 1, "name": "debian-12", "version": "12"}
			}
			bms = append(bms, bm)
		}
		write([]any{map[string]any{
			"project": map[string]any{"id": 7, "name": "p", "description": "", "created_at": "2026-10-01T10:00:00Z"},
			"vps":     []any{}, "networks": []any{}, "baremetals": bms,
		}})
	case r.Method == http.MethodPost && r.URL.Path == "/baremetal/deploy/7":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.deployed = true
		f.hostname, _ = body["hostname"].(string)
		write(map[string]any{"detail": "ok", "task_id": "t1"})
	case strings.HasPrefix(r.URL.Path, "/baremetal/") && strings.HasSuffix(r.URL.Path, "/monitoring"):
		write(map[string]any{"detail": "ok"})
	default:
		w.WriteHeader(http.StatusNotFound)
		write(map[string]any{"detail": "Not Found: " + r.Method + " " + r.URL.Path})
	}
}

func baremetalConfig(baseURL string, withOS bool) string {
	osLine := ""
	if withOS {
		osLine = `os_name    = "debian-12"`
	}
	return fmt.Sprintf(`
provider "cubepath" {
  api_token   = "test-token"
  base_url    = %q
  max_retries = 0
}

resource "cubepath_baremetal" "web" {
  hostname   = "web1"
  project_id = 7
  location   = "us-mia-1"
  model_name = "c1.metal.plus"
  password   = "Str0ngPassw0rd1"
  %s
}
`, baseURL, osLine)
}

// Every computed attribute must be known after apply, also when the API has no IPv6 (or no
// IPv4, or no OS) for the server: Terraform rejects an unknown value left in state.
func TestBaremetalCreateWithoutOptionalAddresses(t *testing.T) {
	v4 := map[string]any{"id": 1, "address": "203.0.113.10", "type": "IPv4"}
	v6 := map[string]any{"id": 2, "address": "2001:db8::10", "type": "IPv6"}
	cases := []struct {
		name   string
		ips    []any
		withOS bool
		attrs  map[string]string
		absent []string
	}{
		{"no ipv6", []any{v4}, true, map[string]string{"main_ip": "203.0.113.10"}, []string{"ipv6"}},
		{"no ipv4", []any{v6}, true, map[string]string{"ipv6": "2001:db8::10"}, []string{"main_ip"}},
		{"no os", []any{v4, v6}, false, map[string]string{"main_ip": "203.0.113.10", "ipv6": "2001:db8::10"}, []string{"os_name"}},
		{"both", []any{v4, v6}, true, map[string]string{"main_ip": "203.0.113.10", "ipv6": "2001:db8::10", "os_name": "debian-12"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireTerraform(t)
			f := &fakeBaremetalAPI{floatingIPs: tc.ips, withOS: tc.withOS}
			srv := httptest.NewServer(f)
			defer srv.Close()
			var checks []resource.TestCheckFunc
			for k, v := range tc.attrs {
				checks = append(checks, resource.TestCheckResourceAttr("cubepath_baremetal.web", k, v))
			}
			for _, k := range tc.absent {
				checks = append(checks, resource.TestCheckNoResourceAttr("cubepath_baremetal.web", k))
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testFactories(),
				Steps: []resource.TestStep{{
					Config: baremetalConfig(srv.URL, tc.withOS),
					Check:  resource.ComposeAggregateTestCheckFunc(checks...),
				}},
			})
		})
	}
}
