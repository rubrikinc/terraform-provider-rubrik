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

func TestAccAwsAccountDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			Config: `
				variable "profile" {
					type = string
				}
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}
				resource "rubrik_aws_account" "account" {
					name    = var.account_name
					profile = var.profile

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions = ["us-east-2"]
					}
				}

				data "rubrik_aws_account" "by_account_id" {
					account_id = var.aws_account_id
					depends_on = [rubrik_aws_account.account]
				}

				data "rubrik_aws_account" "by_cloud_account_id" {
					cloud_account_id = rubrik_aws_account.account.id
				}

				data "rubrik_aws_account" "by_name" {
					name = rubrik_aws_account.account.name
				}
			`,
			ConfigVariables: config.Variables{
				"profile":        config.StringVariable(testAWSProfile(t)),
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// Account.
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyID),
					NonNullUUID()),
				// By Account ID.
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyID),
					"data.rubrik_aws_account.by_account_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyID),
					"data.rubrik_aws_account.by_account_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyName),
					"data.rubrik_aws_account.by_account_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_aws_account.by_account_id",
					tfjsonpath.New(keyFeature),
					knownvalue.SetPartial([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
						}),
					})),
				// By Cloud Account ID.
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyID),
					"data.rubrik_aws_account.by_cloud_account_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyID),
					"data.rubrik_aws_account.by_cloud_account_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyName),
					"data.rubrik_aws_account.by_cloud_account_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_aws_account.by_cloud_account_id",
					tfjsonpath.New(keyFeature),
					knownvalue.SetPartial([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
						}),
					})),
				// By Name.
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyID),
					"data.rubrik_aws_account.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyID),
					"data.rubrik_aws_account.by_name", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_account.account", tfjsonpath.New(keyName),
					"data.rubrik_aws_account.by_name", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_aws_account.by_name",
					tfjsonpath.New(keyFeature),
					knownvalue.SetPartial([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
						}),
					})),
			},
		}},
	})
}

// TestAccAwsAccountDataSource_FrameworkMigration verifies that the migrated
// aws_account data source is backwards compatible with the SDKv2 provider.
func TestAccAwsAccountDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.6.3",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard an AWS account using the SDKv2 resource and verify that
			// the SDKv2 and Framework data sources return identical values.
			Config: `
				variable "profile" {
					type = string
				}
				variable "account_name" {
					type = string
				}
				resource "polaris_aws_account" "account" {
					name    = var.account_name
					profile = var.profile

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions = ["us-east-2"]
					}
				}

				data "polaris_aws_account" "old" {
					provider = polaris-sdkv2

					name = polaris_aws_account.account.name
				}

				data "polaris_aws_account" "new" {
					name = polaris_aws_account.account.name
				}
			`,
			ConfigVariables: config.Variables{
				"profile":      config.StringVariable(testAWSProfile(t)),
				"account_name": config.StringVariable(testAWSAccountName(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.polaris_aws_account.old", tfjsonpath.New(keyID),
					"data.polaris_aws_account.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_account.old", tfjsonpath.New(keyAccountID),
					"data.polaris_aws_account.new", tfjsonpath.New(keyAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_account.old", tfjsonpath.New(keyCloudAccountID),
					"data.polaris_aws_account.new", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_account.old", tfjsonpath.New(keyName),
					"data.polaris_aws_account.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_account.old", tfjsonpath.New(keyFeature),
					"data.polaris_aws_account.new", tfjsonpath.New(keyFeature),
					compare.ValuesSame()),
			},
		}},
	})
}
