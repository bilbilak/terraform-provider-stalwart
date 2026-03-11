data "stalwart_group" "info" {
  name = "info-example"
}

output "group_primary_email" {
  value = data.stalwart_group.info.primary_email
}

output "group_aliases" {
  value = data.stalwart_group.info.aliases
}
