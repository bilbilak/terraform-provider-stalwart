<p align="center">
    Terraform Provider for Stalwart Mail Server
</p>

<p align="center">
    <a href="https://registry.terraform.io/providers/bilbilak/stalwart">
        <img src="https://img.shields.io/badge/Terraform-Registry-7b42bc?logo=terraform&style=flat-square" alt="Terraform Registry"></a>
    <a href="https://github.com/bilbilak/terraform-provider-stalwart/releases/latest">
        <img src="https://img.shields.io/github/v/release/bilbilak/terraform-provider-stalwart?include_prereleases&sort=semver&display_name=tag&style=flat-square&color=blue" alt="Latest Release"></a>
    <a href="https://github.com/bilbilak/terraform-provider-stalwart/blob/main/LICENSE.md">
        <img src="https://img.shields.io/badge/license-AGPL--3.0-be0000?style=flat-square" alt="License: AGPLv3"></a>
</p>

## 📖 About

**_Terraform Provider for Stalwart_** manages [Stalwart Mail Server](https://stalw.art) resources — domains, DKIM keys, mail groups, and user accounts — as Terraform infrastructure.

## 📚 Documentation

For detailed information on the available resources and data sources, please refer to the [Terraform Registry documentation](https://registry.terraform.io/providers/bilbilak/stalwart/latest/docs).

### Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) ≥ 1.0
- [Go](https://golang.org/doc/install) ≥ 1.25 (to build from source)
- A running [Stalwart Mail Server](https://stalw.art) instance with the management API accessible

### Quick Start

```hcl
terraform {
  required_providers {
    stalwart = {
      source  = "bilbilak/stalwart"
      version = "~> 1.2"
    }
  }
}

provider "stalwart" {
  endpoint = "https://mail.example.com/api"
  token    = var.stalwart_token   # or: export STALWART_TOKEN="..."
}
```

### Resources & Data Sources

| Type | Name | Description |
|---|---|---|
| Resource | `stalwart_domain` | Register a domain |
| Resource | `stalwart_dkim` | Generate a DKIM signing key |
| Resource | `stalwart_group` | Manage a mailing list / mail group |
| Resource | `stalwart_account` | Manage a user account |
| Data Source | `stalwart_dns_records` | Read DNS records for a domain |
| Data Source | `stalwart_domain` | Read a registered domain |
| Data Source | `stalwart_account` | Read an existing account (no secrets) |
| Data Source | `stalwart_group` | Read an existing group |

### Example — Multi-domain with unified inbox

A common use case: multiple business domains, one login, mail groups per domain.

```hcl
# 1. Domains
resource "stalwart_domain" "this" {
  for_each = var.domains   # { biz1 = "company1.com", biz2 = "company2.com" }
  name     = each.value
}

# 2. DKIM + DNS records
resource "stalwart_dkim" "this" {
  for_each  = var.domains
  domain    = stalwart_domain.this[each.key].name
  algorithm = "Ed25519"
}

data "stalwart_dns_records" "this" {
  for_each = var.domains
  domain   = stalwart_dkim.this[each.key].domain
}

# 3. Mail groups (before users — users declare membership via member_of)
resource "stalwart_group" "this" {
  for_each      = var.groups
  name          = each.key
  primary_email = each.value.primary_email
  aliases       = each.value.aliases

  enabled_permissions = ["email-send", "email-receive"]
  depends_on          = [stalwart_domain.this]
}

# 4. Accounts
resource "stalwart_account" "this" {
  for_each      = var.accounts
  name          = each.key
  password      = each.value.password
  primary_email = each.value.primary_email
  aliases       = each.value.aliases
  member_of     = each.value.member_of
  depends_on    = [stalwart_group.this]
}

output "dns_records" {
  value = {
    for k, v in data.stalwart_dns_records.this : var.domains[k] => v.records
  }
}
```

See [`examples/multi-domain/`](examples/multi-domain/) for the complete working configuration including variable definitions and a `.tfvars` example.

### Resource Creation Order

```
stalwart_domain
     │
     ├──► stalwart_dkim
     │         │
     │         └──► stalwart_dns_records  (data source)
     │
     └──► stalwart_group
               │
               └──► stalwart_account
```

Terraform resolves this automatically from resource references. The only cases requiring an explicit `depends_on` are when group email addresses or `member_of` values come from plain variable strings rather than resource references.

### Authentication

| Method | Config |
|---|---|
| Bearer token | `token = "api_..."` in provider block |
| Environment variable | `export STALWART_TOKEN="api_..."` |
| Username / password | `username` + `password` in provider block |

### Building from Source

```bash
git clone https://github.com/bilbilak/terraform-provider-stalwart
cd terraform-provider-stalwart

# Build
make build

# Install into local Terraform plugin cache for testing
make install
```

After `make install`, reference the provider locally via `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "bilbilak/stalwart" = "/path/to/terraform-provider-stalwart"
  }
  direct {}
}
```

## 👥 Support

If you need assistance or have any questions regarding **_Terraform Provider for Stalwart_**, please refer to the [Support Policy](https://github.com/bilbilak/terraform-provider-stalwart/blob/main/SUPPORT.md) for information on how to get help. We also welcome suggestions and ideas for new features or improvements.

## 🤝 Contributing

We encourage contributions from the community to help improve **_Terraform Provider for Stalwart_** and keep the project moving forward. If you're interested in contributing, please refer to the [Contribution Guide](https://github.com/bilbilak/terraform-provider-stalwart/blob/main/CONTRIBUTING.md) for guidelines on how to participate in this project.

## ⚖️ License

> Copyright © 2025 [Bilbilak](https://bilbilak.org)

**_Terraform Provider for Stalwart_** is distributed under the terms of the [GNU Affero General Public License version 3](https://github.com/bilbilak/terraform-provider-stalwart/blob/main/LICENSE.md). Unless it is explicitly stated otherwise, any contribution intentionally submitted for inclusion in this project shall be licensed as _AGPL-3.0_, without any additional terms or conditions.
