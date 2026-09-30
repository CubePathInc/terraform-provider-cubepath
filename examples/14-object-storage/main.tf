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
