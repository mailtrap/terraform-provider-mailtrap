data "mailtrap_account" "example" {}

resource "mailtrap_api_token" "example" {
  name = "ci-token"

  resources = [{
    resource_type = "account"
    resource_id   = data.mailtrap_account.example.id
    access_level  = 100
  }]
}

# The full token value. Only available when the token is created by
# Terraform (never returned again by the API).
output "token" {
  value     = mailtrap_api_token.example.token
  sensitive = true
}
