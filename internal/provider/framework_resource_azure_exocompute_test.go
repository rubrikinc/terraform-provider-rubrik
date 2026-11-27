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

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAzureExocomputeResource onboards the Azure subscription with the
// Exocompute feature, creates an RSC managed host configuration for it, sets
// trigger_health_check to false in place, adds the optional_config block, which
// replaces the configuration, sets health_check_vm_name_prefix, which replaces
// it again, removes the prefix from the configuration, which keeps the value in
// RSC, and imports it.
func TestAccAzureExocomputeResource(t *testing.T) {
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
		variable "subnet_id" {
			type = string
		}
		variable "trigger_health_check" {
			type    = bool
			default = null
		}
		variable "cluster_access" {
			type    = string
			default = null
		}
		variable "health_check_vm_name_prefix" {
			type    = string
			default = null
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

		resource "rubrik_azure_exocompute" "exocompute" {
			cloud_account_id     = rubrik_azure_subscription.subscription.id
			region               = var.exocompute_region
			subnet               = var.subnet_id
			trigger_health_check = var.trigger_health_check

			dynamic "optional_config" {
				for_each = var.cluster_access == null ? [] : [var.cluster_access]
				content {
					cluster_access              = optional_config.value
					health_check_vm_name_prefix = var.health_check_vm_name_prefix
				}
			}
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
		"subnet_id":                        config.StringVariable(testAzureExocomputeSubnetID(t)),
	}
	withOptionalConfig := testVariables(t, vars, config.Variables{
		"cluster_access": config.StringVariable("AKS_CLUSTER_ACCESS_TYPE_PRIVATE"),
	})
	withPrefix := testVariables(t, withOptionalConfig, config.Variables{
		"health_check_vm_name_prefix": config.StringVariable("tf-acc-health-check"),
	})

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureExocomputeCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that an RSC managed host configuration can be created.
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.CompareValuePairs(
					"rubrik_azure_exocompute.exocompute", tfjsonpath.New(keyCloudAccountID),
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyRegion), knownvalue.StringExact(testAzureExocomputeRegion(t))),
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keySubnet), knownvalue.StringExact(testAzureExocomputeSubnetID(t))),
				statecheck.ExpectIdentity("rubrik_azure_exocompute.exocompute",
					map[string]knownvalue.Check{
						keyID: NonNullUUID(),
					}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that trigger_health_check is updated in place. No health
			// check runs when the field is set to false.
			Config: conf,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"trigger_health_check": config.BoolVariable(false),
			}),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_exocompute.exocompute",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyTriggerHealthCheck), knownvalue.Bool(false)),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that adding the optional_config block replaces the
			// configuration, and that the fields RSC fills in don't cause a
			// diff afterward.
			Config:          conf,
			ConfigVariables: withOptionalConfig,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_exocompute.exocompute",
						plancheck.ResourceActionDestroyBeforeCreate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyOptionalConfig).AtSliceIndex(0).AtMapKey(keyClusterAccess),
					knownvalue.StringExact("AKS_CLUSTER_ACCESS_TYPE_PRIVATE")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that setting health_check_vm_name_prefix to a value other
			// than the one in RSC replaces the configuration.
			Config:          conf,
			ConfigVariables: withPrefix,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_exocompute.exocompute",
						plancheck.ResourceActionDestroyBeforeCreate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyOptionalConfig).AtSliceIndex(0).AtMapKey(keyHealthCheckVMNamePrefix),
					knownvalue.StringExact("tf-acc-health-check")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that removing health_check_vm_name_prefix from the
			// configuration keeps the value in RSC, without replacing the
			// configuration.
			Config:          conf,
			ConfigVariables: withOptionalConfig,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_exocompute.exocompute",
						plancheck.ResourceActionNoop),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyOptionalConfig).AtSliceIndex(0).AtMapKey(keyHealthCheckVMNamePrefix),
					knownvalue.StringExact("tf-acc-health-check")),
			},
		}, {
			// Terraform import.
			Config:            conf,
			ConfigVariables:   withOptionalConfig,
			ResourceName:      "rubrik_azure_exocompute.exocompute",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			Config:          conf,
			ConfigVariables: withOptionalConfig,
			ResourceName:    "rubrik_azure_exocompute.exocompute",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			Config:          conf,
			ConfigVariables: withOptionalConfig,
			ResourceName:    "rubrik_azure_exocompute.exocompute",
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

// TestAccAzureExocomputeResource_FrameworkMigration verifies that existing
// state created by the SDKv2 provider can be read by the Framework provider
// without drift. The SDKv2 provider stores the unset strings of the
// optional_config block as empty strings, which the state upgrade turns into
// null.
func TestAccAzureExocomputeResource_FrameworkMigration(t *testing.T) {
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
		variable "subnet_id" {
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

		resource "rubrik_azure_exocompute" "exocompute" {
			cloud_account_id = rubrik_azure_subscription.subscription.id
			region           = var.exocompute_region
			subnet           = var.subnet_id

			optional_config {
				cluster_access = "AKS_CLUSTER_ACCESS_TYPE_PRIVATE"
			}
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
		"subnet_id":                        config.StringVariable(testAzureExocomputeSubnetID(t)),
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureExocomputeCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Create the configuration using the published SDKv2 provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"rubrik": {
					Source:            "rubrikinc/rubrik",
					VersionConstraint: "1.12.0",
				},
			},
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_exocompute.exocompute",
					tfjsonpath.New(keyOptionalConfig).AtSliceIndex(0).AtMapKey(keyClusterAccess),
					knownvalue.StringExact("AKS_CLUSTER_ACCESS_TYPE_PRIVATE")),
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

// TestAccAzureExocomputeResource_MoveState verifies that state from a
// polaris_azure_exocompute resource created by the rubrikinc/polaris provider
// can be moved to a rubrik_azure_exocompute resource using the moved {} block.
// The service principal and the subscription stay in the polaris provider.
func TestAccAzureExocomputeResource_MoveState(t *testing.T) {
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
		"subnet_id":                        config.StringVariable(testAzureExocomputeSubnetID(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureExocomputeCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Create the configuration using the polaris provider.
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
				variable "subnet_id" {
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

				resource "polaris_azure_exocompute" "exocompute" {
					cloud_account_id = polaris_azure_subscription.subscription.id
					region           = var.exocompute_region
					subnet           = var.subnet_id
				}
			`,
			ConfigVariables: testVariables(t, vars, config.Variables{
				"rsc_credentials": config.StringVariable(testCredentials(t)),
			}),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_azure_exocompute.exocompute",
					tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			// Move the configuration to the local provider and verify that the
			// plan is empty. Apply to update the state.
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
				variable "subnet_id" {
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
					from = polaris_azure_exocompute.exocompute
					to   = rubrik_azure_exocompute.exocompute
				}

				resource "rubrik_azure_exocompute" "exocompute" {
					cloud_account_id = polaris_azure_subscription.subscription.id
					region           = var.exocompute_region
					subnet           = var.subnet_id
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

// TestAzureExocomputeValidateConfig checks that only a configuration with a
// region but neither a subnet nor a pod overlay network CIDR, which Create
// turns into a customer managed configuration, is warned about.
func TestAzureExocomputeValidateConfig(t *testing.T) {
	r := &azureExocomputeResource{prefix: keyRubrik}

	var schemaRes fwresource.SchemaResponse
	r.Schema(t.Context(), fwresource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}
	configType := schemaRes.Schema.Type().TerraformType(t.Context()).(tftypes.Object)

	tests := []struct {
		name   string
		config map[string]string
		warns  bool
	}{{
		name:   "CustomerManaged",
		config: map[string]string{keyRegion: "eastus2"},
		warns:  true,
	}, {
		name:   "Subnet",
		config: map[string]string{keyRegion: "eastus2", keySubnet: "subnet"},
	}, {
		// Create sends an RSC managed configuration when the pod overlay
		// network CIDR is set, with or without a subnet.
		name:   "PodOverlayNetworkCIDR",
		config: map[string]string{keyRegion: "eastus2", keyPodOverlayNetworkCIDR: "10.244.0.0/16"},
	}, {
		name: "Application",
		config: map[string]string{
			keyCloudAccountID:     "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			keyHostCloudAccountID: "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			values := make(map[string]tftypes.Value, len(configType.AttributeTypes))
			for k, typ := range configType.AttributeTypes {
				values[k] = tftypes.NewValue(typ, nil)
			}
			for k, v := range tc.config {
				values[k] = tftypes.NewValue(tftypes.String, v)
			}
			req := fwresource.ValidateConfigRequest{
				Config: tfsdk.Config{Schema: schemaRes.Schema, Raw: tftypes.NewValue(configType, values)},
			}
			var res fwresource.ValidateConfigResponse
			r.ValidateConfig(t.Context(), req, &res)
			if res.Diagnostics.HasError() {
				t.Fatal(res.Diagnostics)
			}
			if warns := res.Diagnostics.WarningsCount() > 0; warns != tc.warns {
				t.Errorf("expected warning %t, got diagnostics %v", tc.warns, res.Diagnostics)
			}
		})
	}
}
