package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func nullDestination(typ string) *replicationDestinationModel {
	return &replicationDestinationModel{
		Type: types.StringValue(typ), BucketUUID: types.StringNull(), GrantToken: types.StringNull(),
		Provider: types.StringNull(), Endpoint: types.StringNull(), Region: types.StringNull(), Bucket: types.StringNull(),
		PathStyle: types.StringNull(), AccessKeyID: types.StringNull(), SecretAccessKey: types.StringNull(),
		SecretAccessKeyVersion: types.Int64Null(),
	}
}

func externalDestination() *replicationDestinationModel {
	d := nullDestination("external")
	d.Endpoint = types.StringValue("s3.eu-west-1.amazonaws.com")
	d.Region = types.StringValue("eu-west-1")
	d.Bucket = types.StringValue("acme-backup")
	d.AccessKeyID = types.StringValue("AKIAEXAMPLE")
	d.SecretAccessKey = types.StringValue("secret")
	return d
}

func cubepathDestination() *replicationDestinationModel {
	d := nullDestination("cubepath")
	d.BucketUUID = types.StringValue("b2")
	return d
}

func ruleModel(prefix string, tags map[string]string) *replicationRuleModel {
	m := &replicationRuleModel{
		Prefix: types.StringNull(), Tags: types.MapNull(types.StringType),
		DeleteMarkerReplication: types.BoolValue(false), DeleteReplication: types.BoolValue(false),
		ExistingObjects: types.BoolValue(true),
	}
	if prefix != "" {
		m.Prefix = types.StringValue(prefix)
	}
	if tags != nil {
		m.Tags, _ = types.MapValueFrom(context.Background(), types.StringType, tags)
	}
	return m
}

