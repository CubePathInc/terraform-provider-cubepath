# Both buckets need versioning. A bucket with Object Lock cannot be a source.
resource "cubepath_object_storage_bucket" "photos" {
  name       = "acme-photos"
  tier       = "infrequent_access"
  versioning = "enabled"
}

# Off site copy in an external S3 compatible bucket (HTTPS on port 443 only).
# Billed as egress of the source bucket. The secret is kept in the state, marked sensitive.
resource "cubepath_object_storage_replication" "offsite" {
  source_bucket_uuid = cubepath_object_storage_bucket.photos.id

  destination {
    type              = "external"
    provider          = "aws"
    endpoint          = "s3.eu-west-1.amazonaws.com"
    region            = "eu-west-1"
    bucket            = "acme-photos-backup"
    access_key_id     = var.aws_access_key_id
    secret_access_key = var.aws_secret_access_key
    # Bump to send the credentials again without changing them
    secret_access_key_version = 1
  }

  rule {
    prefix                    = "img/"
    delete_marker_replication = false
    delete_replication        = false
    existing_objects          = true
  }

  enabled = true
}

# Copy into another CubePath bucket. It lives on the same storage cluster as the
# source, so it protects against mistakes, not against the loss of the site.
resource "cubepath_object_storage_bucket" "photos_copy" {
  name       = "acme-photos-copy"
  tier       = "infrequent_access"
  versioning = "enabled"
}

resource "cubepath_object_storage_bucket" "logs" {
  name       = "acme-logs"
  tier       = "infrequent_access"
  versioning = "enabled"
}

resource "cubepath_object_storage_replication" "logs_copy" {
  source_bucket_uuid = cubepath_object_storage_bucket.logs.id

  destination {
    type        = "cubepath"
    bucket_uuid = cubepath_object_storage_bucket.photos_copy.id
    # grant_token = var.partner_grant_token # only for a bucket of another organization
  }
}

variable "aws_access_key_id" {
  type = string
}

variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}
