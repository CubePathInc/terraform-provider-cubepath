package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func accessKeyModel(name, permission string, buckets []string, bypass bool) objectStorageAccessKeyResourceModel {
	list := types.ListNull(types.StringType)
	if buckets != nil {
		list, _ = types.ListValueFrom(context.Background(), types.StringType, buckets)
	}
	return objectStorageAccessKeyResourceModel{
		Name: types.StringValue(name), Permission: types.StringValue(permission),
		Buckets: list, BypassGovernance: types.BoolValue(bypass),
	}
}

func TestBuildAccessKeyUpdateSendsOnlyTheChanges(t *testing.T) {
	state := accessKeyModel("ci", "read_write", []string{"b1"}, false)
	cases := map[string]struct {
		plan objectStorageAccessKeyResourceModel
		want string
	}{
		"nothing":           {state, `{}`},
		"name":              {accessKeyModel("ci2", "read_write", []string{"b1"}, false), `{"name":"ci2"}`},
		"permission":        {accessKeyModel("ci", "read_only", []string{"b1"}, false), `{"permission":"read_only"}`},
		"bypass":            {accessKeyModel("ci", "read_write", []string{"b1"}, true), `{"bypass_governance":true}`},
		"bucket list":       {accessKeyModel("ci", "read_write", []string{"b1", "b2"}, false), `{"bucket_uuids":["b1","b2"]}`},
		"list to null":      {accessKeyModel("ci", "read_write", nil, false), `{"bucket_uuids":null}`},
		"name and all keys": {accessKeyModel("ci2", "read_write", nil, false), `{"name":"ci2","bucket_uuids":null}`},
	}
	unknown := accessKeyModel("ci", "read_write", nil, false)
	unknown.Buckets = types.ListUnknown(types.StringType)
	cases["unknown list is not sent"] = struct {
		plan objectStorageAccessKeyResourceModel
		want string
	}{unknown, `{}`}
	for label, tc := range cases {
		body, diags := buildAccessKeyUpdate(context.Background(), tc.plan, state)
		if diags.HasError() {
			t.Fatalf("%s: %v", label, diags)
		}
		raw, _ := json.Marshal(body)
		if string(raw) != tc.want {
			t.Errorf("%s: body %s, want %s", label, raw, tc.want)
		}
	}
}
