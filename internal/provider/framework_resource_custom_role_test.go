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
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/access"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/hierarchy"
)

func TestAccCustomRoleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customRoleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created.
			Config: `
				resource "rubrik_custom_role" "role" {
					name        = "Test Auditor"
					description = "Test Role: Delete Me!"

					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyName),
					knownvalue.StringExact("Test Auditor")),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Test Role: Delete Me!")),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyPermission),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyOperation: knownvalue.StringExact("EXPORT_DATA_CLASS_GLOBAL"),
							keyHierarchy: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
								keySnappableType: knownvalue.StringExact("AllSubHierarchyType"),
								keyObjectIDs: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact(hierarchy.GlobalResource)}),
							})}),
						}),
					})),
				statecheck.ExpectIdentity("rubrik_custom_role.role", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_custom_role.role", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the resource can be updated.
			Config: `
				resource "rubrik_custom_role" "role" {
					name        = "Test Auditor Update"
					description = "Test Role: Delete Me! Update"

					permission {
						operation = "EXPORT_DATA_CLASS_GLOBAL"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["GlobalResource"]
						}
					}
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
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyName),
					knownvalue.StringExact("Test Auditor Update")),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Test Role: Delete Me! Update")),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyPermission),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyOperation: knownvalue.StringExact("EXPORT_DATA_CLASS_GLOBAL"),
							keyHierarchy: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
								keySnappableType: knownvalue.StringExact("AllSubHierarchyType"),
								keyObjectIDs: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact(hierarchy.GlobalResource)}),
							})}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyOperation: knownvalue.StringExact("VIEW_CLUSTER"),
							keyHierarchy: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
								keySnappableType: knownvalue.StringExact("AllSubHierarchyType"),
								keyObjectIDs: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact(hierarchy.ClusterRoot),
								}),
							})}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyOperation: knownvalue.StringExact("VIEW_CLUSTER_REFERENCE"),
							keyHierarchy: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
								keySnappableType: knownvalue.StringExact("AllSubHierarchyType"),
								keyObjectIDs: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact(hierarchy.ClusterRoot),
								}),
							})}),
						}),
					})),
				statecheck.ExpectIdentity("rubrik_custom_role.role", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_custom_role.role", tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_custom_role.role",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_custom_role.role",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_custom_role.role",
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

func TestAccCustomRoleResource_FromTemplate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customRoleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created from a role template.
			Config: `
				data "rubrik_role_template" "auditor" {
				  	name = "Compliance Auditor"
				}
				
				resource "rubrik_custom_role" "role" {
					name        = "Test Auditor"
					description = "Based on the ${data.rubrik_role_template.auditor.name} template: Delete Me!"
					
					dynamic "permission" {
						for_each = data.rubrik_role_template.auditor.permission
						content {
							operation = permission.value["operation"]
							
							dynamic "hierarchy" {
								for_each = permission.value["hierarchy"]
								content {
									snappable_type = hierarchy.value["snappable_type"]
									object_ids     = hierarchy.value["object_ids"]
								}
							}
						}
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyName),
					knownvalue.StringExact("Test Auditor")),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Based on the Compliance Auditor template: Delete Me!")),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyPermission),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyOperation: knownvalue.StringExact("EXPORT_DATA_CLASS_GLOBAL"),
							keyHierarchy: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
								keySnappableType: knownvalue.StringExact("AllSubHierarchyType"),
								keyObjectIDs: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact(hierarchy.GlobalResource)}),
							})}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyOperation: knownvalue.StringExact("VIEW_DATA_CLASS_GLOBAL"),
							keyHierarchy: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
								keySnappableType: knownvalue.StringExact("AllSubHierarchyType"),
								keyObjectIDs: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact(hierarchy.GlobalResource),
								}),
							})}),
						}),
					})),
			},
		}},
	})
}

// TestAccCustomRoleResource_ViewClusterOnly verifies that the config validator
// rejects a role granting VIEW_CLUSTER without VIEW_CLUSTER_REFERENCE.
func TestAccCustomRoleResource_ViewClusterOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				resource "rubrik_custom_role" "role" {
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
			`,
			ExpectError: regexp.MustCompile("(?s)VIEW_CLUSTER requires VIEW_CLUSTER_REFERENCE"),
		}},
	})
}

