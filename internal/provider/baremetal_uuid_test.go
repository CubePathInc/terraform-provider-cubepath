package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// These tests run the real terraform binary against an httptest fake of the CubePath API.
// They are skipped when no terraform binary is available (TF_ACC_TERRAFORM_PATH or PATH).
//
// The state in testdata/state_before_baremetal_uuid.json was written by the provider at
// commit 3aa5312 (v0.11.0 plus the User-Agent commit), when ids and echo fields held the
// server number, and is copied as it is. testdata/state_floating_ip_number.json is written by
// hand in the shape of the floating IP schema version 0 (assign_baremetal_id a number).

const (
	oldStateFixture        = "testdata/state_before_baremetal_uuid.json"
	oldFloatingIPFixture   = "testdata/state_floating_ip_number.json"
	errBaremetalIDIsNumber = `Baremetal ID is a server number`
)

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

func workDir(t *testing.T) string {
	t.Helper()
	dirs, _ := filepath.Glob(filepath.Join(os.Getenv("TF_ACC_TEMP_DIR"), "plugintest*", "work*"))
	if len(dirs) != 1 {
		t.Fatalf("working directories: %v", dirs)
	}
	return dirs[0]
}

// writeState puts a state written by an older provider version in the working directory of
// the test, before the first step runs.
func writeState(t *testing.T, fixture string) func() {
	return func() {
		raw, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workDir(t), "terraform.tfstate"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// stateRm does what 'terraform state rm <type>.<name>' does to the state file.
func stateRm(t *testing.T, typ, name string) func() {
	return func() {
		file := filepath.Join(workDir(t), "terraform.tfstate")
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var st map[string]any
		if err := json.Unmarshal(raw, &st); err != nil {
			t.Fatal(err)
		}
		var kept []any
		for _, r := range st["resources"].([]any) {
			m := r.(map[string]any)
			if m["type"] == typ && m["name"] == name {
				continue
			}
			kept = append(kept, r)
		}
		st["resources"] = kept
		st["serial"] = st["serial"].(float64) + 1
		if raw, err = json.MarshalIndent(st, "", "  "); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func checkAttr(resourceName, key, want string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttr(resourceName, key, want)
}

// EXIT CRITERION: a state written when the server number was the id. The baremetal cannot
// be read with the number and says how to move on (state rm and import by uuid). After that,
// the four echo fields are refreshed to the uuid the API returns, so nothing is replaced;
// the only change is the in-place update that every import of a baremetal shows (the
// password, which import cannot read, goes into the state; the server is not touched). No
// request carries the number.
func TestOldStateMovesToTheUUID(t *testing.T) {
	requireTerraform(t)
	fake, srv := newFakeCubepathAPI(t)
	fake.seedOldState()
	cfg := bmRefConfig(srv.URL)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testFactories(),
		Steps: []resource.TestStep{
			// The plan without refresh stops at the echo fields, which get the number from the
			// baremetal state; with refresh, at the baremetal itself. Both say what to do.
			{PreConfig: writeState(t, oldStateFixture), Config: cfg, PlanOnly: true,
				ExpectError: regexp.MustCompile(`(?s)terraform state rm.*terraform import`)},
			{PreConfig: stateRm(t, "cubepath_baremetal", "web"), Config: cfg, ResourceName: "cubepath_baremetal.web",
				ImportState: true, ImportStateId: fakeBMUUID, ImportStatePersist: true},
			{Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("cubepath_baremetal.web", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("cubepath_network_route.r", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("cubepath_network_bgp_peer.p", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("cubepath_lb_target.t", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("cubepath_alert_rule.a", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					checkAttr("cubepath_baremetal.web", "id", fakeBMUUID),
					checkAttr("cubepath_baremetal.web", "uuid", fakeBMUUID),
					checkAttr("cubepath_network_route.r", "next_hop_target", fakeBMUUID),
					checkAttr("cubepath_network_bgp_peer.p", "peer_target", fakeBMUUID),
					checkAttr("cubepath_lb_target.t", "target_id", fakeBMUUID),
					checkAttr("cubepath_alert_rule.a", "target_id", fakeBMUUID),
				)},
			{Config: cfg, PlanOnly: true},
		},
	})
	checkNoServerNumber(t, fake)
}

func TestBaremetalImportByUUIDOnly(t *testing.T) {
	requireTerraform(t)
	fake, srv := newFakeCubepathAPI(t)
	fake.seedOldState()
	cfg := bmRefConfig(srv.URL)

	importCheck := func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("%d imported instances", len(states))
		}
		a := states[0].Attributes
		if a["id"] != fakeBMUUID || states[0].ID != fakeBMUUID || a["uuid"] != fakeBMUUID || a["hostname"] != "web1" {
			return fmt.Errorf("id %q (%q) uuid %q hostname %q", a["id"], states[0].ID, a["uuid"], a["hostname"])
		}
		return nil
	}
	step := func(id string) resource.TestStep {
		return resource.TestStep{Config: cfg, ResourceName: "cubepath_baremetal.web", ImportState: true, ImportStateId: id}
	}
	byUUID, upper := step(fakeBMUUID), step("0F8E7D6C-5B4A-4392-8171-6F5E4D3C2B1A")
	byUUID.ImportStateCheck, upper.ImportStateCheck = importCheck, importCheck
	byNumber, bad := step(fakeBMNumber), step("not-a-server")
	byNumber.ExpectError = regexp.MustCompile(`(?s)Invalid import ID.*must be a\s+uuid`)
	bad.ExpectError = byNumber.ExpectError
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testFactories(),
		Steps:                    []resource.TestStep{byUUID, upper, byNumber, bad},
	})
	checkNoServerNumber(t, fake)
}

// A server created through the provider has its uuid in id and uuid, and the echo fields
// carry it.
func TestBaremetalCreateSetsTheUUID(t *testing.T) {
	requireTerraform(t)
	fake, srv := newFakeCubepathAPI(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testFactories(),
		Steps: []resource.TestStep{{
			Config: bmRefConfig(srv.URL),
			Check: resource.ComposeTestCheckFunc(
				checkAttr("cubepath_baremetal.web", "id", fakeBMUUID),
				checkAttr("cubepath_baremetal.web", "uuid", fakeBMUUID),
				checkAttr("cubepath_network_route.r", "next_hop_target", fakeBMUUID),
				checkAttr("cubepath_network_bgp_peer.p", "peer_target", fakeBMUUID),
				checkAttr("cubepath_lb_target.t", "target_id", fakeBMUUID),
				checkAttr("cubepath_alert_rule.a", "target_id", fakeBMUUID)),
		}},
	})
	checkNoServerNumber(t, fake)
}

// A floating IP state of schema version 0 keeps the server number in assign_baremetal_id.
// The upgrade keeps it as text and the refresh replaces it with the uuid the API reports for
// the address, so a configuration that names the uuid has no changes: the address is not
// moved.
func TestFloatingIPNumberStateUpgradesWithoutChanges(t *testing.T) {
	requireTerraform(t)
	fake, srv := newFakeCubepathAPI(t)
	fake.seedOldState()
	cfg := floatingIPConfig(srv.URL, fakeBMUUID)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testFactories(),
		Steps: []resource.TestStep{
			{PreConfig: writeState(t, oldFloatingIPFixture), Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("cubepath_floating_ip.f", plancheck.ResourceActionNoop),
				}},
				Check: checkAttr("cubepath_floating_ip.f", "assign_baremetal_id", fakeBMUUID)},
			{Config: cfg, PlanOnly: true},
		},
	})
	for _, req := range fake.Requests() {
		if regexp.MustCompile(`/floating_ips/assign/`).MatchString(req) {
			t.Errorf("the plan must not move the address: %q", req)
		}
	}
	checkNoServerNumber(t, fake)
}

