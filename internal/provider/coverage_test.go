package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestBuildConfigParamsConvertsTypesAndResetsRemoved(t *testing.T) {
	cfg := &client.ManagedDatabaseConfig{
		Engine: "mysql",
		Params: map[string]client.ManagedDatabaseConfigParam{
			"max_connections": {Type: "int", Default: json.RawMessage(`151`)},
			"long_query_time": {Type: "float", Default: json.RawMessage(`10.0`)},
			"slow_query_log":  {Type: "enum", Default: json.RawMessage(`"OFF"`)},
		},
	}
	params, err := buildConfigParams(cfg, map[string]string{"max_connections": "500", "long_query_time": "2.5"}, []string{"slow_query_log"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(params)
	var got map[string]interface{}
	_ = json.Unmarshal(raw, &got)
	want := map[string]interface{}{"max_connections": float64(500), "long_query_time": 2.5, "slow_query_log": "OFF"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if _, err := buildConfigParams(cfg, map[string]string{"max_connections": "many"}, nil); err == nil {
		t.Fatal("a non numeric int must fail")
	}
	_, err = buildConfigParams(cfg, map[string]string{"fsync": "off"}, nil)
	if err == nil || !strings.Contains(err.Error(), "fsync") || !strings.Contains(err.Error(), "max_connections") {
		t.Fatalf("unknown parameters must be named with the allowed ones: %v", err)
	}
}

func TestDiffInts(t *testing.T) {
	add, remove := diffInts([]int64{1, 2, 3}, []int64{3, 4})
	if !reflect.DeepEqual(add, []int64{4}) || !reflect.DeepEqual(remove, []int64{1, 2}) {
		t.Fatalf("add %v remove %v", add, remove)
	}
}

func TestNormalizeCIDR(t *testing.T) {
	cases := map[string]string{
		"192.0.2.7":     "192.0.2.7/32",
		"10.0.0.5/24":   "10.0.0.0/24",
		"2001:db8::1":   "2001:db8::1/128",
		"2001:db8::/32": "2001:db8::/32",
		"not-a-network": "not-a-network",
	}
	for in, want := range cases {
		if got := normalizeCIDR(in); got != want {
			t.Errorf("normalizeCIDR(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAlertActionsAreNotifyOnlyAndStable(t *testing.T) {
	actions := alertActions([]string{"b", "a"})
	if len(actions) != 2 || actions[0].NotificatorID != "a" || actions[1].Order != 1 {
		t.Fatalf("got %+v", actions)
	}
	for _, a := range actions {
		if a.ActionType != "notify" || !a.Enabled {
			t.Fatalf("got %+v", a)
		}
	}
}

// newState builds a state for a resource with the given attribute values; the rest is null.
func newState(t *testing.T, r resource.Resource, values map[string]tftypes.Value) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, typ := range objType.AttributeTypes {
		if v, ok := values[name]; ok {
			vals[name] = v
		} else {
			vals[name] = tftypes.NewValue(typ, nil)
		}
	}
	return tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, vals)}
}

func TestDDoSProfileRoundTrip(t *testing.T) {
	ctx := context.Background()
	values := map[string]tftypes.Value{
		"always_on_mitigation": tftypes.NewValue(tftypes.Bool, true),
		"symmetric_routing":    tftypes.NewValue(tftypes.Bool, false),
	}
	for _, f := range ddosProfileFields {
		values[f.name] = tftypes.NewValue(tftypes.Number, f.def)
	}
	values["udp_threshold_pps"] = tftypes.NewValue(tftypes.Number, 5000)
	state := newState(t, &ddosProfileResource{}, values)

	profile, diags := profileFromPlan(ctx, state.GetAttribute)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if profile.UDPThresholdPPS != 5000 || profile.TCPSynThresholdPPS != 10 || *profile.AlwaysOnMitigation != 1 || *profile.SymmetricRouting != 0 {
		t.Fatalf("got %+v", profile)
	}

	out := newState(t, &ddosProfileResource{}, nil)
	if d := setProfileState(ctx, &out, profile); d.HasError() {
		t.Fatal(d)
	}
	var udp types.Int64
	var alwaysOn types.Bool
	out.GetAttribute(ctx, path.Root("udp_threshold_pps"), &udp)
	out.GetAttribute(ctx, path.Root("always_on_mitigation"), &alwaysOn)
	if udp.ValueInt64() != 5000 || !alwaysOn.ValueBool() {
		t.Fatalf("udp %v always_on %v", udp, alwaysOn)
	}
}

type apiCall struct {
	Method, Path string
}

// fakeAPI answers every request with body and records the calls.
func fakeAPI(t *testing.T, body string) (*client.Client, func() []apiCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []apiCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, apiCall{r.Method, r.URL.Path})
		mu.Unlock()
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	return c, func() []apiCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]apiCall(nil), calls...)
	}
}

