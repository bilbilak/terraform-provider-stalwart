variable "server_hostname" {
  description = "Stalwart Mail Server hostname (without https://)"
  type        = string
  default     = "mail.example.com"
}

variable "api_key" {
  description = "API key for Stalwart Mail Server"
  type        = string
  sensitive   = true
}

variable "john_password" {
  description = "Password for John's account"
  type        = string
  sensitive   = true
}

variable "jane_password" {
  description = "Password for Jane's account"
  type        = string
  sensitive   = true
}

variable "admin_password" {
  description = "Password for admin account"
  type        = string
  sensitive   = true
}
