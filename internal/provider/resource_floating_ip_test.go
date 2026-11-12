package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
)

// upgradeFloatingIP runs the version 0 upgrader on a state whose assign_baremetal_id is v.
func upgradeFloatingIP(t *testing.T, v tftypes.Value) floatingIPResourceModel {
	t.Helper()
	ctx := context.Background()
	r := &floatingIPResource{}
	up, ok := r.UpgradeState(ctx)[0]
	if !ok || up.PriorSchema == nil {
		t.Fatal("no upgrader from version 0")
	}
	priorType := up.PriorSchema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, typ := range priorType.AttributeTypes {
		vals[name] = tftypes.NewValue(typ, nil)
	}
	vals["id"] = tftypes.NewValue(tftypes.String, "203.0.113.20")
	vals["address"] = tftypes.NewValue(tftypes.String, "203.0.113.20")
	vals["assign_baremetal_id"] = v
	prior := tfsdk.State{Schema: *up.PriorSchema, Raw: tftypes.NewValue(priorType, vals)}

	var current resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &current)
	if current.Schema.Version != 1 {
		t.Fatalf("schema version %d", current.Schema.Version)
	}
	resp := &resource.UpgradeStateResponse{State: tfsdk.State{Schema: current.Schema}}
	up.StateUpgrader(ctx, resource.UpgradeStateRequest{State: &prior}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var got floatingIPResourceModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	return got
}

// The API no longer knows server numbers, so the upgrade keeps the number as text (the next
// refresh replaces it with the uuid); 0 and null mean no server.
func TestFloatingIPUpgradeKeepsTheNumberAsText(t *testing.T) {
	cases := map[string]struct {
		in   tftypes.Value
		want types.String
	}{
		"number": {tftypes.NewValue(tftypes.Number, 123), types.StringValue("123")},
		"zero":   {tftypes.NewValue(tftypes.Number, 0), types.StringNull()},
		"null":   {tftypes.NewValue(tftypes.Number, nil), types.StringNull()},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := upgradeFloatingIP(t, c.in)
			if !got.AssignBaremetalID.Equal(c.want) || got.Address.ValueString() != "203.0.113.20" {
				t.Fatalf("assign_baremetal_id %v, address %v", got.AssignBaremetalID, got.Address)
			}
		})
	}
}

// Read replaces a managed baremetal assignment with the uuid the API reports, keeps the
// configured case of the same uuid, and leaves an unmanaged assignment alone.
func TestFloatingIPReadRefreshesTheBaremetalUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"single_ips":[{"id":9,"address":"203.0.113.20","type":"IPv4","status":"active","location_name":"us-mia-1","service_type":"baremetal","service_id":"` + fakeBMUUID + `"}],"subnets":[]}`))
	}))
	defer srv.Close()
	c, err := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	r := &floatingIPResource{client: c}
	ctx := context.Background()
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	for in, want := range map[string]types.String{
		"123":                                  types.StringValue(fakeBMUUID),
		"0F8E7D6C-5B4A-4392-8171-6F5E4D3C2B1A": types.StringValue("0F8E7D6C-5B4A-4392-8171-6F5E4D3C2B1A"),
		"":                                     types.StringNull(),
	} {
		m := floatingIPResourceModel{
			ID: types.StringValue("203.0.113.20"), Address: types.StringValue("203.0.113.20"),
			IPType: types.StringValue("IPv4"), LocationName: types.StringValue("us-mia-1"),
			ReverseDNS: types.StringValue(""), AssignVPSID: types.Int64Null(),
			AssignBaremetalID: types.StringValue(in), Status: types.StringValue("active"),
		}
		if in == "" {
			m.AssignBaremetalID = types.StringNull()
		}
		state := tfsdk.State{Schema: schemaResp.Schema}
		if d := state.Set(ctx, m); d.HasError() {
			t.Fatal(d)
		}
		resp := &resource.ReadResponse{State: state}
		r.Read(ctx, resource.ReadRequest{State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		var got floatingIPResourceModel
		resp.State.Get(ctx, &got)
		if !got.AssignBaremetalID.Equal(want) {
			t.Errorf("%q: assign_baremetal_id %v, want %v", in, got.AssignBaremetalID, want)
		}
	}
}
