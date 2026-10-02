package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]interface{}
}

// newTestClient starts a server that records every request and answers with handler.
func newTestClient(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*Client, *[]recordedRequest) {
	t.Helper()
	var reqs []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery}
		if body, _ := io.ReadAll(r.Body); len(body) > 0 {
			_ = json.Unmarshal(body, &rec.Body)
		}
		reqs = append(reqs, rec)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient("token", srv.URL, WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	return c, &reqs
}

func TestCreateBucketOriginSendsOnlyTheAllowedFields(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"o1","name":"photos","object_storage_bucket_uuid":"b1"}`))
	})

	origin, err := c.CDN.CreateBucketOrigin(context.Background(), "z1", &CreateCDNBucketOriginRequest{
		Name: "photos", ObjectStorageBucketUUID: "b1", Weight: 100, Priority: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if origin.ObjectStorageBucketUUID == nil || *origin.ObjectStorageBucketUUID != "b1" {
		t.Fatalf("bucket uuid not decoded: %+v", origin)
	}

	req := (*reqs)[0]
	if req.Method != http.MethodPost || req.Path != "/cdn/zones/z1/origins" {
		t.Fatalf("got %s %s", req.Method, req.Path)
	}
	// The API refuses any connection field next to object_storage_bucket_uuid.
	allowed := map[string]bool{"name": true, "object_storage_bucket_uuid": true, "weight": true, "priority": true, "is_backup": true}
	for k := range req.Body {
		if !allowed[k] {
			t.Errorf("field %q must not be sent with a bucket: %v", k, req.Body)
		}
	}
}

func TestGetKeyNotFoundIsA404(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"uuid":"k1","name":"a","access_key_id":"CPX","status":"active"}]`))
	})

	key, err := c.ObjectStorage.GetKey(context.Background(), "k1")
	if err != nil || key.AccessKeyID != "CPX" {
		t.Fatalf("got %+v, %v", key, err)
	}

	_, err = c.ObjectStorage.GetKey(context.Background(), "missing")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsNotFound() {
		t.Fatalf("expected a 404 APIError, got %v", err)
	}
}

func TestDeleteBucketPassesForce(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"detail":"Bucket deletion started"}`))
	})

	if err := c.ObjectStorage.DeleteBucket(context.Background(), "b1", true, false); err != nil {
		t.Fatal(err)
	}
	if req := (*reqs)[0]; req.Method != http.MethodDelete || req.Path != "/object-storage/buckets/b1" || req.Query != "force=true" {
		t.Fatalf("got %+v", req)
	}
	if err := c.ObjectStorage.DeleteBucket(context.Background(), "b1", true, true); err != nil {
		t.Fatal(err)
	}
	if q := (*reqs)[1].Query; q != "force=true&bypass_governance=true" {
		t.Fatalf("query %q", q)
	}
	// bypass_governance is only valid together with force
	if err := c.ObjectStorage.DeleteBucket(context.Background(), "b1", false, true); err != nil {
		t.Fatal(err)
	}
	if q := (*reqs)[2].Query; q != "force=false" {
		t.Fatalf("query %q", q)
	}
}

func TestBucketObjectLockBodies(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"uuid":"b1","object_lock":{"enabled":true,"default_retention":{"mode":"governance","days":30,"years":null}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"detail":"Bucket updated"}`))
	})
	ctx := context.Background()

	days := 30
	b, err := c.ObjectStorage.CreateBucket(ctx, &CreateObjectStorageBucketRequest{
		Name: "backups", Tier: "ia", Versioning: true, ObjectLock: true, AcceptObjectLockTerms: true,
		ObjectLockDefault: &ObjectStorageLockRetention{Mode: "governance", Days: &days},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !b.ObjectLock.Enabled || b.ObjectLock.DefaultRetention == nil || *b.ObjectLock.DefaultRetention.Days != 30 || b.ObjectLock.DefaultRetention.Years != nil {
		t.Fatalf("decoded %+v", b.ObjectLock)
	}
	body := (*reqs)[0].Body
	rule, _ := body["object_lock_default"].(map[string]interface{})
	if body["object_lock"] != true || body["accept_object_lock_terms"] != true || body["versioning"] != true || rule["mode"] != "governance" || rule["days"] != float64(30) {
		t.Fatalf("create body %v", body)
	}
	if _, ok := rule["years"]; ok {
		t.Fatalf("years sent when unset: %v", rule)
	}

	if _, err := c.ObjectStorage.CreateBucket(ctx, &CreateObjectStorageBucketRequest{Name: "photos", Tier: "ia"}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"object_lock", "object_lock_default", "accept_object_lock_terms"} {
		if _, ok := (*reqs)[1].Body[k]; ok {
			t.Errorf("%s sent without Object Lock: %v", k, (*reqs)[1].Body)
		}
	}

	years := 1
	if err := c.ObjectStorage.SetBucketObjectLock(ctx, "b1", &SetObjectStorageObjectLockRequest{
		DefaultRetention: &ObjectStorageLockRetention{Mode: "compliance", Years: &years}, AcceptObjectLockTerms: true,
	}); err != nil {
		t.Fatal(err)
	}
	put := (*reqs)[2]
	rule, _ = put.Body["default_retention"].(map[string]interface{})
	if put.Method != http.MethodPut || put.Path != "/object-storage/buckets/b1/object-lock" || rule["years"] != float64(1) || put.Body["accept_object_lock_terms"] != true {
		t.Fatalf("got %+v", put)
	}

	if err := c.ObjectStorage.SetBucketObjectLock(ctx, "b1", &SetObjectStorageObjectLockRequest{}); err != nil {
		t.Fatal(err)
	}
	if v, ok := (*reqs)[3].Body["default_retention"]; !ok || v != nil {
		t.Fatalf("removing the rule must send null: %v", (*reqs)[3].Body)
	}
}

func TestCreateKeyOmitsEmptyOptionalFields(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"k1","access_key_id":"CPX","secret_access_key":"s","status":"pending"}`))
	})

	key, err := c.ObjectStorage.CreateKey(context.Background(), &CreateObjectStorageKeyRequest{
		Name: "backups", Tier: "infrequent_access", Permission: "read_write",
	})
	if err != nil || key.SecretAccessKey != "s" {
		t.Fatalf("got %+v, %v", key, err)
	}
	body := (*reqs)[0].Body
	for _, k := range []string{"bucket_uuids", "expires_at", "project_id", "bypass_governance"} {
		if _, ok := body[k]; ok {
			t.Errorf("%s must be omitted when unset: %v", k, body)
		}
	}
}

func TestGetUsageQuery(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"period":"2026-09","metrics_available":false,"total_cost":1.5,"buckets":[{"uuid":"b1","name":"x","storage_gib_month":null,"cost":1.5}]}`))
	})

	pid := 12
	usage, err := c.ObjectStorage.GetUsage(context.Background(), "2026-09", &pid, "")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Buckets[0].StorageGiBMonth != nil || usage.TotalCost != 1.5 {
		t.Fatalf("got %+v", usage)
	}
	if q := (*reqs)[0].Query; q != "period=2026-09&project_id=12" {
		t.Fatalf("query %q", q)
	}
}

