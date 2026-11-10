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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccGCPPermissionsDataSource(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that omitting both feature and features is rejected at
			// plan time. Steps that expect an error go first: the final
			// destroy reuses the last step's configuration, which must be
			// valid.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`No\s+attribute\s+specified\s+when\s+one\s+\(and\s+only\s+one\)\s+of`),
		}, {
			// Verify that specifying both feature and features is rejected at
			// plan time.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {
					feature  = "CLOUD_NATIVE_PROTECTION"
					features = ["CLOUD_NATIVE_PROTECTION"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`2\s+attributes\s+specified\s+when\s+one\s+\(and\s+only\s+one\)\s+of`),
		}, {
			// Verify that specifying permission groups with the deprecated
			// features field is rejected at plan time.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {
					features          = ["CLOUD_NATIVE_PROTECTION"]
					permission_groups = ["BASIC"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`cannot\s+be\s+specified\s+when`),
		}, {
			// Verify that an unknown feature name is rejected at plan time.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {
					feature = "UNKNOWN_FEATURE"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`got:\s+"UNKNOWN_FEATURE"`),
		}, {
			// Verify that an unknown feature name in the deprecated features
			// field is rejected at plan time.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {
					features = ["UNKNOWN_FEATURE"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`got:\s+"UNKNOWN_FEATURE"`),
		}, {
			// Verify that an unknown permission group name is rejected at plan
			// time.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {
					feature           = "CLOUD_NATIVE_PROTECTION"
					permission_groups = ["UNKNOWN_GROUP"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`got:\s+"UNKNOWN_GROUP"`),
		}, {
			// Verify that an empty features field is rejected at plan time.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {
					features = []
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`set\s+must\s+contain\s+at\s+least\s+1\s+elements`),
		}, {
			// Verify that a permission group not supported by the feature is
			// rejected at plan time.
			Config: `
				data "rubrik_gcp_permissions" "invalid" {
					feature           = "CLOUD_NATIVE_ARCHIVAL"
					permission_groups = ["EXPORT_AND_RESTORE"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid\s+permission\s+group`),
		}, {
			// Verify that permissions are returned for a feature with and
			// without permission groups, and for the deprecated features
			// field.
			Config: `
				data "rubrik_gcp_permissions" "feature" {
					feature = "CLOUD_NATIVE_PROTECTION"
				}

				data "rubrik_gcp_permissions" "feature_groups" {
					feature           = "CLOUD_NATIVE_PROTECTION"
					permission_groups = ["BASIC"]
				}

				data "rubrik_gcp_permissions" "features" {
					features = ["CLOUD_NATIVE_PROTECTION"]
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// Feature without permission groups. The permission groups
				// are read back from RSC.
				statecheck.ExpectKnownValue("data.rubrik_gcp_permissions.feature", tfjsonpath.New(keyPermissionGroups),
					knownvalue.SetPartial([]knownvalue.Check{knownvalue.StringExact("BASIC")})),
				statecheck.ExpectKnownValue("data.rubrik_gcp_permissions.feature", tfjsonpath.New(keyWithoutConditions),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.rubrik_gcp_permissions.feature", tfjsonpath.New(keyServices),
					knownvalue.NotNull()),
				statecheck.CompareValuePairs(
					"data.rubrik_gcp_permissions.feature", tfjsonpath.New(keyID),
					"data.rubrik_gcp_permissions.feature", tfjsonpath.New(keyHash),
					compare.ValuesSame()),
				// Feature with permission groups. The configured permission
				// groups are kept.
				statecheck.ExpectKnownValue("data.rubrik_gcp_permissions.feature_groups", tfjsonpath.New(keyPermissionGroups),
					knownvalue.SetExact([]knownvalue.Check{knownvalue.StringExact("BASIC")})),
				statecheck.ExpectKnownValue("data.rubrik_gcp_permissions.feature_groups", tfjsonpath.New(keyWithoutConditions),
					knownvalue.NotNull()),
				statecheck.CompareValuePairs(
					"data.rubrik_gcp_permissions.feature_groups", tfjsonpath.New(keyID),
					"data.rubrik_gcp_permissions.feature_groups", tfjsonpath.New(keyHash),
					compare.ValuesSame()),
				// Deprecated features field. Only the permissions are set.
				statecheck.ExpectKnownValue("data.rubrik_gcp_permissions.features", tfjsonpath.New(keyPermissions),
					knownvalue.NotNull()),
				statecheck.CompareValuePairs(
					"data.rubrik_gcp_permissions.features", tfjsonpath.New(keyID),
					"data.rubrik_gcp_permissions.features", tfjsonpath.New(keyHash),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccGCPPermissionsDataSourceFrameworkMigration verifies that the migrated
// gcp_permissions data source returns the same values as the SDKv2
// implementation in the polaris provider.
func TestAccGCPPermissionsDataSourceFrameworkMigration(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	attrs := []string{keyID, keyHash, keyConditions, keyPermissionGroups, keyPermissions, keyServices,
		keyWithConditions, keyWithoutConditions}

	var checks []statecheck.StateCheck
	for _, pair := range [][2]string{{"old_feature", "new_feature"}, {"old_groups", "new_groups"},
		{"old_newer_feature", "new_newer_feature"}, {"old_features", "new_features"}} {
		for _, attr := range attrs {
			checks = append(checks, statecheck.CompareValuePairs(
				"data.polaris_gcp_permissions."+pair[0], tfjsonpath.New(attr),
				"data.polaris_gcp_permissions."+pair[1], tfjsonpath.New(attr),
				compare.ValuesSame()))
		}
	}

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.12.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that every attribute matches the SDKv2 implementation,
			// for a feature, a feature with permission groups, a feature added
			// after polaris v1.6.3 and the deprecated features field.
			Config: `
				variable "credentials" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				data "polaris_gcp_permissions" "old_feature" {
					provider = polaris-sdkv2
					feature  = "CLOUD_NATIVE_PROTECTION"
				}

				data "polaris_gcp_permissions" "new_feature" {
					feature = "CLOUD_NATIVE_PROTECTION"
				}

				data "polaris_gcp_permissions" "old_groups" {
					provider          = polaris-sdkv2
					feature           = "CLOUD_NATIVE_ARCHIVAL"
					permission_groups = ["BASIC", "ENCRYPTION"]
				}

				data "polaris_gcp_permissions" "new_groups" {
					feature           = "CLOUD_NATIVE_ARCHIVAL"
					permission_groups = ["BASIC", "ENCRYPTION"]
				}

				data "polaris_gcp_permissions" "old_newer_feature" {
					provider          = polaris-sdkv2
					feature           = "CLOUD_SQL_PROTECTION"
					permission_groups = ["BASIC", "EXPORT_AND_RESTORE"]
				}

				data "polaris_gcp_permissions" "new_newer_feature" {
					feature           = "CLOUD_SQL_PROTECTION"
					permission_groups = ["BASIC", "EXPORT_AND_RESTORE"]
				}

				data "polaris_gcp_permissions" "old_features" {
					provider = polaris-sdkv2
					features = ["CLOUD_NATIVE_PROTECTION", "CLOUD_NATIVE_ARCHIVAL"]
				}

				data "polaris_gcp_permissions" "new_features" {
					features = ["CLOUD_NATIVE_PROTECTION", "CLOUD_NATIVE_ARCHIVAL"]
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
			},
			ConfigStateChecks: checks,
		}},
	})
}
