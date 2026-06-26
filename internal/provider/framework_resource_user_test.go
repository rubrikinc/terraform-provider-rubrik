// Copyright 2023 Rubrik, Inc.
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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccUserResource(t *testing.T) {
	vars := config.Variables{
		"user_email": config.StringVariable(testUserEmail(t)),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             userCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with one role.
			Config: `
				variable "user_email" {
					type = string
				}

				resource "polaris_custom_role" "auditor" {
					name        = "Test Auditor"
					description = "Test Role: Delete Me!"

					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
					permission {
						operation = "VIEW_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
				}

				resource "polaris_user" "user" {
					email = var.user_email

					role_ids = [
						polaris_custom_role.auditor.id,
					]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_user.user", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("polaris_user.user", tfjsonpath.New(keyEmail),
					knownvalue.StringExact(testUserEmail(t))),
				statecheck.ExpectKnownValue("polaris_user.user", tfjsonpath.New(keyDomain),
					knownvalue.StringExact("LOCAL")),
				statecheck.ExpectKnownValue("polaris_user.user", tfjsonpath.New(keyStatus),
					knownvalue.StringExact("ACTIVE")),
				statecheck.ExpectKnownValue("polaris_user.user", tfjsonpath.New(keyIsAccountOwner),
					knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("polaris_user.user", tfjsonpath.New(keyRoleIDs),
					knownvalue.SetSizeExact(1)),
				statecheck.CompareValueCollection(
					"polaris_user.user", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"polaris_custom_role.auditor", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectIdentity("polaris_user.user", map[string]knownvalue.Check{
					keyID: knownvalue.NotNull(),
				}),
				statecheck.ExpectIdentityValueMatchesState("polaris_user.user", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the resource can be updated with an additional role.
			Config: `
				variable "user_email" {
					type = string
				}

				resource "polaris_custom_role" "auditor" {
					name        = "Test Auditor"
					description = "Test Role: Delete Me!"

					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
					permission {
						operation = "VIEW_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
				}

				resource "polaris_custom_role" "cluster_viewer" {
					name        = "Test Cluster Viewer"
					description = "Test Role: Delete Me!"

					permission {
						operation = "VIEW_CLUSTER"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
				}

				resource "polaris_user" "user" {
					email = var.user_email

					role_ids = [
						polaris_custom_role.auditor.id,
						polaris_custom_role.cluster_viewer.id,
					]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_user.user", tfjsonpath.New(keyRoleIDs),
					knownvalue.SetSizeExact(2)),
				statecheck.CompareValueCollection(
					"polaris_user.user", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"polaris_custom_role.auditor", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValueCollection(
					"polaris_user.user", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"polaris_custom_role.cluster_viewer", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectIdentity("polaris_user.user", map[string]knownvalue.Check{
					keyID: knownvalue.NotNull(),
				}),
				statecheck.ExpectIdentityValueMatchesState("polaris_user.user", tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName:      "polaris_user.user",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
			ConfigVariables:   vars,
		}, {
			// import {} block with id attribute.
			ResourceName:    "polaris_user.user",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ConfigVariables: vars,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "polaris_user.user",
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

// TestAccUserResource_FrameworkMigration verifies that existing state created
// by the SDKv2 provider (v1.5.0) can be read by the Framework provider
// without drift.
func TestAccUserResource_FrameworkMigration(t *testing.T) {
	conf := `
		variable "user_email" {
			type = string
		}

		resource "polaris_custom_role" "auditor" {
			name        = "Test Auditor"
			description = "Test Role: Delete Me!"

			permission {
				operation = "EXPORT_DATA_CLASS_GLOBAL"
				hierarchy {
					snappable_type = "AllSubHierarchyType"
					object_ids     = ["GlobalResource"]
				}
			}
			permission {
				operation = "VIEW_DATA_CLASS_GLOBAL"
				hierarchy {
					snappable_type = "AllSubHierarchyType"
					object_ids     = ["GlobalResource"]
				}
			}
		}

		resource "polaris_user" "user" {
			email = var.user_email

			role_ids = [
				polaris_custom_role.auditor.id,
			]
		}
	`

	vars := config.Variables{
		"user_email": config.StringVariable(testUserEmail(t)),
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: userCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config:          conf,
			ConfigVariables: vars,
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables:          vars,
			PlanOnly:                 true,
		}},
	})
}

// TestAccUserResource_MoveState verifies that state from a polaris_user
// resource created by the rubrikinc/polaris provider can be moved to a
// rubrik_user resource using the moved {} block.
func TestAccUserResource_MoveState(t *testing.T) {
	vars := config.Variables{
		"user_email": config.StringVariable(testUserEmail(t)),
		"role_name":  config.StringVariable("Test MoveState Auditor " + uuid.New().String()),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: userCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config: `
				variable "user_email" {
					type = string
				}
				variable "role_name" {
					type = string
				}

				resource "polaris_custom_role" "auditor" {
					name        = var.role_name
					description = "Test Role: Delete Me!"

					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
				}

				resource "polaris_user" "user" {
					email    = var.user_email
					role_ids = [polaris_custom_role.auditor.id]
				}
			`,
			ConfigVariables: vars,
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "user_email" {
					type = string
				}
				variable "role_name" {
					type = string
				}

				moved {
					from = polaris_user.user
					to   = rubrik_user.user
				}
				moved {
					from = polaris_custom_role.auditor
					to   = rubrik_custom_role.auditor
				}

				resource "rubrik_custom_role" "auditor" {
					provider    = rubrik
					name        = var.role_name
					description = "Test Role: Delete Me!"

					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
				}

				resource "rubrik_user" "user" {
					provider = rubrik
					email    = var.user_email
					role_ids = [rubrik_custom_role.auditor.id]
				}
			`,
			ConfigVariables: vars,
			PlanOnly:        true,
		}},
	})
}