// TestAccCustomRoleResource_ViewClusterReferenceOnly verifies that a role with
// only the VIEW_CLUSTER_REFERENCE operation, i.e. without VIEW_CLUSTER, is
// accepted by the validator and does not drift. VIEW_CLUSTER_REFERENCE is a
// narrower permission that RSC does not expand, so the applied permission set
// must remain exactly as configured.
func TestAccCustomRoleResource_ViewClusterReferenceOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customRoleCheckDestroy(t),
		Steps: []resource.TestStep{{
			Config: `
				resource "rubrik_custom_role" "role" {
					name        = "Test Cluster Reference Viewer"
					description = "Test Role: Delete Me!"

					permission {
						operation = "VIEW_CLUSTER_REFERENCE"
						hierarchy {
							snappable_type = "AllSubHierarchyType"
							object_ids     = ["CLUSTER_ROOT"]
						}
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_custom_role.role", tfjsonpath.New(keyPermission),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyOperation: knownvalue.StringExact(string(access.OperationViewClusterReference)),
							keyHierarchy: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
								keySnappableType: knownvalue.StringExact("AllSubHierarchyType"),
								keyObjectIDs: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact(hierarchy.ClusterRoot),
								}),
							})}),
						}),
					})),
			},
		}},
	})
}

// TestAccPolarisCustomRole_FrameworkMigration verifies that existing state
// created by the SDKv2 provider (v1.5.0) can be read by the Framework
// provider without drift. Step 1 creates the resource using the published
// SDKv2 provider; step 2 refreshes state using the local Framework provider
// and asserts the plan is empty.
func TestAccCustomRoleResource_FrameworkMigration(t *testing.T) {
	conf := `
		resource "polaris_custom_role" "role" {
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
	`

	resource.Test(t, resource.TestCase{
		CheckDestroy: customRoleCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config: conf,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_custom_role.role", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("polaris_custom_role.role", tfjsonpath.New(keyName),
					knownvalue.StringExact("Test Auditor")),
				statecheck.ExpectKnownValue("polaris_custom_role.role", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Test Role: Delete Me!")),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			PlanOnly:                 true,
		}},
	})
}

// TestAccCustomRoleResource_MoveState verifies that state from a
// polaris_custom_role resource created by the rubrikinc/polaris provider can be
// moved to a rubrik_custom_role resource using the moved {} block.
func TestAccCustomRoleResource_MoveState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: customRoleCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config: `
				resource "polaris_custom_role" "role" {
					name        = "Test Role Move State"
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
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_custom_role.role", tfjsonpath.New(keyID),
					NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				moved {
					from = polaris_custom_role.role
					to   = rubrik_custom_role.role
				}

				resource "rubrik_custom_role" "role" {
					name        = "Test Role Move State"
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
			`,
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

func TestValidateCustomRoleConfig(t *testing.T) {
	ctx := context.Background()

	// permSet builds a permission set with one hierarchy per operation.
	permSet := func(operations ...access.Operation) types.Set {
		perms := make([]access.Permission, 0, len(operations))
		for _, op := range operations {
			perms = append(perms, access.Permission{
				Operation: string(op),
				ObjectsForHierarchyTypes: []access.ObjectsForHierarchyType{{
					SnappableType: "AllSubHierarchyType",
					ObjectIDs:     []string{hierarchy.ClusterRoot},
				}},
			})
		}
		permSet, diags := fromPermissions(ctx, perms)
		if diags.HasError() {
			t.Fatalf("failed to create permission set: %s", diags)
		}

		return permSet
	}

	tests := []struct {
		name       string
		permission types.Set
		wantErr    bool
	}{{
		name:       "both present",
		permission: permSet(access.OperationViewCluster, access.OperationViewClusterReference),
		wantErr:    false,
	}, {
		name:       "both present with another operation",
		permission: permSet(access.OperationViewCluster, access.OperationViewClusterReference, "EXPORT_DATA_CLASS_GLOBAL"),
		wantErr:    false,
	}, {
		name:       "neither present",
		permission: permSet("EXPORT_DATA_CLASS_GLOBAL"),
		wantErr:    false,
	}, {
		name:       "only ViewCluster",
		permission: permSet(access.OperationViewCluster),
		wantErr:    true,
	}, {
		name:       "only ViewClusterReference is allowed",
		permission: permSet(access.OperationViewClusterReference),
		wantErr:    false,
	}, {
		name:       "null permission set",
		permission: types.SetNull(types.ObjectType{AttrTypes: permissionAttrTypes()}),
		wantErr:    false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateCustomRoleConfig(ctx, customRoleModel{Permission: tt.permission})
			if got := diags.HasError(); got != tt.wantErr {
				t.Errorf("validateCustomRoleConfig() error = %v, wantErr %v: %v", got, tt.wantErr, diags)
			}
		})
	}
}
