# Example: Using the new data sources

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# List all DNS zones
data "cubepath_dns_zones" "all" {}

output "dns_zones" {
  value = data.cubepath_dns_zones.all.zones
}

# List available load balancer plans
data "cubepath_lb_plans" "all" {}

output "lb_plans" {
  value = data.cubepath_lb_plans.all.locations
}

# List available CDN plans
data "cubepath_cdn_plans" "all" {}

output "cdn_plans" {
  value = data.cubepath_cdn_plans.all.plans
}
