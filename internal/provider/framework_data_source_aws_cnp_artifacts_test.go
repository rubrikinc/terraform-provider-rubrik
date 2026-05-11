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
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAwsCnpArtifactsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "polaris_aws_cnp_artifacts" "all_features" {
					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_ARCHIVAL"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_DYNAMODB_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_S3_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
					feature {
						name              = "KUBERNETES_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "RDS_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_aws_cnp_artifacts" "role_chaining" {
					feature {
						name              = "ROLE_CHAINING"
						permission_groups = ["BASIC"]
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// All features.
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.all_features",
					tfjsonpath.New(keyID), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.all_features",
					tfjsonpath.New(keyInstanceProfileKeys),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("EXOCOMPUTE_EKS_WORKERNODE"),
					})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.all_features",
					tfjsonpath.New(keyRoleKeys),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("CROSSACCOUNT"),
						knownvalue.StringExact("EXOCOMPUTE_EKS_LAMBDA"),
						knownvalue.StringExact("EXOCOMPUTE_EKS_MASTERNODE"),
						knownvalue.StringExact("EXOCOMPUTE_EKS_WORKERNODE"),
					})),

				// Role chaining.
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.role_chaining",
					tfjsonpath.New(keyID), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.role_chaining",
					tfjsonpath.New(keyInstanceProfileKeys),
					knownvalue.SetExact([]knownvalue.Check{})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_artifacts.role_chaining",
					tfjsonpath.New(keyRoleKeys),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("CROSSACCOUNT"),
					})),
			},
		}},
	})
}

// TestAccAwsCnpArtifactsDataSource_FrameworkMigration verifies that the
// migrated aws_cnp_artifacts data source is backwards compatible with the SDKv2
// provider.
func TestAccAwsCnpArtifactsDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.6.3",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "polaris_aws_cnp_artifacts" "old" {
					provider = polaris-sdkv2

					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_aws_cnp_artifacts" "new" {
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_aws_cnp_artifacts" "old_multi" {
					provider = polaris-sdkv2

					cloud = "STANDARD"
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
				}

				data "polaris_aws_cnp_artifacts" "new_multi" {
					cloud = "STANDARD"
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// Default-cloud (cloud unset) pair.
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_artifacts.old", tfjsonpath.New(keyID),
					"data.polaris_aws_cnp_artifacts.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_artifacts.old", tfjsonpath.New(keyInstanceProfileKeys),
					"data.polaris_aws_cnp_artifacts.new", tfjsonpath.New(keyInstanceProfileKeys),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_artifacts.old", tfjsonpath.New(keyRoleKeys),
					"data.polaris_aws_cnp_artifacts.new", tfjsonpath.New(keyRoleKeys),
					compare.ValuesSame()),

				// Multi-feature, explicit-cloud pair.
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_artifacts.old_multi", tfjsonpath.New(keyID),
					"data.polaris_aws_cnp_artifacts.new_multi", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_artifacts.old_multi", tfjsonpath.New(keyInstanceProfileKeys),
					"data.polaris_aws_cnp_artifacts.new_multi", tfjsonpath.New(keyInstanceProfileKeys),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_artifacts.old_multi", tfjsonpath.New(keyRoleKeys),
					"data.polaris_aws_cnp_artifacts.new_multi", tfjsonpath.New(keyRoleKeys),
					compare.ValuesSame()),
			},
		}},
	})
}
