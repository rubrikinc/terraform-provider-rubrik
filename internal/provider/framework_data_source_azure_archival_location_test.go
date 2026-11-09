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
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAzureArchivalLocationDataSource onboards the Azure subscription with
// the Cloud Native Archival feature, creates a specific region archival
// location and reads it with the data source by ID, by the deprecated archival
// location ID and by name.
func TestAccAzureArchivalLocationDataSource(t *testing.T) {
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
			storage_account_tags        = var.storage_account_tags
		}

		data "rubrik_azure_archival_location" "by_id" {
			id = rubrik_azure_archival_location.location.id
		}

		data "rubrik_azure_archival_location" "by_archival_location_id" {
			archival_location_id = rubrik_azure_archival_location.location.id
		}

		data "rubrik_azure_archival_location" "by_name" {
			name = rubrik_azure_archival_location.location.name
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
			// Verify that the data source can look up the archival location
			// by ID, by archival location ID and by name. The location has no
			// storage account tags, so the tags must be empty. The tags are
			// set by the update, since RSC doesn't store tags given on create.
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
				"storage_account_tags":        config.MapVariable(map[string]config.Variable{}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By ID.
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyArchivalLocationID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id",
					tfjsonpath.New(keyStorageAccountRegion), knownvalue.StringExact(testAzureArchivalRegion(t))),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id",
					tfjsonpath.New(keyStorageTier), knownvalue.StringExact("COOL")),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id",
					tfjsonpath.New(keyStorageAccountTags), knownvalue.MapSizeExact(0)),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id",
					tfjsonpath.New(keyCustomerManagedKey), knownvalue.SetSizeExact(0)),
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id",
					tfjsonpath.New(keyKeyVaultKey), knownvalue.ListSizeExact(0)),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyConnectionStatus),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyConnectionStatus),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyContainerName),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyContainerName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyNetworkAccessType),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyNetworkAccessType),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyRedundancy),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyRedundancy),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyStorageAccountNamePrefix),
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountNamePrefix),
					compare.ValuesSame()),
				// By archival location ID.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyArchivalLocationID),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyArchivalLocationID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyName),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountRegion),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyStorageAccountRegion),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyLocationTemplate),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyLocationTemplate),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageTier),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyStorageTier),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountTags),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyStorageAccountTags),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyCustomerManagedKey),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyCustomerManagedKey),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyConnectionStatus),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyConnectionStatus),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyContainerName),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyContainerName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyNetworkAccessType),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyNetworkAccessType),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyRedundancy),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyRedundancy),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountNamePrefix),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyStorageAccountNamePrefix),
					compare.ValuesSame()),
				// By name.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyArchivalLocationID),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyArchivalLocationID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyName),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountRegion),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyStorageAccountRegion),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyLocationTemplate),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyLocationTemplate),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageTier),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyStorageTier),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountTags),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyStorageAccountTags),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyCustomerManagedKey),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyCustomerManagedKey),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyConnectionStatus),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyConnectionStatus),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyContainerName),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyContainerName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyNetworkAccessType),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyNetworkAccessType),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyRedundancy),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyRedundancy),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountNamePrefix),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyStorageAccountNamePrefix),
					compare.ValuesSame()),
			},
		}, {
			// Verify that the data source reads the storage account tags.
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
				"storage_account_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
				}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By ID.
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.by_id",
					tfjsonpath.New(keyStorageAccountTags), knownvalue.MapExact(map[string]knownvalue.Check{
						"environment": knownvalue.StringExact("test"),
					})),
				// By archival location ID.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountTags),
					"data.rubrik_azure_archival_location.by_archival_location_id", tfjsonpath.New(keyStorageAccountTags),
					compare.ValuesSame()),
				// By name.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.by_id", tfjsonpath.New(keyStorageAccountTags),
					"data.rubrik_azure_archival_location.by_name", tfjsonpath.New(keyStorageAccountTags),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccAzureArchivalLocationDataSource_FrameworkMigration verifies that the
// migrated azure_archival_location data source is backwards compatible with
// the SDKv2 provider, both without and with storage account tags.
func TestAccAzureArchivalLocationDataSource_FrameworkMigration(t *testing.T) {
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
		variable "storage_account_tags" {
			type = map(string)
		}

		provider "rubrik-sdkv2" {}

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
			storage_account_tags        = var.storage_account_tags
		}

		data "rubrik_azure_archival_location" "old" {
			provider = rubrik-sdkv2

			id = rubrik_azure_archival_location.location.id
		}

		data "rubrik_azure_archival_location" "new" {
			id = rubrik_azure_archival_location.location.id
		}
	`

	storageAccountNamePrefix := "tfacc" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"rubrik-sdkv2": {
				Source:            "rubrikinc/rubrik",
				VersionConstraint: "1.11.2",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureArchivalLocationCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the two data sources are equal for an archival
			// location without storage account tags. The tags are set by the
			// update, since RSC doesn't store tags given on create.
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
				"storage_account_tags":        config.MapVariable(map[string]config.Variable{}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_azure_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyID),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyArchivalLocationID),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyArchivalLocationID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyName),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyStorageAccountRegion),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyStorageAccountRegion),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyLocationTemplate),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyLocationTemplate),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyStorageTier),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyStorageTier),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyStorageAccountTags),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyStorageAccountTags),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyCustomerManagedKey),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyCustomerManagedKey),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyConnectionStatus),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyConnectionStatus),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyContainerName),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyContainerName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyNetworkAccessType),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyNetworkAccessType),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyRedundancy),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyRedundancy),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyStorageAccountNamePrefix),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyStorageAccountNamePrefix),
					compare.ValuesSame()),
			},
		}, {
			// Verify that the two data sources are equal for an archival
			// location with storage account tags.
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
				"storage_account_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
				}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_azure_archival_location.new",
					tfjsonpath.New(keyStorageAccountTags), knownvalue.MapExact(map[string]knownvalue.Check{
						"environment": knownvalue.StringExact("test"),
					})),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_archival_location.old", tfjsonpath.New(keyStorageAccountTags),
					"data.rubrik_azure_archival_location.new", tfjsonpath.New(keyStorageAccountTags),
					compare.ValuesSame()),
			},
		}},
	})
}
