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

// TestAccAzurePermissionsDataSource reads the permissions for the Cloud Native
// Protection feature, using both the feature and the deprecated features
// fields.
func TestAccAzurePermissionsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the data source returns the permissions for the
			// feature, with and without permission groups. The specific
			// permissions are not pinned, the RSC catalog evolves over time.
			Config: `
				data "rubrik_azure_permissions" "by_feature" {
					feature           = "CLOUD_NATIVE_PROTECTION"
					permission_groups = ["BASIC"]
				}

				data "rubrik_azure_permissions" "by_features" {
					features = ["CLOUD_NATIVE_PROTECTION"]
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// By feature.
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_feature", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(sha256Hex)),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.by_feature", tfjsonpath.New(keyID),
					"data.rubrik_azure_permissions.by_feature", tfjsonpath.New(keyHash),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_feature", tfjsonpath.New(keyFeature),
					knownvalue.StringExact("CLOUD_NATIVE_PROTECTION")),
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_feature", tfjsonpath.New(keyPermissionGroups),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("BASIC"),
					})),
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_feature", tfjsonpath.New(keySubscriptionActions),
					knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.NotNull(),
					})),
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_feature", tfjsonpath.New(keyActions),
					knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.NotNull(),
					})),
				// By features.
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_features", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(sha256Hex)),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.by_features", tfjsonpath.New(keyID),
					"data.rubrik_azure_permissions.by_features", tfjsonpath.New(keyHash),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_features", tfjsonpath.New(keyFeatures),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
					})),
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_features", tfjsonpath.New(keySubscriptionActions),
					knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.NotNull(),
					})),
				statecheck.ExpectKnownValue("data.rubrik_azure_permissions.by_features", tfjsonpath.New(keyActions),
					knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.NotNull(),
					})),
			},
		}},
	})
}

// TestAccAzurePermissionsDataSource_FrameworkMigration verifies that the
// migrated azure_permissions data source is backwards compatible with the
// SDKv2 provider.
func TestAccAzurePermissionsDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"rubrik-sdkv2": {
				Source:            "rubrikinc/rubrik",
				VersionConstraint: "1.11.2",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the two data sources are equal, using both the
			// feature and the deprecated features fields.
			Config: `
				provider "rubrik-sdkv2" {}

				data "rubrik_azure_permissions" "old_by_feature" {
					provider = rubrik-sdkv2

					feature           = "CLOUD_NATIVE_PROTECTION"
					permission_groups = ["BASIC"]
				}

				data "rubrik_azure_permissions" "new_by_feature" {
					feature           = "CLOUD_NATIVE_PROTECTION"
					permission_groups = ["BASIC"]
				}

				data "rubrik_azure_permissions" "old_by_features" {
					provider = rubrik-sdkv2

					features = ["CLOUD_NATIVE_PROTECTION"]
				}

				data "rubrik_azure_permissions" "new_by_features" {
					features = ["CLOUD_NATIVE_PROTECTION"]
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// By feature.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyID),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyDataActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyFeature),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyFeature),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyFeatures),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyFeatures),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyHash),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyHash),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyNotActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyNotActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyNotDataActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyNotDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyPermissionGroups),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyPermissionGroups),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyResourceGroupActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyResourceGroupActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyResourceGroupDataActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyResourceGroupDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyResourceGroupNotActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyResourceGroupNotActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keyResourceGroupNotDataActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keyResourceGroupNotDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keySubscriptionActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keySubscriptionActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keySubscriptionDataActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keySubscriptionDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keySubscriptionNotActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keySubscriptionNotActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_feature", tfjsonpath.New(keySubscriptionNotDataActions),
					"data.rubrik_azure_permissions.new_by_feature", tfjsonpath.New(keySubscriptionNotDataActions),
					compare.ValuesSame()),
				// By features.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyID),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyDataActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyFeature),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyFeature),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyFeatures),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyFeatures),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyHash),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyHash),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyNotActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyNotActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyNotDataActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyNotDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyPermissionGroups),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyPermissionGroups),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyResourceGroupActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyResourceGroupActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyResourceGroupDataActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyResourceGroupDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyResourceGroupNotActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyResourceGroupNotActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keyResourceGroupNotDataActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keyResourceGroupNotDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keySubscriptionActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keySubscriptionActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keySubscriptionDataActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keySubscriptionDataActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keySubscriptionNotActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keySubscriptionNotActions),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_permissions.old_by_features", tfjsonpath.New(keySubscriptionNotDataActions),
					"data.rubrik_azure_permissions.new_by_features", tfjsonpath.New(keySubscriptionNotDataActions),
					compare.ValuesSame()),
			},
		}},
	})
}
