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

// gcpExocomputeTestProject is the GCP project onboarded with the Exocompute
// feature, using customer managed networking.
const gcpExocomputeTestProject = `
	variable "credentials" {
		type      = string
		sensitive = true
	}
	variable "project_id" {
		type = string
	}
	variable "project_name" {
		type = string
	}
	variable "project_number" {
		type = string
	}
	variable "region" {
		type = string
	}
	variable "subnet_name" {
		type = string
	}
	variable "vpc_name" {
		type = string
	}
	variable "secondary_range_name" {
		type    = string
		default = null
	}
	variable "trigger_health_check" {
		type    = bool
		default = null
	}

	resource "rubrik_gcp_project" "project" {
		credentials    = var.credentials
		project        = var.project_id
		project_name   = var.project_name
		project_number = var.project_number

		feature {
			name              = "EXOCOMPUTE"
			permission_groups = ["BASIC"]
		}

		lifecycle {
			ignore_changes = [organization_name]
		}
	}
`

// TestAccGcpExocomputeResource onboards the GCP project with the Exocompute
// feature, creates an Exocompute configuration, imports it and updates it.
func TestAccGcpExocomputeResource(t *testing.T) {
	const conf = gcpExocomputeTestProject + `
		resource "rubrik_gcp_exocompute" "exocompute" {
			cloud_account_id     = rubrik_gcp_project.project.id
			trigger_health_check = var.trigger_health_check

			regional_config {
				region               = var.region
				subnet_name          = var.subnet_name
				vpc_name             = var.vpc_name
				secondary_range_name = var.secondary_range_name
			}
		}
	`

	vars := func(secondaryRangeName string, triggerHealthCheck bool) config.Variables {
		v := config.Variables{
			"credentials":    config.StringVariable(testGCPCredentials(t)),
			"project_id":     config.StringVariable(testGCPProjectID(t)),
			"project_name":   config.StringVariable(testGCPProjectName(t)),
			"project_number": config.StringVariable(testGCPProjectNumber(t)),
			"region":         config.StringVariable(testGCPExocomputeRegion(t)),
			"subnet_name":    config.StringVariable(testGCPExocomputeSubnetName(t)),
			"vpc_name":       config.StringVariable(testGCPExocomputeVPCName(t)),
		}
		if secondaryRangeName != "" {
			v["secondary_range_name"] = config.StringVariable(secondaryRangeName)
		}
		if triggerHealthCheck {
			v["trigger_health_check"] = config.BoolVariable(true)
		}
		return v
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpExocomputeCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the Exocompute configuration is created with the
			// default secondary range name and no host project.
			Config:          conf,
			ConfigVariables: vars("", false),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyName:   knownvalue.StringExact("EXOCOMPUTE"),
							keyStatus: knownvalue.StringExact("CONNECTED"),
						}),
					})),
				statecheck.CompareValuePairs(
					"rubrik_gcp_project.project", tfjsonpath.New(keyID),
					"rubrik_gcp_exocompute.exocompute", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_project.project", tfjsonpath.New(keyID),
					"rubrik_gcp_exocompute.exocompute", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_gcp_exocompute.exocompute", tfjsonpath.New(keyRegionalConfig),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyHostProjectID:      knownvalue.Null(),
							keyRegion:             knownvalue.StringExact(testGCPExocomputeRegion(t)),
							keySecondaryRangeName: knownvalue.StringExact(gcpDefaultSecondaryRangeName),
							keySubnetName:         knownvalue.StringExact(testGCPExocomputeSubnetName(t)),
							keyVPCName:            knownvalue.StringExact(testGCPExocomputeVPCName(t)),
						}),
					})),
				statecheck.ExpectIdentity("rubrik_gcp_exocompute.exocompute", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_exocompute.exocompute",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that setting the default secondary range name explicitly
			// gives an empty plan.
			Config:          conf,
			ConfigVariables: vars(gcpDefaultSecondaryRangeName, false),
			PlanOnly:        true,
		}, {
			// Terraform import.
			ResourceName:      "rubrik_gcp_exocompute.exocompute",
			Config:            conf,
			ConfigVariables:   vars("", false),
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_gcp_exocompute.exocompute",
			Config:          conf,
			ConfigVariables: vars("", false),
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity.
			ResourceName:    "rubrik_gcp_exocompute.exocompute",
			Config:          conf,
			ConfigVariables: vars("", false),
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// Verify that enabling trigger_health_check updates the resource in
			// place, sending the full regional configuration, and that the
			// regional configuration is unchanged after the update.
			Config:          conf,
			ConfigVariables: vars("", true),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_exocompute.exocompute",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_exocompute.exocompute", tfjsonpath.New(keyTriggerHealthCheck),
					knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("rubrik_gcp_exocompute.exocompute", tfjsonpath.New(keyRegionalConfig),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyHostProjectID:      knownvalue.Null(),
							keyRegion:             knownvalue.StringExact(testGCPExocomputeRegion(t)),
							keySecondaryRangeName: knownvalue.StringExact(gcpDefaultSecondaryRangeName),
							keySubnetName:         knownvalue.StringExact(testGCPExocomputeSubnetName(t)),
							keyVPCName:            knownvalue.StringExact(testGCPExocomputeVPCName(t)),
						}),
					})),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_exocompute.exocompute",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that a refresh after the update gives an empty plan, i.e.
			// RSC returns the regional configuration sent by Update.
			Config:          conf,
			ConfigVariables: vars("", true),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

