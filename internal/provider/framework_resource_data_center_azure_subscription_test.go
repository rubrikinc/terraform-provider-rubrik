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
)

// TestAccAzureDataCenterSubscriptionResource adds the Azure subscription to
// RSC as a data center subscription, updates and imports it. The name is
// unique since RSC doesn't allow names of removed data center subscriptions to
// be reused.
func TestAccAzureDataCenterSubscriptionResource(t *testing.T) {
	name := testUniqueName(t)

	vars := config.Variables{
		"name":            config.StringVariable(name),
		"subscription_id": config.StringVariable(testAzureSubscriptionID(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             dataCenterAzureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the data center subscription can be added without a
			// description.
			Config: `
				variable "name" {
					type = string
				}
				variable "subscription_id" {
					type = string
				}

				resource "rubrik_data_center_azure_subscription" "subscription" {
					name            = var.name
					subscription_id = var.subscription_id
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyDescription), knownvalue.StringExact("")),
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyName), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keySubscriptionID), knownvalue.StringExact(testAzureSubscriptionID(t))),
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyTenantID), knownvalue.StringExact(testAzureTenantID(t))),
				statecheck.ExpectIdentity("rubrik_data_center_azure_subscription.subscription",
					map[string]knownvalue.Check{
						keyID: NonNullUUID(),
					}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name is updated and the description is added in
			// place.
			Config: `
				variable "name" {
					type = string
				}
				variable "subscription_id" {
					type = string
				}

				resource "rubrik_data_center_azure_subscription" "subscription" {
					name            = var.name
					description     = "Test Description"
					subscription_id = var.subscription_id
				}
			`,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"name": config.StringVariable(name + "-update"),
			}),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_data_center_azure_subscription.subscription",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyDescription), knownvalue.StringExact("Test Description")),
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyName), knownvalue.StringExact(name+"-update")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_data_center_azure_subscription.subscription",
			ConfigVariables: testVariables(t, vars, config.Variables{
				"name": config.StringVariable(name + "-update"),
			}),
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_data_center_azure_subscription.subscription",
			ConfigVariables: testVariables(t, vars, config.Variables{
				"name": config.StringVariable(name + "-update"),
			}),
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName: "rubrik_data_center_azure_subscription.subscription",
			ConfigVariables: testVariables(t, vars, config.Variables{
				"name": config.StringVariable(name + "-update"),
			}),
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

// TestAccAzureDataCenterSubscriptionResource_FrameworkMigration verifies that
// existing state created by the SDKv2 provider can be read by the Framework
// provider without drift. Step 1 creates the resource without a description
// using the published SDKv2 provider, step 2 refreshes the state using the
// local Framework provider and asserts that the plan is empty.
func TestAccAzureDataCenterSubscriptionResource_FrameworkMigration(t *testing.T) {
	name := testUniqueName(t)

	const conf = `
		variable "name" {
			type = string
		}
		variable "subscription_id" {
			type = string
		}

		resource "rubrik_data_center_azure_subscription" "subscription" {
			name            = var.name
			subscription_id = var.subscription_id
		}
	`

	vars := config.Variables{
		"name":            config.StringVariable(name),
		"subscription_id": config.StringVariable(testAzureSubscriptionID(t)),
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: dataCenterAzureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"rubrik": {
					Source:            "rubrikinc/rubrik",
					VersionConstraint: "1.11.2",
				},
			},
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyID),
					NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables:          vars,
			PlanOnly:                 true,
		}},
	})
}

// TestAccAzureDataCenterSubscriptionResource_MoveState verifies that state
// from a polaris_data_center_azure_subscription resource created by the
// rubrikinc/polaris provider can be moved to a
// rubrik_data_center_azure_subscription resource using the moved {} block.
func TestAccAzureDataCenterSubscriptionResource_MoveState(t *testing.T) {
	name := testUniqueName(t)

	vars := config.Variables{
		"credentials":     config.StringVariable(testCredentials(t)),
		"name":            config.StringVariable(name),
		"subscription_id": config.StringVariable(testAzureSubscriptionID(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: dataCenterAzureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "credentials" {
					type = string
				}
				variable "name" {
					type = string
				}
				variable "subscription_id" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_data_center_azure_subscription" "subscription" {
					name            = var.name
					subscription_id = var.subscription_id
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_data_center_azure_subscription.subscription",
					tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "credentials" {
					type = string
				}
				variable "name" {
					type = string
				}
				variable "subscription_id" {
					type = string
				}

				moved {
					from = polaris_data_center_azure_subscription.subscription
					to   = rubrik_data_center_azure_subscription.subscription
				}

				resource "rubrik_data_center_azure_subscription" "subscription" {
					name            = var.name
					subscription_id = var.subscription_id
				}
			`,
			ConfigVariables: vars,
			// Verify the plan is empty, move succeeded without drift, and
			// apply to update the state.
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
