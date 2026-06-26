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
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

func TestAccAwsCnpAccountResource(t *testing.T) {
	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsCnpAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}
				resource "polaris_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyCloud), knownvalue.StringExact("STANDARD")),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyDeleteSnapshotsOnDestroy), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyNativeID), knownvalue.StringExact(testAWSAccountID(t))),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyName), knownvalue.StringExact(testAWSAccountName(t))),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyRegions),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("us-east-2"),
					})),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("CLOUD_DISCOVERY"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("EXOCOMPUTE"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
								knownvalue.StringExact("RSC_MANAGED_CLUSTER"),
							}),
						}),
					})),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyTrustPolicies),
					knownvalue.SetPartial([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyRoleKey: knownvalue.StringExact("CROSSACCOUNT"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyRoleKey: knownvalue.StringExact("EXOCOMPUTE_EKS_MASTERNODE"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyRoleKey: knownvalue.StringExact("EXOCOMPUTE_EKS_WORKERNODE"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyRoleKey: knownvalue.StringExact("EXOCOMPUTE_EKS_LAMBDA"),
						}),
					})),
			},
		}, {
			// Terraform import.
			ResourceName:      "polaris_aws_cnp_account.account",
			ConfigVariables:   vars,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "polaris_aws_cnp_account.account",
			ConfigVariables: vars,
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "polaris_aws_cnp_account.account",
			ConfigVariables: vars,
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

func TestAccAwsCnpAccountResource_ExternalID(t *testing.T) {
	// importIDFunc builds the legacy "<uuid>:<external-id>" composite import
	// id from the post-create state. Used by the two string-id import kinds.
	importIDFunc := func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources["polaris_aws_cnp_account.account"]
		if !ok {
			return "", fmt.Errorf("resource polaris_aws_cnp_account.account not found in state")
		}
		return fmt.Sprintf("%s:%s", rs.Primary.ID, rs.Primary.Attributes[keyExternalID]), nil
	}

	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsCnpAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}
				resource "polaris_aws_cnp_account" "account" {
					name        = var.account_name
					native_id   = var.aws_account_id
					external_id = "test-external-id"
					regions     = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyExternalID), knownvalue.StringExact("test-external-id")),
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyTrustPolicies),
					knownvalue.SetPartial([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyRoleKey: knownvalue.StringExact("CROSSACCOUNT"),
						}),
					})),
			},
		}, {
			// Terraform import.
			ResourceName:      "polaris_aws_cnp_account.account",
			ConfigVariables:   vars,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateIdFunc: importIDFunc,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:      "polaris_aws_cnp_account.account",
			ConfigVariables:   vars,
			ImportStateKind:   resource.ImportBlockWithID,
			ImportState:       true,
			ImportStateIdFunc: importIDFunc,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "polaris_aws_cnp_account.account",
			ConfigVariables: vars,
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

// TestAccAwsCnpAccountResource_FrameworkMigration verifies that the migrated
// aws_cnp_account resource is backwards compatible with the SDKv2 provider.
// Step 1 onboards the account with the SDKv2 provider; Step 2 swaps to the
// framework provider with the same config and asserts an empty plan.
func TestAccAwsCnpAccountResource_FrameworkMigration(t *testing.T) {
	conf := `
		variable "account_name" {
			type = string
		}
		variable "aws_account_id" {
			type = string
		}
		resource "polaris_aws_cnp_account" "account" {
			name      = var.account_name
			native_id = var.aws_account_id
			regions   = ["us-east-2"]

			feature {
				name              = "CLOUD_DISCOVERY"
				permission_groups = ["BASIC"]
			}
			feature {
				name              = "CLOUD_NATIVE_PROTECTION"
				permission_groups = ["BASIC"]
			}
		}
	`

	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: awsCnpAccountCheckDestroy(t),
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"polaris": {
						Source:            "rubrikinc/polaris",
						VersionConstraint: "1.6.3",
					},
				},
				Config:          conf,
				ConfigVariables: vars,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
						tfjsonpath.New(keyID), NonNullUUID()),
					statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
						tfjsonpath.New(keyName), knownvalue.StringExact(testAWSAccountName(t))),
					statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
						tfjsonpath.New(keyNativeID), knownvalue.StringExact(testAWSAccountID(t))),
					statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
						tfjsonpath.New(keyRegions),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("us-east-2"),
						})),
					statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
						tfjsonpath.New(keyFeature),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								keyName: knownvalue.StringExact("CLOUD_DISCOVERY"),
								keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact("BASIC"),
								}),
							}),
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								keyName: knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
								keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact("BASIC"),
								}),
							}),
						})),
				},
			},
			{
				ProtoV6ProviderFactories: protoV6ProviderFactories,
				Config:                   conf,
				ConfigVariables:          vars,
				PlanOnly:                 true,
			},
		},
	})
}

