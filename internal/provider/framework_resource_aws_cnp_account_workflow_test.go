// Copyright 2026 Rubrik, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

// TestAccAwsCnpAccountWorkflow exercises the full AWS IAM-roles onboarding
// workflow end-to-end.
func TestAccAwsCnpAccountWorkflow(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: ">=6.0.0",
			},
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Note, the Terraform Plugin Testing package does not support
			// for_each on data source or resources, only count.
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}

				data "polaris_aws_cnp_artifacts" "artifacts" {
					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_aws_cnp_permissions" "crossaccount" {
					role_key = "CROSSACCOUNT"

					dynamic "feature" {
						for_each = data.polaris_aws_cnp_artifacts.artifacts.feature
						content {
							name              = feature.value["name"]
							permission_groups = feature.value["permission_groups"]
						}
					}
				}

				resource "polaris_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					dynamic "feature" {
						for_each = data.polaris_aws_cnp_artifacts.artifacts.feature
						content {
							name              = feature.value["name"]
							permission_groups = feature.value["permission_groups"]
						}
					}
				}

				resource "aws_iam_role" "crossaccount" {
					name_prefix        = "tfacc-crossaccount-"
					assume_role_policy = one(polaris_aws_cnp_account.account.trust_policies).policy
				}

				resource "aws_iam_policy" "crossaccount" {
					count       = length(data.polaris_aws_cnp_permissions.crossaccount.customer_managed_policies)
					name_prefix = "tfacc-crossaccount-${data.polaris_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].name}-"
					policy      = data.polaris_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].policy
				}

				resource "aws_iam_role_policy_attachment" "crossaccount" {
					count      = length(aws_iam_policy.crossaccount)
					role       = aws_iam_role.crossaccount.name
					policy_arn = aws_iam_policy.crossaccount[count.index].arn
				}

				resource "aws_iam_role_policy_attachments_exclusive" "crossaccount" {
					role_name   = aws_iam_role.crossaccount.name
					policy_arns = concat(data.polaris_aws_cnp_permissions.crossaccount.managed_policies, aws_iam_policy.crossaccount[*].arn)
				}

				resource "polaris_aws_cnp_account_attachments" "attachments" {
					account_id = polaris_aws_cnp_account.account.id
					features   = data.polaris_aws_cnp_artifacts.artifacts.feature.*.name

					role {
						key         = "CROSSACCOUNT"
						arn         = aws_iam_role.crossaccount.arn
						permissions = data.polaris_aws_cnp_permissions.crossaccount.id
					}
				}
			`,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.artifacts",
					tfjsonpath.New(keyRoleKeys),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("CROSSACCOUNT"),
					})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.artifacts",
					tfjsonpath.New(keyInstanceProfileKeys),
					knownvalue.SetExact([]knownvalue.Check{})),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyName), knownvalue.StringExact(testAWSAccountName(t))),
				statecheck.CompareValuePairs(
					"polaris_aws_cnp_account.account", tfjsonpath.New(keyID),
					"polaris_aws_cnp_account_attachments.attachments", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"polaris_aws_cnp_account.account", tfjsonpath.New(keyID),
					"polaris_aws_cnp_account_attachments.attachments", tfjsonpath.New(keyAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyFeatures),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("CLOUD_DISCOVERY"),
						knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
					})),
				statecheck.CompareValuePairs(
					"aws_iam_role.crossaccount", tfjsonpath.New(keyARN),
					"polaris_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyRole).AtSliceIndex(0).AtMapKey(keyARN), compare.ValuesSame()),
			},
		}},
	})
}

// TestAccAwsCnpAccountWorkflow_RoleChaining exercises the AWS IAM-roles
// onboarding workflow for a role-chaining account. RSC returns a duplicate
// CROSSACCOUNT artifact alongside the ROLE_CHAINING artifact for these
// accounts; the SDK filters it out so only the ROLE_CHAINING role is required
// and registered. The post-apply plan check guards against the perpetual diff
// that the duplicate artifact would otherwise cause.
func TestAccAwsCnpAccountWorkflow_RoleChaining(t *testing.T) {
	skipUnlessFeatureEnabled(t, core.FeatureFlagAWSManualRoleChaining)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: ">=6.0.0",
			},
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}

				data "polaris_aws_cnp_artifacts" "artifacts" {
					feature {
						name              = "ROLE_CHAINING"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_aws_cnp_permissions" "role_chaining" {
					role_key = "ROLE_CHAINING"

					dynamic "feature" {
						for_each = data.polaris_aws_cnp_artifacts.artifacts.feature
						content {
							name              = feature.value["name"]
							permission_groups = feature.value["permission_groups"]
						}
					}
				}

				resource "polaris_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					dynamic "feature" {
						for_each = data.polaris_aws_cnp_artifacts.artifacts.feature
						content {
							name              = feature.value["name"]
							permission_groups = feature.value["permission_groups"]
						}
					}
				}

				resource "aws_iam_role" "role_chaining" {
					name_prefix        = "tfacc-rolechaining-"
					assume_role_policy = one(polaris_aws_cnp_account.account.trust_policies).policy
				}

				resource "aws_iam_policy" "role_chaining" {
					count       = length(data.polaris_aws_cnp_permissions.role_chaining.customer_managed_policies)
					name_prefix = "tfacc-rolechaining-${data.polaris_aws_cnp_permissions.role_chaining.customer_managed_policies[count.index].name}-"
					policy      = data.polaris_aws_cnp_permissions.role_chaining.customer_managed_policies[count.index].policy
				}

				resource "aws_iam_role_policy_attachment" "role_chaining" {
					count      = length(aws_iam_policy.role_chaining)
					role       = aws_iam_role.role_chaining.name
					policy_arn = aws_iam_policy.role_chaining[count.index].arn
				}

				resource "aws_iam_role_policy_attachments_exclusive" "role_chaining" {
					role_name   = aws_iam_role.role_chaining.name
					policy_arns = concat(data.polaris_aws_cnp_permissions.role_chaining.managed_policies, aws_iam_policy.role_chaining[*].arn)
				}

				resource "polaris_aws_cnp_account_attachments" "attachments" {
					account_id = polaris_aws_cnp_account.account.id
					features   = data.polaris_aws_cnp_artifacts.artifacts.feature.*.name

					role {
						key         = "ROLE_CHAINING"
						arn         = aws_iam_role.role_chaining.arn
						permissions = data.polaris_aws_cnp_permissions.role_chaining.id
					}
				}
			`,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.artifacts",
					tfjsonpath.New(keyRoleKeys),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("ROLE_CHAINING"),
					})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.artifacts",
					tfjsonpath.New(keyInstanceProfileKeys),
					knownvalue.SetExact([]knownvalue.Check{})),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyName), knownvalue.StringExact(testAWSAccountName(t))),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyTrustPolicies),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyRoleKey: knownvalue.StringExact("ROLE_CHAINING"),
						}),
					})),
				statecheck.CompareValuePairs(
					"polaris_aws_cnp_account.account", tfjsonpath.New(keyID),
					"polaris_aws_cnp_account_attachments.attachments", tfjsonpath.New(keyAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyRole),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyKey: knownvalue.StringExact("ROLE_CHAINING"),
						}),
					})),
				statecheck.CompareValuePairs(
					"aws_iam_role.role_chaining", tfjsonpath.New(keyARN),
					"polaris_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyRole).AtSliceIndex(0).AtMapKey(keyARN), compare.ValuesSame()),
			},
		}},
	})
}
