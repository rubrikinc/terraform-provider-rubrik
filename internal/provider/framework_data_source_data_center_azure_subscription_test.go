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

// TestAccAzureDataCenterSubscriptionDataSource adds the Azure subscription to
// RSC as a data center subscription and reads it with the data source by name.
// The name is unique since RSC doesn't allow names of removed data center
// subscriptions to be reused.
func TestAccAzureDataCenterSubscriptionDataSource(t *testing.T) {
	name := testUniqueName(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             dataCenterAzureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the data source can look up the data center
			// subscription by name.
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

				data "rubrik_data_center_azure_subscription" "subscription" {
					name = rubrik_data_center_azure_subscription.subscription.name
				}
			`,
			ConfigVariables: config.Variables{
				"name":            config.StringVariable(name),
				"subscription_id": config.StringVariable(testAzureSubscriptionID(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyID),
					"data.rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyConnectionStatus),
					knownvalue.StringExact("CONNECTED")),
				statecheck.ExpectKnownValue("data.rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Test Description")),
				statecheck.ExpectKnownValue("data.rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("data.rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keySubscriptionID),
					knownvalue.StringExact(testAzureSubscriptionID(t))),
				statecheck.CompareValuePairs(
					"rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyTenantID),
					"data.rubrik_data_center_azure_subscription.subscription", tfjsonpath.New(keyTenantID),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccAzureDataCenterSubscriptionDataSource_FrameworkMigration verifies
// that the migrated data_center_azure_subscription data source is backwards
// compatible with the SDKv2 provider.
func TestAccAzureDataCenterSubscriptionDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"rubrik-sdkv2": {
				Source:            "rubrikinc/rubrik",
				VersionConstraint: "1.11.2",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             dataCenterAzureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the two data sources are equal.
			Config: `
				variable "name" {
					type = string
				}
				variable "subscription_id" {
					type = string
				}

				provider "rubrik-sdkv2" {}

				resource "rubrik_data_center_azure_subscription" "subscription" {
					name            = var.name
					description     = "Test Description"
					subscription_id = var.subscription_id
				}

				data "rubrik_data_center_azure_subscription" "old" {
					provider = rubrik-sdkv2

					name = rubrik_data_center_azure_subscription.subscription.name
				}

				data "rubrik_data_center_azure_subscription" "new" {
					name = rubrik_data_center_azure_subscription.subscription.name
				}
			`,
			ConfigVariables: config.Variables{
				"name":            config.StringVariable(testUniqueName(t)),
				"subscription_id": config.StringVariable(testAzureSubscriptionID(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.rubrik_data_center_azure_subscription.old", tfjsonpath.New(keyID),
					"data.rubrik_data_center_azure_subscription.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_data_center_azure_subscription.old", tfjsonpath.New(keyConnectionStatus),
					"data.rubrik_data_center_azure_subscription.new", tfjsonpath.New(keyConnectionStatus),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_data_center_azure_subscription.old", tfjsonpath.New(keyDescription),
					"data.rubrik_data_center_azure_subscription.new", tfjsonpath.New(keyDescription),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_data_center_azure_subscription.old", tfjsonpath.New(keyName),
					"data.rubrik_data_center_azure_subscription.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_data_center_azure_subscription.old", tfjsonpath.New(keySubscriptionID),
					"data.rubrik_data_center_azure_subscription.new", tfjsonpath.New(keySubscriptionID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_data_center_azure_subscription.old", tfjsonpath.New(keyTenantID),
					"data.rubrik_data_center_azure_subscription.new", tfjsonpath.New(keyTenantID),
					compare.ValuesSame()),
			},
		}},
	})
}
