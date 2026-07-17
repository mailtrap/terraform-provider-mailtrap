terraform {
  required_providers {
    mailtrap = {
      source = "mailtrap/mailtrap"
    }
  }
}

# The token can also be provided via the MAILTRAP_API_TOKEN environment
# variable instead of the api_token attribute.
provider "mailtrap" {
  api_token = var.mailtrap_api_token
}
