# Look up a project by name...
data "mailtrap_project" "by_name" {
  name = "My Project"
}

# ...or by ID.
data "mailtrap_project" "by_id" {
  id = 12345
}
