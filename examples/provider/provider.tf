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

# Extra headers are sent with every request, e.g. when the API sits behind
# a gateway that requires its own credentials.
# provider "mailtrap" {
#   api_token = var.mailtrap_api_token
#   headers = {
#     "X-Gateway-Token" = var.gateway_token
#   }
# }
