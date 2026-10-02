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
