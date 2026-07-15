# Look up a sandbox by name...
data "mailtrap_sandbox" "by_name" {
  name = "Staging"
}

# ...or by ID.
data "mailtrap_sandbox" "by_id" {
  id = 12345
}
