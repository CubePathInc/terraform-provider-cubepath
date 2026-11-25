package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
)

// The defaults of the health check attributes are the ones of the API, so an origin created
// before they existed shows no diff.
func TestCDNOriginHealthCheckDefaults(t *testing.T) {
	var resp resource.SchemaResponse
	(&cdnOriginResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Schema.ValidateImplementation(context.Background()).HasError() {
		t.Fatal("invalid schema")
	}
	ctx := context.Background()
	for name, want := range map[string]string{
		"health_check_type": "tcp", "health_check_method": "GET", "health_check_expected_status": "2xx 3xx",
	} {
		a, ok := resp.Schema.Attributes[name].(schema.StringAttribute)
		if !ok || a.Default == nil {
			t.Fatalf("%s: no default", name)
		}
		var r defaults.StringResponse
		a.Default.DefaultString(ctx, defaults.StringRequest{}, &r)
		if r.PlanValue.ValueString() != want {
			t.Errorf("%s default %q, want %q", name, r.PlanValue.ValueString(), want)
		}
	}
	for name, want := range map[string]int64{"health_check_interval": 10, "health_check_threshold": 3} {
		a, ok := resp.Schema.Attributes[name].(schema.Int64Attribute)
		if !ok || a.Default == nil {
			t.Fatalf("%s: no default", name)
		}
		var r defaults.Int64Response
		a.Default.DefaultInt64(ctx, defaults.Int64Request{}, &r)
		if r.PlanValue.ValueInt64() != want {
			t.Errorf("%s default %d, want %d", name, r.PlanValue.ValueInt64(), want)
		}
	}
	for _, name := range []string{"health_check_type", "health_check_method", "health_check_expected_status", "health_check_interval", "health_check_threshold"} {
		if a := resp.Schema.Attributes[name]; a == nil || !a.IsOptional() || !a.IsComputed() {
			t.Errorf("%s must be optional and computed", name)
		}
	}
	for _, name := range []string{"health_detail", "health_changed_at"} {
		if a := resp.Schema.Attributes[name]; a == nil || a.IsOptional() || !a.IsComputed() {
			t.Errorf("%s must be computed only", name)
		}
	}
}

func TestCDNOriginMapToStateHealthFields(t *testing.T) {
	detail := "timeout"
	var st cdnOriginResourceModel
	(&cdnOriginResource{}).mapToState(&st, &client.CDNOrigin{
		UUID: "o1", HealthCheckType: "http", HealthCheckMethod: "GET", HealthCheckExpectedStatus: "2xx 3xx",
		HealthCheckInterval: 10, HealthCheckThreshold: 3, HealthDetail: &detail,
	})
	if st.HealthCheckType.ValueString() != "http" || st.HealthCheckExpect.ValueString() != "2xx 3xx" ||
		st.HealthCheckInterval.ValueInt64() != 10 || st.HealthCheckThreshold.ValueInt64() != 3 ||
		st.HealthDetail.ValueString() != "timeout" || st.HealthChangedAt.ValueString() != "" {
		t.Fatalf("state: %+v", st)
	}
}

func TestValidateOriginHealthCheck(t *testing.T) {
	ok := cdnOriginResourceModel{
		HealthCheckType: types.StringValue("http"), HealthCheckMethod: types.StringValue("HEAD"),
		HealthCheckInterval: types.Int64Value(5), HealthCheckThreshold: types.Int64Value(10),
		HealthCheckEnabled: types.BoolValue(true),
	}
	var d diag.Diagnostics
	validateOriginHealthCheck(&ok, &d)
	if d.HasError() {
		t.Fatalf("valid config refused: %v", d)
	}
	bad := map[string]func(m *cdnOriginResourceModel){
		"type":      func(m *cdnOriginResourceModel) { m.HealthCheckType = types.StringValue("icmp") },
		"method":    func(m *cdnOriginResourceModel) { m.HealthCheckMethod = types.StringValue("POST") },
		"interval":  func(m *cdnOriginResourceModel) { m.HealthCheckInterval = types.Int64Value(4) },
		"threshold": func(m *cdnOriginResourceModel) { m.HealthCheckThreshold = types.Int64Value(11) },
		"disabled":  func(m *cdnOriginResourceModel) { m.HealthCheckEnabled = types.BoolValue(false) },
	}
	for name, mut := range bad {
		m := ok
		mut(&m)
		var d diag.Diagnostics
		validateOriginHealthCheck(&m, &d)
		if !d.HasError() {
			t.Errorf("%s: not refused", name)
		}
	}
	// Unknown and null values are left alone.
	var d2 diag.Diagnostics
	validateOriginHealthCheck(&cdnOriginResourceModel{
		HealthCheckType: types.StringUnknown(), HealthCheckInterval: types.Int64Null(),
	}, &d2)
	if d2.HasError() {
		t.Fatal(d2)
	}
}
