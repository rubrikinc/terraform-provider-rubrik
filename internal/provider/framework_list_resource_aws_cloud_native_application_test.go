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

	"github.com/google/uuid"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccAwsCloudNativeApplicationListResource(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagAWSAppResilience)
	accountID, targetAccountID, region := testCloudApplicationAccounts(t)

	vars := config.Variables{
		// Application names may only hold letters, digits, - and _.
		"name":              config.StringVariable("terraform-test-" + uuid.NewString()),
		"account_id":        config.StringVariable(accountID),
		"target_account_id": config.StringVariable(targetAccountID),
		"region":            config.StringVariable(region),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsCloudNativeApplicationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Create an application so the list resource has something
			// deterministic to return. The query step below runs against it.
			Config: `
				variable "name" {
					type = string
				}
				variable "account_id" {
					type = string
				}
				variable "target_account_id" {
					type = string
				}
				variable "region" {
					type = string
				}
				resource "rubrik_aws_cloud_native_application" "app" {
					name                    = var.name
					cloud_account_id        = var.account_id
					target_cloud_account_id = var.target_account_id
					region                  = var.region
					matching_tags           = { Application = "terraform-test" }
				}
			`,
			ConfigVariables: vars,
		}, {
			// List the applications whose name contains the unique test name,
			// which matches exactly the one created above.
			Query: true,
			Config: `
				provider "rubrik" {}

				list "rubrik_aws_cloud_native_application" "filtered" {
					provider = rubrik

					config {
						name = var.name
					}
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_aws_cloud_native_application.filtered", map[string]knownvalue.Check{
					keyID: knownvalue.NotNull(),
				}),
				querycheck.ExpectLength("rubrik_aws_cloud_native_application.filtered", 1),
			},
		}},
	})
}
