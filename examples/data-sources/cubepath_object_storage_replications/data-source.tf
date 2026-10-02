# Replications from your buckets and into them from other organizations
data "cubepath_object_storage_replications" "all" {}

# Only the replications that write into one bucket
data "cubepath_object_storage_replications" "incoming" {
  direction   = "incoming"
  bucket_uuid = "00000000-0000-0000-0000-000000000000"
}

output "failing_replications" {
  value = [for r in data.cubepath_object_storage_replications.all.replications : r.uuid if r.health == "failing"]
}
