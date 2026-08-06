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
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAzureResourceGroupObjectsDataSource(t *testing.T) {
	// The resource group must appear in both the subscription-scoped read and
	// the search across all subscriptions.
	resourceGroupCheck := knownvalue.SetPartial([]knownvalue.Check{
		knownvalue.ObjectPartial(map[string]knownvalue.Check{
			keyName: knownvalue.StringExact(testAzureResourceGroupName(t)),
		}),
	})

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: ">=0.14.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard the subscription, force native discovery with a refresh
			// so its resource groups become visible, then read the objects data
			// sources gated on that refresh.
			Config: `
				variable "azure_credentials" {
					type = string
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

				resource "time_static" "timestamp" {}

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

				data "rubrik_object" "subscription" {
					name        = rubrik_azure_subscription.subscription.subscription_name
					object_type = "AzureNativeSubscription"
				}

				resource "rubrik_refresh" "subscription" {
					object_id   = data.rubrik_object.subscription.id
					object_type = "AzureNativeSubscription"
					timestamp   = time_static.timestamp.rfc3339
				}

				data "rubrik_objects" "resource_groups" {
					object_type = "AzureNativeResourceGroup"

					depends_on = [rubrik_refresh.subscription]
				}

				data "rubrik_objects" "resource_groups_by_subscription" {
					object_type     = "AzureNativeResourceGroup"
					subscription_id = rubrik_azure_subscription.subscription.id

					depends_on = [rubrik_refresh.subscription]
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
				// Scoped to the fixture's subscription.
				statecheck.ExpectKnownValue("data.rubrik_objects.resource_groups", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(sha256Hex)),
				statecheck.ExpectKnownValue("data.rubrik_objects.resource_groups", tfjsonpath.New(keyObjects),
					resourceGroupCheck),

				// Searching across all subscriptions still finds it.
				statecheck.ExpectKnownValue("data.rubrik_objects.resource_groups_by_subscription", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(sha256Hex)),
				statecheck.ExpectKnownValue("data.rubrik_objects.resource_groups_by_subscription", tfjsonpath.New(keyObjects),
					resourceGroupCheck),
			},
		}},
	})
}
