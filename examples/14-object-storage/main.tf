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

# Off site copy: replicate the bucket (versioning enabled) to an external S3 compatible
# bucket over HTTPS. Billed as egress of the source bucket. A CubePath destination
# (type = "cubepath", bucket_uuid) is on the same storage cluster, so it is not a
# disaster recovery copy. Buckets with Object Lock cannot be sources.
variable "offsite_access_key_id" {
  type = string
}

variable "offsite_secret_access_key" {
  type      = string
  sensitive = true # kept in the state, marked sensitive
}

resource "cubepath_object_storage_replication" "assets_offsite" {
  source_bucket_uuid = cubepath_object_storage_bucket.assets.id

  destination {
    type              = "external"
    provider          = "wasabi"
    endpoint          = "s3.eu-central-1.wasabisys.com"
    region            = "eu-central-1"
    bucket            = "my-company-assets-offsite"
    access_key_id     = var.offsite_access_key_id
    secret_access_key = var.offsite_secret_access_key
  }
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
