# Example: CDN zone with origin and rules

terraform {
  required_providers {
    cubepath = {
      source  = "cubepathinc/cubepath"
      version = "~> 0.6"
    }
  }
}

provider "cubepath" {}

# Create a CDN zone
resource "cubepath_cdn_zone" "main" {
  name          = "my-site"
  plan_name     = "cdn.starter"
  custom_domain = "cdn.example.com"

  # Compress text responses (off by default) and purge by tag from the Cache-Tag header of the origin
  compression_enabled = true
  cache_tag_header    = "Cache-Tag"

  # Managed WAF in detection mode: it logs what it finds and blocks nothing.
  managed_waf = {
    mode  = "log"
    level = 1
  }
}

# Add an origin server
resource "cubepath_cdn_origin" "primary" {
  zone_uuid          = cubepath_cdn_zone.main.id
  name               = "primary-origin"
  address            = "origin.example.com"
  port               = 443
  protocol           = "https"
  weight             = 100
  priority           = 1

  # Opt in to an http check; the default is a tcp check
  health_check_type            = "http"
  health_check_path            = "/ping"
  health_check_expected_status = "200"
}

# Add a backup origin
resource "cubepath_cdn_origin" "backup" {
  zone_uuid          = cubepath_cdn_zone.main.id
  name               = "backup-origin"
  address            = "backup.example.com"
  port               = 443
  protocol           = "https"
  is_backup          = true
  health_check_path  = "/health"
}

# Cache rule for static assets
resource "cubepath_cdn_rule" "cache_static" {
  zone_uuid   = cubepath_cdn_zone.main.id
  name        = "cache-static-assets"
  rule_type   = "cache"
  priority    = 10
  action_config    = jsonencode({ ttl = 86400, browser_ttl = 3600 })
  match_conditions = jsonencode({ path_pattern = "\\.(css|js|png|jpg|gif|svg|woff2?)$" })
}

# Cache key: ignore every query parameter. Changing a cache key rule empties the zone cache.
resource "cubepath_cdn_rule" "cache_key" {
  zone_uuid     = cubepath_cdn_zone.main.id
  name          = "ignore-query"
  rule_type     = "cache_key"
  priority      = 20
  action_config = jsonencode({ query = "none" })
}

# Rewrite /old/ to /new/ for the origin; the cache uses the rewritten URL
resource "cubepath_cdn_rule" "rewrite" {
  zone_uuid     = cubepath_cdn_zone.main.id
  name          = "old-to-new"
  rule_type     = "url_rewrite"
  priority      = 30
  action_config = jsonencode({ path = { mode = "prefix", from = "/old/", to = "/new/" } })
}

# Rate limit WAF rule
resource "cubepath_cdn_waf_rule" "rate_limit" {
  zone_uuid   = cubepath_cdn_zone.main.id
  name        = "api-rate-limit"
  rule_type   = "rate_limit"
  priority    = 100
  action_config    = jsonencode({ requests = 100, period = 60, by = "ip" })
  match_conditions = jsonencode({ path_pattern = "^/api/" })
}

# Country blocking WAF rule
resource "cubepath_cdn_waf_rule" "country_block" {
  zone_uuid   = cubepath_cdn_zone.main.id
  name        = "block-countries"
  rule_type   = "firewall_country"
  priority    = 50
  action_config = jsonencode({ block_countries = ["CN", "RU"] })
}

output "cdn_domain" {
  value = cubepath_cdn_zone.main.domain
}
