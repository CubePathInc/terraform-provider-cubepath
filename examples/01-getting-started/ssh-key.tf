# Example: Create an SSH key

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Create an SSH key for secure server access
resource "cubepath_ssh_key" "example" {
  name       = "my-ssh-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Outputs
output "ssh_key_id" {
  description = "SSH key ID"
  value       = cubepath_ssh_key.example.id
}

output "ssh_key_fingerprint" {
  description = "SSH key fingerprint"
  value       = cubepath_ssh_key.example.fingerprint
}

output "ssh_key_type" {
  description = "SSH key type"
  value       = cubepath_ssh_key.example.key_type
}
