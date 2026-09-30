package client

import (
	"context"
	"net/http"
	"testing"
)

func TestVPSUpdateUsesPatchUpdateEndpoint(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	name, label := "web-2", "production"
	if err := c.VPS.Update(context.Background(), 42, &UpdateVPSRequest{Name: &name, Label: &label}); err != nil {
		t.Fatal(err)
	}
	got := (*reqs)[0]
	if got.Method != http.MethodPatch || got.Path != "/vps/update/42" {
		t.Fatalf("got %s %s, want PATCH /vps/update/42", got.Method, got.Path)
	}
	if got.Body["name"] != "web-2" || got.Body["label"] != "production" {
		t.Fatalf("body %v", got.Body)
	}
}

func TestVPSRenameSendsOnlyTheName(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	if err := c.VPS.Rename(context.Background(), 7, "db-1"); err != nil {
		t.Fatal(err)
	}
	got := (*reqs)[0]
	if got.Method != http.MethodPatch || got.Path != "/vps/update/7" {
		t.Fatalf("got %s %s", got.Method, got.Path)
	}
	if _, ok := got.Body["label"]; ok || got.Body["name"] != "db-1" {
		t.Fatalf("body %v", got.Body)
	}
}