func TestBucketTagsRequestBodies(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte(`{"uuid":"b1","tags":{"env":"prod"}}`))
	})
	ctx := context.Background()

	b, err := c.ObjectStorage.CreateBucket(ctx, &CreateObjectStorageBucketRequest{Name: "photos", Tier: "ia", Tags: map[string]string{"env": "prod"}})
	if err != nil {
		t.Fatal(err)
	}
	if tags, _ := (*reqs)[0].Body["tags"].(map[string]interface{}); tags["env"] != "prod" || b.Tags["env"] != "prod" {
		t.Fatalf("create body %v, decoded %v", (*reqs)[0].Body, b.Tags)
	}
	if _, err := c.ObjectStorage.CreateBucket(ctx, &CreateObjectStorageBucketRequest{Name: "photos", Tier: "ia"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := (*reqs)[1].Body["tags"]; ok {
		t.Fatalf("tags sent when empty: %v", (*reqs)[1].Body)
	}

	protected := true
	if err := c.ObjectStorage.UpdateBucket(ctx, "b1", &UpdateObjectStorageBucketRequest{Protected: &protected}); err != nil {
		t.Fatal(err)
	}
	if _, ok := (*reqs)[2].Body["tags"]; ok {
		t.Fatalf("tags sent when nil: %v", (*reqs)[2].Body)
	}
	empty := map[string]string{}
	if err := c.ObjectStorage.UpdateBucket(ctx, "b1", &UpdateObjectStorageBucketRequest{Tags: &empty}); err != nil {
		t.Fatal(err)
	}
	if tags, ok := (*reqs)[3].Body["tags"].(map[string]interface{}); !ok || len(tags) != 0 {
		t.Fatalf("clear body %v", (*reqs)[3].Body)
	}
}

func TestBucketLifecycleRoutes(t *testing.T) {
	c, reqs := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"bucket_uuid":"b1","status":"active","generation":2,"applied_generation":2,"rules":[{"id":"r","enabled":true,"expiration":{"days":30}}]}`))
		case http.MethodPut:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"detail":"Lifecycle rules are being applied","generation":3}`))
		default:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"detail":"Lifecycle rules are being removed","generation":4}`))
		}
	})
	ctx := context.Background()
	lc, err := c.ObjectStorage.GetBucketLifecycle(ctx, "b1")
	if err != nil || lc.Rules[0].ID != "r" || *lc.Rules[0].Expiration.Days != 30 {
		t.Fatalf("got %+v, %v", lc, err)
	}
	days := int64(7)
	change, err := c.ObjectStorage.PutBucketLifecycle(ctx, "b1", []ObjectStorageLifecycleRule{{ID: "r", Enabled: true, Expiration: &ObjectStorageLifecycleExpiration{Days: &days}}})
	if err != nil || *change.Generation != 3 {
		t.Fatalf("got %+v, %v", change, err)
	}
	if err := c.ObjectStorage.DeleteBucketLifecycle(ctx, "b1"); err != nil {
		t.Fatal(err)
	}
	put := (*reqs)[1]
	if put.Method != http.MethodPut || put.Path != "/object-storage/buckets/b1/lifecycle" {
		t.Fatalf("got %+v", put)
	}
	rule := put.Body["rules"].([]interface{})[0].(map[string]interface{})
	if _, ok := rule["filter"]; ok {
		t.Fatalf("an empty filter must be omitted: %v", rule)
	}
	if (*reqs)[2].Method != http.MethodDelete || (*reqs)[2].Path != "/object-storage/buckets/b1/lifecycle" {
		t.Fatalf("got %+v", (*reqs)[2])
	}
}
