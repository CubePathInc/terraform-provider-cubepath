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
