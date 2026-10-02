package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
)

func strp(s string) *string { return &s }

func TestValidateBucketTags(t *testing.T) {
	ok := []map[string]*string{
		nil,
		{"env": strp("prod")},
		{"team": strp("")},
		{"path/to:x": strp("a=b+c@d.e-f_g")},
		{"Größe": strp("日本 語")},
		{"unknown": nil},
		{strings.Repeat("k", 128): strp(strings.Repeat("v", 256))},
		// 64 surrogate pairs = 128 UTF-16 code units.
		{strings.Repeat("\U0001D400", 64): strp("")},
	}
	for _, tags := range ok {
		if errs := validateBucketTags(tags); len(errs) != 0 {
			t.Errorf("%v: unexpected errors %v", tags, errs)
		}
	}

	many := map[string]*string{}
	for i := 0; i < 51; i++ {
		many[fmt.Sprintf("k%d", i)] = strp("")
	}
	bad := map[string]map[string]*string{
		"at most 50":       many,
		"cannot be empty":  {"": strp("x")},
		"longer than 128":  {strings.Repeat("k", 129): strp("")},
		"surrogates":       {strings.Repeat("\U0001D400", 65): strp("")},
		"longer than 256":  {"k": strp(strings.Repeat("v", 257))},
		"cannot contain =": {"a=b": strp("")},
		"start or end":     {" env": strp("")},
		"reserved prefix":  {"AWS:createdBy": strp("")},
		"cp prefix":        {"cp:x": strp("")},
		"cubepath prefix":  {"CubePath:x": strp("")},
		"key charset":      {"env!": strp("")},
		"value charset":    {"env": strp("a,b")},
	}
	for name, tags := range bad {
		if errs := validateBucketTags(tags); len(errs) == 0 {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestBucketValidateConfigRejectsBadTags(t *testing.T) {
	ctx := context.Background()
	r := &objectStorageBucketResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

	config := func(tags map[string]tftypes.Value) tfsdk.Config {
		vals := map[string]tftypes.Value{}
		for name, typ := range objType.AttributeTypes {
			vals[name] = tftypes.NewValue(typ, nil)
		}
		vals["name"] = tftypes.NewValue(tftypes.String, "photos")
		vals["tier"] = tftypes.NewValue(tftypes.String, "ia")
		vals["tags"] = tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, tags)
		return tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, vals)}
	}

	resp := &resource.ValidateConfigResponse{}
	r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: config(map[string]tftypes.Value{
		"cp:owner": tftypes.NewValue(tftypes.String, "me"),
	})}, resp)
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "reserved prefix") {
		t.Fatalf("diagnostics %v", resp.Diagnostics)
	}

	// An unknown value (from another resource) is only checked for its key.
	resp = &resource.ValidateConfigResponse{}
	r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: config(map[string]tftypes.Value{
		"env":   tftypes.NewValue(tftypes.String, "prod"),
		"owner": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics %v", resp.Diagnostics)
	}
}

// bucketValues is a full resource object with every attribute null except the given ones.
func bucketValues(ctx context.Context, schemaResp resource.SchemaResponse, set map[string]tftypes.Value) tftypes.Value {
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, typ := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(typ, nil)
	}
	for name, v := range set {
		vals[name] = v
	}
	return tftypes.NewValue(objType, vals)
}

func tagsValue(tags map[string]string) tftypes.Value {
	elems := map[string]tftypes.Value{}
	for k, v := range tags {
		elems[k] = tftypes.NewValue(tftypes.String, v)
	}
	return tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, elems)
}

