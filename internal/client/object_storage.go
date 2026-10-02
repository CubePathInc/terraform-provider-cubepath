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

func lifecyclePath(uuid string) string {
	return "/object-storage/buckets/" + url.PathEscape(uuid) + "/lifecycle"
}

// GetBucketLifecycle retrieves the lifecycle rules of a bucket and whether they are applied
func (s *ObjectStorageService) GetBucketLifecycle(ctx context.Context, uuid string) (*ObjectStorageLifecycle, error) {
	var result ObjectStorageLifecycle
	if err := s.client.Get(ctx, lifecyclePath(uuid), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PutBucketLifecycle replaces every lifecycle rule of a bucket; it is applied asynchronously
func (s *ObjectStorageService) PutBucketLifecycle(ctx context.Context, uuid string, rules []ObjectStorageLifecycleRule) (*ObjectStorageLifecycleChange, error) {
	var result ObjectStorageLifecycleChange
	if err := s.client.Put(ctx, lifecyclePath(uuid), map[string]interface{}{"rules": rules}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteBucketLifecycle removes every lifecycle rule of a bucket
func (s *ObjectStorageService) DeleteBucketLifecycle(ctx context.Context, uuid string) error {
	return s.client.Delete(ctx, lifecyclePath(uuid))
}

// ListReplications retrieves the organization's replications. direction is outgoing, incoming or
// all ("" = all); bucketUUID limits them to one source (outgoing) or destination (incoming) bucket.
func (s *ObjectStorageService) ListReplications(ctx context.Context, direction, bucketUUID string) ([]ObjectStorageReplication, error) {
	q := url.Values{}
	if direction != "" {
		q.Set("direction", direction)
	}
	if bucketUUID != "" {
		q.Set("bucket_uuid", bucketUUID)
	}
	path := "/object-storage/replications"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var result []ObjectStorageReplication
	if err := s.client.Get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func replicationPath(uuid string) string {
	return "/object-storage/replications/" + url.PathEscape(uuid)
}

// GetReplication retrieves a replication of a source bucket of the organization
func (s *ObjectStorageService) GetReplication(ctx context.Context, uuid string) (*ObjectStorageReplication, error) {
	var result ObjectStorageReplication
	if err := s.client.Get(ctx, replicationPath(uuid), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateReplication creates a replication; it starts as pending
func (s *ObjectStorageService) CreateReplication(ctx context.Context, req *CreateObjectStorageReplicationRequest) (*ObjectStorageReplicationCreated, error) {
	var result ObjectStorageReplicationCreated
	if err := s.client.Post(ctx, "/object-storage/replications", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateReplication changes the rules, pauses or resumes, or rotates external credentials. The body
// is a map because a null prefix or tags removes them.
func (s *ObjectStorageService) UpdateReplication(ctx context.Context, uuid string, body map[string]interface{}) error {
	return s.client.Patch(ctx, replicationPath(uuid), body, nil)
}

// DeleteReplication starts the removal of a replication; the data already replicated stays
func (s *ObjectStorageService) DeleteReplication(ctx context.Context, uuid string) error {
	return s.client.Delete(ctx, replicationPath(uuid))
}

func replicationGrantsPath(bucketUUID string) string {
	return "/object-storage/buckets/" + url.PathEscape(bucketUUID) + "/replication-grants"
}

// CreateReplicationGrant authorizes another organization to replicate into a bucket. The token is
// only returned by this call.
func (s *ObjectStorageService) CreateReplicationGrant(ctx context.Context, bucketUUID string, req *CreateObjectStorageReplicationGrantRequest) (*ObjectStorageReplicationGrant, error) {
	var result ObjectStorageReplicationGrant
	if err := s.client.Post(ctx, replicationGrantsPath(bucketUUID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListReplicationGrants retrieves the grants of a bucket (never the tokens)
func (s *ObjectStorageService) ListReplicationGrants(ctx context.Context, bucketUUID string) ([]ObjectStorageReplicationGrant, error) {
	var result []ObjectStorageReplicationGrant
	if err := s.client.Get(ctx, replicationGrantsPath(bucketUUID), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetReplicationGrant finds a grant of a bucket by UUID. The API has no detail route, so it
// searches the list. It returns an APIError with status 404 when the grant does not exist.
func (s *ObjectStorageService) GetReplicationGrant(ctx context.Context, bucketUUID, uuid string) (*ObjectStorageReplicationGrant, error) {
	grants, err := s.ListReplicationGrants(ctx, bucketUUID)
	if err != nil {
		return nil, err
	}
	for i := range grants {
		if grants[i].UUID == uuid {
			return &grants[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: "Replication grant not found"}
}

// DeleteReplicationGrant revokes a grant that was not used
func (s *ObjectStorageService) DeleteReplicationGrant(ctx context.Context, uuid string) error {
	return s.client.Delete(ctx, "/object-storage/replication-grants/"+url.PathEscape(uuid))
}
