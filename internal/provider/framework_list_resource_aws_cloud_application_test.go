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
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAwsCloudApplicationListResource onboards a source and a target AWS
// account, creates an application between them and lists it, with and without
// the resource values.
func TestAccAwsCloudApplicationListResource(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagAWSAppResilience)

	// Application names may only hold letters, digits, - and _.
	name := "terraform-test-" + uuid.NewString()
	vars := testAWSCloudApplicationAccountsVariables(t, name)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		ExternalProviders:        testAWSCloudApplicationExternalProviders,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCloudApplicationCheckDestroy(t),
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Create an application so the list resource has something
			// deterministic to return. The query steps below run against it.
			Config: testAWSCloudApplicationAccountsConfig + `
				resource "rubrik_aws_cloud_application" "app" {
					name                    = var.name
					cloud_account_id        = rubrik_aws_cnp_account_attachments.source.account_id
					target_cloud_account_id = rubrik_aws_cnp_account_attachments.target.account_id
					region                  = var.region
					services                = ["EC2", "LAMBDA"]
					matching_tags = {
						Application = "terraform-test"
						Environment = "test"
					}
				}
			`,
			ConfigVariables: vars,
		}, {
			// List the applications whose name contains the unique test name,
			// which matches exactly the one created above.
			Query: true,
			Config: `
				provider "rubrik" {}

				list "rubrik_aws_cloud_application" "filtered" {
					provider = rubrik

					config {
						name = var.name
					}
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_aws_cloud_application.filtered", map[string]knownvalue.Check{
					keyID: knownvalue.NotNull(),
				}),
				querycheck.ExpectLength("rubrik_aws_cloud_application.filtered", 1),
			},
		}, {
			// List with include_resource, which is what generates the
			// configuration for an import, and verify that the resource values
			// match the application created above.
			Query: true,
			Config: `
				provider "rubrik" {}

				list "rubrik_aws_cloud_application" "with_resource" {
					provider         = rubrik
					include_resource = true

					config {
						name = var.name
					}
				}
			`,
			ConfigVariables: vars,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectResourceKnownValues("rubrik_aws_cloud_application.with_resource",
					queryfilter.ByDisplayName(knownvalue.StringExact(name)),
					[]querycheck.KnownValueCheck{{
						Path:       tfjsonpath.New(keyName),
						KnownValue: knownvalue.StringExact(name),
					}, {
						// The RSC cloud account IDs are only known once the
						// accounts have been onboarded, which query checks
						// can't refer to.
						Path:       tfjsonpath.New(keyCloudAccountID),
						KnownValue: NonNullUUID(),
					}, {
						Path:       tfjsonpath.New(keyTargetCloudAccountID),
						KnownValue: NonNullUUID(),
					}, {
						Path:       tfjsonpath.New(keyRegion),
						KnownValue: knownvalue.StringExact(testAWSCloudApplicationRegion),
					}, {
						Path: tfjsonpath.New(keyMatchingTags),
						KnownValue: knownvalue.MapExact(map[string]knownvalue.Check{
							"Application": knownvalue.StringExact("terraform-test"),
							"Environment": knownvalue.StringExact("test"),
						}),
					}, {
						Path: tfjsonpath.New(keyServices),
						KnownValue: knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("EC2"),
							knownvalue.StringExact("LAMBDA"),
						}),
					}, {
						Path:       tfjsonpath.New(keyResourceARNs),
						KnownValue: knownvalue.Null(),
					}, {
						Path:       tfjsonpath.New(keyDeleteSnapshotsOnDestroy),
						KnownValue: knownvalue.Bool(false),
					}},
				),
			},
		}},
	})
}
