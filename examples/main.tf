# Example Terraform configuration for Stalwart Mail Server Provider

terraform {
  required_providers {
    stalwart = {
      source  = "bilbilak/stalwart"
      version = "~> 0.1"
    }
  }
}

provider "stalwart" {
  server_hostname = var.server_hostname
  api_key         = var.api_key
}

# Create a domain
resource "stalwart_domain" "example" {
  name        = "example.com"
  description = "Example Domain"
}

# Create email groups
resource "stalwart_group" "support" {
  name        = "support_team"
  description = "Customer Support Team"
  email       = "support@example.com"

  aliases = [
    "help@example.com",
    "contact@example.com"
  ]
}

resource "stalwart_group" "sales" {
  name        = "sales_team"
  description = "Sales Team"
  email       = "sales@example.com"

  aliases = [
    "info@example.com"
  ]
}

resource "stalwart_group" "admins" {
  name        = "administrators"
  description = "System Administrators"
  email       = "admin@example.com"
}

# Create email accounts
resource "stalwart_account" "john" {
  name        = "john"
  description = "John Doe"
  email       = "john@example.com"

  aliases = [
    "j.doe@example.com"
  ]

  groups  = [stalwart_group.support.name]
  roles   = ["user"]
  secrets = [var.john_password]
}

resource "stalwart_account" "jane" {
  name        = "jane"
  description = "Jane Smith"
  email       = "jane@example.com"

  aliases = [
    "j.smith@example.com"
  ]

  groups  = [stalwart_group.sales.name]
  roles   = ["user"]
  secrets = [var.jane_password]
}

resource "stalwart_account" "admin" {
  name        = "admin"
  description = "System Administrator"
  email       = "sysadmin@example.com"

  groups  = [stalwart_group.admins.name, stalwart_group.support.name]
  roles   = ["user", "admin"]
  secrets = [var.admin_password]
}

# Outputs
output "domain_id" {
  description = "The principal ID of the domain"
  value       = stalwart_domain.example.id
}

output "sts_policy_id" {
  description = "MTA-STS policy ID for DNS configuration"
  value       = stalwart_domain.example.sts_policy
}

output "dkim_rsa" {
  description = "RSA DKIM key information"
  value = stalwart_domain.example.dkim.rsa != null ? {
    selector   = stalwart_domain.example.dkim.rsa.selector
    public_key = stalwart_domain.example.dkim.rsa.public_key
  } : null
  sensitive = true
}

output "dkim_ed25519" {
  description = "Ed25519 DKIM key information"
  value = stalwart_domain.example.dkim.ed25519 != null ? {
    selector   = stalwart_domain.example.dkim.ed25519.selector
    public_key = stalwart_domain.example.dkim.ed25519.public_key
  } : null
  sensitive = true
}

output "support_group_id" {
  description = "The principal ID of the support group"
  value       = stalwart_group.support.id
}

output "admin_account_id" {
  description = "The principal ID of the admin account"
  value       = stalwart_account.admin.id
}