// TestAccGcpExocomputeResource_MoveState verifies that state from a
// polaris_gcp_exocompute resource created by the rubrikinc/polaris provider can
// be moved to a rubrik_gcp_exocompute resource using the moved {} block.
func TestAccGcpExocomputeResource_MoveState(t *testing.T) {
	vars := config.Variables{
		"rsc_credentials": config.StringVariable(testCredentials(t)),
		"credentials":     config.StringVariable(testGCPCredentials(t)),
		"project_id":      config.StringVariable(testGCPProjectID(t)),
		"project_name":    config.StringVariable(testGCPProjectName(t)),
		"project_number":  config.StringVariable(testGCPProjectNumber(t)),
		"region":          config.StringVariable(testGCPExocomputeRegion(t)),
		"subnet_name":     config.StringVariable(testGCPExocomputeSubnetName(t)),
		"vpc_name":        config.StringVariable(testGCPExocomputeVPCName(t)),
	}

	// The project stays with the polaris provider, so the polaris name can't
	// also be served by the provider under test.
	const project = `
		variable "rsc_credentials" {
			type      = string
			sensitive = true
		}
		variable "credentials" {
			type      = string
			sensitive = true
		}
		variable "project_id" {
			type = string
		}
		variable "project_name" {
			type = string
		}
		variable "project_number" {
			type = string
		}
		variable "region" {
			type = string
		}
		variable "subnet_name" {
			type = string
		}
		variable "vpc_name" {
			type = string
		}

		provider "polaris" {
			credentials = var.rsc_credentials
		}

		resource "polaris_gcp_project" "project" {
			credentials    = var.credentials
			project        = var.project_id
			project_name   = var.project_name
			project_number = var.project_number

			feature {
				name              = "EXOCOMPUTE"
				permission_groups = ["BASIC"]
			}

			lifecycle {
				ignore_changes = [organization_name]
			}
		}
	`

	polarisProvider := map[string]resource.ExternalProvider{
		"polaris": {
			Source:            "rubrikinc/polaris",
			VersionConstraint: "1.11.2",
		},
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpExocomputeCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Create the Exocompute configuration with the polaris provider.
			ExternalProviders: polarisProvider,
			Config: project + `
				resource "polaris_gcp_exocompute" "exocompute" {
					cloud_account_id = polaris_gcp_project.project.id

					regional_config {
						region      = var.region
						subnet_name = var.subnet_name
						vpc_name    = var.vpc_name
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_exocompute.exocompute", tfjsonpath.New(keyID),
					NonNullUUID()),
			},
		}, {
			// Verify that the move gives an empty plan, then apply to save the
			// moved state, without it destroy can fail on the dependencies.
			ExternalProviders: polarisProvider,
			ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
				"rubrik": newMuxedProviderServer,
			},
			Config: project + `
				moved {
					from = polaris_gcp_exocompute.exocompute
					to   = rubrik_gcp_exocompute.exocompute
				}

				resource "rubrik_gcp_exocompute" "exocompute" {
					cloud_account_id = polaris_gcp_project.project.id

					regional_config {
						region      = var.region
						subnet_name = var.subnet_name
						vpc_name    = var.vpc_name
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectIdentity("rubrik_gcp_exocompute.exocompute", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
			},
		}},
	})
}
