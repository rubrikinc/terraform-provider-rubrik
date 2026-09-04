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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

func TestAccGcpCustomLabelsResource(t *testing.T) {
	tagKey1 := testUniqueTagKey(t)
	tagKey2 := testUniqueTagKey(t)
	tagKey3 := testUniqueTagKey(t)

	exKey1 := testUniqueTagKey(t)
	exKey2 := testUniqueTagKey(t)
	exKey3 := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customTagsCheckDestroy(t, core.CloudVendorGCP),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created without excluded labels.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key1) = "value1"
						(var.key2) = "value2"
					}
				}
			`,
			ConfigVariables: config.Variables{
				"key1": config.StringVariable(tagKey1),
				"key2": config.StringVariable(tagKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(gcpCustomLabelsID)),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCloudAccountID),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("value1"),
						tagKey2: knownvalue.StringExact("value2"),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyExcludedLabels),
					knownvalue.Null()),
				// The override_resource_tags field defaults to true.
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyOverrideResourceLabels),
					knownvalue.Bool(true)),
			},
		}, {
			// Verify that the resource can be created to include excluded
			// labels.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key1) = "value1"
						(var.key2) = "value2"
					}

					excluded_labels = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey2),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(gcpCustomLabelsID)),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCloudAccountID),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("value1"),
						tagKey2: knownvalue.StringExact("value2"),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey2),
					})),
				// The override_resource_tags field defaults to true.
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyOverrideResourceLabels),
					knownvalue.Bool(true)),
			},
		}, {
			// Verify that the resource can be updated by changing its values.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key1) = "updated1"
						(var.key2) = "value3"
					}

					excluded_labels = [
						var.exKey1,
						var.exKey2,
					]

					override_resource_labels = false
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("updated1"),
						tagKey3: knownvalue.StringExact("value3"),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey3),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyOverrideResourceLabels),
					knownvalue.Bool(false)),
			},
		}, {
			// Restore the override field to its default value. The field refers
			// to a single global value in RSC, so leaving it turned off would
			// affect other tests using the same RSC account.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key1) = "updated1"
						(var.key2) = "value3"
					}

					excluded_labels = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey3),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyOverrideResourceLabels),
					knownvalue.Bool(true)),
			},
		}, {
			// Terraform import. Note, the import will take over all custom tags
			// of the cloud vendor, not just the ones managed by this resource,
			// so the test will fail if there are existing tags.
			ResourceName:      "rubrik_gcp_custom_labels.tags",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportStateId:     keyGlobal,
			ImportState:       true,
			ImportStateVerify: true,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
		}, {
			// import {} block with id attribute. Note, the import will take
			// over all custom tags of the cloud vendor, not just the ones
			// managed by this resource, so the test will fail if there are
			// existing tags.
			ResourceName:    "rubrik_gcp_custom_labels.tags",
			ImportStateKind: resource.ImportBlockWithID,
			ImportStateId:   keyGlobal,
			ImportState:     true,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
		}, {
			// An import ID which is neither a cloud account ID nor global is
			// rejected, so that a malformed cloud account ID does not silently
			// import the global scope.
			ResourceName:    "rubrik_gcp_custom_labels.tags",
			ImportStateKind: resource.ImportCommandWithID,
			ImportStateId:   "not-a-cloud-account-id",
			ImportState:     true,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ExpectError: regexp.MustCompile(`is not a valid import ID`),
		}},
	})
}

// TestAccGcpCustomLabelsResource_ExcludedLabelsOnly verifies that a resource
// can manage excluded labels without any custom labels.
func TestAccGcpCustomLabelsResource_ExcludedLabelsOnly(t *testing.T) {
	exKey1 := testUniqueTagKey(t)
	exKey2 := testUniqueTagKey(t)
	exKey3 := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customTagsCheckDestroy(t, core.CloudVendorGCP),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with only excluded
			// labels.
			Config: `
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_gcp_custom_labels" "tags" {
					excluded_labels = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(gcpCustomLabelsID)),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCloudAccountID),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCustomLabels),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey2),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyOverrideResourceLabels),
					knownvalue.Bool(true)),
			},
		}, {
			// Verify that the resource can be updated. One excluded label is
			// added and one is removed, still without any custom labels.
			Config: `
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_gcp_custom_labels" "tags" {
					excluded_labels = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCustomLabels),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey3),
					})),
			},
		}},
	})
}

// TestAccGcpCustomLabelsResource_CloudAccountScoped verifies that custom labels
// and excluded labels can be scoped to a single cloud account, and that the
// scoped and the global configurations are independent of each other.
func TestAccGcpCustomLabelsResource_CloudAccountScoped(t *testing.T) {
	labelKey := testUniqueTagKey(t)
	exKey1 := testUniqueTagKey(t)
	exKey2 := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpProjectCheckDestroy(t),
			customTagsCheckDestroy(t, core.CloudVendorGCP),
		),
		Steps: []resource.TestStep{{
			// Verify that a scoped resource can be created alongside a global
			// resource sharing the same label key.
			Config: `
				variable "gcp_credentials" {
					type = string
				}
				variable "project_id" {
					type = string
				}
				variable "project_name" {
					type = string
				}
				variable "project_number" {
					type = string
				}
				variable "organization_name" {
					type = string
				}
				variable "key" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}

				resource "rubrik_gcp_project" "project" {
					credentials       = var.gcp_credentials
					project           = var.project_id
					project_name      = var.project_name
					project_number    = var.project_number
					organization_name = var.organization_name

					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "rubrik_gcp_custom_labels" "global" {
					custom_labels = {
						(var.key) = "global"
					}

					excluded_labels = [var.exKey1]
				}

				resource "rubrik_gcp_custom_labels" "account" {
					cloud_account_id = rubrik_gcp_project.project.id

					custom_labels = {
						(var.key) = "account"
					}

					excluded_labels = [var.exKey2]
				}
			`,
			ConfigVariables: config.Variables{
				"gcp_credentials":   config.StringVariable(testGCPCredentials(t)),
				"project_id":        config.StringVariable(testGCPProjectID(t)),
				"project_name":      config.StringVariable(testGCPProjectName(t)),
				"project_number":    config.StringVariable(testGCPProjectNumber(t)),
				"organization_name": config.StringVariable(testGCPOrganizationName(t)),
				"key":               config.StringVariable(labelKey),
				"exKey1":            config.StringVariable(exKey1),
				"exKey2":            config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// The global resource.
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.global", tfjsonpath.New(keyID),
					knownvalue.StringExact(gcpCustomLabelsID)),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.global", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						labelKey: knownvalue.StringExact("global"),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.global", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
					})),

				// The scoped resource.
				statecheck.CompareValuePairs(
					"rubrik_gcp_custom_labels.account", tfjsonpath.New(keyID),
					"rubrik_gcp_custom_labels.account", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.account", tfjsonpath.New(keyCloudAccountID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.account", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						labelKey: knownvalue.StringExact("account"),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.account", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey2),
					})),
			},
		}, {
			// Destroying the global resource must leave the scoped
			// configuration untouched.
			Config: `
				variable "gcp_credentials" {
					type = string
				}
				variable "project_id" {
					type = string
				}
				variable "project_name" {
					type = string
				}
				variable "project_number" {
					type = string
				}
				variable "organization_name" {
					type = string
				}
				variable "key" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}

				resource "rubrik_gcp_project" "project" {
					credentials       = var.gcp_credentials
					project           = var.project_id
					project_name      = var.project_name
					project_number    = var.project_number
					organization_name = var.organization_name

					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "rubrik_gcp_custom_labels" "account" {
					cloud_account_id = rubrik_gcp_project.project.id

					custom_labels = {
						(var.key) = "account"
					}

					excluded_labels = [var.exKey2]
				}
			`,
			ConfigVariables: config.Variables{
				"gcp_credentials":   config.StringVariable(testGCPCredentials(t)),
				"project_id":        config.StringVariable(testGCPProjectID(t)),
				"project_name":      config.StringVariable(testGCPProjectName(t)),
				"project_number":    config.StringVariable(testGCPProjectNumber(t)),
				"organization_name": config.StringVariable(testGCPOrganizationName(t)),
				"key":               config.StringVariable(labelKey),
				"exKey1":            config.StringVariable(exKey1),
				"exKey2":            config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.account", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						labelKey: knownvalue.StringExact("account"),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.account", tfjsonpath.New(keyExcludedLabels),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey2),
					})),
			},
		}, {
			// Terraform import of the scoped resource, using the cloud account
			// ID as the import ID.
			ResourceName:      "rubrik_gcp_custom_labels.account",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportStateIdFunc: customTagsImportID("rubrik_gcp_custom_labels.account"),
			ImportState:       true,
			ImportStateVerify: true,
			ConfigVariables: config.Variables{
				"gcp_credentials":   config.StringVariable(testGCPCredentials(t)),
				"project_id":        config.StringVariable(testGCPProjectID(t)),
				"project_name":      config.StringVariable(testGCPProjectName(t)),
				"project_number":    config.StringVariable(testGCPProjectNumber(t)),
				"organization_name": config.StringVariable(testGCPOrganizationName(t)),
				"key":               config.StringVariable(labelKey),
				"exKey1":            config.StringVariable(exKey1),
				"exKey2":            config.StringVariable(exKey2),
			},
		}},
	})
}

// TestAccGcpCustomLabelsResource_InvalidConfigs verifies that the custom_labels
// and excluded_labels fields reject an explicitly empty collection, and that at
// least one of them must be specified. Terraform distinguishes an unset field
// from an empty one, and only the unset form is meaningful for these fields.
func TestAccGcpCustomLabelsResource_InvalidConfigs(t *testing.T) {
	tagKey := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {}
				}
			`,
			ExpectError: regexp.MustCompile(`custom_labels map must contain at least 1 elements`),
		}, {
			Config: `
				variable "key" {
					type = string
				}

				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key) = "value"
					}

					excluded_labels = []
				}
			`,
			ConfigVariables: config.Variables{
				"key": config.StringVariable(tagKey),
			},
			ExpectError: regexp.MustCompile(`excluded_labels set must contain at least 1 elements`),
		}, {
			Config: `
				resource "rubrik_gcp_custom_labels" "tags" {
				}
			`,
			ExpectError: regexp.MustCompile(`At least one of these attributes must be configured:\s+\[custom_labels,excluded_labels\]`),
		}},
	})
}

