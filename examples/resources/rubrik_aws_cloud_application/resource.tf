# The source and the target AWS accounts, both already onboarded with the
# CLOUD_NATIVE_CONFIG_PROTECTION feature, including the RECOVERY permission
# groups.
data "rubrik_aws_account" "source" {
  account_id = "123456789012"
}

data "rubrik_aws_account" "recovery" {
  account_id = "210987654321"
}

# The application holds the resources tagged Application = web-store, plus
# one explicitly listed Lambda function.
resource "rubrik_aws_cloud_application" "web_store" {
  name                    = "web-store"
  cloud_account_id        = data.rubrik_aws_account.source.id
  target_cloud_account_id = data.rubrik_aws_account.recovery.id
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
  object_ids    = [rubrik_aws_cloud_application.web_store.id]
}
