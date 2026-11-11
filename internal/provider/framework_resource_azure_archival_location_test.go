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

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAzureArchivalLocationResource onboards the Azure subscription with the
// Cloud Native Archival feature, creates a specific region archival location,
// updates and imports it.
func TestAccAzureArchivalLocationResource(t *testing.T) {
	skipUnlessAzureArchivalDefined(t)

	name := testUniqueName(t)
	prefix := testUniqueStorageAccountNamePrefix()

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
		variable "region" {
			type = string
		}
		variable "resource_group_name" {
			type = string
		}
		variable "resource_group_region" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "storage_account_name_prefix" {
			type = string
		}
		variable "storage_tier" {
			type = string
		}
		variable "storage_account_tags" {
			type = map(string)
		}

		resource "rubrik_azure_service_principal" "service_principal" {
			credentials   = var.credentials
			tenant_domain = var.tenant_domain
		}

		resource "rubrik_azure_subscription" "subscription" {
			subscription_id   = var.subscription_id
			subscription_name = var.subscription_name
			tenant_domain     = var.tenant_domain

			cloud_native_archival {
				permission_groups     = ["BASIC"]
				regions               = [var.region]
				resource_group_name   = var.resource_group_name
				resource_group_region = var.resource_group_region
			}

			depends_on = [rubrik_azure_service_principal.service_principal]
		}

		resource "rubrik_azure_archival_location" "location" {
			cloud_account_id            = rubrik_azure_subscription.subscription.id
			name                        = var.name
			storage_account_name_prefix = var.storage_account_name_prefix
			storage_account_region      = var.region
			storage_tier                = var.storage_tier
			storage_account_tags        = var.storage_account_tags
		}
	`

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureArchivalLocationCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the subscription can be onboarded with the Cloud
			// Native Archival feature and that an archival location can be
			// created. The storage account tags are set by the update, since
			// RSC doesn't store tags given on create.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"storage_tier":                config.StringVariable("COOL"),
				"storage_account_tags":        config.MapVariable(map[string]config.Variable{}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_subscription.subscription",
					tfjsonpath.New(keyCloudNativeArchival).AtSliceIndex(0).AtMapKey(keyStatus),
					knownvalue.StringExact("CONNECTED")),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.CompareValuePairs(
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyStorageAccountRegion), knownvalue.StringExact(testAzureArchivalRegion(t))),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyStorageTier), knownvalue.StringExact("COOL")),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyStorageAccountTags), knownvalue.MapSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_azure_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name, the storage tier and the storage account
			// tags are updated in place.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"storage_tier":                config.StringVariable("HOT"),
				"storage_account_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyStorageTier), knownvalue.StringExact("HOT")),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyStorageAccountTags), knownvalue.MapExact(map[string]knownvalue.Check{
						"environment": knownvalue.StringExact("test"),
						"owner":       knownvalue.StringExact("terraform"),
					})),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_azure_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"storage_tier":                config.StringVariable("HOT"),
				"storage_account_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_azure_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"storage_tier":                config.StringVariable("HOT"),
				"storage_account_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
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
			ResourceName: "rubrik_azure_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"storage_tier":                config.StringVariable("HOT"),
				"storage_account_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
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

// TestAccAzureArchivalLocationKMSKeyResource onboards the Azure subscription
// with the Cloud Native Archival and UEM key management features, registers a
// key manager, creates a client-side encrypted archival location, renames it,
// rekeys it and imports it.
func TestAccAzureArchivalLocationKMSKeyResource(t *testing.T) {
	skipUnlessAzureArchivalDefined(t)
	skipUnlessAzureKMSDefined(t)
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	name := testUniqueName(t)
	prefix := testUniqueStorageAccountNamePrefix()

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
		variable "region" {
			type = string
		}
		variable "resource_group_name" {
			type = string
		}
		variable "resource_group_region" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "storage_account_name_prefix" {
			type = string
		}
		variable "kms_region" {
			type = string
		}
		variable "vault_name" {
			type = string
		}
		variable "key_name" {
			type = string
		}
		variable "key_version" {
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

			cloud_native_archival {
				permission_groups     = ["BASIC"]
				regions               = [var.region]
				resource_group_name   = var.resource_group_name
				resource_group_region = var.resource_group_region
			}

			cloud_native_uem_key_management {
				permission_groups = ["BASIC"]
			}

			depends_on = [rubrik_azure_service_principal.service_principal]
		}

		resource "rubrik_azure_key_manager" "key_manager" {
			name             = var.name
			cloud_account_id = rubrik_azure_subscription.subscription.id
			region           = var.kms_region
			vault_name       = var.vault_name
		}

		resource "rubrik_azure_archival_location" "location" {
			cloud_account_id            = rubrik_azure_subscription.subscription.id
			name                        = var.name
			storage_account_name_prefix = var.storage_account_name_prefix
			storage_account_region      = var.region

			key_vault_key {
				key_manager_id = rubrik_azure_key_manager.key_manager.id
				key_name       = var.key_name
				key_version    = var.key_version
			}
		}

		data "rubrik_azure_archival_location" "location" {
			id = rubrik_azure_archival_location.location.id
		}
	`

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureArchivalLocationCheckDestroy(t),
			azureKeyManagerCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that a client-side encrypted archival location can be
			// created with a key from the key manager.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[0].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[0].Version),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testAzureKMSKeys(t)[0].Name)),
				statecheck.CompareValuePairs(
					"rubrik_azure_key_manager.key_manager", tfjsonpath.New(keyID),
					"rubrik_azure_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyManagerID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyCustomerManagedKey), knownvalue.SetSizeExact(0)),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyKeyVaultKey),
					"data.rubrik_azure_archival_location.location", tfjsonpath.New(keyKeyVaultKey),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.location",
					tfjsonpath.New(keyCustomerManagedKey), knownvalue.SetSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_azure_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that a client-side encrypted archival location can be
			// renamed, an update must not send customer managed keys.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[0].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[0].Version),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that changing the key name and the key version rekeys the
			// archival location in place.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[1].Version),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testAzureKMSKeys(t)[1].Name)),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyVersion),
					knownvalue.StringExact(testAzureKMSKeys(t)[1].Version)),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyKeyVaultKey),
					"data.rubrik_azure_archival_location.location", tfjsonpath.New(keyKeyVaultKey),
					compare.ValuesSame()),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import, verifying that the key vault key is read back.
			ResourceName: "rubrik_azure_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[1].Version),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_azure_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[1].Version),
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
			ResourceName: "rubrik_azure_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(prefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[1].Version),
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