// The fields that name a baremetal server take its uuid: a number is rejected before any
// request.
func TestBaremetalFieldsRejectANumber(t *testing.T) {
	requireTerraform(t)
	provider := `provider "cubepath" {
  api_token = "t"
  base_url  = "http://127.0.0.1:1"
}
`
	reject := regexp.MustCompile(`(?s)Use the baremetal server uuid.*no longer accepts server numbers`)
	cases := map[string]string{
		"route": `resource "cubepath_network_route" "r" {
  network_id = 5
  destination = "10.1.0.0/24"
  next_hop_type = "baremetal"
  next_hop_target = "` + fakeBMNumber + `"
}`,
		"bgp peer": `resource "cubepath_network_bgp_peer" "p" {
  network_id = 5
  peer_type = "baremetal"
  peer_target = "` + fakeBMNumber + `"
  remote_asn = 65001
}`,
		"lb target": `resource "cubepath_lb_target" "t" {
  loadbalancer_id = "` + fakeLB + `"
  listener_id = "` + fakeListener + `"
  target_type = "baremetal"
  target_id = "` + fakeBMNumber + `"
}`,
		"alert rule": `resource "cubepath_alert_rule" "a" {
  project_id = 7
  name = "x"
  target_type = "baremetal"
  target_id = "` + fakeBMNumber + `"
  metric_type = "network_in"
  operator = "gt"
  threshold = 1
  channel_ids = ["` + fakeChannel + `"]
}`,
		"floating ip": `resource "cubepath_floating_ip" "f" {
  ip_type = "IPv4"
  location_name = "us-mia-1"
  assign_baremetal_id = "` + fakeBMNumber + `"
}`,
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TF_ACC_TEMP_DIR", t.TempDir())
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testFactories(),
				Steps:                    []resource.TestStep{{Config: provider + cfg, PlanOnly: true, ExpectError: reject}},
			})
		})
	}

	// A number is still the right value for a VPS target.
	t.Run("number stays valid for a vps", func(t *testing.T) {
		t.Setenv("TF_ACC_TEMP_DIR", t.TempDir())
		cfg := provider + `resource "cubepath_lb_target" "t" {
  loadbalancer_id = "` + fakeLB + `"
  listener_id = "` + fakeListener + `"
  target_type = "vps"
  target_id = "` + fakeBMNumber + `"
}`
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testFactories(),
			Steps:                    []resource.TestStep{{Config: cfg, PlanOnly: true, ExpectNonEmptyPlan: true}},
		})
	})
}
