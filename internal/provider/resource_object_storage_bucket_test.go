package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
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
