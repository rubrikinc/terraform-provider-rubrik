# The source and the target AWS accounts, both onboarded with the
# CLOUD_NATIVE_CONFIG_PROTECTION feature. See the rubrik_aws_cnp_account
# resource for the full onboarding, including the IAM roles.
resource "rubrik_aws_cnp_account" "source" {
  name      = "production"
  native_id = "123456789012"
  regions   = ["us-east-1"]

  feature {
    name              = "CLOUD_DISCOVERY"
    permission_groups = ["BASIC"]
  }
  feature {
    name              = "CLOUD_NATIVE_CONFIG_PROTECTION"
    permission_groups = ["BASIC", "BASIC_2", "RECOVERY", "RECOVERY_2", "RECOVERY_3", "RECOVERY_4"]
  }
}

resource "rubrik_aws_cnp_account" "recovery" {
  name      = "recovery"
  native_id = "210987654321"
  regions   = ["us-east-1"]

  feature {
    name              = "CLOUD_DISCOVERY"
    permission_groups = ["BASIC"]
  }
  feature {
    name              = "CLOUD_NATIVE_CONFIG_PROTECTION"
    permission_groups = ["BASIC", "BASIC_2", "RECOVERY", "RECOVERY_2", "RECOVERY_3", "RECOVERY_4"]
  }
}

# The application holds the resources tagged Application = web-store, plus
# one explicitly listed Lambda function.
resource "rubrik_aws_cloud_native_application" "web_store" {
  name                    = "web-store"
  cloud_account_id        = rubrik_aws_cnp_account.source.id
  target_cloud_account_id = rubrik_aws_cnp_account.recovery.id
  region                  = "us-east-1"

  matching_tags = {
    Application = "web-store"
  }
  resource_arns = [
    "arn:aws:lambda:us-east-1:123456789012:function:web-store-checkout",
  ]
}

# Protect the application configuration with an SLA Domain.
resource "rubrik_sla_domain" "cloud_applications" {
  name         = "cloud-applications"
  object_types = ["AWS_CONFIG_OBJECT_TYPE"]

  hourly_schedule {
    frequency      = 6
    retention      = 7
    retention_unit = "DAYS"
  }
}

resource "rubrik_sla_domain_assignment" "web_store" {
  sla_domain_id = rubrik_sla_domain.cloud_applications.id
  object_ids    = [rubrik_aws_cloud_native_application.web_store.id]
}
