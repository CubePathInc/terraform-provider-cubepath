package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ObjectStorageService handles Object Storage (S3 compatible) API calls
type ObjectStorageService struct {
	client *Client
}

// NewObjectStorageService creates a new Object Storage service
func NewObjectStorageService(client *Client) *ObjectStorageService {
	return &ObjectStorageService{client: client}
}

// ListTiers retrieves the active storage tiers
func (s *ObjectStorageService) ListTiers(ctx context.Context) ([]ObjectStorageTier, error) {
	var result []ObjectStorageTier
	if err := s.client.Get(ctx, "/object-storage/tiers", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListBuckets retrieves the organization's buckets
func (s *ObjectStorageService) ListBuckets(ctx context.Context) ([]ObjectStorageBucket, error) {
	var result []ObjectStorageBucket
	if err := s.client.Get(ctx, "/object-storage/buckets", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBucket retrieves a bucket by UUID
func (s *ObjectStorageService) GetBucket(ctx context.Context, uuid string) (*ObjectStorageBucket, error) {
	var result ObjectStorageBucket
	if err := s.client.Get(ctx, "/object-storage/buckets/"+url.PathEscape(uuid), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateBucket creates a bucket; it starts as pending
func (s *ObjectStorageService) CreateBucket(ctx context.Context, req *CreateObjectStorageBucketRequest) (*ObjectStorageBucket, error) {
	var result ObjectStorageBucket
	if err := s.client.Post(ctx, "/object-storage/buckets", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateBucket changes versioning, deletion protection or tags
func (s *ObjectStorageService) UpdateBucket(ctx context.Context, uuid string, req *UpdateObjectStorageBucketRequest) error {
	return s.client.Patch(ctx, "/object-storage/buckets/"+url.PathEscape(uuid), req, nil)
}

// SetBucketObjectLock changes or removes the default retention of a bucket with Object Lock
func (s *ObjectStorageService) SetBucketObjectLock(ctx context.Context, uuid string, req *SetObjectStorageObjectLockRequest) error {
	return s.client.Put(ctx, "/object-storage/buckets/"+url.PathEscape(uuid)+"/object-lock", req, nil)
}

// DeleteBucket starts the deletion of a bucket. With force its content is purged first;
// bypassGovernance (only with force) also deletes versions under governance retention.
func (s *ObjectStorageService) DeleteBucket(ctx context.Context, uuid string, force, bypassGovernance bool) error {
	path := fmt.Sprintf("/object-storage/buckets/%s?force=%s", url.PathEscape(uuid), strconv.FormatBool(force))
	if force && bypassGovernance {
		path += "&bypass_governance=true"
	}
	return s.client.Delete(ctx, path)
}

// ListKeys retrieves the organization's access keys (never secrets)
func (s *ObjectStorageService) ListKeys(ctx context.Context) ([]ObjectStorageAccessKey, error) {
	var result []ObjectStorageAccessKey
	if err := s.client.Get(ctx, "/object-storage/keys", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetKey finds an access key by UUID. The API has no detail route, so it searches the list.
// It returns an APIError with status 404 when the key does not exist.
func (s *ObjectStorageService) GetKey(ctx context.Context, uuid string) (*ObjectStorageAccessKey, error) {
	keys, err := s.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].UUID == uuid {
			return &keys[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: "Access key not found"}
}

// CreateKey creates an access key. The secret is only returned by this call.
func (s *ObjectStorageService) CreateKey(ctx context.Context, req *CreateObjectStorageKeyRequest) (*ObjectStorageAccessKey, error) {
	var result ObjectStorageAccessKey
	if err := s.client.Post(ctx, "/object-storage/keys", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteKey starts the revocation of an access key
func (s *ObjectStorageService) DeleteKey(ctx context.Context, uuid string) error {
	return s.client.Delete(ctx, "/object-storage/keys/"+url.PathEscape(uuid))
}

// GetUsage retrieves the month usage and cost. An empty period means the current month.
func (s *ObjectStorageService) GetUsage(ctx context.Context, period string, projectID *int, tier string) (*ObjectStorageUsage, error) {
	q := url.Values{}
	if period != "" {
		q.Set("period", period)
	}
	if projectID != nil {
		q.Set("project_id", strconv.Itoa(*projectID))
	}
	if tier != "" {
		q.Set("tier", tier)
	}
	path := "/object-storage/usage"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var result ObjectStorageUsage
	if err := s.client.Get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
