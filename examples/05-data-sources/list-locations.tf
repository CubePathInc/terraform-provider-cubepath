# Example: List all available locations

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Get all available locations
data "cubepath_locations" "all" {}

# Output locations
output "locations" {
  description = "All available CubePath locations"
  value = {
    for loc in data.cubepath_locations.all.locations :
    loc.location_name => loc.description
  }
}

output "location_count" {
  description = "Total number of available locations"
  value       = length(data.cubepath_locations.all.locations)
}
