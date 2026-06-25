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
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccAwsCnpAccountListResource(t *testing.T) {
	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsCnpAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Create the AWS CNP account so the list resource has something
			// deterministic to return. The query steps below run against the
			// same account.
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
			`,
			ConfigVariables: vars,
		}, {
			Query: true,
			Config: `
				provider "polaris" {}

				list "polaris_aws_cnp_account" "all" {
					provider = polaris
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("polaris_aws_cnp_account.all", map[string]knownvalue.Check{
					keyID:         knownvalue.NotNull(),
					keyExternalID: knownvalue.Null(),
				}),
			},
		}, {
			Query: true,
			Config: `
				provider "polaris" {}

				list "polaris_aws_cnp_account" "filtered" {
					provider = polaris

					config {
						native_id = var.aws_account_id
					}
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("polaris_aws_cnp_account.filtered", map[string]knownvalue.Check{
					keyID:         knownvalue.NotNull(),
					keyExternalID: knownvalue.Null(),
				}),
				querycheck.ExpectLength("polaris_aws_cnp_account.filtered", 1),
			},
		}},
	})
}