// Tags set from the dashboard must survive a plan whose configuration has no tags: the
// attribute has no default, so the plan keeps the bucket's tags instead of {}.
func TestBucketPlanWithoutTagsKeepsTheBucketTags(t *testing.T) {
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}
	r := &objectStorageBucketResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx)

	common := map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, "b-1"),
		"name": tftypes.NewValue(tftypes.String, "photos"),
		"tier": tftypes.NewValue(tftypes.String, "infrequent_access"),
	}
	with := func(extra map[string]tftypes.Value) map[string]tftypes.Value {
		m := map[string]tftypes.Value{}
		for k, v := range common {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	dyn := func(v tftypes.Value) *tfprotov6.DynamicValue {
		d, err := tfprotov6.NewDynamicValue(objType, v)
		if err != nil {
			t.Fatal(err)
		}
		return &d
	}
	dashboardTags := tagsValue(map[string]string{"env": "prod"})
	prior := bucketValues(ctx, schemaResp, with(map[string]tftypes.Value{
		"protected":  tftypes.NewValue(tftypes.Bool, false),
		"versioning": tftypes.NewValue(tftypes.String, "off"),
		"tags":       dashboardTags,
	}))

	plan := func(config map[string]tftypes.Value) tftypes.Value {
		cfg := bucketValues(ctx, schemaResp, with(config))
		// Terraform proposes the prior value for an optional computed attribute left out of the
		// configuration, and the configured value otherwise.
		proposed := with(config)
		if _, ok := config["tags"]; !ok {
			proposed["tags"] = dashboardTags
		}
		if _, ok := config["versioning"]; !ok {
			proposed["versioning"] = tftypes.NewValue(tftypes.String, "off")
		}
		resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
			TypeName:         "cubepath_object_storage_bucket",
			PriorState:       dyn(prior),
			Config:           dyn(cfg),
			ProposedNewState: dyn(bucketValues(ctx, schemaResp, proposed)),
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range resp.Diagnostics {
			t.Fatalf("%s: %s", d.Summary, d.Detail)
		}
		planned, err := resp.PlannedState.Unmarshal(objType)
		if err != nil {
			t.Fatal(err)
		}
		var attrs map[string]tftypes.Value
		if err := planned.As(&attrs); err != nil {
			t.Fatal(err)
		}
		return attrs["tags"]
	}

	// No tags in the configuration, another attribute changes: the tags stay as they are.
	if got := plan(map[string]tftypes.Value{"protected": tftypes.NewValue(tftypes.Bool, true)}); !got.Equal(dashboardTags) {
		t.Errorf("tags left out: planned %v, want %v", got, dashboardTags)
	}
	// Declared tags are managed as a whole, {} included.
	declared := tagsValue(map[string]string{"team": "data"})
	if got := plan(map[string]tftypes.Value{"tags": declared}); !got.Equal(declared) {
		t.Errorf("declared tags: planned %v, want %v", got, declared)
	}
	empty := tagsValue(map[string]string{})
	if got := plan(map[string]tftypes.Value{"tags": empty}); !got.Equal(empty) {
		t.Errorf("tags = {}: planned %v, want {}", got)
	}
}

// Update sends tags only when the plan changes them.
func TestBucketUpdateSendsTagsOnlyWhenTheyChange(t *testing.T) {
	ctx := context.Background()
	var patches []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			patches = append(patches, body)
			_, _ = w.Write([]byte(`{"detail":"Bucket updated"}`))
		default:
			_, _ = w.Write([]byte(`{"uuid":"b-1","name":"photos","status":"active","versioning":"off",` +
				`"protected":true,"tier":{"slug":"infrequent_access"},"tags":{"env":"prod"}}`))
		}
	}))
	defer srv.Close()
	c, err := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	r := &objectStorageBucketResource{client: c}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	base := map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "b-1"),
		"name":       tftypes.NewValue(tftypes.String, "photos"),
		"tier":       tftypes.NewValue(tftypes.String, "infrequent_access"),
		"versioning": tftypes.NewValue(tftypes.String, "off"),
		"protected":  tftypes.NewValue(tftypes.Bool, false),
		"tags":       tagsValue(map[string]string{"env": "prod"}),
	}
	update := func(planTags tftypes.Value) {
		planVals := map[string]tftypes.Value{}
		for k, v := range base {
			planVals[k] = v
		}
		planVals["protected"] = tftypes.NewValue(tftypes.Bool, true)
		planVals["tags"] = planTags
		state := tfsdk.State{Schema: schemaResp.Schema, Raw: bucketValues(ctx, schemaResp, base)}
		plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: bucketValues(ctx, schemaResp, planVals)}
		resp := &resource.UpdateResponse{State: state}
		r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("update: %v", resp.Diagnostics)
		}
	}

	update(tagsValue(map[string]string{"env": "prod"}))
	update(tagsValue(map[string]string{}))
	if len(patches) != 2 {
		t.Fatalf("patches %v", patches)
	}
	if _, ok := patches[0]["tags"]; ok {
		t.Errorf("unchanged tags were sent: %v", patches[0])
	}
	if tags, ok := patches[1]["tags"].(map[string]any); !ok || len(tags) != 0 {
		t.Errorf("tags = {} should send an empty map: %v", patches[1])
	}
}

func retentionValue(mode string, days, years interface{}) tftypes.Value {
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"mode": tftypes.String, "days": tftypes.Number, "years": tftypes.Number,
	}}
	return tftypes.NewValue(typ, map[string]tftypes.Value{
		"mode":  tftypes.NewValue(tftypes.String, mode),
		"days":  tftypes.NewValue(tftypes.Number, days),
		"years": tftypes.NewValue(tftypes.Number, years),
	})
}

