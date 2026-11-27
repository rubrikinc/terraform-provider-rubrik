terraform {
  required_providers {
    rubrik = {
      source = "rubrikinc/rubrik"
    }
    aws = {
      source = "hashicorp/aws"
    }
  }
}

variable "application_account_id" {
  type        = string
  description = "RSC UUID of the manually onboarded application account."
}

variable "host_account_id" {
  type        = string
  description = "RSC UUID of the Exocompute host account."
}

variable "application_role_name" {
  type        = string
  description = "Name of the RSC cross-account IAM role in the application AWS account."
}

resource "rubrik_aws_exocompute" "application" {
  account_id            = var.application_account_id
  host_account_id       = var.host_account_id
  automated_key_sharing = true
}

# Configure the AWS provider to manage IAM in the application account.
resource "aws_iam_role_policy" "exocompute_kms" {
  name   = "rubrik-exocompute-kms-sharing"
  role   = var.application_role_name
  policy = rubrik_aws_exocompute.application.kms_role_policy

  lifecycle {
    replace_triggered_by = [rubrik_aws_exocompute.application]
  }
}

resource "rubrik_aws_exocompute_kms_confirmation" "application" {
  account_id = rubrik_aws_exocompute.application.account_id

  depends_on = [aws_iam_role_policy.exocompute_kms]

  lifecycle {
    replace_triggered_by = [rubrik_aws_exocompute.application]
  }
}
