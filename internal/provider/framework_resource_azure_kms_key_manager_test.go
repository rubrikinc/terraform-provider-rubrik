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
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAzureKMSKeyManagerResource onboards the Azure subscription with the
// UEM key management feature, registers a key manager, updates and imports it.
func TestAccAzureKMSKeyManagerResource(t *testing.T) {
	skipUnlessAzureKMSDefined(t)
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	name := testUniqueName(t)

	const conf = `
		variable "credentials" {
			type      = string
			sensitive = true
		}
		variable "tenant_domain" {
			type = string
		}
		variable "subscription_id" {
			type = string
		}
		variable "subscription_name" {
			type = string
		}
		variable "kms_region" {
			type = string
		}
		variable "vault_name" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "description" {
			type = string
		}

		resource "rubrik_azure_service_principal" "service_principal" {
			credentials   = var.credentials
			tenant_domain = var.tenant_domain
		}

		resource "rubrik_azure_subscription" "subscription" {
			subscription_id   = var.subscription_id
			subscription_name = var.subscription_name
			tenant_domain     = var.tenant_domain

			cloud_native_uem_key_management {
				permission_groups = ["BASIC"]
			}

			depends_on = [rubrik_azure_service_principal.service_principal]
		}

		resource "rubrik_azure_kms_key_manager" "key_manager" {
			name             = var.name
			description      = var.description
			cloud_account_id = rubrik_azure_subscription.subscription.id
			region           = var.kms_region
			vault_name       = var.vault_name
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureKMSKeyManagerCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that a key manager can be registered for the
			// subscription.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"name":              config.StringVariable(name),
				"description":       config.StringVariable("Test key manager."),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_azure_kms_key_manager.key_manager",
					tfjsonpath.New(keyDescription), knownvalue.StringExact("Test key manager.")),
				statecheck.ExpectKnownValue("rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyRegion),
					knownvalue.StringExact(testAzureKMSRegion(t))),
				statecheck.ExpectKnownValue("rubrik_azure_kms_key_manager.key_manager",
					tfjsonpath.New(keyVaultName), knownvalue.StringExact(testAzureKMSVaultName(t))),
				statecheck.CompareValuePairs(
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectIdentity("rubrik_azure_kms_key_manager.key_manager", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_kms_key_manager.key_manager",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name and the description are updated in place.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"name":              config.StringVariable(name + " Update"),
				"description":       config.StringVariable("Updated test key manager."),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_kms_key_manager.key_manager",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectKnownValue("rubrik_azure_kms_key_manager.key_manager",
					tfjsonpath.New(keyDescription), knownvalue.StringExact("Updated test key manager.")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_kms_key_manager.key_manager",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_azure_kms_key_manager.key_manager",
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"name":              config.StringVariable(name + " Update"),
				"description":       config.StringVariable("Updated test key manager."),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_azure_kms_key_manager.key_manager",
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"name":              config.StringVariable(name + " Update"),
				"description":       config.StringVariable("Updated test key manager."),
			},
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName: "rubrik_azure_kms_key_manager.key_manager",
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"name":              config.StringVariable(name + " Update"),
				"description":       config.StringVariable("Updated test key manager."),
			},
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
