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

// TestAccAzurePrivateContainerRegistryResource onboards the Azure subscription
// with the Exocompute feature, registers a private container registry for it,
// updates and imports it. The app ID is the app ID of the test service
// principal. RSC does not verify that the registry exists.
func TestAccAzurePrivateContainerRegistryResource(t *testing.T) {
	const conf = `
		variable "credentials" {
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
		variable "exocompute_region" {
			type = string
		}
		variable "exocompute_resource_group_name" {
			type = string
		}
		variable "exocompute_resource_group_region" {
			type = string
		}
		variable "app_id" {
			type = string
		}
		variable "url" {
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

			cloud_native_protection {
				permission_groups     = ["BASIC"]
				regions               = ["eastus2"]
				resource_group_name   = var.resource_group_name
				resource_group_region = var.resource_group_region
			}

			exocompute {
				permission_groups     = ["BASIC"]
				regions               = [var.exocompute_region]
				resource_group_name   = var.exocompute_resource_group_name
				resource_group_region = var.exocompute_resource_group_region
			}

			depends_on = [rubrik_azure_service_principal.service_principal]
		}

		resource "rubrik_azure_private_container_registry" "registry" {
			cloud_account_id = rubrik_azure_subscription.subscription.id
			app_id           = var.app_id
			url              = var.url
		}
	`

	vars := config.Variables{
		"credentials":                      config.StringVariable(testAzureCredentials(t)),
		"tenant_domain":                    config.StringVariable(testAzureTenantDomain(t)),
		"subscription_id":                  config.StringVariable(testAzureSubscriptionID(t)),
		"subscription_name":                config.StringVariable(testAzureSubscriptionName(t)),
		"resource_group_name":              config.StringVariable(testAzureResourceGroupName(t)),
		"resource_group_region":            config.StringVariable(testAzureResourceGroupRegion(t)),
		"exocompute_region":                config.StringVariable(testAzureExocomputeRegion(t)),
		"exocompute_resource_group_name":   config.StringVariable(testAzureExocomputeResourceGroupName(t)),
		"exocompute_resource_group_region": config.StringVariable(testAzureExocomputeResourceGroupRegion(t)),
		"app_id":                           config.StringVariable(testAzurePrincipalID(t)),
		"url":                              config.StringVariable("terraformtest.azurecr.io"),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azurePrivateContainerRegistryCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the private container registry can be registered for
			// the subscription.
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_azure_private_container_registry.registry", tfjsonpath.New(keyID),
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_azure_private_container_registry.registry", tfjsonpath.New(keyCloudAccountID),
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_azure_private_container_registry.registry",
					tfjsonpath.New(keyAppID), knownvalue.StringExact(testAzurePrincipalID(t))),
				statecheck.ExpectKnownValue("rubrik_azure_private_container_registry.registry",
					tfjsonpath.New(keyURL), knownvalue.StringExact("terraformtest.azurecr.io")),
				statecheck.ExpectIdentity("rubrik_azure_private_container_registry.registry",
					map[string]knownvalue.Check{
						keyID: NonNullUUID(),
					}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_private_container_registry.registry",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the url is updated in place.
			Config: conf,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"url": config.StringVariable("terraformtestupdated.azurecr.io"),
			}),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_private_container_registry.registry",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_private_container_registry.registry",
					tfjsonpath.New(keyURL), knownvalue.StringExact("terraformtestupdated.azurecr.io")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_private_container_registry.registry",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			Config: conf,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"url": config.StringVariable("terraformtestupdated.azurecr.io"),
			}),
			ResourceName:      "rubrik_azure_private_container_registry.registry",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			Config: conf,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"url": config.StringVariable("terraformtestupdated.azurecr.io"),
			}),
			ResourceName:    "rubrik_azure_private_container_registry.registry",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			Config: conf,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"url": config.StringVariable("terraformtestupdated.azurecr.io"),
			}),
			ResourceName:    "rubrik_azure_private_container_registry.registry",
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

// TestAccAzurePrivateContainerRegistryResource_FrameworkMigration verifies
// that existing state created by the SDKv2 provider can be read by the
// Framework provider without drift.
func TestAccAzurePrivateContainerRegistryResource_FrameworkMigration(t *testing.T) {
	const conf = `
		variable "credentials" {
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
		variable "exocompute_region" {
			type = string
		}
		variable "exocompute_resource_group_name" {
			type = string
		}
		variable "exocompute_resource_group_region" {
			type = string
		}
		variable "app_id" {
			type = string
		}
		variable "url" {
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

			cloud_native_protection {
				permission_groups     = ["BASIC"]
				regions               = ["eastus2"]
				resource_group_name   = var.resource_group_name
				resource_group_region = var.resource_group_region
			}

			exocompute {
				permission_groups     = ["BASIC"]
				regions               = [var.exocompute_region]
				resource_group_name   = var.exocompute_resource_group_name
				resource_group_region = var.exocompute_resource_group_region
			}

			depends_on = [rubrik_azure_service_principal.service_principal]
		}

		resource "rubrik_azure_private_container_registry" "registry" {
			cloud_account_id = rubrik_azure_subscription.subscription.id
			app_id           = var.app_id
			url              = var.url
		}
	`

	vars := config.Variables{
		"credentials":                      config.StringVariable(testAzureCredentials(t)),
		"tenant_domain":                    config.StringVariable(testAzureTenantDomain(t)),
		"subscription_id":                  config.StringVariable(testAzureSubscriptionID(t)),
		"subscription_name":                config.StringVariable(testAzureSubscriptionName(t)),
		"resource_group_name":              config.StringVariable(testAzureResourceGroupName(t)),
		"resource_group_region":            config.StringVariable(testAzureResourceGroupRegion(t)),
		"exocompute_region":                config.StringVariable(testAzureExocomputeRegion(t)),
		"exocompute_resource_group_name":   config.StringVariable(testAzureExocomputeResourceGroupName(t)),
		"exocompute_resource_group_region": config.StringVariable(testAzureExocomputeResourceGroupRegion(t)),
		"app_id":                           config.StringVariable(testAzurePrincipalID(t)),
		"url":                              config.StringVariable("terraformtest.azurecr.io"),
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azurePrivateContainerRegistryCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Register the private container registry using the published
			// SDKv2 provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"rubrik": {
					Source:            "rubrikinc/rubrik",
					VersionConstraint: "1.12.0",
				},
			},
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_private_container_registry.registry",
					tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			// Verify that the plan is empty when the provider is upgraded to the
			// local provider.
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables:          vars,
			PlanOnly:                 true,
		}},
	})
}

