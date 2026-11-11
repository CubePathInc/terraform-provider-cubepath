package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
)

// terraform destroy must never cancel a dedicated server: Delete only drops it from state.
func TestBaremetalDeleteOnlyRemovesFromState(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c, err := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	r := &baremetalResource{client: c}

	ctx := context.Background()
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, typ := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(typ, nil)
	}
	vals["id"] = tftypes.NewValue(tftypes.String, "123")
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, vals)}

	resp := &resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if calls != 0 {
		t.Fatalf("Delete made %d API calls, want 0", calls)
	}
	warnings := resp.Diagnostics.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), "cancel it from the CubePath dashboard") {
		t.Fatalf("warnings %v", warnings)
	}
}

func baremetalReadState(t *testing.T, r *baremetalResource, id string) (tfsdk.State, resource.SchemaResponse) {
	t.Helper()
	ctx := context.Background()
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, typ := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(typ, nil)
	}
	vals["id"] = tftypes.NewValue(tftypes.String, id)
	return tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, vals)}, schemaResp
}

// A server that /projects/ does not list must stay in the state with an error: removing it
// would make the next plan create (and bill) a second server.
func TestBaremetalReadNotFoundIsAnErrorNotRemoval(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"project":{"id":1,"name":"p"},"baremetals":[{"id":999,"hostname":"other"}]}]`))
	}))
	defer srv.Close()
	c, err := client.NewClient("token", srv.URL, client.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	r := &baremetalResource{client: c}
	state, _ := baremetalReadState(t, r, "123")

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Read of a missing baremetal returned no error")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "terraform state rm") {
		t.Fatalf("error does not explain how to proceed: %v", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("Read removed the resource from the state")
	}
}
