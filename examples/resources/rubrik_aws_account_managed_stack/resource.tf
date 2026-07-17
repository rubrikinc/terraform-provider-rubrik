# Full RSC-managed AWS (BaaS) onboarding flow:
#
#   1. rubrik_aws_account_managed         - validate + finalize; returns the RSC
#                                           account UUID and CloudFormation template.
#   2. aws_cloudformation_stack           - the AWS provider deploys the stack.
#   3. rubrik_aws_account_managed_stack   - waits for features to connect and
#                                           completes onboarding.

resource "rubrik_aws_account_managed" "example" {
  native_id = "123456789012"
  name      = "my-aws-account"
}

resource "aws_cloudformation_stack" "rubrik" {
  name         = rubrik_aws_account_managed.example.stack_name
  template_url = rubrik_aws_account_managed.example.template_url
  capabilities = ["CAPABILITY_IAM", "CAPABILITY_NAMED_IAM"]
  tags         = {} # avoids the hashicorp/aws empty-tags refresh drift
}

resource "rubrik_aws_account_managed_stack" "example" {
  account_id          = rubrik_aws_account_managed.example.id
  stack_arn           = aws_cloudformation_stack.rubrik.id
  permissions_version = rubrik_aws_account_managed.example.permissions_version
}
