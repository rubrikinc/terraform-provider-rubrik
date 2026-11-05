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

// TestAccAzureKeyManagerDataSource onboards the Azure subscription with the
// UEM key management feature, registers a key manager and reads it with the
// data source by ID and by name.
func TestAccAzureKeyManagerDataSource(t *testing.T) {
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

		resource "rubrik_azure_key_manager" "key_manager" {
			name             = var.name
			cloud_account_id = rubrik_azure_subscription.subscription.id
			region           = var.kms_region
			vault_name       = var.vault_name
		}

		data "rubrik_azure_key_manager" "by_id" {
			id = rubrik_azure_key_manager.key_manager.id
		}

		data "rubrik_azure_key_manager" "by_name" {
			name = rubrik_azure_key_manager.key_manager.name
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureKeyManagerCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the data source reads the key manager by ID and by
			// name.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"name":              config.StringVariable(name),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By ID.
				statecheck.CompareValuePairs(
					"rubrik_azure_key_manager.key_manager", tfjsonpath.New(keyID),
					"data.rubrik_azure_key_manager.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_key_manager.key_manager", tfjsonpath.New(keyName),
					"data.rubrik_azure_key_manager.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_key_manager.key_manager", tfjsonpath.New(keyDescription),
					"data.rubrik_azure_key_manager.by_id", tfjsonpath.New(keyDescription),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_azure_key_manager.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_key_manager.key_manager", tfjsonpath.New(keyRegion),
					"data.rubrik_azure_key_manager.by_id", tfjsonpath.New(keyRegion),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_azure_key_manager.by_id",
					tfjsonpath.New(keyProvisioningMethod), knownvalue.StringExact("CLOUD_ACCOUNT_PROVISION")),
				statecheck.ExpectKnownValue("data.rubrik_azure_key_manager.by_id",
					tfjsonpath.New(keyKeyVaultURL), knownvalue.NotNull()),
				// By name.
				statecheck.CompareValuePairs(
					"rubrik_azure_key_manager.key_manager", tfjsonpath.New(keyID),
					"data.rubrik_azure_key_manager.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}},
	})
}
