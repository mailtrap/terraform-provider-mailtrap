# With access to a single account, no filters are needed.
data "mailtrap_account" "current" {}

# With access to several accounts, filter by name (or id).
data "mailtrap_account" "acme" {
  name = "Acme"
}