// TestAccGcpCustomLabelsResource_FrameworkMigration verifies that existing state
// created by the SDKv2 provider (v1.5.0) can be read by the Framework provider
// without drift. Step 1 creates the resource using the published SDKv2
// provider, step 2 refreshes state using the local Framework provider and
// asserts the plan is empty.
func TestAccGcpCustomLabelsResource_FrameworkMigration(t *testing.T) {
	conf := `
		variable "credentials" {
			type = string
		}
		variable "key" {
			type = string
		}

		provider "polaris" {
			credentials = var.credentials
		}

		resource "polaris_gcp_custom_labels" "tags" {
			custom_labels = {
				(var.key) = "value"
			}
		}
	`
	tagKey := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		CheckDestroy: customTagsCheckDestroy(t, core.CloudVendorGCP),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_custom_labels.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(gcpCustomLabelsID)),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			PlanOnly: true,
		}},
	})
}

// TestAccGcpCustomLabelsResource_MoveState verifies that state from a
// polaris_gcp_custom_labels resource created by the rubrikinc/polaris provider
// can be moved to a rubrik_gcp_custom_labels resource using the moved {} block.
func TestAccGcpCustomLabelsResource_MoveState(t *testing.T) {
	tagKey := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: customTagsCheckDestroy(t, core.CloudVendorGCP),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config: `
				variable "credentials" {
					type = string
				}
				variable "key" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key) = "value"
					}
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_custom_labels.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(gcpCustomLabelsID)),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "credentials" {
					type = string
				}
				variable "key" {
					type = string
				}

				moved {
					from = polaris_gcp_custom_labels.tags
					to   = rubrik_gcp_custom_labels.tags
				}

				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key) = "value"
					}
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			// Verify the plan is empty, move succeeded without drift, and
			// apply to update the state. Without the apply step, destroy can
			// fail due to resource dependency issues.
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