func TestReplicationValidation(t *testing.T) {
	ctx := context.Background()
	if d := validateReplication(ctx, cubepathDestination(), nil); d.HasError() {
		t.Fatal(d)
	}
	if d := validateReplication(ctx, externalDestination(), ruleModel("img/", nil)); d.HasError() {
		t.Fatal(d)
	}
	cases := map[string]func() (*replicationDestinationModel, *replicationRuleModel){
		"no destination": func() (*replicationDestinationModel, *replicationRuleModel) { return nil, nil },
		"cubepath without bucket": func() (*replicationDestinationModel, *replicationRuleModel) {
			return nullDestination("cubepath"), nil
		},
		"cubepath with credentials": func() (*replicationDestinationModel, *replicationRuleModel) {
			d := cubepathDestination()
			d.SecretAccessKey = types.StringValue("x")
			return d, nil
		},
		"external without secret": func() (*replicationDestinationModel, *replicationRuleModel) {
			d := externalDestination()
			d.SecretAccessKey = types.StringNull()
			return d, nil
		},
		"external with grant": func() (*replicationDestinationModel, *replicationRuleModel) {
			d := externalDestination()
			d.GrantToken = types.StringValue("cprg_x")
			return d, nil
		},
		"scheme in endpoint": func() (*replicationDestinationModel, *replicationRuleModel) {
			d := externalDestination()
			d.Endpoint = types.StringValue("https://s3.amazonaws.com")
			return d, nil
		},
		"other port": func() (*replicationDestinationModel, *replicationRuleModel) {
			d := externalDestination()
			d.Endpoint = types.StringValue("s3.example.com:9000")
			return d, nil
		},
		"bad region": func() (*replicationDestinationModel, *replicationRuleModel) {
			d := externalDestination()
			d.Region = types.StringValue("EU_West")
			return d, nil
		},
		"bad bucket": func() (*replicationDestinationModel, *replicationRuleModel) {
			d := externalDestination()
			d.Bucket = types.StringValue("Acme_Backup")
			return d, nil
		},
		"prefix and tags": func() (*replicationDestinationModel, *replicationRuleModel) {
			return cubepathDestination(), ruleModel("img/", map[string]string{"a": "1"})
		},
		"delete markers with tags": func() (*replicationDestinationModel, *replicationRuleModel) {
			r := ruleModel("", map[string]string{"a": "1"})
			r.DeleteMarkerReplication = types.BoolValue(true)
			return cubepathDestination(), r
		},
	}
	for name, build := range cases {
		dest, rule := build()
		if d := validateReplication(ctx, dest, rule); !d.HasError() {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestNormalizeReplicationEndpoint(t *testing.T) {
	for in, want := range map[string]string{
		"S3.EU-West-1.amazonaws.com":    "s3.eu-west-1.amazonaws.com",
		" s3.wasabisys.com:443 ":        "s3.wasabisys.com",
		"s3.eu-central-1.wasabisys.com": "s3.eu-central-1.wasabisys.com",
	} {
		got, err := normalizeReplicationEndpoint(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"localhost", "s3.example.com:80", "user@s3.example.com", "s3.example.com/path", ""} {
		if _, err := normalizeReplicationEndpoint(in); err == nil {
			t.Errorf("%q must be refused", in)
		}
	}
}

func TestReplicationCreateBody(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	plan := &replicationResourceModel{SourceBucketUUID: types.StringValue("b1"), Destination: cubepathDestination()}
	body := buildReplicationCreate(ctx, plan, &diags)
	raw, _ := json.Marshal(body)
	var got map[string]interface{}
	_ = json.Unmarshal(raw, &got)
	dest := got["destination"].(map[string]interface{})
	if len(dest) != 2 || got["existing_objects"] != true || got["prefix"] != nil {
		t.Fatalf("without a rule block the API defaults apply and only CubePath fields go: %s", raw)
	}

	plan = &replicationResourceModel{
		SourceBucketUUID: types.StringValue("b1"), Destination: externalDestination(),
		Rule: ruleModel("", map[string]string{"z": "1", "a": "2"}),
	}
	body = buildReplicationCreate(ctx, plan, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if body.Destination.SecretAccessKey != "secret" || body.Destination.BucketUUID != "" || len(body.Tags) != 2 || body.Tags[0].Key != "a" {
		t.Fatalf("got %+v", body)
	}
}

func TestReplicationPatchBody(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	prior := &replicationResourceModel{
		Enabled: types.BoolValue(true), Destination: externalDestination(), Rule: ruleModel("img/", nil),
	}

	same := *prior
	if body := buildReplicationPatch(ctx, prior, &same, &diags); body != nil {
		t.Fatalf("no change must send nothing: %v", body)
	}

	plan := *prior
	plan.Rule = nil
	plan.Enabled = types.BoolValue(false)
	body := buildReplicationPatch(ctx, prior, &plan, &diags)
	if v, ok := body["prefix"]; !ok || v != nil || body["enabled"] != false || len(body) != 2 {
		t.Fatalf("removing the rule block must send prefix null: %v", body)
	}

	plan = *prior
	dest := *prior.Destination
	dest.SecretAccessKeyVersion = types.Int64Value(2)
	plan.Destination = &dest
	body = buildReplicationPatch(ctx, prior, &plan, &diags)
	creds, ok := body["destination"].(map[string]string)
	if !ok || creds["secret_access_key"] != "secret" || creds["access_key_id"] != "AKIAEXAMPLE" || len(body) != 1 {
		t.Fatalf("a new secret version must send the credentials: %v", body)
	}

	plan = *prior
	plan.Rule = ruleModel("", map[string]string{"backup": "yes"})
	body = buildReplicationPatch(ctx, prior, &plan, &diags)
	if tags, ok := body["tags"].([]client.ObjectStorageLifecycleTag); !ok || len(tags) != 1 || body["prefix"] != nil {
		t.Fatalf("got %v", body)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func apiReplication(prefix *string) *client.ObjectStorageReplication {
	ep, region, bucket, provider, style, key := "s3.eu-west-1.amazonaws.com", "eu-west-1", "acme-backup", "other", "auto", "****MPLE"
	src := "b1"
	return &client.ObjectStorageReplication{
		UUID: "r1", Status: "active", Direction: "outgoing", Health: "ok",
		Source: client.ObjectStorageReplicationSource{BucketUUID: &src},
		Destination: client.ObjectStorageReplicationDestination{
			Type: "external", Endpoint: &ep, Region: &region, Bucket: &bucket, Provider: &provider, PathStyle: &style, AccessKeyID: &key,
		},
		Rules:    client.ObjectStorageReplicationRules{Enabled: true, Prefix: prefix, ExistingObjects: true},
		Backfill: client.ObjectStorageReplicationBackfill{Status: "running"},
	}
}

func TestReplicationMapToState(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	r := &objectStorageReplicationResource{}

	dest := externalDestination()
	dest.Endpoint = types.StringValue("S3.eu-west-1.amazonaws.com:443")
	state := &replicationResourceModel{Destination: dest}
	r.mapToState(ctx, state, apiReplication(nil), &diags)
	if state.Destination.Endpoint.ValueString() != "S3.eu-west-1.amazonaws.com:443" {
		t.Errorf("an equivalent endpoint must keep the configured spelling: %v", state.Destination.Endpoint)
	}
	if state.Destination.AccessKeyID.ValueString() != "AKIAEXAMPLE" || state.Destination.SecretAccessKey.ValueString() != "secret" {
		t.Errorf("credentials must not be read back: %+v", state.Destination)
	}
	if !state.Destination.Provider.IsNull() || !state.Destination.PathStyle.IsNull() {
		t.Errorf("unset provider and path_style must stay null with the API defaults: %+v", state.Destination)
	}
	if state.Rule != nil {
		t.Errorf("default rules without a rule block must stay without it: %+v", state.Rule)
	}
	if state.BackfillStatus.ValueString() != "running" || state.Health.ValueString() != "ok" || !state.Enabled.ValueBool() {
		t.Errorf("computed fields: %+v", state)
	}

	prefix := "img/"
	r.mapToState(ctx, state, apiReplication(&prefix), &diags)
	if state.Rule == nil || state.Rule.Prefix.ValueString() != "img/" {
		t.Errorf("a prefix set outside Terraform must show up: %+v", state.Rule)
	}

	other := "s3.us-east-1.amazonaws.com"
	moved := apiReplication(&prefix)
	moved.Destination.Endpoint = &other
	r.mapToState(ctx, state, moved, &diags)
	if state.Destination.Endpoint.ValueString() != other {
		t.Errorf("a different endpoint must be written: %v", state.Destination.Endpoint)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func TestReplicationUpdateSendsOnlyTheChange(t *testing.T) {
	ctx := context.Background()
	var patches []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			patches = append(patches, body)
			_, _ = w.Write([]byte(`{"detail":"Replication updated"}`))
			return
		}
		_, _ = w.Write([]byte(`{"uuid":"r1","status":"paused","pause_reason":"customer","direction":"outgoing",` +
			`"source":{"bucket_uuid":"b1","same_organization":true},` +
			`"destination":{"type":"cubepath","bucket_uuid":"b2","same_organization":true},` +
			`"rules":{"enabled":false,"prefix":null,"tags":[],"delete_marker_replication":false,"delete_replication":false,"existing_objects":true},` +
			`"health":"ok","backfill":{"status":"completed","objects":0,"bytes":0,"failed_objects":0}}`))
	}))
	defer srv.Close()
	c, err := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	r := &objectStorageReplicationResource{client: c}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	prior := replicationResourceModel{
		ID: types.StringValue("r1"), SourceBucketUUID: types.StringValue("b1"), Enabled: types.BoolValue(true),
		Destination: cubepathDestination(), Status: types.StringValue("active"), PauseReason: types.StringNull(),
		Health: types.StringValue("ok"), HealthReason: types.StringNull(), BackfillStatus: types.StringValue("completed"),
		ErrorMessage: types.StringNull(),
	}
	planModel := prior
	planModel.Enabled = types.BoolValue(false)
	planModel.Status = types.StringUnknown()

	state := tfsdk.State{Schema: schemaResp.Schema}
	if d := state.Set(ctx, &prior); d.HasError() {
		t.Fatal(d)
	}
	plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
	if d := plan.Set(ctx, &planModel); d.HasError() {
		t.Fatal(d)
	}
	resp := &resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(patches) != 1 || len(patches[0]) != 1 || patches[0]["enabled"] != false {
		t.Fatalf("patches %v", patches)
	}
	var got replicationResourceModel
	resp.State.Get(ctx, &got)
	if got.Status.ValueString() != "paused" || got.PauseReason.ValueString() != "customer" || got.Rule != nil {
		t.Fatalf("state %+v", got)
	}
}

func TestReplicationBusyIsRetried(t *testing.T) {
	busy := &client.APIError{StatusCode: 409, Detail: "The replication is busy with another operation. Try again when it finishes."}
	gone := &client.APIError{StatusCode: 409, Detail: "The replication is already being removed."}
	if !isReplicationBusy(busy) || isReplicationBusy(gone) {
		t.Fatal("only the busy conflict is retried")
	}
}

func TestReplicationGrantDeleteOfAUsedGrantWarns(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"detail":"This replication grant has already been used."}`))
	}))
	defer srv.Close()
	c, _ := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	r := &objectStorageReplicationGrantResource{client: c}
	state := newState(t, r, map[string]tftypes.Value{
		"id":          tftypes.NewValue(tftypes.String, "g1"),
		"bucket_uuid": tftypes.NewValue(tftypes.String, "b1"),
	})
	resp := &resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, resp)
	if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("a used grant must only warn: %v", resp.Diagnostics)
	}
}

func TestReplicationGrantRevokedOutsideIsRemoved(t *testing.T) {
	ctx := context.Background()
	c, _ := fakeAPI(t, `[{"uuid":"g1","token_prefix":"cprg_AbCd","status":"revoked"},`+
		`{"uuid":"g2","token_prefix":"cprg_EfGh","status":"used","used_at":"2026-10-02T10:00:00"}]`)
	r := &objectStorageReplicationGrantResource{client: c}

	read := func(id string) *resource.ReadResponse {
		state := newState(t, r, map[string]tftypes.Value{
			"id":          tftypes.NewValue(tftypes.String, id),
			"bucket_uuid": tftypes.NewValue(tftypes.String, "b1"),
			"token":       tftypes.NewValue(tftypes.String, "cprg_secret"),
		})
		resp := &resource.ReadResponse{State: state}
		r.Read(ctx, resource.ReadRequest{State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		return resp
	}
	if resp := read("g1"); !resp.State.Raw.IsNull() {
		t.Fatal("a revoked grant must leave the state")
	}
	resp := read("g2")
	var got replicationGrantResourceModel
	resp.State.Get(ctx, &got)
	if got.Status.ValueString() != "used" || got.Token.ValueString() != "cprg_secret" || got.UsedAt.IsNull() {
		t.Fatalf("a used grant stays with its token: %+v", got)
	}
}

func TestReplicationRows(t *testing.T) {
	prefix := "img/"
	rows := replicationRows([]client.ObjectStorageReplication{*apiReplication(&prefix)})
	if len(rows) != 1 || rows[0].DestinationEndpoint.ValueString() != "s3.eu-west-1.amazonaws.com" ||
		!rows[0].DestinationBucketUUID.IsNull() || rows[0].Prefix.ValueString() != "img/" || rows[0].BackfillStatus.ValueString() != "running" {
		t.Fatalf("rows %+v", rows)
	}
	if !strings.HasPrefix(rows[0].UUID.ValueString(), "r1") {
		t.Fatalf("uuid %v", rows[0].UUID)
	}
}
