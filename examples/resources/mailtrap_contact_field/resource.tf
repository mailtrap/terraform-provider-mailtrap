resource "mailtrap_contact_field" "example" {
  name      = "Subscription Plan"
  data_type = "text"
  merge_tag = "subscription_plan"
}