// TestAccAzurePrivateContainerRegistryResource_MoveState verifies that state
// from a polaris_azure_private_container_registry resource created by the
// rubrikinc/polaris provider can be moved to a
// rubrik_azure_private_container_registry resource using the moved {} block.
// The service principal and the subscription stay in the polaris provider.
func TestAccAzurePrivateContainerRegistryResource_MoveState(t *testing.T) {
	vars := config.Variables{
		"credentials":                      config.StringVariable(testAzureCredentials(t)),
		"tenant_domain":                    config.StringVariable(testAzureTenantDomain(t)),
		"subscription_id":                  config.StringVariable(testAzureSubscriptionID(t)),
		"subscription_name":                config.StringVariable(testAzureSubscriptionName(t)),
		"resource_group_name":              config.StringVariable(testAzureResourceGroupName(t)),
		"resource_group_region":            config.StringVariable(testAzureResourceGroupRegion(t)),
		"exocompute_region":                config.StringVariable(testAzureExocomputeRegion(t)),
		"exocompute_resource_group_name":   config.StringVariable(testAzureExocomputeResourceGroupName(t)),
		"exocompute_resource_group_region": config.StringVariable(testAzureExocomputeResourceGroupRegion(t)),
		"app_id":                           config.StringVariable(testAzurePrincipalID(t)),
		"url":                              config.StringVariable("terraformtest.azurecr.io"),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azurePrivateContainerRegistryCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Register the private container registry using the polaris
			// provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.12.1",
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
				variable "exocompute_region" {
					type = string
				}
				variable "exocompute_resource_group_name" {
					type = string
				}
				variable "exocompute_resource_group_region" {
					type = string
				}
				variable "app_id" {
					type = string
				}
				variable "url" {
					type = string
				}

				provider "polaris" {
					credentials = var.rsc_credentials
				}

				resource "polaris_azure_service_principal" "service_principal" {
					credentials   = var.credentials
					tenant_domain = var.tenant_domain
				}

				resource "polaris_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_native_protection {
						permission_groups     = ["BASIC"]
						regions               = ["eastus2"]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					exocompute {
						permission_groups     = ["BASIC"]
						regions               = [var.exocompute_region]
						resource_group_name   = var.exocompute_resource_group_name
						resource_group_region = var.exocompute_resource_group_region
					}

					depends_on = [polaris_azure_service_principal.service_principal]
				}

				resource "polaris_azure_private_container_registry" "registry" {
					cloud_account_id = polaris_azure_subscription.subscription.id
					app_id           = var.app_id
					url              = var.url
				}
			`,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"rsc_credentials": config.StringVariable(testCredentials(t)),
			}),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_azure_private_container_registry.registry",
					tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			// Move the private container registry to the local provider and
			// verify that the plan is empty. Apply to update the state.
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.12.1",
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
				variable "credentials" {
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
				variable "exocompute_region" {
					type = string
				}
				variable "exocompute_resource_group_name" {
					type = string
				}
				variable "exocompute_resource_group_region" {
					type = string
				}
				variable "app_id" {
					type = string
				}
				variable "url" {
					type = string
				}

				provider "polaris" {
					credentials = var.rsc_credentials
				}

				resource "polaris_azure_service_principal" "service_principal" {
					credentials   = var.credentials
					tenant_domain = var.tenant_domain
				}

				resource "polaris_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_native_protection {
						permission_groups     = ["BASIC"]
						regions               = ["eastus2"]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					exocompute {
						permission_groups     = ["BASIC"]
						regions               = [var.exocompute_region]
						resource_group_name   = var.exocompute_resource_group_name
						resource_group_region = var.exocompute_resource_group_region
					}

					depends_on = [polaris_azure_service_principal.service_principal]
				}

				moved {
					from = polaris_azure_private_container_registry.registry
					to   = rubrik_azure_private_container_registry.registry
				}

				resource "rubrik_azure_private_container_registry" "registry" {
					cloud_account_id = polaris_azure_subscription.subscription.id
					app_id           = var.app_id
					url              = var.url
				}
			`,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"rsc_credentials": config.StringVariable(testCredentials(t)),
			}),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
