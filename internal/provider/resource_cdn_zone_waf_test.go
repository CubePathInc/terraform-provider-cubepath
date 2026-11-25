package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
)

func TestManagedWAFRoundTrip(t *testing.T) {
	ctx := context.Background()
	obj, diags := managedWAFFromAPI(ctx, &client.CDNManagedWAF{Mode: "log", Level: 2, ExcludeRuleIDs: []int64{942100, 941100}, ExcludeCategories: []string{"sqli"}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	req, diags := managedWAFRequest(ctx, obj)
	if diags.HasError() || req == nil {
		t.Fatal(diags)
	}
	if req.Mode != "log" || req.Level != 2 || len(req.ExcludeRuleIDs) != 2 || req.ExcludeRuleIDs[0] != 941100 || req.ExcludeCategories[0] != "sqli" {
		t.Fatalf("got %+v", req)
	}
}

func TestManagedWAFNullWhenTheAPIHasNone(t *testing.T) {
	obj, diags := managedWAFFromAPI(context.Background(), nil)
	if diags.HasError() || !obj.IsNull() {
		t.Fatalf("got %v %v", obj, diags)
	}
	if req, _ := managedWAFRequest(context.Background(), obj); req != nil {
		t.Fatalf("a null attribute must send nothing, got %+v", req)
	}
}

func TestManagedWAFRequestRefusesWhatTheAPIRefuses(t *testing.T) {
	ctx := context.Background()
	bad := []*client.CDNManagedWAF{
		{Mode: "block", Level: 1},
		{Mode: "log", Level: 3},
		{Mode: "log", Level: 1, ExcludeRuleIDs: []int64{0}},
		{Mode: "log", Level: 1, ExcludeCategories: []string{"SQLi"}},
	}
	for _, w := range bad {
		obj, _ := managedWAFFromAPI(ctx, w)
		if _, diags := managedWAFRequest(ctx, obj); !diags.HasError() {
			t.Fatalf("accepted %+v", w)
		}
	}
}

func TestManagedWAFPartialConfigTakesTheDefaults(t *testing.T) {
	ctx := context.Background()
	obj, _ := types.ObjectValue(managedWAFAttrTypes, map[string]attr.Value{
		"mode":               types.StringValue("log"),
		"level":              types.Int64Null(),
		"exclude_rule_ids":   types.SetNull(types.Int64Type),
		"exclude_categories": types.SetNull(types.StringType),
	})
	req, diags := managedWAFRequest(ctx, obj)
	if diags.HasError() || req.Level != 1 || req.Mode != "log" || req.ExcludeRuleIDs == nil || req.ExcludeCategories == nil {
		t.Fatalf("got %+v %v", req, diags)
	}
}
