terraform {
  required_providers {
    stalwart = {
      source  = "bilbilak/stalwart"
      version = "~> 1.2"
    }
  }
}

provider "stalwart" {
  endpoint = var.stalwart_endpoint
  token    = var.stalwart_token
}

# ══════════════════════════════════════════════════════════════════════════════
# VARIABLES — fill these in (or pass via .tfvars / env)
# ══════════════════════════════════════════════════════════════════════════════

variable "stalwart_endpoint" {
  description = "Stalwart management API base URL, e.g. https://mail.example.com/api"
  type        = string
}

variable "stalwart_token" {
  description = "Admin bearer token. Alternatively set STALWART_TOKEN env var."
  type        = string
  sensitive   = true
}

variable "domains" {
  description = "Map of domains to register. Key is a local label, value is the domain name."
  type        = map(string)
  # example:
  # domains = {
  #   biz1 = "company1.com"
  #   biz2 = "company2.com"
  # }
}

variable "groups" {
  description = "Mail groups (mailing lists). Key is the internal group name used across the config."
  type = map(object({
    description          = optional(string, "")
    primary_email        = string
    aliases              = optional(list(string), [])
    enabled_permissions  = optional(list(string), ["email-send", "email-receive"])
    disabled_permissions = optional(list(string), [])
    external_members     = optional(list(string), [])
  }))
  # example:
  # groups = {
  #   "info-biz1" = {
  #     primary_email = "info@company1.com"
  #     aliases       = ["hello@company1.com"]
  #   }
  #   "info-biz2" = {
  #     primary_email = "info@company2.com"
  #     aliases       = ["hello@company2.com", "contact@company2.com"]
  #   }
  # }
}

variable "accounts" {
  description = "User accounts to create."
  type = map(object({
    description   = optional(string, "")
    password      = string
    quota         = optional(number, 0)
    primary_email = string
    aliases       = optional(list(string), [])
    member_of     = optional(list(string), [])
    roles         = optional(list(string), ["user"])
  }))
  sensitive = true
  # example:
  # accounts = {
  #   sam = {
  #     password      = "secretPassw0rd"
  #     primary_email = "sam@company1.com"
  #     aliases       = ["sam@company2.com"]
  #     member_of     = ["info-biz1", "info-biz2"]
  #   }
  # }
}

# ══════════════════════════════════════════════════════════════════════════════
# 1. DOMAINS
# ══════════════════════════════════════════════════════════════════════════════

resource "stalwart_domain" "this" {
  for_each = var.domains

  name        = each.value
  description = each.key
}

# ══════════════════════════════════════════════════════════════════════════════
# 2. DKIM + DNS RECORDS
# ══════════════════════════════════════════════════════════════════════════════

resource "stalwart_dkim" "this" {
  for_each = var.domains

  domain    = stalwart_domain.this[each.key].name
  algorithm = "Ed25519"
  selector  = "stalwart"
}

data "stalwart_dns_records" "this" {
  for_each = var.domains

  domain = stalwart_dkim.this[each.key].domain
}

# ══════════════════════════════════════════════════════════════════════════════
# 3. GROUPS  (before users — users reference groups via member_of)
# ══════════════════════════════════════════════════════════════════════════════

resource "stalwart_group" "this" {
  for_each = var.groups

  name                 = each.key
  description          = each.value.description
  primary_email        = each.value.primary_email
  aliases              = each.value.aliases
  enabled_permissions  = each.value.enabled_permissions
  disabled_permissions = each.value.disabled_permissions
  external_members     = each.value.external_members

  # primary_email is a plain string from var.groups, not a resource reference,
  # so Terraform can't infer the domain dependency. Explicit dep ensures:
  #   create: domain before group
  #   destroy: group before domain
  #   ForceNew on domain: group destroyed before domain, recreated after
  depends_on = [stalwart_domain.this]
}

# ══════════════════════════════════════════════════════════════════════════════
# 4. ACCOUNTS
# ══════════════════════════════════════════════════════════════════════════════

resource "stalwart_account" "this" {
  for_each = var.accounts

  name          = each.key
  description   = each.value.description
  password      = each.value.password
  quota         = each.value.quota
  primary_email = each.value.primary_email
  aliases       = each.value.aliases
  member_of     = each.value.member_of
  roles         = each.value.roles

  depends_on = [stalwart_group.this]
}

# ══════════════════════════════════════════════════════════════════════════════
# OUTPUTS — DNS records to publish per domain
# ══════════════════════════════════════════════════════════════════════════════

output "dns_records" {
  description = "DNS records to publish at your registrar for each domain."
  value = {
    for k, v in data.stalwart_dns_records.this : var.domains[k] => v.records
  }
}
