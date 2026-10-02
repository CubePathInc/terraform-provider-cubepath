package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
