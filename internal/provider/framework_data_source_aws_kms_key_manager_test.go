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
)

// TestAccAWSKMSKeyManagerDataSource onboards the AWS account with the UEM key
// management feature, registers a key manager and reads it with the data
// source by ID and by name.
func TestAccAWSKMSKeyManagerDataSource(t *testing.T) {
	skipUnlessAWSKMSDefined(t)
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	name := testUniqueName(t)

	// Note, the Terraform Plugin Testing package does not support for_each on
	// data source or resources, only count.
	const conf = `
		variable "account_name" {
			type = string
		}
		variable "aws_account_id" {
			type = string
		}
		variable "kms_region" {
			type = string
		}
		variable "name" {
			type = string
		}

		data "rubrik_aws_cnp_artifacts" "artifacts" {
			feature {
				name              = "CLOUD_NATIVE_UEM_KEY_MANAGEMENT"
				permission_groups = ["BASIC"]
			}
		}

		data "rubrik_aws_cnp_permissions" "crossaccount" {
			role_key = "CROSSACCOUNT"

			dynamic "feature" {
				for_each = data.rubrik_aws_cnp_artifacts.artifacts.feature
				content {
					name              = feature.value["name"]
					permission_groups = feature.value["permission_groups"]
				}
			}
		}

		resource "rubrik_aws_cnp_account" "account" {
			name      = var.account_name
			native_id = var.aws_account_id
			regions   = [var.kms_region]

			dynamic "feature" {
				for_each = data.rubrik_aws_cnp_artifacts.artifacts.feature
				content {
					name              = feature.value["name"]
					permission_groups = feature.value["permission_groups"]
				}
			}
		}

		resource "aws_iam_role" "crossaccount" {
			name_prefix        = "tfacc-crossaccount-"
			assume_role_policy = one(rubrik_aws_cnp_account.account.trust_policies).policy
		}

		resource "aws_iam_policy" "crossaccount" {
			count       = length(data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies)
			name_prefix = "tfacc-crossaccount-${data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].name}-"
			policy      = data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].policy
		}

		resource "aws_iam_role_policy_attachment" "crossaccount" {
			count      = length(data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies)
			role       = aws_iam_role.crossaccount.name
			policy_arn = aws_iam_policy.crossaccount[count.index].arn
		}

		resource "aws_iam_role_policy_attachments_exclusive" "crossaccount" {
			role_name   = aws_iam_role.crossaccount.name
			policy_arns = concat(data.rubrik_aws_cnp_permissions.crossaccount.managed_policies, aws_iam_policy.crossaccount[*].arn)
		}

		resource "rubrik_aws_cnp_account_attachments" "attachments" {
			account_id = rubrik_aws_cnp_account.account.id
			features   = data.rubrik_aws_cnp_artifacts.artifacts.feature.*.name

			role {
				key         = "CROSSACCOUNT"
				arn         = aws_iam_role.crossaccount.arn
				permissions = data.rubrik_aws_cnp_permissions.crossaccount.id
			}
		}

		resource "rubrik_aws_kms_key_manager" "key_manager" {
			name             = var.name
			cloud_account_id = rubrik_aws_cnp_account.account.id
			region           = var.kms_region

			depends_on = [rubrik_aws_cnp_account_attachments.attachments]
		}

		data "rubrik_aws_kms_key_manager" "by_id" {
			id = rubrik_aws_kms_key_manager.key_manager.id
		}

		data "rubrik_aws_kms_key_manager" "by_name" {
			name = rubrik_aws_kms_key_manager.key_manager.name
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: ">=6.0.0",
			},
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsKMSKeyManagerCheckDestroy(t),
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the data source reads the key manager by ID and by
			// name.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"kms_region":     config.StringVariable(testAWSKMSRegion(t)),
				"name":           config.StringVariable(name),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By ID.
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"data.rubrik_aws_kms_key_manager.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyName),
					"data.rubrik_aws_kms_key_manager.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyDescription),
					"data.rubrik_aws_kms_key_manager.by_id", tfjsonpath.New(keyDescription),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_aws_kms_key_manager.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyRegion),
					"data.rubrik_aws_kms_key_manager.by_id", tfjsonpath.New(keyRegion),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_aws_kms_key_manager.by_id",
					tfjsonpath.New(keyProvisioningMethod), knownvalue.StringExact("CLOUD_ACCOUNT_PROVISION")),
				statecheck.ExpectKnownValue("data.rubrik_aws_kms_key_manager.by_id",
					tfjsonpath.New(keyIAMRoleARN), knownvalue.NotNull()),
				// By name.
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"data.rubrik_aws_kms_key_manager.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}},
	})
}