func TestBucketValidateConfigObjectLock(t *testing.T) {
	ctx := context.Background()
	r := &objectStorageBucketResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	validate := func(set map[string]tftypes.Value) []string {
		vals := map[string]tftypes.Value{
			"name": tftypes.NewValue(tftypes.String, "backups"),
			"tier": tftypes.NewValue(tftypes.String, "ia"),
		}
		for k, v := range set {
			vals[k] = v
		}
		resp := &resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{
			Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: bucketValues(ctx, schemaResp, vals)},
		}, resp)
		var out []string
		for _, d := range resp.Diagnostics.Errors() {
			out = append(out, d.Detail())
		}
		return out
	}
	on := tftypes.NewValue(tftypes.Bool, true)
	enabled := tftypes.NewValue(tftypes.String, "enabled")

	if errs := validate(map[string]tftypes.Value{
		"object_lock_enabled": on, "versioning": enabled, "accept_object_lock_terms": on,
		"object_lock_default_retention": retentionValue("governance", 30, nil),
	}); len(errs) != 0 {
		t.Fatalf("valid config: %v", errs)
	}
	cases := map[string]map[string]tftypes.Value{
		"versioning": {"object_lock_enabled": on, "accept_object_lock_terms": on},
		"suspended": {"object_lock_enabled": on, "accept_object_lock_terms": on,
			"versioning": tftypes.NewValue(tftypes.String, "suspended")},
		"accept_object_lock_terms": {"object_lock_enabled": on, "versioning": enabled},
		"only be set when": {"versioning": enabled,
			"object_lock_default_retention": retentionValue("governance", 30, nil)},
		"not both": {"object_lock_enabled": on, "versioning": enabled, "accept_object_lock_terms": on,
			"object_lock_default_retention": retentionValue("governance", 30, 1)},
		"either days": {"object_lock_enabled": on, "versioning": enabled, "accept_object_lock_terms": on,
			"object_lock_default_retention": retentionValue("compliance", nil, nil)},
		"positive": {"object_lock_enabled": on, "versioning": enabled, "accept_object_lock_terms": on,
			"object_lock_default_retention": retentionValue("governance", 0, nil)},
		"governance or compliance": {"object_lock_enabled": on, "versioning": enabled, "accept_object_lock_terms": on,
			"object_lock_default_retention": retentionValue("legal", nil, 1)},
	}
	for want, set := range cases {
		errs := validate(set)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, " | "), want) {
			t.Errorf("%s: got %v", want, errs)
		}
	}
}

// A bucket with Object Lock is protected unless protected is set in the configuration.
func TestBucketPlanProtectsObjectLockBuckets(t *testing.T) {
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}
	r := &objectStorageBucketResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx)
	dyn := func(v tftypes.Value) *tfprotov6.DynamicValue {
		d, err := tfprotov6.NewDynamicValue(objType, v)
		if err != nil {
			t.Fatal(err)
		}
		return &d
	}
	planProtected := func(config map[string]tftypes.Value) tftypes.Value {
		cfg := map[string]tftypes.Value{
			"name":                     tftypes.NewValue(tftypes.String, "backups"),
			"tier":                     tftypes.NewValue(tftypes.String, "ia"),
			"versioning":               tftypes.NewValue(tftypes.String, "enabled"),
			"object_lock_enabled":      tftypes.NewValue(tftypes.Bool, true),
			"accept_object_lock_terms": tftypes.NewValue(tftypes.Bool, true),
		}
		for k, v := range config {
			cfg[k] = v
		}
		proposed := map[string]tftypes.Value{}
		for k, v := range cfg {
			proposed[k] = v
		}
		// Computed attributes are unknown in a new resource's proposed state.
		for _, k := range []string{"id", "project_id", "status", "region", "endpoint", "location_name", "tags", "locked_content_kept"} {
			proposed[k] = tftypes.NewValue(objType.(tftypes.Object).AttributeTypes[k], tftypes.UnknownValue)
		}
		resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
			TypeName:         "cubepath_object_storage_bucket",
			PriorState:       dyn(tftypes.NewValue(objType, nil)),
			Config:           dyn(bucketValues(ctx, schemaResp, cfg)),
			ProposedNewState: dyn(bucketValues(ctx, schemaResp, proposed)),
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range resp.Diagnostics {
			t.Fatalf("%s: %s", d.Summary, d.Detail)
		}
		planned, err := resp.PlannedState.Unmarshal(objType)
		if err != nil {
			t.Fatal(err)
		}
		var attrs map[string]tftypes.Value
		if err := planned.As(&attrs); err != nil {
			t.Fatal(err)
		}
		return attrs["protected"]
	}
	if got := planProtected(nil); !got.Equal(tftypes.NewValue(tftypes.Bool, true)) {
		t.Errorf("protected not configured: planned %v, want true", got)
	}
	off := tftypes.NewValue(tftypes.Bool, false)
	if got := planProtected(map[string]tftypes.Value{"protected": off}); !got.Equal(off) {
		t.Errorf("protected = false: planned %v, want false", got)
	}
}

