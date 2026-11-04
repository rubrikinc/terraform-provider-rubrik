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
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAzureArchivalLocationResource onboards the Azure subscription with the
// Cloud Native Archival feature, creates a specific region archival location,
// updates and imports it, and reads it with the data source.
func TestAccAzureArchivalLocationResource(t *testing.T) {
	skipUnlessAzureArchivalDefined(t)

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

	storageAccountNamePrefix := "tfacc" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	resource.Test(t, resource.TestCase{
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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
					tfjsonpath.New(keyStorageAccountTags), knownvalue.Null()),
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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
			},
		}, {
			// Verify that the data source reads the archival location by ID
			// and by name.
			Config: conf + `
				data "rubrik_azure_archival_location" "by_id" {
					id = rubrik_azure_archival_location.location.id
				}
				data "rubrik_azure_archival_location" "by_name" {
					name = rubrik_azure_archival_location.location.name
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
				"storage_tier":                config.StringVariable("HOT"),
				"storage_account_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyName),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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
			// import {} block with id attribute. The resource has no identity,
			// so it can't be imported with an identity.
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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

		resource "rubrik_azure_kms_key_manager" "key_manager" {
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
				key_manager_id = rubrik_azure_kms_key_manager.key_manager.id
				key_name       = var.key_name
				key_version    = var.key_version
			}
		}
	`

	storageAccountNamePrefix := "tfacc" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureArchivalLocationCheckDestroy(t),
			azureKMSKeyManagerCheckDestroy(t),
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"rubrik_azure_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyManagerID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.location",
					tfjsonpath.New(keyCustomerManagedKey), knownvalue.SetSizeExact(0)),
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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
			},
		}, {
			// Verify that the key manager data source reads the key manager by
			// ID and by name.
			Config: conf + `
				data "rubrik_azure_kms_key_manager" "by_id" {
					id = rubrik_azure_kms_key_manager.key_manager.id
				}
				data "rubrik_azure_kms_key_manager" "by_name" {
					name = rubrik_azure_kms_key_manager.key_manager.name
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":                 config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":               config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":             config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":           config.StringVariable(testAzureSubscriptionName(t)),
				"region":                      config.StringVariable(testAzureArchivalRegion(t)),
				"resource_group_name":         config.StringVariable(testAzureArchivalResourceGroupName(t)),
				"resource_group_region":       config.StringVariable(testAzureArchivalResourceGroupRegion(t)),
				"name":                        config.StringVariable(name + " Update"),
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[0].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[0].Version),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyName),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyRegion),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyRegion),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_azure_kms_key_manager.by_id",
					tfjsonpath.New(keyProvisioningMethod), knownvalue.StringExact("CLOUD_ACCOUNT_PROVISION")),
				statecheck.ExpectKnownValue("data.rubrik_azure_kms_key_manager.by_id",
					tfjsonpath.New(keyDataLocationsCount), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.rubrik_azure_kms_key_manager.by_id",
					tfjsonpath.New(keyKeyVaultURL), knownvalue.NotNull()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyID),
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
				"kms_region":                  config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":                  config.StringVariable(testAzureKMSVaultName(t)),
				"key_name":                    config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":                 config.StringVariable(testAzureKMSKeys(t)[1].Version),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute. The resource has no identity,
			// so it can't be imported with an identity.
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
				"storage_account_name_prefix": config.StringVariable(storageAccountNamePrefix),
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
		}},
	})
}
