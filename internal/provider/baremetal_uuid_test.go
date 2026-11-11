package provider

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// These tests run the real terraform binary against an httptest fake of the CubePath API.
// They are skipped when no terraform binary is available (TF_ACC_TERRAFORM_PATH or PATH).
//
// The state in testdata/state_before_baremetal_uuid.json was written by the provider at
// commit 3aa5312 (the main branch before this change, v0.11.0 plus the User-Agent commit),
// by applying bmRefConfig against the fake in the shape of today's API, and copied as it is.

const oldStateFixture = "testdata/state_before_baremetal_uuid.json"

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

// writeOldState puts the state of the previous provider version in the working directory
// of the test, before the first step runs.
func writeOldState(t *testing.T) func() {
	return func() {
		raw, err := os.ReadFile(oldStateFixture)
		if err != nil {
			t.Fatal(err)
		}
		dirs, _ := filepath.Glob(filepath.Join(os.Getenv("TF_ACC_TEMP_DIR"), "plugintest*", "work*"))
		if len(dirs) != 1 {
			t.Fatalf("working directories: %v", dirs)
		}
		if err := os.WriteFile(filepath.Join(dirs[0], "terraform.tfstate"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func checkAttr(resourceName, key, want string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttr(resourceName, key, want)
}

// EXIT CRITERION: the plan of the state written by the previous provider shows no changes
// for the five resources, against the API of today and the API after the cutover.
func TestOldStateHasNoChangesWithNewProvider(t *testing.T) {
	requireTerraform(t)
	for _, shape := range []struct {
		name     string
		withUUID bool
	}{{"api without uuid", false}, {"api with uuid", true}} {
		t.Run(shape.name, func(t *testing.T) {
			t.Setenv("TF_ACC_TEMP_DIR", t.TempDir())
			fake, srv := newFakeCubepathAPI(t, shape.withUUID)
			fake.seedOldState()
			cfg := bmRefConfig(srv.URL)

			wantUUID := resource.TestCheckNoResourceAttr("cubepath_baremetal.web", "uuid")
			if shape.withUUID {
				wantUUID = checkAttr("cubepath_baremetal.web", "uuid", fakeBMUUID)
			}
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testFactories(),
				Steps: []resource.TestStep{
					{PreConfig: writeOldState(t), Config: cfg, PlanOnly: true},
					// Refresh writes the new state: id stays the number, uuid is filled when the API sends it.
					{RefreshState: true, Check: resource.ComposeTestCheckFunc(
						checkAttr("cubepath_baremetal.web", "id", fakeBMNumber), wantUUID,
						checkAttr("cubepath_network_route.r", "next_hop_target", fakeBMNumber),
						checkAttr("cubepath_network_bgp_peer.p", "peer_target", fakeBMNumber),
						checkAttr("cubepath_lb_target.t", "target_id", fakeBMNumber),
						checkAttr("cubepath_alert_rule.a", "target_id", fakeBMNumber),
					)},
					{Config: cfg, PlanOnly: true},
				},
			})
			for _, req := range fake.Requests() {
				if regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-`).MatchString(req) && !regexp.MustCompile(`^(GET|DELETE) /(loadbalancer|networks)/`).MatchString(req) {
					t.Errorf("unexpected uuid in request %q", req)
				}
			}
		})
	}
}

func TestBaremetalImportByNumberAndByUUID(t *testing.T) {
	requireTerraform(t)
	for _, shape := range []struct {
		name     string
		withUUID bool
	}{{"api without uuid", false}, {"api with uuid", true}} {
		t.Run(shape.name, func(t *testing.T) {
			t.Setenv("TF_ACC_TEMP_DIR", t.TempDir())
			fake, srv := newFakeCubepathAPI(t, shape.withUUID)
			fake.seedOldState()
			cfg := bmRefConfig(srv.URL)

			importCheck := func(wantUUID string) resource.ImportStateCheckFunc {
				return func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("%d imported instances", len(states))
					}
					a := states[0].Attributes
					if a["id"] != fakeBMNumber || states[0].ID != fakeBMNumber {
						return fmt.Errorf("id after import is %q (%q), want the number", a["id"], states[0].ID)
					}
					if a["uuid"] != wantUUID || a["hostname"] != "web1" {
						return fmt.Errorf("uuid %q hostname %q", a["uuid"], a["hostname"])
					}
					return nil
				}
			}
			wantUUID := ""
			if shape.withUUID {
				wantUUID = fakeBMUUID
			}
			byUUID := resource.TestStep{
				Config: cfg, ResourceName: "cubepath_baremetal.web", ImportState: true,
				ImportStateId: fakeBMUUID, ImportStateCheck: importCheck(wantUUID),
			}
			if !shape.withUUID {
				// The API of today has no uuid: the server cannot be found, and nothing is invented.
				byUUID.ImportStateCheck = nil
				byUUID.ExpectError = regexp.MustCompile(`(?i)non-existent remote object`)
			}
			upper := byUUID
			upper.ImportStateId = "0F8E7D6C-5B4A-4392-8171-6F5E4D3C2B1A"
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testFactories(),
				Steps: []resource.TestStep{
					{PreConfig: writeOldState(t), Config: cfg, PlanOnly: true},
					{Config: cfg, ResourceName: "cubepath_baremetal.web", ImportState: true,
						ImportStateId: fakeBMNumber, ImportStateCheck: importCheck(wantUUID)},
					byUUID, upper,
					{Config: cfg, ResourceName: "cubepath_baremetal.web", ImportState: true,
						ImportStateId: "not-a-server", ExpectError: regexp.MustCompile(`Invalid import ID`)},
				},
			})
		})
	}
}

// A server created through the provider keeps the number in id and gets the uuid when the API sends it.
func TestBaremetalCreateSetsNumberAndUUID(t *testing.T) {
	requireTerraform(t)
	for _, withUUID := range []bool{false, true} {
		t.Run(fmt.Sprintf("uuid=%v", withUUID), func(t *testing.T) {
			t.Setenv("TF_ACC_TEMP_DIR", t.TempDir())
			_, srv := newFakeCubepathAPI(t, withUUID)
			check := resource.TestCheckNoResourceAttr("cubepath_baremetal.web", "uuid")
			if withUUID {
				check = checkAttr("cubepath_baremetal.web", "uuid", fakeBMUUID)
			}
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testFactories(),
				Steps: []resource.TestStep{{
					Config: bmRefConfig(srv.URL),
					Check: resource.ComposeTestCheckFunc(checkAttr("cubepath_baremetal.web", "id", fakeBMNumber), check,
						checkAttr("cubepath_network_route.r", "next_hop_target", fakeBMNumber)),
				}},
			})
		})
	}
}

// The four echo fields take the server number: a uuid is rejected before any request.
func TestEchoFieldsRejectABaremetalUUID(t *testing.T) {
	requireTerraform(t)
	provider := `provider "cubepath" {
  api_token = "t"
  base_url  = "http://127.0.0.1:1"
}
`
	reject := regexp.MustCompile(`(?s)Use the server number, not its uuid.*server number`)
	cases := map[string]string{
		"route": `resource "cubepath_network_route" "r" {
  network_id = 5
  destination = "10.1.0.0/24"
  next_hop_type = "baremetal"
  next_hop_target = "` + fakeBMUUID + `"
}`,
		"bgp peer": `resource "cubepath_network_bgp_peer" "p" {
  network_id = 5
  peer_type = "baremetal"
  peer_target = "` + fakeBMUUID + `"
  remote_asn = 65001
}`,
		"lb target": `resource "cubepath_lb_target" "t" {
  loadbalancer_id = "` + fakeLB + `"
  listener_id = "` + fakeListener + `"
  target_type = "baremetal"
  target_id = "` + fakeBMUUID + `"
}`,
		"alert rule": `resource "cubepath_alert_rule" "a" {
  project_id = 7
  name = "x"
  target_type = "baremetal"
  target_id = "` + fakeBMUUID + `"
  metric_type = "network_in"
  operator = "gt"
  threshold = 1
  channel_ids = ["` + fakeChannel + `"]
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

	// A uuid is the right value for the other target types.
	t.Run("uuid stays valid for an availability group", func(t *testing.T) {
		t.Setenv("TF_ACC_TEMP_DIR", t.TempDir())
		cfg := provider + `resource "cubepath_lb_target" "t" {
  loadbalancer_id = "` + fakeLB + `"
  listener_id = "` + fakeListener + `"
  target_type = "availability_group"
  target_id = "` + fakeBMUUID + `"
}`
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testFactories(),
			Steps:                    []resource.TestStep{{Config: cfg, PlanOnly: true, ExpectNonEmptyPlan: true}},
		})
	})
}