// TestAccAwsCnpAccountResource_MoveState verifies that state from a
// polaris_aws_cnp_account resource created by the rubrikinc/polaris provider
// can be moved to a rubrik_aws_cnp_account resource using the moved {} block.
func TestAccAwsCnpAccountResource_MoveState(t *testing.T) {
	vars := config.Variables{
		"account_name":   config.StringVariable(testAWSAccountName(t)),
		"aws_account_id": config.StringVariable(testAWSAccountID(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: awsCnpAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.6.3",
				},
			},
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}
				resource "polaris_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_cnp_account.account",
					tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "account_name" {
					type = string
				}
				variable "aws_account_id" {
					type = string
				}
				moved {
					from = polaris_aws_cnp_account.account
					to   = rubrik_aws_cnp_account.account
				}
				resource "rubrik_aws_cnp_account" "account" {
					name      = var.account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
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

func TestDiffFeatures(t *testing.T) {
	cd := core.FeatureCloudDiscovery.WithPermissionGroups(core.PermissionGroupBasic)
	cnp := core.FeatureCloudNativeProtection.WithPermissionGroups(core.PermissionGroupBasic)
	exo := core.FeatureExocompute.WithPermissionGroups(core.PermissionGroupBasic)
	exoWithCluster := core.FeatureExocompute.WithPermissionGroups(
		core.PermissionGroupBasic, core.PermissionGroupRSCManagedCluster)

	tt := []struct {
		name           string
		old            []core.Feature
		new            []core.Feature
		removeFeatures []core.Feature
		updateFeatures []core.Feature
	}{{
		name: "BothEmpty",
	}, {
		name:           "AllAdded",
		new:            []core.Feature{cd, cnp},
		updateFeatures: []core.Feature{cd, cnp},
	}, {
		name:           "AllRemoved",
		old:            []core.Feature{cd, cnp},
		removeFeatures: []core.Feature{cd, cnp},
	}, {
		name: "Identical",
		old:  []core.Feature{cd},
		new:  []core.Feature{cd},
	}, {
		name:           "OneUnchangedOneRemoved",
		old:            []core.Feature{cd, cnp},
		new:            []core.Feature{cd},
		removeFeatures: []core.Feature{cnp},
	}, {
		name:           "OneUnchangedOneAdded",
		old:            []core.Feature{cd},
		new:            []core.Feature{cd, cnp},
		updateFeatures: []core.Feature{cnp},
	}, {
		name:           "PermissionGroupsModified",
		old:            []core.Feature{exo},
		new:            []core.Feature{exoWithCluster},
		updateFeatures: []core.Feature{exoWithCluster},
	}, {
		name:           "MixOfAddedRemovedAndModified",
		old:            []core.Feature{cd, cnp, exo},
		new:            []core.Feature{cd, exoWithCluster},
		removeFeatures: []core.Feature{cnp},
		updateFeatures: []core.Feature{exoWithCluster},
	}}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			remove, update := diffFeatures(tc.old, tc.new)

			if !slices.EqualFunc(remove, tc.removeFeatures, func(a, b core.Feature) bool { return a.DeepEqual(b) }) {
				t.Errorf("remove: got %v, want %v", remove, tc.removeFeatures)
			}
			if !slices.EqualFunc(update, tc.updateFeatures, func(a, b core.Feature) bool { return a.DeepEqual(b) }) {
				t.Errorf("update: got %v, want %v", update, tc.updateFeatures)
			}
		})
	}
}

func TestSplitAccountID(t *testing.T) {
	tt := []struct {
		name       string
		id         string
		accountID  uuid.UUID
		externalID string
		errPrefix  string
	}{{
		name:      "InvalidAccountID",
		id:        "a7b9eafe-e0b8-496d-814f",
		errPrefix: "invalid resource id",
	}, {
		name:      "AccountID",
		id:        "a7b9eafe-e0b8-496d-814f-f81a97af853e",
		accountID: uuid.MustParse("a7b9eafe-e0b8-496d-814f-f81a97af853e"),
	}, {
		name:       "AccountIDWithExternalIDDashSeparator",
		id:         "a7b9eafe-e0b8-496d-814f-f81a97af853e-external-id",
		accountID:  uuid.MustParse("a7b9eafe-e0b8-496d-814f-f81a97af853e"),
		externalID: "external-id",
	}, {
		name:       "AccountIDWithExternalIDColonSeparator",
		id:         "a7b9eafe-e0b8-496d-814f-f81a97af853e:external-id",
		accountID:  uuid.MustParse("a7b9eafe-e0b8-496d-814f-f81a97af853e"),
		externalID: "external-id",
	}, {
		name:       "AccountIDWithExternalIDContainingColon",
		id:         "a7b9eafe-e0b8-496d-814f-f81a97af853e:foo:bar",
		accountID:  uuid.MustParse("a7b9eafe-e0b8-496d-814f-f81a97af853e"),
		externalID: "foo:bar",
	}, {
		name:      "AccountIDWithEmptyExternalIDAfterColon",
		id:        "a7b9eafe-e0b8-496d-814f-f81a97af853e:",
		errPrefix: "invalid resource id",
	}}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			accountID, externalID, err := splitAccountID(tc.id)
			if err == nil {
				if accountID != tc.accountID {
					t.Errorf("invalid account id: %s", accountID)
				}
				if externalID != tc.externalID {
					t.Errorf("invalid external id: %s", externalID)
				}
			} else {
				if tc.errPrefix == "" || !strings.HasPrefix(err.Error(), tc.errPrefix) {
					t.Errorf("expected error prefix: %q, got: %s", tc.errPrefix, err)
				}
			}
		})
	}
}
