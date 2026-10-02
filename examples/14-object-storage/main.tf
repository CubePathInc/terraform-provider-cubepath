# Example: Object Storage bucket, access key and CDN delivery

terraform {
  required_providers {
    cubepath = {
      source  = "cubepathinc/cubepath"
      version = "~> 0.7"
    }
  }
}

provider "cubepath" {}

# Tiers with their endpoint, prices and free tier
data "cubepath_object_storage_tiers" "all" {}

# A private bucket. Names are unique across all CubePath customers.
resource "cubepath_object_storage_bucket" "assets" {
  name       = "my-company-assets"
  tier       = "infrequent_access"
  versioning = "enabled"
  protected  = true # set to false before destroying it

  # Labels for organizing and filtering buckets, changed in place. They are not
  # visible through S3 (aws_s3_bucket_tagging gets 403); object tags are, for
  # example aws_s3_object with tags.
  tags = {
    env  = "prod"
    team = "web"
  }
}

# Lifecycle rules of the bucket (every rule of the bucket is managed by this one resource).
# Deletions are permanent; objects go within 48 hours of their due date.
resource "cubepath_object_storage_bucket_lifecycle" "assets" {
  bucket_uuid = cubepath_object_storage_bucket.assets.id

  rule {
    id              = "tmp-7d"
    prefix          = "tmp/"
    expiration_days = 7
  }

  # The bucket is versioned: an expiration only adds a delete marker, this frees the space
  rule {
    id                        = "old-versions"
    noncurrent_days           = 30
    newer_noncurrent_versions = 3
  }

  rule {
    id                                     = "failed-uploads"
    abort_incomplete_multipart_upload_days = 2
  }
}

# A read only key limited to that bucket, for an application or a backup job
resource "cubepath_object_storage_access_key" "app" {
  name       = "app-read-only"
  tier       = "infrequent_access"
  permission = "read_only"
  buckets    = [cubepath_object_storage_bucket.assets.id]
}

# Immutable backups: a bucket with Object Lock (only possible at creation, needs
# versioning). Every new version is kept for 30 days in governance mode.
resource "cubepath_object_storage_bucket" "backups" {
  name                     = "my-company-backups"
  tier                     = "infrequent_access"
  versioning               = "enabled"
  object_lock_enabled      = true
  accept_object_lock_terms = true
  # Created protected; set protected = false before destroying it. Versions still
  # under retention are kept (locked_content_kept) and keep being billed.

  object_lock_default_retention = {
    mode = "governance" # or "compliance" if support enabled it for your organization
    days = 30
  }
}

# Key for the backup tool (Veeam, Kopia). bypass_governance lets it remove
# governance versions with the x-amz-bypass-governance-retention header.
resource "cubepath_object_storage_access_key" "backups" {
  name              = "backup-tool"
  tier              = "infrequent_access"
  buckets           = [cubepath_object_storage_bucket.backups.id]
  bypass_governance = true
}

# Buckets are never public: serve one through the CDN by adding it as an origin
resource "cubepath_cdn_zone" "assets" {
  name      = "my-company-assets"
  plan_name = "cdn.standard"
}

resource "cubepath_cdn_origin" "assets" {
  zone_uuid                  = cubepath_cdn_zone.assets.id
  name                       = "assets-bucket"
  object_storage_bucket_uuid = cubepath_object_storage_bucket.assets.id
}

output "s3_endpoint" {
  value = cubepath_object_storage_bucket.assets.endpoint
}

output "s3_region" {
  value = cubepath_object_storage_bucket.assets.region
}

output "access_key_id" {
  value = cubepath_object_storage_access_key.app.access_key_id
}

output "secret_access_key" {
  value     = cubepath_object_storage_access_key.app.secret_access_key
  sensitive = true
}

output "cdn_url" {
  value = "https://${cubepath_cdn_zone.assets.domain}/"
}

# Event notifications: every new object under uploads/ is sent to a signed webhook.
resource "cubepath_object_storage_event_destination" "uploads" {
  name = "uploads-hook"
  type = "webhook"
  url  = "https://example.com/hooks/storage"
}

resource "cubepath_object_storage_event_rule" "uploads" {
  bucket_uuid      = cubepath_object_storage_bucket.assets.id
  name             = "new-uploads"
  destination_uuid = cubepath_object_storage_event_destination.uploads.id
  events           = ["object.created"]
  prefix           = "uploads/"
}

# Verify CubePath-Signature in the receiver with this secret (shown only at creation).
output "events_signing_secret" {
  value     = cubepath_object_storage_event_destination.uploads.signing_secret
  sensitive = true
}
