package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLifecycleRulesRoundTrip(t *testing.T) {
	ctx := context.Background()
	days, gt, keep := int64(30), int64(1024), int64(3)
	prefix := "logs/"
	t1 := true
	apiRules := []client.ObjectStorageLifecycleRule{
		{
			ID: "logs", Enabled: true,
			Filter: &client.ObjectStorageLifecycleFilter{
				Prefix: &prefix, ObjectSizeGreaterThan: &gt,
				Tags: []client.ObjectStorageLifecycleTag{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}},
			},
			Expiration:                  &client.ObjectStorageLifecycleExpiration{Days: &days},
			NoncurrentVersionExpiration: &client.ObjectStorageLifecycleNoncurrentExpiration{NoncurrentDays: 7, NewerNoncurrentVersions: &keep},
		},
		{ID: "markers", Enabled: false, Expiration: &client.ObjectStorageLifecycleExpiration{ExpiredObjectDeleteMarker: &t1}},
		{ID: "uploads", Enabled: true, AbortIncompleteMultipartUpload: &client.ObjectStorageLifecycleAbortUpload{DaysAfterInitiation: 2}},
	}
	models := modelFromRules(apiRules)
	if models[0].Prefix.ValueString() != "logs/" || models[0].NewerNoncurrentVersions.ValueInt64() != 3 || !models[1].ExpiredObjectDeleteMarker.ValueBool() {
		t.Fatalf("models %+v", models)
	}
	if !models[2].Prefix.IsNull() || !models[2].Tags.IsNull() || !models[2].ExpirationDays.IsNull() {
		t.Fatalf("absent fields must be null: %+v", models[2])
	}
	back, diags := rulesFromModel(ctx, models)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(back) != 3 || back[2].Filter != nil || back[1].Enabled || *back[0].Expiration.Days != 30 {
		t.Fatalf("back %+v", back)
	}
	if got := back[0].Filter.Tags; len(got) != 2 || got[0].Key != "a" || got[1].Key != "b" {
		t.Fatalf("tags must be sorted by key: %+v", got)
	}
}

func TestLifecycleValidation(t *testing.T) {
	base := func(id string) lifecycleRuleModel {
		m := modelFromRules([]client.ObjectStorageLifecycleRule{{ID: id, Enabled: true}})[0]
		m.ExpirationDays = types.Int64Value(30)
		return m
	}
	if d := validateLifecycleRules([]lifecycleRuleModel{base("ok")}); d.HasError() {
		t.Fatal(d)
	}
	cases := map[string]func() []lifecycleRuleModel{
		"none":        func() []lifecycleRuleModel { return nil },
		"reserved id": func() []lifecycleRuleModel { return []lifecycleRuleModel{base("cubepath-x")} },
		"bad id":      func() []lifecycleRuleModel { return []lifecycleRuleModel{base("a b")} },
		"repeated id": func() []lifecycleRuleModel { return []lifecycleRuleModel{base("a"), base("a")} },
		"slash prefix": func() []lifecycleRuleModel {
			m := base("a")
			m.Prefix = types.StringValue("/logs/")
			return []lifecycleRuleModel{m}
		},
		"days and date": func() []lifecycleRuleModel {
			m := base("a")
			m.ExpirationDate = types.StringValue("2030-01-01")
			return []lifecycleRuleModel{m}
		},
		"bad date": func() []lifecycleRuleModel {
			m := base("a")
			m.ExpirationDays = types.Int64Null()
			m.ExpirationDate = types.StringValue("01/01/2030")
			return []lifecycleRuleModel{m}
		},
		"no action": func() []lifecycleRuleModel {
			m := base("a")
			m.ExpirationDays = types.Int64Null()
			return []lifecycleRuleModel{m}
		},
		"newer without days": func() []lifecycleRuleModel {
			m := base("a")
			m.NewerNoncurrentVersions = types.Int64Value(2)
			return []lifecycleRuleModel{m}
		},
	}
	for name, rules := range cases {
		if d := validateLifecycleRules(rules()); !d.HasError() {
			t.Errorf("%s: expected an error", name)
		}
	}
	m := base("tags")
	m.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{"k": types.StringValue("v")})
	if d := validateLifecycleRules([]lifecycleRuleModel{m}); d.HasError() {
		t.Fatalf("a tag filter with days is valid: %v", d)
	}
	m.ExpirationDays = types.Int64Null()
	m.ExpiredObjectDeleteMarker = types.BoolValue(false)
	d := validateLifecycleRules([]lifecycleRuleModel{m})
	if !d.HasError() || !strings.Contains(d.Errors()[0].Detail(), "at least one") {
		t.Fatalf("expired_object_delete_marker = false is not an action: %v", d)
	}
}
