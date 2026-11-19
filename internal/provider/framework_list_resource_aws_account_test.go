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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAWSAccountListResource onboards the AWS account and lists it, with
// and without a filter.
func TestAccAWSAccountListResource(t *testing.T) {
	vars := config.Variables{
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
		"name":           config.StringVariable(testAWSAccountName(t)),
		"profile":        config.StringVariable(testAWSProfile(t)),
	}

	// The RSC account ID of the onboarded account, recorded by the first step
	// and compared against the list results by the query steps.
	var accountID string

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard the AWS account so the list resource has something
			// deterministic to return. The query steps below run against the
			// same account.
			Config: `
				variable "aws_account_id" {
					type = string
				}
				variable "name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					name    = var.name
					profile = var.profile

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyID),
					knownvalue.StringFunc(func(v string) error {
						if err := NonNullUUID().CheckValue(v); err != nil {
							return err
						}
						accountID = v
						return nil
					})),
			},
		}, {
			// Verify that the onboarded account is listed, with its identity.
			Query: true,
			Config: `
				provider "polaris" {}

				list "rubrik_aws_account" "all" {
					provider = polaris
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_aws_account.all", map[string]knownvalue.Check{
					keyID: knownvalue.StringFunc(func(v string) error {
						if v != accountID {
							return fmt.Errorf("expected %s, got %s", accountID, v)
						}
						return nil
					}),
				}),
			},
		}, {
			// Verify that the native ID filter returns only the onboarded
			// account.
			Query: true,
			Config: `
				provider "polaris" {}

				list "rubrik_aws_account" "by_native_id" {
					provider = polaris

					config {
						native_id = var.aws_account_id
					}
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_aws_account.by_native_id", map[string]knownvalue.Check{
					keyID: knownvalue.StringFunc(func(v string) error {
						if v != accountID {
							return fmt.Errorf("expected %s, got %s", accountID, v)
						}
						return nil
					}),
				}),
				querycheck.ExpectLength("rubrik_aws_account.by_native_id", 1),
			},
		}, {
			// Verify that the name filter matches the onboarded account by a
			// prefix of its name.
			Query: true,
			Config: `
				provider "polaris" {}

				list "rubrik_aws_account" "by_name" {
					provider = polaris

					config {
						name = var.name
					}
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_aws_account.by_name", map[string]knownvalue.Check{
					keyID: knownvalue.StringFunc(func(v string) error {
						if v != accountID {
							return fmt.Errorf("expected %s, got %s", accountID, v)
						}
						return nil
					}),
				}),
				querycheck.ExpectLength("rubrik_aws_account.by_name", 1),
			},
		}},
	})
}
