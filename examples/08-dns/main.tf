# Example: DNS zone and record management

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Create a DNS zone
resource "cubepath_dns_zone" "main" {
  domain = "example.com"
}

# A record pointing to a VPS
resource "cubepath_dns_record" "root" {
  zone_uuid = cubepath_dns_zone.main.id
  name      = "@"
  type      = "A"
  content   = "203.0.113.10"
  ttl       = 3600
}

# CNAME for www
resource "cubepath_dns_record" "www" {
  zone_uuid = cubepath_dns_zone.main.id
  name      = "www"
  type      = "CNAME"
  content   = "example.com"
  ttl       = 3600
}

# MX record for email
resource "cubepath_dns_record" "mx" {
  zone_uuid = cubepath_dns_zone.main.id
  name      = "@"
  type      = "MX"
  content   = "mail.example.com"
  ttl       = 3600
  priority  = 10
}

# TXT record for SPF
resource "cubepath_dns_record" "spf" {
  zone_uuid = cubepath_dns_zone.main.id
  name      = "@"
  type      = "TXT"
  content   = "v=spf1 include:_spf.example.com ~all"
  ttl       = 3600
}

output "nameservers" {
  value = cubepath_dns_zone.main.nameservers
}
