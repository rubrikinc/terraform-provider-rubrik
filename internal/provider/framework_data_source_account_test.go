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
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/gcp"
)

func TestSortedAWSFeatures(t *testing.T) {
	input := []aws.FeaturePermissionGroups{
		{
			Feature: "EXOCOMPUTE",
			PermissionGroups: []aws.PermissionGroupInfo{
				{PermissionGroup: core.PermissionGroupRSCManagedCluster},
				{PermissionGroup: core.PermissionGroupBasic},
			},
		},
		{
			Feature: "CLOUD_NATIVE_PROTECTION",
			PermissionGroups: []aws.PermissionGroupInfo{
				{PermissionGroup: core.PermissionGroupBasic},
			},
		},
	}

	result := sortedAWSFeatures(input)

	// Verify features are sorted by name.
	if result[0].Name.ValueString() != "CLOUD_NATIVE_PROTECTION" {
		t.Errorf("expected first feature to be CLOUD_NATIVE_PROTECTION, got %s", result[0].Name.ValueString())
	}
	if result[1].Name.ValueString() != "EXOCOMPUTE" {
		t.Errorf("expected second feature to be EXOCOMPUTE, got %s", result[1].Name.ValueString())
	}

	// Verify permission groups are sorted.
	expectedGroups := []string{"BASIC", "RSC_MANAGED_CLUSTER"}
	if !reflect.DeepEqual(result[1].PermissionGroups, expectedGroups) {
		t.Errorf("expected permission groups %v, got %v", expectedGroups, result[1].PermissionGroups)
	}
}

func TestSortedAzureFeatures(t *testing.T) {
	input := []azure.FeaturePermissionGroups{
		{
			Feature: "EXOCOMPUTE",
			PermissionGroups: []azure.PermissionGroupInfo{
				{PermissionGroup: core.PermissionGroupRSCManagedCluster},
				{PermissionGroup: core.PermissionGroupBasic},
			},
		},
		{
			Feature: "CLOUD_NATIVE_PROTECTION",
			PermissionGroups: []azure.PermissionGroupInfo{
				{PermissionGroup: core.PermissionGroupBasic},
			},
		},
	}

	result := sortedAzureFeatures(input)

	// Verify features are sorted by name.
	if result[0].Name.ValueString() != "CLOUD_NATIVE_PROTECTION" {
		t.Errorf("expected first feature to be CLOUD_NATIVE_PROTECTION, got %s", result[0].Name.ValueString())
	}
	if result[1].Name.ValueString() != "EXOCOMPUTE" {
		t.Errorf("expected second feature to be EXOCOMPUTE, got %s", result[1].Name.ValueString())
	}

	// Verify permission groups are sorted.
	expectedGroups := []string{"BASIC", "RSC_MANAGED_CLUSTER"}
	if !reflect.DeepEqual(result[1].PermissionGroups, expectedGroups) {
		t.Errorf("expected permission groups %v, got %v", expectedGroups, result[1].PermissionGroups)
	}
}

func TestSortedGCPFeatures(t *testing.T) {
	input := []gcp.FeaturePermissionGroups{
		{
			Feature: "EXOCOMPUTE",
			PermissionGroups: []gcp.PermissionGroupInfo{
				{PermissionGroup: core.PermissionGroupRSCManagedCluster},
				{PermissionGroup: core.PermissionGroupBasic},
			},
		},
		{
			Feature: "CLOUD_NATIVE_PROTECTION",
			PermissionGroups: []gcp.PermissionGroupInfo{
				{PermissionGroup: core.PermissionGroupBasic},
			},
		},
	}

	result := sortedGCPFeatures(input)

	// Verify features are sorted by name.
	if result[0].Name.ValueString() != "CLOUD_NATIVE_PROTECTION" {
		t.Errorf("expected first feature to be CLOUD_NATIVE_PROTECTION, got %s", result[0].Name.ValueString())
	}
	if result[1].Name.ValueString() != "EXOCOMPUTE" {
		t.Errorf("expected second feature to be EXOCOMPUTE, got %s", result[1].Name.ValueString())
	}

	// Verify permission groups are sorted.
	expectedGroups := []string{"BASIC", "RSC_MANAGED_CLUSTER"}
	if !reflect.DeepEqual(result[1].PermissionGroups, expectedGroups) {
		t.Errorf("expected permission groups %v, got %v", expectedGroups, result[1].PermissionGroups)
	}
}

func TestAccAccountDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the data source returns the account details, the
			// features, the operations and workloads, and the permission groups
			// of the supported cloud vendors.
			Config: `
				data "rubrik_account" "current" {}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{64}$`))),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyName),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyFQDN),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyFeatures),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyOperations),
					knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.NotNull(),
					})),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyWorkloads),
					knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.NotNull(),
					})),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyAws),
					knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyAzure),
					knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.rubrik_account.current", tfjsonpath.New(keyGcp),
					knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.rubrik_account.current",
					tfjsonpath.New(keyAws).AtSliceIndex(0).AtMapKey(keyFeatures).AtSliceIndex(0).AtMapKey(keyPermissionGroups),
					knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.NotNull(),
					})),
			},
		}},
	})
}

// TestAccAccountDataSource_FrameworkMigration verifies that the migrated
// account data source is backwards compatible with the SDKv2 provider.
func TestAccAccountDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.11.2",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the SDKv2 and Framework data sources return
			// identical values for every attribute.
			Config: `
				variable "credentials" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				data "polaris_account" "old" {
					provider = polaris-sdkv2
				}

				data "polaris_account" "new" {}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyID),
					"data.polaris_account.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyName),
					"data.polaris_account.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyFQDN),
					"data.polaris_account.new", tfjsonpath.New(keyFQDN),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyFeatures),
					"data.polaris_account.new", tfjsonpath.New(keyFeatures),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyOperations),
					"data.polaris_account.new", tfjsonpath.New(keyOperations),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyWorkloads),
					"data.polaris_account.new", tfjsonpath.New(keyWorkloads),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyAws),
					"data.polaris_account.new", tfjsonpath.New(keyAws),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyAzure),
					"data.polaris_account.new", tfjsonpath.New(keyAzure),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_account.old", tfjsonpath.New(keyGcp),
					"data.polaris_account.new", tfjsonpath.New(keyGcp),
					compare.ValuesSame()),
			},
		}},
	})
}