func TestVPSInPlaceChanges(t *testing.T) {
	ctx := context.Background()
	c, calls := fakeAPI(t, `{"detail":"ok","enabled":false,"schedule_hour":3,"retention_days":7,"max_backups":7}`)
	r := &vpsResource{client: c}

	keys := func(ids ...int64) types.Set {
		s, _ := types.SetValueFrom(ctx, types.Int64Type, ids)
		return s
	}
	prior := &vpsResourceModel{
		ProjectID: types.Int64Value(1), Protected: types.BoolValue(false), NetworkID: types.Int64Value(10),
		SSHKeyIDs: keys(1, 2), AvailabilityGroupUUID: types.StringValue("ag-old"), ISOID: types.StringValue(""),
		EnableBackups: types.BoolValue(false), BackupScheduleHour: types.Int64Value(3),
		BackupRetentionDays: types.Int64Value(7), BackupMaxBackups: types.Int64Value(7),
	}
	plan := *prior
	plan.ProjectID = types.Int64Value(2)
	plan.Protected = types.BoolValue(true)
	plan.NetworkID = types.Int64Value(20)
	plan.SSHKeyIDs = keys(2, 3)
	plan.AvailabilityGroupUUID = types.StringValue("")
	plan.BackupScheduleHour = types.Int64Value(5)
	plan.ISOID = types.StringValue("iso-1")

	var diags diag.Diagnostics
	r.applyInPlaceChanges(ctx, 7, &plan, prior, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}

	var got []string
	for _, c := range calls() {
		got = append(got, c.Method+" "+c.Path)
	}
	want := []string{
		"POST /vps/7/move-project",
		"POST /vps/7/protection",
		"DELETE /vps/7/network",
		"POST /vps/7/network",
		"DELETE /vps/7/ssh-keys/1",
		"POST /vps/7/ssh-keys",
		"DELETE /vps/availability-groups/ag-old/vps/7",
		"GET /vps/7/backup/settings",
		"PUT /vps/7/backup/settings",
		"POST /vps/7/iso",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestVPSCreateOnlyCallsWhatIsSet(t *testing.T) {
	ctx := context.Background()
	c, calls := fakeAPI(t, `{"detail":"ok"}`)
	r := &vpsResource{client: c}

	plan := &vpsResourceModel{
		Protected: types.BoolUnknown(), ISOID: types.StringUnknown(), EnableBackups: types.BoolUnknown(),
		BackupScheduleHour: types.Int64Unknown(), BackupRetentionDays: types.Int64Unknown(), BackupMaxBackups: types.Int64Unknown(),
	}
	var diags diag.Diagnostics
	r.applyInPlaceChanges(ctx, 7, plan, nil, &diags)
	if diags.HasError() || len(calls()) != 0 {
		t.Fatalf("a VPS without extra settings must not call the API: %v %v", calls(), diags)
	}
}

func TestManagedDatabaseStateMapping(t *testing.T) {
	label := ""
	host := "198.51.100.4"
	port := 6379
	var state managedDatabaseResourceModel
	state.Config = types.MapUnknown(types.StringType)
	mapManagedDatabaseToState(&state, &client.ManagedDatabase{
		UUID: "md1", ProjectID: 882, Name: "cache", Label: &label, Engine: "valkey", Version: "7.2.11",
		Topology: "replication", Replicas: 2, Status: "active", EndpointHost: &host, EndpointPort: &port,
		Plan:                &client.ManagedDatabasePlan{UUID: "p1", Name: "valkey.micro"},
		Location:            &client.ManagedDatabaseLocation{LocationName: "eu-bcn-1"},
		BackupRetentionDays: 7, BillingType: "hourly",
	})
	if !state.Label.IsNull() {
		t.Errorf("an empty label must be null, got %v", state.Label)
	}
	if !state.Config.IsNull() {
		t.Errorf("an unknown config must become null, got %v", state.Config)
	}
	if state.EndpointPort.ValueInt64() != 6379 || state.PlanName.ValueString() != "valkey.micro" || state.LocationName.ValueString() != "eu-bcn-1" {
		t.Errorf("got %+v", state)
	}
}

func TestAlertRuleMapping(t *testing.T) {
	ctx := context.Background()
	var state alertRuleModel
	var diags diag.Diagnostics
	mapAlertRule(ctx, &state, &client.AlertRule{
		ID: "r1", ProjectID: 1, Name: "cpu", TargetType: "vps", TargetID: "5", MetricType: "cpu", Operator: "gt",
		Threshold: 80, DurationSeconds: 300, CooldownSeconds: 600, Status: "triggered",
		Actions: []client.AlertRuleAction{{ActionType: "notify", NotificatorID: "n2"}, {ActionType: "notify", NotificatorID: "n1"}},
	}, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Enabled.ValueBool() {
		t.Error("a triggered rule is enabled")
	}
	var channels []string
	state.ChannelIDs.ElementsAs(ctx, &channels, false)
	sort.Strings(channels)
	if !reflect.DeepEqual(channels, []string{"n1", "n2"}) {
		t.Errorf("channels %v", channels)
	}
}

func TestLBBusyErrors(t *testing.T) {
	busy := &client.APIError{StatusCode: 400, Detail: "Cannot add target while Load Balancer is in 'updating' status"}
	if !lbBusy(busy) || !lbBusy(&client.APIError{StatusCode: 409}) {
		t.Fatal("updating and pending-task answers must be retried")
	}
	if lbBusy(&client.APIError{StatusCode: 400, Detail: "Target already exists in this listener"}) {
		t.Fatal("other validation errors must not be retried")
	}
}
