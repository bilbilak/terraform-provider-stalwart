# Option A — bearer token (recommended)
provider "stalwart" {
  endpoint = "https://mail.example.com/api"
  token    = "api_dGVycmFmb3J..."
}

# Option B — username / password
provider "stalwart" {
  endpoint = "https://mail.example.com/api"
  username = "admin"
  password = "secret"
}

# Option C — token from environment variable STALWART_TOKEN
provider "stalwart" {
  endpoint = "https://mail.example.com/api"
}
