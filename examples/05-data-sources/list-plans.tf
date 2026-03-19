# Example: List VPS plans for a location

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Get VPS plans for Miami
data "cubepath_vps_plans" "miami" {
  location = "us-mia-1"
}

# Get VPS plans for Houston
data "cubepath_vps_plans" "houston" {
  location = "us-hou-1"
}

# Output Miami plans with pricing
output "miami_plans" {
  description = "VPS plans available in Miami"
  value = [
    for plan in data.cubepath_vps_plans.miami.plans :
    {
      name           = plan.plan_name
      vcpus          = plan.cpu
      ram_gb         = plan.ram / 1024
      storage_gb     = plan.storage
      bandwidth_gb   = plan.bandwidth
      price_per_hour = plan.price_per_hour
      price_per_month = plan.price_per_hour * 24 * 30
    }
  ]
}

# Output Houston plans
output "houston_plans" {
  description = "VPS plans available in Houston"
  value = [
    for plan in data.cubepath_vps_plans.houston.plans :
    format("%s: %d vCPUs, %dGB RAM - $%.4f/h ($%.2f/mo)",
      plan.plan_name,
      plan.cpu,
      plan.ram / 1024,
      plan.price_per_hour,
      plan.price_per_hour * 24 * 30
    )
  ]
}

# Find cheapest plan in Miami
output "cheapest_miami_plan" {
  value = [
    for plan in data.cubepath_vps_plans.miami.plans :
    plan if plan.price_per_hour == min(data.cubepath_vps_plans.miami.plans[*].price_per_hour)...
  ][0]
}
