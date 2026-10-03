package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEventDestinationUpdateSendsOnlyChanges(t *testing.T) {
	state := objectStorageEventDestinationModel{
		Name: types.StringValue("hook"), URL: types.StringValue("https://a.example/h"),
		PayloadFormat: types.StringValue("cubepath"), Enabled: types.BoolValue(true),
	}
	plan := state
	if upd := eventDestinationUpdate(plan, state); upd.Name != nil || upd.URL != nil || upd.PayloadFormat != nil || upd.Enabled != nil {
		t.Fatalf("no change must send nothing: %+v", upd)
	}
	plan.URL = types.StringValue("https://b.example/h")
	plan.Enabled = types.BoolValue(false)
	upd := eventDestinationUpdate(plan, state)
	if upd.URL == nil || *upd.URL != "https://b.example/h" || upd.Enabled == nil || *upd.Enabled || upd.Name != nil {
		t.Fatalf("got %+v", upd)
	}
	// An imported destination has no url in the state; a null url in the plan is never sent.
	plan = state
	plan.URL = types.StringNull()
	if upd := eventDestinationUpdate(plan, state); upd.URL != nil {
		t.Fatalf("null url must not be sent: %+v", upd)
	}
}

func TestMapEventDestinationKeepsSecretAndURL(t *testing.T) {
	masked, reason := "https://a.example/***", "failing"
	m := objectStorageEventDestinationModel{
		URL:           types.StringValue("https://a.example/h"),
		SigningSecret: types.StringValue("whsec_x"),
	}
	mapEventDestination(&m, &client.ObjectStorageEventDestination{
		UUID: "d1", Name: "hook", Type: "webhook", URLMasked: &masked, PayloadFormat: "s3",
		Status: "auto_disabled", DisabledReason: &reason, RulesCount: 2,
	})
	if m.URL.ValueString() != "https://a.example/h" || m.SigningSecret.ValueString() != "whsec_x" {
		t.Fatalf("configured url and secret must stay: %+v", m)
	}
	if m.Enabled.ValueBool() || m.Status.ValueString() != "auto_disabled" || m.DisabledReason.ValueString() != "failing" || m.RulesCount.ValueInt64() != 2 {
		t.Fatalf("got %+v", m)
	}
	unknown := objectStorageEventDestinationModel{SigningSecret: types.StringUnknown()}
	mapEventDestination(&unknown, &client.ObjectStorageEventDestination{UUID: "d2", Status: "active"})
	if !unknown.SigningSecret.IsNull() || !unknown.Enabled.ValueBool() {
		t.Fatalf("unknown secret must become null: %+v", unknown)
	}
}

func TestDestinationHasRules(t *testing.T) {
	inUse := &client.APIError{StatusCode: 400, Detail: "This destination is used by 1 event rules. Delete or move them first."}
	if !destinationHasRules(inUse) {
		t.Fatal("a destination in use must be retried")
	}
	for _, err := range []error{
		&client.APIError{StatusCode: 403, Detail: "Organization is suspended. Cannot modify resources."},
		&client.APIError{StatusCode: 400, Detail: "No fields to update"},
		errors.New("boom"),
	} {
		if destinationHasRules(err) {
			t.Fatalf("%v must not be retried", err)
		}
	}
}

func TestValidateStorageEvents(t *testing.T) {
	if d := validateStorageEvents([]string{"object.created", "object.tagging"}); d.HasError() {
		t.Fatal(d)
	}
	for _, bad := range [][]string{nil, {"object.copied"}, {"created"}} {
		if d := validateStorageEvents(bad); !d.HasError() {
			t.Fatalf("%v must fail", bad)
		}
	}
}

func TestEventRuleUpdateAndMapping(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	msg := "bucket busy"
	var state objectStorageEventRuleModel
	rule := &client.ObjectStorageEventRule{UUID: "r1", BucketUUID: "b1", Name: "uploads", Events: []string{"object.created"}, Prefix: "in/", Enabled: true, Status: "error", ErrorMessage: &msg}
	rule.Destination = &struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
		Type string `json:"type"`
	}{UUID: "d1"}
	diags.Append(mapEventRule(ctx, &state, rule)...)
	if diags.HasError() || state.DestinationUUID.ValueString() != "d1" || state.ErrorMessage.ValueString() != msg || state.Suffix.ValueString() != "" {
		t.Fatalf("mapped %+v %v", state, diags)
	}

	plan := state
	if upd := eventRuleUpdate(ctx, plan, state, &diags); upd.Name != nil || upd.Events != nil || upd.Prefix != nil || upd.Enabled != nil {
		t.Fatalf("no change must send nothing: %+v", upd)
	}
	plan.Events, _ = types.SetValueFrom(ctx, types.StringType, []string{"object.removed", "object.created"})
	plan.Prefix = types.StringValue("")
	upd := eventRuleUpdate(ctx, plan, state, &diags)
	if upd.Events == nil || len(*upd.Events) != 2 || (*upd.Events)[0] != "object.created" || upd.Prefix == nil || *upd.Prefix != "" || upd.Name != nil {
		t.Fatalf("got %+v", upd)
	}
}
