package client

import (
	"context"
	"net/http"
	"testing"
)

func TestEventDestinationRoutesAndSecret(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/object-storage/event-destinations":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"destination":{"uuid":"d1","name":"hook","type":"webhook","url_masked":"https://example.com/***","status":"active"},"signing_secret":"whsec_x"}`))
		case r.URL.Path == "/object-storage/buckets/b1/event-rules" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"uuid":"r1","bucket_uuid":"b1","destination":{"uuid":"d1","name":"hook","type":"webhook"},"status":"active","events":["object.created"],"created_at":"2026-10-02T10:00:00"},{"uuid":"r2","bucket_uuid":"b1","destination":null,"status":"pending"}]`))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"detail":"Event rule deleted. The bucket stops sending its events within a minute."}`))
		default:
			_, _ = w.Write([]byte(`{"uuid":"d1","status":"active"}`))
		}
	})
	ctx := context.Background()
	u := "https://example.com/hook"
	created, err := c.ObjectStorage.CreateEventDestination(ctx, &CreateObjectStorageEventDestinationRequest{Name: "hook", Type: "webhook", URL: &u})
	if err != nil || created.SigningSecret == nil || *created.SigningSecret != "whsec_x" || created.Destination.UUID != "d1" {
		t.Fatalf("got %+v, %v", created, err)
	}
	if b := (*reqs)[0].Body; b["url"] != u || b["type"] != "webhook" || b["notificator_id"] != nil {
		t.Fatalf("body %v", b)
	}
	off := false
	if _, err := c.ObjectStorage.UpdateEventDestination(ctx, "d1", &UpdateObjectStorageEventDestinationRequest{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if b := (*reqs)[1].Body; len(b) != 1 || b["enabled"] != false {
		t.Fatalf("patch must send only enabled: %v", b)
	}
	if err := c.ObjectStorage.DeleteEventDestination(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	rule, err := c.ObjectStorage.GetEventRule(ctx, "b1", "r2")
	if err != nil || rule == nil || rule.Status != "pending" || rule.Destination != nil {
		t.Fatalf("got %+v, %v", rule, err)
	}
	if missing, err := c.ObjectStorage.GetEventRule(ctx, "b1", "r9"); err != nil || missing != nil {
		t.Fatalf("absent rule must be nil: %+v, %v", missing, err)
	}
	if err := c.ObjectStorage.DeleteEventRule(ctx, "b1", "r1"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"POST /object-storage/event-destinations",
		"PATCH /object-storage/event-destinations/d1",
		"DELETE /object-storage/event-destinations/d1",
		"GET /object-storage/buckets/b1/event-rules",
		"GET /object-storage/buckets/b1/event-rules",
		"DELETE /object-storage/buckets/b1/event-rules/r1",
	}
	for i, w := range want {
		if got := (*reqs)[i].Method + " " + (*reqs)[i].Path; got != w {
			t.Fatalf("request %d: got %s, want %s", i, got, w)
		}
	}
}
