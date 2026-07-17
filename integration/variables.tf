variable "name_prefix" {
  type        = string
  default     = "tftest"
  description = "Prefix for every resource name so concurrent runs in the shared test account do not collide. CI sets it to tftest-<run_id>-<tool>."

  validation {
    # The prefix is also used as a DNS label in the sending domain name.
    condition     = can(regex("^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$", var.name_prefix))
    error_message = "name_prefix must be a valid lowercase DNS label (letters, digits, and dashes)."
  }
}

variable "name_suffix" {
  type        = string
  default     = ""
  description = "Suffix appended to resource names by the update test run to exercise in-place updates."
}

variable "expected_ids" {
  type        = map(number)
  default     = {}
  description = "Resource ids from a previous test run (the `ids` output). Only used by test assertions to verify updates happen in place; unused by the configuration itself."
}

locals {
  name = "${var.name_prefix}${var.name_suffix}"
}
