package client

// ObjectStorageTierSummary is the short tier object nested in buckets and access keys
type ObjectStorageTierSummary struct {
	UUID  string `json:"uuid"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Media string `json:"media"`
}

// ObjectStorageTierPrices are the USD prices of a storage tier
type ObjectStorageTierPrices struct {
	StorageGBMonth float64 `json:"storage_gb_month"`
	EgressGB       float64 `json:"egress_gb"`
	ClassAPer1K    float64 `json:"class_a_per_1k"`
	ClassBPer1K    float64 `json:"class_b_per_1k"`
}

// ObjectStorageTierFreeTier is the monthly free allowance of a tier, per organization
type ObjectStorageTierFreeTier struct {
	StorageGBMonth float64 `json:"storage_gb_month"`
	EgressGB       float64 `json:"egress_gb"`
	Requests       int64   `json:"requests"`
}

// ObjectStorageTier represents a storage tier (GET /object-storage/tiers)
type ObjectStorageTier struct {
	UUID                string                    `json:"uuid"`
	Slug                string                    `json:"slug"`
	Name                string                    `json:"name"`
	Media               string                    `json:"media"`
	LocationName        string                    `json:"location_name"`
	LocationDescription string                    `json:"location_description"`
	Region              string                    `json:"region"`
	Endpoint            string                    `json:"endpoint"`
	Prices              ObjectStorageTierPrices   `json:"prices"`
	FreeTier            ObjectStorageTierFreeTier `json:"free_tier"`
	AcceptingNew        bool                      `json:"accepting_new"`
}

// ObjectStorageBucketConnection holds the S3 endpoint details of a bucket
type ObjectStorageBucketConnection struct {
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	PathStyleURL   string `json:"path_style_url"`
	VirtualHostURL string `json:"virtual_host_url"`
}

// ObjectStorageBucketCDN describes the CDN origin that serves a bucket
type ObjectStorageBucketCDN struct {
	Status        string `json:"status"`
	ZoneUUID      string `json:"zone_uuid"`
	ZoneName      string `json:"zone_name"`
	Domain        string `json:"domain"`
	CustomDomain  string `json:"custom_domain"`
	ZoneStatus    string `json:"zone_status"`
	OriginUUID    string `json:"origin_uuid"`
	OriginEnabled bool   `json:"origin_enabled"`
}

// ObjectStorageBucket represents a bucket (list and detail)
type ObjectStorageBucket struct {
	UUID          string                   `json:"uuid"`
	Name          string                   `json:"name"`
	Status        string                   `json:"status"`
	SuspendReason *string                  `json:"suspend_reason"`
	WriteBlocked  bool                     `json:"write_blocked"`
	ErrorMessage  *string                  `json:"error_message"`
	ProjectID     *int                     `json:"project_id"`
	Tier          ObjectStorageTierSummary `json:"tier"`
	LocationName  string                   `json:"location_name"`
	Region        string                   `json:"region"`
	Endpoint      string                   `json:"endpoint"`
	Versioning    string                   `json:"versioning"`
	Protected     bool                     `json:"protected"`
	SizeBytes     int64                    `json:"size_bytes"`
	ObjectsCount  int64                    `json:"objects_count"`
	CDNConnected  bool                     `json:"cdn_connected"`
	Tags          map[string]string        `json:"tags"`
	ObjectLock    ObjectStorageObjectLock  `json:"object_lock"`
	// LockedContentKept is true when the last delete left versions protected by Object Lock
	LockedContentKept bool                           `json:"locked_content_kept"`
	Connection        *ObjectStorageBucketConnection `json:"connection"`
	CDN               *ObjectStorageBucketCDN        `json:"cdn"`
	// Encryption is the encryption at rest of the bucket; nil until the bucket default is applied
	Encryption *ObjectStorageBucketEncryption `json:"encryption"`
}

// ObjectStorageBucketEncryption is the encryption at rest of a bucket: Algorithm AES256 (SSE-S3)
// and Scope all_objects, or new_objects while objects written before the bucket default may
// still be stored unencrypted
type ObjectStorageBucketEncryption struct {
	Algorithm string `json:"algorithm"`
	Scope     string `json:"scope"`
}

// ObjectStorageLockRetention is a default retention rule: a mode (governance or compliance)
// and exactly one of Days or Years
type ObjectStorageLockRetention struct {
	Mode  string `json:"mode"`
	Days  *int   `json:"days,omitempty"`
	Years *int   `json:"years,omitempty"`
}

// ObjectStorageObjectLock is the Object Lock state of a bucket
type ObjectStorageObjectLock struct {
	Enabled          bool                        `json:"enabled"`
	DefaultRetention *ObjectStorageLockRetention `json:"default_retention"`
}

// CreateObjectStorageBucketRequest is the body of POST /object-storage/buckets.
// With ObjectLock the API requires versioning, so Versioning must be true then.
type CreateObjectStorageBucketRequest struct {
	Name                  string                      `json:"name"`
	Tier                  string                      `json:"tier"`
	ProjectID             *int                        `json:"project_id,omitempty"`
	Versioning            bool                        `json:"versioning"`
	Tags                  map[string]string           `json:"tags,omitempty"`
	ObjectLock            bool                        `json:"object_lock,omitempty"`
	ObjectLockDefault     *ObjectStorageLockRetention `json:"object_lock_default,omitempty"`
	AcceptObjectLockTerms bool                        `json:"accept_object_lock_terms,omitempty"`
}

// SetObjectStorageObjectLockRequest is the body of PUT /object-storage/buckets/{uuid}/object-lock.
// A nil DefaultRetention removes the default retention (sent as null).
type SetObjectStorageObjectLockRequest struct {
	DefaultRetention      *ObjectStorageLockRetention `json:"default_retention"`
	AcceptObjectLockTerms bool                        `json:"accept_object_lock_terms"`
}

// UpdateObjectStorageBucketRequest is the body of PATCH /object-storage/buckets/{uuid}
// Tags replaces every tag of the bucket when not nil (a pointer to an empty map removes them).
type UpdateObjectStorageBucketRequest struct {
	Versioning *string            `json:"versioning,omitempty"`
	Protected  *bool              `json:"protected,omitempty"`
	Tags       *map[string]string `json:"tags,omitempty"`
}

// ObjectStorageBucketRef is a bucket reference inside an access key's scope
type ObjectStorageBucketRef struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// ObjectStorageAccessKey represents an access key (never carries the secret, except on create)
type ObjectStorageAccessKey struct {
	UUID            string                   `json:"uuid"`
	Name            string                   `json:"name"`
	AccessKeyID     string                   `json:"access_key_id"`
	SecretAccessKey string                   `json:"secret_access_key,omitempty"`
	Permission      string                   `json:"permission"`
	BucketScope     []ObjectStorageBucketRef `json:"bucket_scope"`
	ProjectID       *int                     `json:"project_id"`
	Tier            ObjectStorageTierSummary `json:"tier"`
	Region          string                   `json:"region"`
	Endpoint        string                   `json:"endpoint"`
	Status          string                   `json:"status"`
	ExpiresAt       *string                  `json:"expires_at"`
	// BypassGovernance: read_write keys that may delete versions under governance retention
	BypassGovernance bool `json:"bypass_governance"`
}

// CreateObjectStorageKeyRequest is the body of POST /object-storage/keys
type CreateObjectStorageKeyRequest struct {
	Name        string   `json:"name"`
	Tier        string   `json:"tier"`
	ProjectID   *int     `json:"project_id,omitempty"`
	Permission  string   `json:"permission"`
	BucketUUIDs []string `json:"bucket_uuids,omitempty"`
	ExpiresAt   *string  `json:"expires_at,omitempty"`
	// BypassGovernance is only accepted for read_write keys
	BypassGovernance bool `json:"bypass_governance,omitempty"`
}

// ObjectStorageFreeTierUse is the included and used amount of one free tier allowance
type ObjectStorageFreeTierUse struct {
	Included float64  `json:"included"`
	Used     *float64 `json:"used"`
}

// ObjectStorageUsageTier is the month usage of one tier
type ObjectStorageUsageTier struct {
	Tier              ObjectStorageTierSummary            `json:"tier"`
	StorageGiBMonth   *float64                            `json:"storage_gib_month"`
	EgressBytes       *int64                              `json:"egress_bytes"`
	CDNBytes          *int64                              `json:"cdn_bytes"`
	ClassARequests    *int64                              `json:"class_a_requests"`
	ClassBRequests    *int64                              `json:"class_b_requests"`
	ClassBCDNRequests *int64                              `json:"class_b_cdn_requests"`
	Cost              float64                             `json:"cost"`
	ProjectedCost     float64                             `json:"projected_cost"`
	FreeTier          map[string]ObjectStorageFreeTierUse `json:"free_tier"`
}

// ObjectStorageUsageBucket is the month usage of one bucket
type ObjectStorageUsageBucket struct {
	UUID              string   `json:"uuid"`
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	ProjectID         *int     `json:"project_id"`
	TierUUID          string   `json:"tier_uuid"`
	StorageGiBMonth   *float64 `json:"storage_gib_month"`
	EgressBytes       *int64   `json:"egress_bytes"`
	CDNBytes          *int64   `json:"cdn_bytes"`
	ClassARequests    *int64   `json:"class_a_requests"`
	ClassBRequests    *int64   `json:"class_b_requests"`
	ClassBCDNRequests *int64   `json:"class_b_cdn_requests"`
	Cost              float64  `json:"cost"`
}

// ObjectStorageUsage is the response of GET /object-storage/usage
type ObjectStorageUsage struct {
	Period           string                     `json:"period"`
	Since            string                     `json:"since"`
	Until            string                     `json:"until"`
	MetricsAvailable bool                       `json:"metrics_available"`
	TotalCost        float64                    `json:"total_cost"`
	ProjectedCost    float64                    `json:"projected_cost"`
	Tiers            []ObjectStorageUsageTier   `json:"tiers"`
	Buckets          []ObjectStorageUsageBucket `json:"buckets"`
}

// ObjectStorageLifecycleTag is one tag of a lifecycle rule filter
type ObjectStorageLifecycleTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ObjectStorageLifecycleFilter limits a rule to part of the bucket (nil = the whole bucket)
type ObjectStorageLifecycleFilter struct {
	Prefix                *string                     `json:"prefix,omitempty"`
	Tags                  []ObjectStorageLifecycleTag `json:"tags,omitempty"`
	ObjectSizeGreaterThan *int64                      `json:"object_size_greater_than,omitempty"`
	ObjectSizeLessThan    *int64                      `json:"object_size_less_than,omitempty"`
}

// ObjectStorageLifecycleExpiration deletes current objects after Days, on Date, or removes
// orphan delete markers
type ObjectStorageLifecycleExpiration struct {
	Days                      *int64  `json:"days,omitempty"`
	Date                      *string `json:"date,omitempty"`
	ExpiredObjectDeleteMarker *bool   `json:"expired_object_delete_marker,omitempty"`
}

// ObjectStorageLifecycleNoncurrentExpiration deletes noncurrent versions
type ObjectStorageLifecycleNoncurrentExpiration struct {
	NoncurrentDays          int64  `json:"noncurrent_days"`
	NewerNoncurrentVersions *int64 `json:"newer_noncurrent_versions,omitempty"`
}

// ObjectStorageLifecycleAbortUpload aborts incomplete multipart uploads
type ObjectStorageLifecycleAbortUpload struct {
	DaysAfterInitiation int64 `json:"days_after_initiation"`
}

// ObjectStorageLifecycleRule is one lifecycle rule of a bucket
type ObjectStorageLifecycleRule struct {
	ID                             string                                      `json:"id"`
	Enabled                        bool                                        `json:"enabled"`
	Filter                         *ObjectStorageLifecycleFilter               `json:"filter,omitempty"`
	Expiration                     *ObjectStorageLifecycleExpiration           `json:"expiration,omitempty"`
	NoncurrentVersionExpiration    *ObjectStorageLifecycleNoncurrentExpiration `json:"noncurrent_version_expiration,omitempty"`
	AbortIncompleteMultipartUpload *ObjectStorageLifecycleAbortUpload          `json:"abort_incomplete_multipart_upload,omitempty"`
}

// ObjectStorageLifecycle is the answer of GET /object-storage/buckets/{uuid}/lifecycle
type ObjectStorageLifecycle struct {
	BucketUUID        string                       `json:"bucket_uuid"`
	Status            string                       `json:"status"`
	Rules             []ObjectStorageLifecycleRule `json:"rules"`
	Generation        int64                        `json:"generation"`
	AppliedGeneration int64                        `json:"applied_generation"`
	Error             *string                      `json:"error"`
	Notes             []string                     `json:"notes"`
}

// ObjectStorageLifecycleChange is the answer of PUT and DELETE of the lifecycle; Generation is
// nil when nothing changed
type ObjectStorageLifecycleChange struct {
	Detail     string `json:"detail"`
	Generation *int64 `json:"generation"`
}

// ObjectStorageReplicationSource is the source bucket of a replication. For an incoming replication
// of another organization only the names are returned.
type ObjectStorageReplicationSource struct {
	BucketUUID       *string `json:"bucket_uuid"`
	BucketName       *string `json:"bucket_name"`
	ProjectID        *int    `json:"project_id"`
	OrganizationName *string `json:"organization_name"`
	SameOrganization bool    `json:"same_organization"`
}

// ObjectStorageReplicationDestination is the destination of a replication: a CubePath bucket
// (Type "cubepath") or an external S3 compatible bucket (Type "external"). The access key id of an
// external destination is masked and its secret is never returned.
type ObjectStorageReplicationDestination struct {
	Type string `json:"type"`
	// cubepath
	BucketUUID       *string `json:"bucket_uuid"`
	BucketName       *string `json:"bucket_name"`
	ProjectID        *int    `json:"project_id"`
	OrganizationName *string `json:"organization_name"`
	SameOrganization bool    `json:"same_organization"`
	// external
	Provider    *string `json:"provider"`
	Endpoint    *string `json:"endpoint"`
	Region      *string `json:"region"`
	Bucket      *string `json:"bucket"`
	PathStyle   *string `json:"path_style"`
	AccessKeyID *string `json:"access_key_id"`
}

// ObjectStorageReplicationRules are the rules of a replication
type ObjectStorageReplicationRules struct {
	Enabled                 bool                        `json:"enabled"`
	Prefix                  *string                     `json:"prefix"`
	Tags                    []ObjectStorageLifecycleTag `json:"tags"`
	DeleteMarkerReplication bool                        `json:"delete_marker_replication"`
	DeleteReplication       bool                        `json:"delete_replication"`
	ExistingObjects         bool                        `json:"existing_objects"`
}

// ObjectStorageReplicationBackfill is the copy of the objects the source already held
type ObjectStorageReplicationBackfill struct {
	Status        string  `json:"status"`
	StartedAt     *string `json:"started_at"`
	FinishedAt    *string `json:"finished_at"`
	Objects       int64   `json:"objects"`
	Bytes         int64   `json:"bytes"`
	FailedObjects int64   `json:"failed_objects"`
}

// ObjectStorageReplication is a replication of a source bucket to one destination
type ObjectStorageReplication struct {
	UUID            string                              `json:"uuid"`
	Status          string                              `json:"status"`
	PauseReason     *string                             `json:"pause_reason"`
	Direction       string                              `json:"direction"`
	Source          ObjectStorageReplicationSource      `json:"source"`
	Destination     ObjectStorageReplicationDestination `json:"destination"`
	Rules           ObjectStorageReplicationRules       `json:"rules"`
	Health          string                              `json:"health"`
	HealthReason    *string                             `json:"health_reason"`
	HealthCheckedAt *string                             `json:"health_checked_at"`
	Backfill        ObjectStorageReplicationBackfill    `json:"backfill"`
	ErrorMessage    *string                             `json:"error_message"`
	CreatedAt       *string                             `json:"created_at"`
	ActiveAt        *string                             `json:"active_at"`
}

// CreateObjectStorageReplicationDestination is the destination of POST /object-storage/replications.
// Only the fields of the chosen type may be sent: the API refuses the others.
type CreateObjectStorageReplicationDestination struct {
	Type            string `json:"type"`
	BucketUUID      string `json:"bucket_uuid,omitempty"`
	GrantToken      string `json:"grant_token,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	Region          string `json:"region,omitempty"`
	Bucket          string `json:"bucket,omitempty"`
	PathStyle       string `json:"path_style,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
}

// CreateObjectStorageReplicationRequest is the body of POST /object-storage/replications
type CreateObjectStorageReplicationRequest struct {
	SourceBucketUUID        string                                    `json:"source_bucket_uuid"`
	Destination             CreateObjectStorageReplicationDestination `json:"destination"`
	Prefix                  *string                                   `json:"prefix,omitempty"`
	Tags                    []ObjectStorageLifecycleTag               `json:"tags,omitempty"`
	DeleteMarkerReplication bool                                      `json:"delete_marker_replication"`
	DeleteReplication       bool                                      `json:"delete_replication"`
	ExistingObjects         bool                                      `json:"existing_objects"`
}

// ObjectStorageReplicationCreated is the answer of POST /object-storage/replications
type ObjectStorageReplicationCreated struct {
	Detail string `json:"detail"`
	UUID   string `json:"uuid"`
	Status string `json:"status"`
}

// ObjectStorageReplicationGrant is a one use authorization for another organization to replicate
// into a bucket. Token is only set in the answer of the create.
type ObjectStorageReplicationGrant struct {
	UUID        string  `json:"uuid"`
	Token       string  `json:"token,omitempty"`
	TokenPrefix string  `json:"token_prefix"`
	BucketUUID  string  `json:"bucket_uuid,omitempty"`
	Note        *string `json:"note"`
	Status      string  `json:"status,omitempty"`
	ExpiresAt   *string `json:"expires_at"`
	UsedAt      *string `json:"used_at"`
	RevokedAt   *string `json:"revoked_at"`
	CreatedAt   *string `json:"created_at"`
}

// CreateObjectStorageReplicationGrantRequest is the body of POST /object-storage/buckets/{uuid}/replication-grants
type CreateObjectStorageReplicationGrantRequest struct {
	Note          *string `json:"note,omitempty"`
	ExpiresInDays int     `json:"expires_in_days,omitempty"`
}
