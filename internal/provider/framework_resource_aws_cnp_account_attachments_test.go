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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccAwsCnpAccountAttachmentsResource(t *testing.T) {
	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

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

				resource "rubrik_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "aws_iam_role" "crossaccount" {
					name_prefix        = "rubrik-tfacc-"
					assume_role_policy = one([
						for p in rubrik_aws_cnp_account.account.trust_policies : p.policy if p.role_key == "CROSSACCOUNT"
					])
				}

				resource "rubrik_aws_cnp_account_attachments" "attachments" {
					account_id = rubrik_aws_cnp_account.account.id
					features   = rubrik_aws_cnp_account.account.feature.*.name

					role {
						key         = "CROSSACCOUNT"
						arn         = aws_iam_role.crossaccount.arn
						permissions = "v1"
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cnp_account.account",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.CompareValuePairs(
					"rubrik_aws_cnp_account.account", tfjsonpath.New(keyID),
					"rubrik_aws_cnp_account_attachments.attachments", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_cnp_account.account", tfjsonpath.New(keyID),
					"rubrik_aws_cnp_account_attachments.attachments", tfjsonpath.New(keyAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyFeatures),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("CLOUD_DISCOVERY"),
						knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyRole),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyKey:         knownvalue.StringExact("CROSSACCOUNT"),
							keyPermissions: knownvalue.StringExact("v1"),
						}),
					})),
				statecheck.CompareValuePairs(
					"aws_iam_role.crossaccount", tfjsonpath.New(keyARN),
					"rubrik_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyRole).AtSliceIndex(0).AtMapKey(keyARN), compare.ValuesSame()),
			},
		}, {
			// Change the role ARN and the permission sentinel. Note, we drop
			// the permission sentinel so that the imports results in empty
			// plans.
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}

				resource "rubrik_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "aws_iam_role" "crossaccount" {
					name_prefix        = "rubrik-tfacc-"
					assume_role_policy = one([
						for p in rubrik_aws_cnp_account.account.trust_policies : p.policy if p.role_key == "CROSSACCOUNT"
					])
				}

				resource "rubrik_aws_cnp_account_attachments" "attachments" {
					account_id = rubrik_aws_cnp_account.account.id
					features   = rubrik_aws_cnp_account.account.feature.*.name

					role {
						key = "CROSSACCOUNT"
						arn = aws_iam_role.crossaccount.arn
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyRole),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyKey:         knownvalue.StringExact("CROSSACCOUNT"),
							keyPermissions: knownvalue.Null(),
						}),
					})),
				statecheck.CompareValuePairs(
					"aws_iam_role.crossaccount", tfjsonpath.New(keyARN),
					"rubrik_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyRole).AtSliceIndex(0).AtMapKey(keyARN), compare.ValuesSame()),
			},
		}, {
			ResourceName:      "rubrik_aws_cnp_account_attachments.attachments",
			ConfigVariables:   vars,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			ResourceName:    "rubrik_aws_cnp_account_attachments.attachments",
			ConfigVariables: vars,
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			ResourceName:    "rubrik_aws_cnp_account_attachments.attachments",
			ConfigVariables: vars,
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

// TestAccAwsCnpAccountAttachmentsResource_FrameworkMigration verifies that
// existing state created by the SDKv2 provider can be read by the Framework
// provider without drift. Step 1 creates the resource using a published SDKv2
// provider; step 2 refreshes state using the local Framework provider and
// asserts the plan is empty.
func TestAccAwsCnpAccountAttachmentsResource_FrameworkMigration(t *testing.T) {
	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

	tfConfig := `
		variable "account_name" {
			type = string
		}
		variable "aws_account_id" {
			type = string
		}

		resource "polaris_aws_cnp_account" "account" {
			name      = var.account_name
			native_id = var.aws_account_id
			regions   = ["us-east-2"]

			feature {
				name              = "CLOUD_DISCOVERY"
				permission_groups = ["BASIC"]
			}
			feature {
				name              = "CLOUD_NATIVE_PROTECTION"
				permission_groups = ["BASIC"]
			}
		}

		resource "aws_iam_role" "crossaccount" {
			name_prefix        = "rubrik-tfacc-"
			assume_role_policy = one([
				for p in polaris_aws_cnp_account.account.trust_policies : p.policy if p.role_key == "CROSSACCOUNT"
			])
		}

		resource "polaris_aws_cnp_account_attachments" "attachments" {
			account_id = polaris_aws_cnp_account.account.id
			features   = polaris_aws_cnp_account.account.feature.*.name

			role {
				key         = "CROSSACCOUNT"
				arn         = aws_iam_role.crossaccount.arn
				permissions = "v1"
			}
		}
	`

	resource.Test(t, resource.TestCase{
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"aws": {
					Source:            "hashicorp/aws",
					VersionConstraint: ">=6.0.0",
				},
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.6.3",
				},
			},
			Config:          tfConfig,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			ExternalProviders: map[string]resource.ExternalProvider{
				"aws": {
					Source:            "hashicorp/aws",
					VersionConstraint: ">=6.0.0",
				},
			},
			Config:          tfConfig,
			ConfigVariables: vars,
			PlanOnly:        true,
		}},
	})
}

// TestAccAwsCnpAccountAttachmentsResource_MoveState verifies that state from a
// polaris_aws_cnp_account_attachments resource created by the rubrikinc/polaris
// provider can be moved to a rubrik_aws_cnp_account_attachments resource using
// the moved {} block.
func TestAccAwsCnpAccountAttachmentsResource_MoveState(t *testing.T) {
	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"aws": {
					Source:            "hashicorp/aws",
					VersionConstraint: ">=6.0.0",
				},
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.6.3",
				},
			},
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}

				resource "polaris_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "aws_iam_role" "crossaccount" {
					name_prefix        = "rubrik-tfacc-"
					assume_role_policy = one([
						for p in polaris_aws_cnp_account.account.trust_policies : p.policy if p.role_key == "CROSSACCOUNT"
					])
				}

				resource "polaris_aws_cnp_account_attachments" "attachments" {
					account_id = polaris_aws_cnp_account.account.id
					features   = polaris_aws_cnp_account.account.feature.*.name
					role {
						key         = "CROSSACCOUNT"
						arn         = aws_iam_role.crossaccount.arn
						permissions = "v1"
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_cnp_account_attachments.attachments",
					tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			ExternalProviders: map[string]resource.ExternalProvider{
				"aws": {
					Source:            "hashicorp/aws",
					VersionConstraint: ">=6.0.0",
				},
			},
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}

				moved {
					from = polaris_aws_cnp_account.account
					to   = rubrik_aws_cnp_account.account
				}
				resource "rubrik_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "aws_iam_role" "crossaccount" {
					name_prefix        = "rubrik-tfacc-"
					assume_role_policy = one([
						for p in rubrik_aws_cnp_account.account.trust_policies : p.policy if p.role_key == "CROSSACCOUNT"
					])
				}

				moved {
					from = polaris_aws_cnp_account_attachments.attachments
					to   = rubrik_aws_cnp_account_attachments.attachments
				}
				resource "rubrik_aws_cnp_account_attachments" "attachments" {
					account_id = rubrik_aws_cnp_account.account.id
					features   = rubrik_aws_cnp_account.account.feature.*.name
					role {
						key         = "CROSSACCOUNT"
						arn         = aws_iam_role.crossaccount.arn
						permissions = "v1"
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
