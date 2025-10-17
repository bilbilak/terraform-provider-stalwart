---
page_title: "stalwart_account Resource - terraform-provider-stalwart"
subcategory: ""
description: |-
  Manages an email account (user) in Stalwart Mail Server.
---

# stalwart_account (Resource)

Manages an email account (user) in Stalwart Mail Server. Accounts can have multiple email aliases, be members of groups, and have assigned roles for access control.

## Example Usage

```terraform
# Basic account
resource "stalwart_account" "john" {
  name        = "john"
  description = "John Doe"
  email       = "john@example.com"
  secrets     = [var.john_password]
}

# Account with aliases and group membership
resource "stalwart_account" "admin" {
  name        = "admin"
  description = "System Administrator"
  email       = "admin@example.com"

  aliases = [
    "administrator@example.com",
    "sysadmin@example.com"
  ]

  groups = [
    stalwart_group.it_team.name,
    stalwart_group.admins.name
  ]

  roles   = ["user", "admin"]
  secrets = [var.admin_password]
  locale  = "en"
}

# Service account with multiple passwords
resource "stalwart_account" "service" {
  name        = "api_service"
  description = "API Service Account"
  email       = "service@example.com"

  roles = ["user"]
  secrets = [
    var.service_password_primary,
    var.service_password_backup
  ]
}
```

## Schema

### Required

- `name` (String) The unique username for the account. Changing this forces a new resource to be created.
- `description` (String) Description or full name of the account holder.
- `email` (String) The primary email address for the account. Changing this forces a new resource to be created.
- `secrets` (List of String, Sensitive) List of passwords for the account. At least one password is required. Passwords are hashed by the server and cannot be read back.

### Optional

- `aliases` (List of String) Additional email aliases for the account. Changing this forces a new resource to be created.
- `locale` (String) Account locale for internationalization (default: "en").
- `groups` (List of String) List of group names this account is a member of. Use the group's `name` attribute, not the `id`.
- `roles` (List of String) Roles assigned to this account for access control (default: ["user"]). Common roles include "user" and "admin".

### Read-Only

- `id` (Number) The unique principal ID assigned by Stalwart Mail Server.

## Notes

### Secrets Management

- Secrets (passwords) are write-only and stored as bcrypt hashes by Stalwart Mail Server
- You cannot read back passwords from the API
- Changing passwords requires updating the `secrets` list
- Multiple passwords can be configured for an account

### Group Membership

- Use the group's `name` attribute (not `id`) when specifying group membership
- Example: `groups = [stalwart_group.support.name]`
- Group changes do not force resource replacement

### Roles

- If no roles are specified, the account defaults to `["user"]`
- The "admin" role grants administrative privileges in Stalwart Mail Server
- Custom roles can be defined based on your Stalwart configuration

## Import

Accounts can be imported using the principal ID:

```shell
terraform import stalwart_account.john 789
```

Where `789` is the principal ID of the account in Stalwart Mail Server.

**Note**: When importing, you must provide the `secrets` attribute in your configuration, as passwords cannot be read from the API.
