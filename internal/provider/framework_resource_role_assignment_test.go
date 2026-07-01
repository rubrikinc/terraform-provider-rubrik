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

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccRoleAssignmentResource(t *testing.T) {
	createTestUser(t, testUserEmail(t), createTestRoleWithUniqueName(t))

	vars := config.Variables{
		"user_email": config.StringVariable(testUserEmail(t)),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             roleAssignmentCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created.
			Config: `
				variable "user_email" {
					type = string
				}

				data "rubrik_user" "user" {
					email = var.user_email
				}

				resource "rubrik_custom_role" "auditor" {
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

				resource "rubrik_role_assignment" "auditor" {
					user_id  = data.rubrik_user.user.id

					role_ids = [
						rubrik_custom_role.auditor.id
					]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_role_assignment.auditor", tfjsonpath.New(keyID),
					"data.rubrik_user.user", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_role_assignment.auditor", tfjsonpath.New(keyUserID),
					"data.rubrik_user.user", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_role_assignment.auditor", tfjsonpath.New(keyRoleIDs),
					knownvalue.SetSizeExact(1)),
				statecheck.CompareValueCollection(
					"rubrik_role_assignment.auditor", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"rubrik_custom_role.auditor", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}, {
			// Verify that the resource can be updated.
			Config: `
				variable "user_email" {
					type = string
				}

				data "rubrik_user" "user" {
					email = var.user_email
				}

				resource "rubrik_custom_role" "auditor" {
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

				resource "rubrik_custom_role" "cluster_viewer" {
					name        = "Test Cluster Viewer"
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

				resource "rubrik_role_assignment" "auditor" {
					user_id  = data.rubrik_user.user.id

					role_ids = [
						rubrik_custom_role.auditor.id,
						rubrik_custom_role.cluster_viewer.id,
					]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_role_assignment.auditor", tfjsonpath.New(keyID),
					"data.rubrik_user.user", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_role_assignment.auditor", tfjsonpath.New(keyUserID),
					"data.rubrik_user.user", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_role_assignment.auditor", tfjsonpath.New(keyRoleIDs),
					knownvalue.SetSizeExact(2)),
				statecheck.CompareValueCollection(
					"rubrik_role_assignment.auditor", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"rubrik_custom_role.auditor", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValueCollection(
					"rubrik_role_assignment.auditor", []tfjsonpath.Path{tfjsonpath.New(keyRoleIDs)},
					"rubrik_custom_role.cluster_viewer", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}, {
			// Terraform import. Note, the import will take over all roles
			// assigned to the user, not just the ones managed by this resource.
			// The import {} block forms (with id or identity) are not testable
			// here because the framework requires plannable imports to be a
			// no-op, while this resource's import takes ownership of all roles.
			ResourceName:            "rubrik_role_assignment.auditor",
			ImportStateKind:         resource.ImportCommandWithID,
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{keyUserEmail, keyRoleIDs},
			ConfigVariables:         vars,
		}},
	})
}

// TestAccRoleAssignmentResource_FrameworkMigration verifies that existing state
// created by the SDKv2 provider (v1.5.0) can be read by the Framework
// provider without drift.
func TestAccRoleAssignmentResource_FrameworkMigration(t *testing.T) {
	createTestUser(t, testUserEmail(t), createTestRoleWithUniqueName(t))

	vars := config.Variables{
		"user_email": config.StringVariable(testUserEmail(t)),
	}

	// Test 1: Modern fields (user_id + role_ids).
	conf := `
		variable "user_email" {
			type = string
		}

		data "polaris_user" "user" {
			email = var.user_email
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

		resource "polaris_role_assignment" "auditor" {
			user_id  = data.polaris_user.user.id

			role_ids = [
				polaris_custom_role.auditor.id,
			]
		}
	`

	resource.Test(t, resource.TestCase{
		CheckDestroy: roleAssignmentCheckDestroy(t),
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

	// Test 2: Deprecated fields (user_email + role_id).
	conf = `
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

		resource "polaris_role_assignment" "auditor" {
			user_email = var.user_email
			role_id    = polaris_custom_role.auditor.id
		}
	`

	resource.Test(t, resource.TestCase{
		CheckDestroy: roleAssignmentCheckDestroy(t),
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

// TestAccRoleAssignmentResource_MoveState verifies that state from a
// polaris_role_assignment resource created by the rubrikinc/polaris provider can
// be moved to a rubrik_role_assignment resource using the moved {} block.
func TestAccRoleAssignmentResource_MoveState(t *testing.T) {
	createTestUser(t, testUserEmail(t), createTestRoleWithUniqueName(t))

	vars := config.Variables{
		"user_email": config.StringVariable(testUserEmail(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: roleAssignmentCheckDestroy(t),
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

				data "polaris_user" "user" {
					email = var.user_email
				}

				resource "polaris_custom_role" "auditor" {
					name        = "Test Role Assignment Move State"
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

				resource "polaris_role_assignment" "auditor" {
					user_id  = data.polaris_user.user.id

					role_ids = [
						polaris_custom_role.auditor.id,
					]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_role_assignment.auditor", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "user_email" {
					type = string
				}

				moved {
					from = polaris_role_assignment.auditor
					to   = rubrik_role_assignment.auditor
				}
				moved {
					from = polaris_custom_role.auditor
					to   = rubrik_custom_role.auditor
				}

				data "rubrik_user" "user" {
					email = var.user_email
				}

				resource "rubrik_custom_role" "auditor" {
					name        = "Test Role Assignment Move State"
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

				resource "rubrik_role_assignment" "auditor" {
					user_id  = data.rubrik_user.user.id

					role_ids = [
						rubrik_custom_role.auditor.id,
					]
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
