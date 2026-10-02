# Let another organization replicate into this bucket. The token is used once.
resource "cubepath_object_storage_bucket" "inbox" {
  name       = "acme-partner-inbox"
  tier       = "infrequent_access"
  versioning = "enabled"
}

resource "cubepath_object_storage_replication_grant" "partner" {
  bucket_uuid     = cubepath_object_storage_bucket.inbox.id
  note            = "for Partner Inc"
  expires_in_days = 7
}

# Give this token to the owner of the source bucket
output "grant_token" {
  value     = cubepath_object_storage_replication_grant.partner.token
  sensitive = true
}
