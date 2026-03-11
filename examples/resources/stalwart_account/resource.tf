resource "stalwart_domain" "example" {
  name = "example.com"
}

resource "stalwart_group" "info" {
  name          = "info-example"
  primary_email = "info@${stalwart_domain.example.name}"

  enabled_permissions = ["email-send", "email-receive"]

  depends_on = [stalwart_domain.example]
}

resource "stalwart_account" "alice" {
  name        = "alice"
  description = "Alice Smith"
  password    = "changeme"

  primary_email = "alice@${stalwart_domain.example.name}"
  aliases       = ["a.smith@${stalwart_domain.example.name}"]

  member_of = [stalwart_group.info.name]

  quota = 10737418240 # 10 GB; omit or set to 0 for unlimited

  roles = ["user"]

  depends_on = [stalwart_group.info]
}
