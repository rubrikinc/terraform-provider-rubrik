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

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccAwsCnpAccountAttachmentsListResource(t *testing.T) {
	account, err := loadAWSTestConf()
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: ">=6.0.0",
			},
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCnpAccountCheckDestroy(t.Context()),
			awsCnpAccountAttachmentsCheckDestroy(t.Context()),
		),
		Steps: []resource.TestStep{{
			// Seed the AWS CNP account and its attachments so the list resource
			// has something deterministic to return. The query steps below run
			// against the same account.
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
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(account.AccountName),
				"aws_account_id": config.StringVariable(account.AccountID),
			},
		}, {
			Query: true,
			Config: `
				provider "polaris" {}

				list "polaris_aws_cnp_account_attachments" "all" {
					provider = polaris
				}
			`,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(account.AccountName),
				"aws_account_id": config.StringVariable(account.AccountID),
			},
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("polaris_aws_cnp_account_attachments.all", map[string]knownvalue.Check{
					keyID: knownvalue.NotNull(),
				}),
			},
		}, {
			Query: true,
			Config: `
				provider "polaris" {}

				list "polaris_aws_cnp_account_attachments" "filtered" {
					provider = polaris

					config {
						native_id = var.aws_account_id
					}
				}
			`,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(account.AccountName),
				"aws_account_id": config.StringVariable(account.AccountID),
			},
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("polaris_aws_cnp_account_attachments.filtered", map[string]knownvalue.Check{
					keyID: knownvalue.NotNull(),
				}),
				querycheck.ExpectLength("polaris_aws_cnp_account_attachments.filtered", 1),
			},
		}, {
			Query: true,
			Config: `
				provider "polaris" {}

				list "polaris_aws_cnp_account_attachments" "with_resource" {
					provider         = polaris
					include_resource = true

					config {
						native_id = var.aws_account_id
					}
				}
			`,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(account.AccountName),
				"aws_account_id": config.StringVariable(account.AccountID),
			},
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectResourceKnownValues(
					"polaris_aws_cnp_account_attachments.with_resource",
					queryfilter.ByResourceIdentity(map[string]knownvalue.Check{
						keyID: knownvalue.NotNull(),
					}),
					[]querycheck.KnownValueCheck{
						{Path: tfjsonpath.New(keyID), KnownValue: knownvalue.NotNull()},
						{Path: tfjsonpath.New(keyAccountID), KnownValue: knownvalue.NotNull()},
						{Path: tfjsonpath.New(keyFeatures), KnownValue: knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("CLOUD_DISCOVERY"),
							knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
						})},
						{Path: tfjsonpath.New(keyRole), KnownValue: knownvalue.SetExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								keyKey:         knownvalue.StringExact("CROSSACCOUNT"),
								keyARN:         knownvalue.NotNull(),
								keyPermissions: knownvalue.Null(),
							}),
						})},
						{Path: tfjsonpath.New(keyInstanceProfile), KnownValue: knownvalue.SetExact([]knownvalue.Check{})},
					},
				),
			},
		}},
	})
}
