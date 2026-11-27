variable "application_account_id" {
  type        = string
  description = "RSC UUID of the application account onboarded through a single-account CloudFormation stack."
}

variable "host_account_id" {
  type        = string
  description = "RSC UUID of the Exocompute host account."
}

variable "application_aws_profile" {
  type        = string
  description = "AWS profile authorized to update the application account's onboarding stack."
}

resource "rubrik_aws_exocompute" "cft_application" {
  account_id            = var.application_account_id
  host_account_id       = var.host_account_id
  automated_key_sharing = true
  profile               = var.application_aws_profile
}
