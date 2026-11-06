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
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAzureSubscriptionDataSource onboards the Azure subscription with the
// Cloud Discovery and Cloud Native Protection features and reads it with the
// data source by ID, by subscription ID, by name, and by name and tenant
// domain.
func TestAccAzureSubscriptionDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the data source can look up the subscription by ID,
			// by subscription ID, by name, and by name and tenant domain, and
			// that it reads back the enabled features.
			Config: `
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
				variable "resource_group_name" {
					type = string
				}
				variable "resource_group_region" {
					type = string
				}

				resource "rubrik_azure_service_principal" "principal" {
					credentials   = var.azure_credentials
					tenant_domain = var.tenant_domain
				}

				resource "rubrik_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["eastus2"]
					}

					cloud_native_protection {
						permission_groups     = ["BASIC"]
						regions               = ["eastus2"]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					depends_on = [rubrik_azure_service_principal.principal]
				}

				data "rubrik_azure_subscription" "by_id" {
					id = rubrik_azure_subscription.subscription.id
				}

				data "rubrik_azure_subscription" "by_subscription_id" {
					subscription_id = rubrik_azure_subscription.subscription.subscription_id
				}

				data "rubrik_azure_subscription" "by_name" {
					name = rubrik_azure_subscription.subscription.subscription_name
				}

				data "rubrik_azure_subscription" "by_name_and_tenant_domain" {
					name          = rubrik_azure_subscription.subscription.subscription_name
					tenant_domain = rubrik_azure_subscription.subscription.tenant_domain
				}
			`,
			ConfigVariables: config.Variables{
				"azure_credentials":     config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":         config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":       config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":     config.StringVariable(testAzureSubscriptionName(t)),
				"resource_group_name":   config.StringVariable(testAzureResourceGroupName(t)),
				"resource_group_region": config.StringVariable(testAzureResourceGroupRegion(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By ID.
				statecheck.CompareValuePairs(
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyName),
					knownvalue.StringExact(testAzureSubscriptionName(t))),
				statecheck.ExpectKnownValue("data.rubrik_azure_subscription.by_id", tfjsonpath.New(keySubscriptionID),
					knownvalue.StringExact(testAzureSubscriptionID(t))),
				statecheck.ExpectKnownValue("data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyTenantDomain),
					knownvalue.StringExact(testAzureTenantDomain(t))),
				statecheck.ExpectKnownValue("data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyName:              knownvalue.StringExact("CLOUD_DISCOVERY"),
							keyStatus:            knownvalue.StringExact("CONNECTED"),
							keyResourceGroupName: knownvalue.Null(),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyName:                knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyStatus:              knownvalue.StringExact("CONNECTED"),
							keyResourceGroupName:   knownvalue.StringExact(testAzureResourceGroupName(t)),
							keyResourceGroupRegion: knownvalue.StringExact(testAzureResourceGroupRegion(t)),
						}),
					})),
				// By subscription ID.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyID),
					"data.rubrik_azure_subscription.by_subscription_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyName),
					"data.rubrik_azure_subscription.by_subscription_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keySubscriptionID),
					"data.rubrik_azure_subscription.by_subscription_id", tfjsonpath.New(keySubscriptionID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyTenantDomain),
					"data.rubrik_azure_subscription.by_subscription_id", tfjsonpath.New(keyTenantDomain),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyFeature),
					"data.rubrik_azure_subscription.by_subscription_id", tfjsonpath.New(keyFeature),
					compare.ValuesSame()),
				// By name.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyID),
					"data.rubrik_azure_subscription.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyName),
					"data.rubrik_azure_subscription.by_name", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keySubscriptionID),
					"data.rubrik_azure_subscription.by_name", tfjsonpath.New(keySubscriptionID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyTenantDomain),
					"data.rubrik_azure_subscription.by_name", tfjsonpath.New(keyTenantDomain),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyFeature),
					"data.rubrik_azure_subscription.by_name", tfjsonpath.New(keyFeature),
					compare.ValuesSame()),
				// By name and tenant domain.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyID),
					"data.rubrik_azure_subscription.by_name_and_tenant_domain", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyName),
					"data.rubrik_azure_subscription.by_name_and_tenant_domain", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keySubscriptionID),
					"data.rubrik_azure_subscription.by_name_and_tenant_domain", tfjsonpath.New(keySubscriptionID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyTenantDomain),
					"data.rubrik_azure_subscription.by_name_and_tenant_domain", tfjsonpath.New(keyTenantDomain),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.by_id", tfjsonpath.New(keyFeature),
					"data.rubrik_azure_subscription.by_name_and_tenant_domain", tfjsonpath.New(keyFeature),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccAzureSubscriptionDataSource_FrameworkMigration verifies that the
// migrated azure_subscription data source is backwards compatible with the
// SDKv2 provider.
func TestAccAzureSubscriptionDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"rubrik-sdkv2": {
				Source:            "rubrikinc/rubrik",
				VersionConstraint: "1.11.2",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the two data sources are equal.
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
				variable "resource_group_name" {
					type = string
				}
				variable "resource_group_region" {
					type = string
				}

				provider "rubrik-sdkv2" {
					credentials = var.credentials
				}

				resource "rubrik_azure_service_principal" "principal" {
					credentials   = var.azure_credentials
					tenant_domain = var.tenant_domain
				}

				resource "rubrik_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["eastus2"]
					}

					cloud_native_protection {
						permission_groups     = ["BASIC"]
						regions               = ["eastus2"]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					depends_on = [rubrik_azure_service_principal.principal]
				}

				data "rubrik_azure_subscription" "old" {
					provider = rubrik-sdkv2

					id = rubrik_azure_subscription.subscription.id
				}

				data "rubrik_azure_subscription" "new" {
					id = rubrik_azure_subscription.subscription.id
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":           config.StringVariable(testCredentials(t)),
				"azure_credentials":     config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":         config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":       config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":     config.StringVariable(testAzureSubscriptionName(t)),
				"resource_group_name":   config.StringVariable(testAzureResourceGroupName(t)),
				"resource_group_region": config.StringVariable(testAzureResourceGroupRegion(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					"data.rubrik_azure_subscription.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.old", tfjsonpath.New(keyID),
					"data.rubrik_azure_subscription.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.old", tfjsonpath.New(keyName),
					"data.rubrik_azure_subscription.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.old", tfjsonpath.New(keySubscriptionID),
					"data.rubrik_azure_subscription.new", tfjsonpath.New(keySubscriptionID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_subscription.old", tfjsonpath.New(keyTenantDomain),
					"data.rubrik_azure_subscription.new", tfjsonpath.New(keyTenantDomain),
					compare.ValuesSame()),
			},
		}},
	})
}
