resource "cubepath_alert_channel" "email" {
  name = "ops-email"
  type = "email"
}

# CPU of a VPS above 85 % for 5 minutes
resource "cubepath_alert_rule" "cpu" {
  project_id       = var.project_id
  name             = "web cpu high"
  target_type      = "vps"
  target_id        = var.vps_id
  metric_type      = "cpu"
  operator         = "gt"
  threshold        = 85
  duration_seconds = 300
  channel_ids      = [cubepath_alert_channel.email.id]
}

# Monthly Object Storage budget: notifies once when the organization has been
# billed 50 USD this month, and resets on the 1st (UTC). The rule is listed
# under (and deleted with) the given project.
resource "cubepath_alert_rule" "storage_budget" {
  project_id  = var.project_id
  name        = "object storage budget"
  target_type = "organization"
  target_id   = var.organization_id
  metric_type = "storage_cost_month"
  operator    = "gte"
  threshold   = 50
  channel_ids = [cubepath_alert_channel.email.id]
}

# Bucket size above 500 GiB. The rule must live in the bucket's project.
resource "cubepath_alert_rule" "bucket_size" {
  project_id  = cubepath_object_storage_bucket.assets.project_id
  name        = "assets bucket size"
  target_type = "object_storage_bucket"
  target_id   = cubepath_object_storage_bucket.assets.id
  metric_type = "storage_size_gb"
  operator    = "gt"
  threshold   = 500
  channel_ids = [cubepath_alert_channel.email.id]
}
