# Example: manage a VPS in place: protection, backups, private network, BGP and load balancing

terraform {
  required_providers {
    cubepath = {
      source  = "cubepathinc/cubepath"
      version = "~> 0.8"
    }
  }
}

provider "cubepath" {}

variable "project_id" {
  type = number
}

variable "ssh_key_id" {
  type = number
}

resource "cubepath_network" "private" {
  name       = "backend"
  project_id = var.project_id
  location   = "eu-bcn-1"
  ip_range   = "10.20.0.0"
  prefix     = 24
}

resource "cubepath_availability_group" "web" {
  project_id    = var.project_id
  name          = "web"
  location_name = "eu-bcn-1"
}

resource "cubepath_vps" "router" {
  name          = "router"
  project_id    = var.project_id # changing it moves the VPS, it is not recreated
  location      = "eu-bcn-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_ids   = [var.ssh_key_id]
  network_id    = cubepath_network.private.id # can also be changed in place

  availability_group_uuid = cubepath_availability_group.web.id
  protected               = true # set to false before destroying it

  enable_backups        = true
  backup_schedule_hour  = 4
  backup_retention_days = 7
  backup_max_backups    = 7
}

# Dynamic routes: the VPS announces its prefixes over eBGP to the network gateway
resource "cubepath_network_bgp_peer" "router" {
  network_id  = cubepath_network.private.id
  peer_type   = "vps"
  peer_target = cubepath_vps.router.id
  remote_asn  = 65010
  description = "router on the backend network"
}

resource "cubepath_loadbalancer" "web" {
  name          = "web"
  plan_name     = "lb.small"
  location_name = "eu-bcn-1"
  project_id    = var.project_id
  network_id    = cubepath_network.private.id
  protected     = true
}

resource "cubepath_lb_listener" "http" {
  loadbalancer_id = cubepath_loadbalancer.web.id
  name            = "http"
  protocol        = "http"
  source_port     = 80
  target_port     = 8080
  algorithm       = "round_robin"
}

# Send traffic to every VPS of the availability group
resource "cubepath_lb_target" "web" {
  loadbalancer_id = cubepath_loadbalancer.web.id
  listener_id     = cubepath_lb_listener.http.id
  target_type     = "availability_group"
  target_id       = cubepath_availability_group.web.id
}

resource "cubepath_lb_health_check" "http" {
  loadbalancer_id = cubepath_loadbalancer.web.id
  listener_id     = cubepath_lb_listener.http.id
  path            = "/healthz"
  expected_codes  = "200"
}