// TestAccAzureArchivalLocationResource_FrameworkMigration verifies that
// existing state created by the SDKv2 provider can be read by the Framework
// provider without drift. Step 1 creates a specific region and a source region
// archival location using the published SDKv2 provider, step 2 plans using the
// local Framework provider and asserts that the plan is empty.
func TestAccAzureArchivalLocationResource_FrameworkMigration(t *testing.T) {
	skipUnlessAzureArchivalDefined(t)

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
		variable "region" {
			type = string
		}
		variable "resource_group_name" {
			type = string
		}
		variable "resource_group_region" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "name_prefix" {
			type = string
		}
		variable "source_region_name" {
			type = string
		}
		variable "source_region_name_prefix" {
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

			cloud_native_archival {
				permission_groups     = ["BASIC"]
				regions               = [var.region]
				resource_group_name   = var.resource_group_name
				resource_group_region = var.resource_group_region
			}

			depends_on = [rubrik_azure_service_principal.service_principal]
		}

		resource "rubrik_azure_archival_location" "location" {
			cloud_account_id            = rubrik_azure_subscription.subscription.id
			name                        = var.name
			storage_account_name_prefix = var.name_prefix
			storage_account_region      = var.region
		}

		resource "rubrik_azure_archival_location" "source_region" {
			cloud_account_id            = rubrik_azure_subscription.subscription.id
			name                        = var.source_region_name
			storage_account_name_prefix = var.source_region_name_prefix
		}
	`

	vars := config.Variables{
		"credentials":               config.StringVariable(testAzureCredentials(t)),
		"tenant_domain":             config.StringVariable(testAzureTenantDomain(t)),
		"subscription_id":           config.StringVariable(testAzureSubscriptionID(t)),
		"subscription_name":         config.StringVariable(testAzureSubscriptionName(t)),
		"region":                    config.StringVariable(testAzureArchivalRegion(t)),
		"resource_group_name":       config.StringVariable(testAzureArchivalResourceGroupName(t)),
		"resource_group_region":     config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
		"name":                      config.StringVariable(testUniqueName(t)),
		"name_prefix":               config.StringVariable(testUniqueStorageAccountNamePrefix()),
		"source_region_name":        config.StringVariable(testUniqueName(t)),
		"source_region_name_prefix": config.StringVariable(testUniqueStorageAccountNamePrefix()),
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureArchivalLocationCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
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
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.source_region", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.source_region",
					tfjsonpath.New(keyStorageAccountRegion), knownvalue.StringExact("")),
			},
		}, {
			// Verify that the state created by the SDKv2 provider plans
			// cleanly with the local Framework provider.
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables:          vars,
			PlanOnly:                 true,
		}},
	})
}

// TestAccAzureArchivalLocationResource_MoveState verifies that state from a
// polaris_azure_archival_location resource created by the rubrikinc/polaris
// provider can be moved to a rubrik_azure_archival_location resource using the
// moved {} block. The service principal and the subscription stay with the
// polaris provider, since they are SDKv2 resources in this provider and SDKv2
// doesn't support moving state between resource types.
func TestAccAzureArchivalLocationResource_MoveState(t *testing.T) {
	skipUnlessAzureArchivalDefined(t)

	vars := config.Variables{
		"credentials":                 config.StringVariable(testCredentials(t)),
		"azure_credentials":           config.StringVariable(testAzureCredentials(t)),
		"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
		"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
		"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
		"region":                      config.StringVariable(testAzureArchivalRegion(t)),
		"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
		"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
		"name":                        config.StringVariable(testUniqueName(t)),
		"storage_account_name_prefix": config.StringVariable(testUniqueStorageAccountNamePrefix()),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureArchivalLocationCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "credentials" {
					type      = string
					sensitive = true
				}
				variable "azure_credentials" {
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
				variable "region" {
					type = string
				}
				variable "resource_group_name" {
					type = string
				}
				variable "resource_group_region" {
					type = string
				}
				variable "name" {
					type = string
				}
				variable "storage_account_name_prefix" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_azure_service_principal" "service_principal" {
					credentials   = var.azure_credentials
					tenant_domain = var.tenant_domain
				}

				resource "polaris_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_native_archival {
						permission_groups     = ["BASIC"]
						regions               = [var.region]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					depends_on = [polaris_azure_service_principal.service_principal]
				}

				resource "polaris_azure_archival_location" "location" {
					cloud_account_id            = polaris_azure_subscription.subscription.id
					name                        = var.name
					storage_account_name_prefix = var.storage_account_name_prefix
					storage_account_region      = var.region
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_azure_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
			},
		}, {
			// Verify the plan is empty, move succeeded without drift, and
			// apply to update the state.
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			// The polaris name refers to the external provider, so only the
			// rubrik name is served locally.
			ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
				"rubrik": newMuxedProviderServer,
			},
			Config: `
				variable "credentials" {
					type      = string
					sensitive = true
				}
				variable "azure_credentials" {
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
				variable "region" {
					type = string
				}
				variable "resource_group_name" {
					type = string
				}
				variable "resource_group_region" {
					type = string
				}
				variable "name" {
					type = string
				}
				variable "storage_account_name_prefix" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_azure_service_principal" "service_principal" {
					credentials   = var.azure_credentials
					tenant_domain = var.tenant_domain
				}

				resource "polaris_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_native_archival {
						permission_groups     = ["BASIC"]
						regions               = [var.region]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					depends_on = [polaris_azure_service_principal.service_principal]
				}

				moved {
					from = polaris_azure_archival_location.location
					to   = rubrik_azure_archival_location.location
				}

				resource "rubrik_azure_archival_location" "location" {
					cloud_account_id            = polaris_azure_subscription.subscription.id
					name                        = var.name
					storage_account_name_prefix = var.storage_account_name_prefix
					storage_account_region      = var.region
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
