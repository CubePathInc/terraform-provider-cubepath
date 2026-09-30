# Example: Cloud Alerts on a VPS, sent to email and Slack

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

variable "vps_id" {
  type = string
}

variable "slack_webhook_url" {
  type      = string
  sensitive = true
}

# Email channels send to the email of the account that creates them
resource "cubepath_alert_channel" "email" {
  name = "ops-email"
  type = "email"
}

resource "cubepath_alert_channel" "slack" {
  name        = "ops-slack"
  type        = "slack"
  webhook_url = var.slack_webhook_url
}

resource "cubepath_alert_rule" "cpu" {
  project_id       = var.project_id
  name             = "web cpu high"
  target_type      = "vps"
  target_id        = var.vps_id
  metric_type      = "cpu"
  operator         = "gt"
  threshold        = 85
  duration_seconds = 300
  channel_ids      = [cubepath_alert_channel.email.id, cubepath_alert_channel.slack.id]
}

resource "cubepath_alert_rule" "disk" {
  project_id  = var.project_id
  name        = "web disk almost full"
  target_type = "vps"
  target_id   = var.vps_id
  metric_type = "disk"
  operator    = "gte"
  threshold   = 90
  channel_ids = [cubepath_alert_channel.email.id]
}
