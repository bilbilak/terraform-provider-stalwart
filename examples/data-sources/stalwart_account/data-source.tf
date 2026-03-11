data "stalwart_account" "alice" {
  name = "alice"
}

output "alice_primary_email" {
  value = data.stalwart_account.alice.primary_email
}

output "alice_member_of" {
  value = data.stalwart_account.alice.member_of
}
