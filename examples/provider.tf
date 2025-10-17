terraform {
  required_providers {
    stalwart = {
      source = "bilbilak/stalwart"
    }
  }
}

provider "stalwart" {
  server_hostname = "iris.cloud.olandiz.com"
  api_key         = "your-api-key-here" # or set STALWART_API_KEY environment variable
}
