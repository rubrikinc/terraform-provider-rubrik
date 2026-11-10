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
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAzureServicePrincipalResource adds the Azure service principal to RSC,
// first from a credentials file and then from the individual values. There is
// no CheckDestroy, since the service principal cannot be removed from RSC.
func TestAccAzureServicePrincipalResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the service principal can be added from a credentials
			// file.
			Config: `
				variable "credentials" {
					type = string
				}
				variable "tenant_domain" {
					type = string
				}

				resource "rubrik_azure_service_principal" "service_principal" {
					credentials   = var.credentials
					tenant_domain = var.tenant_domain
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":   config.StringVariable(testAzureCredentials(t)),
				"tenant_domain": config.StringVariable(testAzureTenantDomain(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyID), knownvalue.StringExact(testAzurePrincipalID(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyCredentials), knownvalue.StringExact(testAzureCredentials(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyTenantDomain), knownvalue.StringExact(testAzureTenantDomain(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyUseCase), knownvalue.StringExact(useCaseCloudNativeProtection)),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyAppID), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyAppName), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyAppSecret), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyTenantID), knownvalue.Null()),
			},
		}, {
			// Verify that the service principal can be added from the
			// individual values. Switching from the credentials file replaces
			// the resource.
			Config: `
				variable "app_id" {
					type = string
				}
				variable "app_name" {
					type = string
				}
				variable "app_secret" {
					type      = string
					sensitive = true
				}
				variable "tenant_domain" {
					type = string
				}
				variable "tenant_id" {
					type = string
				}

				resource "rubrik_azure_service_principal" "service_principal" {
					app_id        = var.app_id
					app_name      = var.app_name
					app_secret    = var.app_secret
					tenant_domain = var.tenant_domain
					tenant_id     = var.tenant_id
				}
			`,
			ConfigVariables: config.Variables{
				"app_id":        config.StringVariable(testAzurePrincipalID(t)),
				"app_name":      config.StringVariable(testAzurePrincipalName(t)),
				"app_secret":    config.StringVariable(testAzurePrincipalSecret(t)),
				"tenant_domain": config.StringVariable(testAzureTenantDomain(t)),
				"tenant_id":     config.StringVariable(testAzureTenantID(t)),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_service_principal.service_principal",
						plancheck.ResourceActionReplace),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyID), knownvalue.StringExact(testAzurePrincipalID(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyAppID), knownvalue.StringExact(testAzurePrincipalID(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyAppName), knownvalue.StringExact(testAzurePrincipalName(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyAppSecret), knownvalue.StringExact(testAzurePrincipalSecret(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyTenantDomain), knownvalue.StringExact(testAzureTenantDomain(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyTenantID), knownvalue.StringExact(testAzureTenantID(t))),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyUseCase), knownvalue.StringExact(useCaseCloudNativeProtection)),
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyCredentials), knownvalue.Null()),
			},
		}},
	})
}

// TestAccAzureServicePrincipalResource_FrameworkMigration verifies that
// existing state created by the SDKv2 provider can be read by the Framework
// provider without drift.
func TestAccAzureServicePrincipalResource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{{
			// Add the service principal from a credentials file using the
			// published SDKv2 provider and assert that the plan is empty when
			// the provider is upgraded to the local provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"rubrik-sdkv2": {
					Source:            "rubrikinc/rubrik",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "credentials" {
					type = string
				}
				variable "tenant_domain" {
					type = string
				}

				provider "rubrik-sdkv2" {}

				resource "rubrik_azure_service_principal" "service_principal" {
					provider = rubrik-sdkv2

					credentials   = var.credentials
					tenant_domain = var.tenant_domain
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":   config.StringVariable(testAzureCredentials(t)),
				"tenant_domain": config.StringVariable(testAzureTenantDomain(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyID), knownvalue.StringExact(testAzurePrincipalID(t))),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "credentials" {
					type = string
				}
				variable "tenant_domain" {
					type = string
				}

				resource "rubrik_azure_service_principal" "service_principal" {
					credentials   = var.credentials
					tenant_domain = var.tenant_domain
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":   config.StringVariable(testAzureCredentials(t)),
				"tenant_domain": config.StringVariable(testAzureTenantDomain(t)),
			},
			PlanOnly: true,
		}, {
			// Add the service principal from its individual values using the
			// published SDKv2 provider and assert that the plan is empty when
			// the provider is upgraded to the local provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"rubrik-sdkv2": {
					Source:            "rubrikinc/rubrik",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "app_id" {
					type = string
				}
				variable "app_name" {
					type = string
				}
				variable "app_secret" {
					type      = string
					sensitive = true
				}
				variable "tenant_domain" {
					type = string
				}
				variable "tenant_id" {
					type = string
				}

				provider "rubrik-sdkv2" {}

				resource "rubrik_azure_service_principal" "service_principal" {
					provider = rubrik-sdkv2

					app_id        = var.app_id
					app_name      = var.app_name
					app_secret    = var.app_secret
					tenant_domain = var.tenant_domain
					tenant_id     = var.tenant_id
				}
			`,
			ConfigVariables: config.Variables{
				"app_id":        config.StringVariable(testAzurePrincipalID(t)),
				"app_name":      config.StringVariable(testAzurePrincipalName(t)),
				"app_secret":    config.StringVariable(testAzurePrincipalSecret(t)),
				"tenant_domain": config.StringVariable(testAzureTenantDomain(t)),
				"tenant_id":     config.StringVariable(testAzureTenantID(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_service_principal.service_principal",
					tfjsonpath.New(keyID), knownvalue.StringExact(testAzurePrincipalID(t))),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "app_id" {
					type = string
				}
				variable "app_name" {
					type = string
				}
				variable "app_secret" {
					type      = string
					sensitive = true
				}
				variable "tenant_domain" {
					type = string
				}
				variable "tenant_id" {
					type = string
				}

				resource "rubrik_azure_service_principal" "service_principal" {
					app_id        = var.app_id
					app_name      = var.app_name
					app_secret    = var.app_secret
					tenant_domain = var.tenant_domain
					tenant_id     = var.tenant_id
				}
			`,
			ConfigVariables: config.Variables{
				"app_id":        config.StringVariable(testAzurePrincipalID(t)),
				"app_name":      config.StringVariable(testAzurePrincipalName(t)),
				"app_secret":    config.StringVariable(testAzurePrincipalSecret(t)),
				"tenant_domain": config.StringVariable(testAzureTenantDomain(t)),
				"tenant_id":     config.StringVariable(testAzureTenantID(t)),
			},
			PlanOnly: true,
		}},
	})
}

// TestAccAzureServicePrincipalResource_MoveState verifies that state from a
// polaris_azure_service_principal resource created by the rubrikinc/polaris
// provider can be moved to a rubrik_azure_service_principal resource using the
// moved {} block. Steps 1 and 2 use a credentials file, steps 3 and 4 the
// individual values.
func TestAccAzureServicePrincipalResource_MoveState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "rsc_credentials" {
					type = string
				}
				variable "credentials" {
					type = string
				}
				variable "tenant_domain" {
					type = string
				}

				provider "polaris" {
					credentials = var.rsc_credentials
				}

				resource "polaris_azure_service_principal" "service_principal" {
					credentials   = var.credentials
					tenant_domain = var.tenant_domain
				}
			`,
			ConfigVariables: config.Variables{
				"rsc_credentials": config.StringVariable(testCredentials(t)),
				"credentials":     config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":   config.StringVariable(testAzureTenantDomain(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_azure_service_principal.service_principal",
					tfjsonpath.New(keyID), knownvalue.StringExact(testAzurePrincipalID(t))),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "rsc_credentials" {
					type = string
				}
				variable "credentials" {
					type = string
				}
				variable "tenant_domain" {
					type = string
				}

				moved {
					from = polaris_azure_service_principal.service_principal
					to   = rubrik_azure_service_principal.service_principal
				}

				resource "rubrik_azure_service_principal" "service_principal" {
					credentials   = var.credentials
					tenant_domain = var.tenant_domain
				}
			`,
			ConfigVariables: config.Variables{
				"rsc_credentials": config.StringVariable(testCredentials(t)),
				"credentials":     config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":   config.StringVariable(testAzureTenantDomain(t)),
			},
			// Verify the plan is empty, move succeeded without drift, and
			// apply to update the state.
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// Add the service principal from the individual values using the
			// polaris provider. This destroys the rubrik resource from step 2,
			// which only removes it from the state.
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
				variable "rsc_credentials" {
					type = string
				}
				variable "app_id" {
					type = string
				}
				variable "app_name" {
					type = string
				}
				variable "app_secret" {
					type      = string
					sensitive = true
				}
				variable "tenant_domain" {
					type = string
				}
				variable "tenant_id" {
					type = string
				}

				provider "polaris" {
					credentials = var.rsc_credentials
				}

				resource "polaris_azure_service_principal" "service_principal" {
					app_id        = var.app_id
					app_name      = var.app_name
					app_secret    = var.app_secret
					tenant_domain = var.tenant_domain
					tenant_id     = var.tenant_id
				}
			`,
			ConfigVariables: config.Variables{
				"rsc_credentials": config.StringVariable(testCredentials(t)),
				"app_id":          config.StringVariable(testAzurePrincipalID(t)),
				"app_name":        config.StringVariable(testAzurePrincipalName(t)),
				"app_secret":      config.StringVariable(testAzurePrincipalSecret(t)),
				"tenant_domain":   config.StringVariable(testAzureTenantDomain(t)),
				"tenant_id":       config.StringVariable(testAzureTenantID(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_azure_service_principal.service_principal",
					tfjsonpath.New(keyID), knownvalue.StringExact(testAzurePrincipalID(t))),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "rsc_credentials" {
					type = string
				}
				variable "app_id" {
					type = string
				}
				variable "app_name" {
					type = string
				}
				variable "app_secret" {
					type      = string
					sensitive = true
				}
				variable "tenant_domain" {
					type = string
				}
				variable "tenant_id" {
					type = string
				}

				moved {
					from = polaris_azure_service_principal.service_principal
					to   = rubrik_azure_service_principal.service_principal
				}

				resource "rubrik_azure_service_principal" "service_principal" {
					app_id        = var.app_id
					app_name      = var.app_name
					app_secret    = var.app_secret
					tenant_domain = var.tenant_domain
					tenant_id     = var.tenant_id
				}
			`,
			ConfigVariables: config.Variables{
				"rsc_credentials": config.StringVariable(testCredentials(t)),
				"app_id":          config.StringVariable(testAzurePrincipalID(t)),
				"app_name":        config.StringVariable(testAzurePrincipalName(t)),
				"app_secret":      config.StringVariable(testAzurePrincipalSecret(t)),
				"tenant_domain":   config.StringVariable(testAzureTenantDomain(t)),
				"tenant_id":       config.StringVariable(testAzureTenantID(t)),
			},
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
