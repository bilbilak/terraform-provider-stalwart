---
page_title: "stalwart_group Resource - terraform-provider-stalwart"
subcategory: ""
description: |-
  Manages an email group (distribution list) in Stalwart Mail Server.
---

# stalwart_group (Resource)

Manages an email group (also known as a distribution list or mailing list) in Stalwart Mail Server. Groups can have a primary email address and multiple aliases.

## Example Usage

```terraform
resource "stalwart_group" "support" {
  name        = "support_team"
  description = "Customer Support Team"
  email       = "support@example.com"

  aliases = [
    "help@example.com",
    "contact@example.com",
    "feedback@example.com"
  ]
}

# Group without aliases
resource "stalwart_group" "admins" {
  name        = "administrators"
  description = "System Administrators"
  email       = "admin@example.com"
}
```

## Schema

### Required

- `name` (String) The unique name identifier for the group. Changing this forces a new resource to be created.
- `description` (String) Description of the group for identification purposes.
- `email` (String) The primary email address for the group. Changing this forces a new resource to be created.

### Optional

- `aliases` (List of String) Additional email aliases for the group. Changing this forces a new resource to be created.

### Read-Only

- `id` (Number) The unique principal ID assigned by Stalwart Mail Server.

## Import

Groups can be imported using the principal ID:

```shell
terraform import stalwart_group.support 456
```

Where `456` is the principal ID of the group in Stalwart Mail Server.
