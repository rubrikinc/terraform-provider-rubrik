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

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customTagsCheckDestroy(t, core.CloudVendorGCP),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created.
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
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("value1"),
						tagKey2: knownvalue.StringExact("value2"),
					})),
				// The override_resource_tags field defaults to true.
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyOverrideResourceLabels),
					knownvalue.Bool(true)),
			},
		}, {
			// Verify that the resource can be updated. One tag is added, one is
			// removed, one has its value changed and the override field is
			// explicitly turned off.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}

				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key1) = "updated1"
						(var.key2) = "value3"
					}

					override_resource_labels = false
				}
			`,
			ConfigVariables: config.Variables{
				"key1": config.StringVariable(tagKey1),
				"key2": config.StringVariable(tagKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyCustomLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("updated1"),
						tagKey3: knownvalue.StringExact("value3"),
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

				resource "rubrik_gcp_custom_labels" "tags" {
					custom_labels = {
						(var.key1) = "updated1"
						(var.key2) = "value3"
					}
				}
			`,
			ConfigVariables: config.Variables{
				"key1": config.StringVariable(tagKey1),
				"key2": config.StringVariable(tagKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_custom_labels.tags", tfjsonpath.New(keyOverrideResourceLabels),
					knownvalue.Bool(true)),
			},
		}, {
			// Terraform import. Note, the import will take over all custom tags
			// of the cloud vendor, not just the ones managed by this resource,
			// so the test will fail if there are existing tags.
			ResourceName:      "rubrik_gcp_custom_labels.tags",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportStateId:     "dummy",
			ImportState:       true,
			ImportStateVerify: true,
			ConfigVariables: config.Variables{
				"key1": config.StringVariable(tagKey1),
				"key2": config.StringVariable(tagKey3),
			},
		}, {
			// import {} block with id attribute. Note, the import will take
			// over all custom tags of the cloud vendor, not just the ones
			// managed by this resource, so the test will fail if there are
			// existing tags.
			ResourceName:    "rubrik_gcp_custom_labels.tags",
			ImportStateKind: resource.ImportBlockWithID,
			ImportStateId:   "dummy",
			ImportState:     true,
			ConfigVariables: config.Variables{
				"key1": config.StringVariable(tagKey1),
				"key2": config.StringVariable(tagKey3),
			},
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
