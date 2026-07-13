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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccSSOGroupResource(t *testing.T) {
	skipUnlessSSOGroupDefined(t)

	vars := config.Variables{
		"auth_domain_id": config.StringVariable(testAuthDomainID(t)),
		"sso_group_name": config.StringVariable(testSSOGroupName(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			customRoleCheckDestroy(t),
			ssoGroupCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created.
			Config: `
				variable "auth_domain_id" {
					type = string
				}
				variable "sso_group_name" {
					type = string
				}
			
				resource "rubrik_custom_role" "role1" {
					name        = "Test Role 1 for SSO Group"
					description = "Test Role: Delete Me!"
			
					permission {
						operation = "VIEW_CLUSTER"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
					permission {
						operation = "VIEW_CLUSTER_REFERENCE"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
				}
			
				resource "rubrik_custom_role" "role2" {
					name        = "Test Role 2 for SSO Group"
					description = "Test Role: Delete Me!"
			
					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
				}
			
				resource "rubrik_sso_group" "group" {
					auth_domain_id = var.auth_domain_id
					group_name     = var.sso_group_name
					role_ids       = [rubrik_custom_role.role1.id]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_sso_group.group", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("rubrik_sso_group.group", tfjsonpath.New(keyAuthDomainID),
					knownvalue.StringExact(testAuthDomainID(t))),
				statecheck.ExpectKnownValue("rubrik_sso_group.group", tfjsonpath.New(keyGroupName),
					knownvalue.StringExact(testSSOGroupName(t))),
				statecheck.CompareValueCollection(
					"rubrik_sso_group.group", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"rubrik_custom_role.role1", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectIdentity("rubrik_sso_group.group", map[string]knownvalue.Check{
					keyID:           knownvalue.NotNull(),
					keyAuthDomainID: knownvalue.StringExact(testAuthDomainID(t)),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_sso_group.group", tfjsonpath.New(keyID)),
				statecheck.ExpectIdentityValueMatchesState("rubrik_sso_group.group", tfjsonpath.New(keyAuthDomainID)),
			},
		}, {
			// Verify that the resource's role_ids can be updated.
			Config: `
				variable "auth_domain_id" {
					type = string
				}
				variable "sso_group_name" {
					type = string
				}
			
				resource "rubrik_custom_role" "role1" {
					name        = "Test Role 1 for SSO Group"
					description = "Test Role: Delete Me!"
			
					permission {
						operation = "VIEW_CLUSTER"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
					permission {
						operation = "VIEW_CLUSTER_REFERENCE"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
				}
			
				resource "rubrik_custom_role" "role2" {
					name        = "Test Role 2 for SSO Group"
					description = "Test Role: Delete Me!"
			
					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
				}
			
				resource "rubrik_sso_group" "group" {
					auth_domain_id = var.auth_domain_id
					group_name     = var.sso_group_name
					role_ids       = [rubrik_custom_role.role2.id]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_sso_group.group", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("rubrik_sso_group.group", tfjsonpath.New(keyAuthDomainID),
					knownvalue.StringExact(testAuthDomainID(t))),
				statecheck.ExpectKnownValue("rubrik_sso_group.group", tfjsonpath.New(keyGroupName),
					knownvalue.StringExact(testSSOGroupName(t))),
				statecheck.CompareValueCollection(
					"rubrik_sso_group.group", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"rubrik_custom_role.role2", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.ExpectIdentity("rubrik_sso_group.group", map[string]knownvalue.Check{
					keyID:           knownvalue.NotNull(),
					keyAuthDomainID: knownvalue.StringExact(testAuthDomainID(t)),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_sso_group.group", tfjsonpath.New(keyID)),
				statecheck.ExpectIdentityValueMatchesState("rubrik_sso_group.group", tfjsonpath.New(keyAuthDomainID)),
			},
		}, {
			// Terraform import via the composite string format
			// "<group_name>:<auth_domain_id>".
			ResourceName:      "rubrik_sso_group.group",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateIdFunc: ssoGroupImportID("rubrik_sso_group.group"),
			ConfigVariables:   vars,
		}, {
			// import {} block with id attribute, composite string format
			// "<group_name>:<auth_domain_id>".
			ResourceName:      "rubrik_sso_group.group",
			ImportStateKind:   resource.ImportBlockWithID,
			ImportState:       true,
			ImportStateIdFunc: ssoGroupImportID("rubrik_sso_group.group"),
			ConfigVariables:   vars,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_sso_group.group",
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ConfigVariables: vars,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

// TestAccSSOGroupResource_MoveState verifies that state from a
// polaris_sso_group resource created by the rubrikinc/polaris provider can be
// moved to a rubrik_sso_group resource using the moved {} block.
func TestAccSSOGroupResource_MoveState(t *testing.T) {
	skipUnlessSSOGroupDefined(t)

	vars := config.Variables{
		"credentials":    config.StringVariable(testCredentials(t)),
		"auth_domain_id": config.StringVariable(testAuthDomainID(t)),
		"sso_group_name": config.StringVariable(testSSOGroupName(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			customRoleCheckDestroy(t),
			ssoGroupCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.6.1",
				},
			},
			Config: `
				variable "credentials" {
					type = string
				}
				variable "auth_domain_id" {
					type = string
				}
				variable "sso_group_name" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_custom_role" "role" {
					name        = "Test Role for SSO Group Move State"
					description = "Test Role: Delete Me!"

					permission {
						operation = "VIEW_CLUSTER"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
					permission {
						operation = "VIEW_CLUSTER_REFERENCE"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
				}

				resource "polaris_sso_group" "group" {
					auth_domain_id = var.auth_domain_id
					group_name     = var.sso_group_name
					role_ids       = [polaris_custom_role.role.id]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_sso_group.group", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "credentials" {
					type = string
				}
				variable "auth_domain_id" {
					type = string
				}
				variable "sso_group_name" {
					type = string
				}

				moved {
					from = polaris_custom_role.role
					to   = rubrik_custom_role.role
				}

				moved {
					from = polaris_sso_group.group
					to   = rubrik_sso_group.group
				}

				resource "rubrik_custom_role" "role" {
					name        = "Test Role for SSO Group Move State"
					description = "Test Role: Delete Me!"

					permission {
						operation = "VIEW_CLUSTER"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
					permission {
						operation = "VIEW_CLUSTER_REFERENCE"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
				}

				resource "rubrik_sso_group" "group" {
					auth_domain_id = var.auth_domain_id
					group_name     = var.sso_group_name
					role_ids       = [rubrik_custom_role.role.id]
				}
			`,
			ConfigVariables: vars,
			// Verify the plan is empty, move succeeded without drift, and
			// apply to update the state. Without the apply step, destroy can
			// fail due to resource dependency issues.
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

// ssoGroupImportID returns an ImportStateIdFunc that builds the composite
// "<group_name>:<auth_domain_id>" import string for the SSO group resource.
func ssoGroupImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}

		groupName := rs.Primary.Attributes[keyGroupName]
		authDomainID := rs.Primary.Attributes[keyAuthDomainID]
		return fmt.Sprintf("%s:%s", groupName, authDomainID), nil
	}
}