// The default retention changes in place through PUT .../object-lock, and only when it changes.
func TestBucketUpdateSetsTheDefaultRetention(t *testing.T) {
	ctx := context.Background()
	var puts []map[string]any
	current := `{"mode":"governance","days":30,"years":null}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			if r.URL.Path != "/object-storage/buckets/b-1/object-lock" {
				t.Errorf("path %s", r.URL.Path)
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			puts = append(puts, body)
			current = "null"
			if rule, ok := body["default_retention"].(map[string]any); ok {
				b, _ := json.Marshal(rule)
				current = string(b)
			}
			_, _ = w.Write([]byte(`{"detail":"Bucket updated"}`))
		case http.MethodPatch:
			t.Errorf("unexpected PATCH")
		default:
			_, _ = w.Write([]byte(`{"uuid":"b-1","name":"backups","status":"active","versioning":"enabled",` +
				`"protected":true,"tier":{"slug":"infrequent_access"},"tags":{},` +
				`"object_lock":{"enabled":true,"default_retention":` + current + `}}`))
		}
	}))
	defer srv.Close()
	c, err := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	r := &objectStorageBucketResource{client: c}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	base := map[string]tftypes.Value{
		"id":                            tftypes.NewValue(tftypes.String, "b-1"),
		"name":                          tftypes.NewValue(tftypes.String, "backups"),
		"tier":                          tftypes.NewValue(tftypes.String, "infrequent_access"),
		"versioning":                    tftypes.NewValue(tftypes.String, "enabled"),
		"protected":                     tftypes.NewValue(tftypes.Bool, true),
		"tags":                          tagsValue(map[string]string{}),
		"object_lock_enabled":           tftypes.NewValue(tftypes.Bool, true),
		"accept_object_lock_terms":      tftypes.NewValue(tftypes.Bool, true),
		"object_lock_default_retention": retentionValue("governance", 30, nil),
	}
	update := func(rule tftypes.Value) tfsdk.State {
		planVals := map[string]tftypes.Value{}
		for k, v := range base {
			planVals[k] = v
		}
		planVals["object_lock_default_retention"] = rule
		state := tfsdk.State{Schema: schemaResp.Schema, Raw: bucketValues(ctx, schemaResp, base)}
		plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: bucketValues(ctx, schemaResp, planVals)}
		resp := &resource.UpdateResponse{State: state}
		r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("update: %v", resp.Diagnostics)
		}
		return resp.State
	}

	update(retentionValue("governance", 30, nil))
	if len(puts) != 0 {
		t.Fatalf("unchanged rule sent: %v", puts)
	}
	st := update(retentionValue("compliance", nil, 1))
	if len(puts) != 1 || puts[0]["accept_object_lock_terms"] != true {
		t.Fatalf("puts %v", puts)
	}
	if rule := puts[0]["default_retention"].(map[string]any); rule["mode"] != "compliance" || rule["years"] != float64(1) {
		t.Fatalf("rule %v", rule)
	}
	var years types.Int64
	st.GetAttribute(ctx, path.Root("object_lock_default_retention").AtName("years"), &years)
	if years.ValueInt64() != 1 {
		t.Fatalf("state years %v", years)
	}
	var accept types.Bool
	st.GetAttribute(ctx, path.Root("accept_object_lock_terms"), &accept)
	if !accept.ValueBool() {
		t.Fatalf("accept_object_lock_terms must keep the configured value")
	}

	update(tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"mode": tftypes.String, "days": tftypes.Number, "years": tftypes.Number,
	}}, nil))
	if v, ok := puts[1]["default_retention"]; len(puts) != 2 || !ok || v != nil {
		t.Fatalf("removing the rule must send null: %v", puts)
	}
}

func TestAccessKeyBypassGovernanceNeedsReadWrite(t *testing.T) {
	ctx := context.Background()
	r := &objectStorageAccessKeyResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	validate := func(permission interface{}) bool {
		raw := bucketValues(ctx, resource.SchemaResponse{Schema: schemaResp.Schema}, map[string]tftypes.Value{
			"name":              tftypes.NewValue(tftypes.String, "veeam"),
			"tier":              tftypes.NewValue(tftypes.String, "ia"),
			"permission":        tftypes.NewValue(tftypes.String, permission),
			"bypass_governance": tftypes.NewValue(tftypes.Bool, true),
		})
		resp := &resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw}}, resp)
		return resp.Diagnostics.HasError()
	}
	if validate(nil) || validate("read_write") {
		t.Error("bypass_governance must be accepted on read_write keys (the default)")
	}
	if !validate("read_only") {
		t.Error("bypass_governance on a read_only key must fail")
	}
}
