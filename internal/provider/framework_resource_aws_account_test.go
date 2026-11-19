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
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

// TestAccAWSAccountResource onboards the AWS account using a profile, updates
// the regions and imports the account.
func TestAccAWSAccountResource(t *testing.T) {
	name := testAWSAccountName(t)

	vars := config.Variables{
		"name":    config.StringVariable(name),
		"profile": config.StringVariable(testAWSProfile(t)),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the account can be onboarded using a profile.
			Config: `
				variable "name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					name    = var.name
					profile = var.profile

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyDeleteSnapshotsOnDestroy),
					knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudDiscovery),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_account.account", tfjsonpath.New(keyID)),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyProfile),
					knownvalue.StringExact(testAWSProfile(t))),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyAssumeRole),
					knownvalue.Null()),
			},
		}, {
			// Verify that adding a region updates the account in place.
			Config: `
				variable "name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					name    = var.name
					profile = var.profile

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2", "us-west-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2", "us-west-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_account.account", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudDiscovery),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
								knownvalue.StringExact("us-west-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
								knownvalue.StringExact("us-west-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
			},
		}, {
			// Verify that removing a region updates the account in place.
			Config: `
				variable "name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					name    = var.name
					profile = var.profile

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_account.account", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudDiscovery),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
			},
		}, {
			// Terraform import. The profile isn't stored in RSC, so it isn't
			// imported.
			ResourceName:            "rubrik_aws_account.account",
			ConfigVariables:         vars,
			ImportStateKind:         resource.ImportCommandWithID,
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{keyProfile},
		}, {
			// import {} block with id attribute. The profile isn't imported,
			// so the plan updates the state with the configured profile. The
			// import plan isn't applied by the test framework.
			ResourceName:       "rubrik_aws_account.account",
			ConfigVariables:    vars,
			ImportStateKind:    resource.ImportBlockWithID,
			ImportState:        true,
			ExpectNonEmptyPlan: true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_account.account", plancheck.ResourceActionUpdate),
					plancheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyProfile),
						knownvalue.StringExact(testAWSProfile(t))),
				},
			},
		}, {
			// import {} block with identity, with the same update of the
			// profile.
			ResourceName:       "rubrik_aws_account.account",
			ConfigVariables:    vars,
			ImportStateKind:    resource.ImportBlockWithResourceIdentity,
			ImportState:        true,
			ExpectNonEmptyPlan: true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_account.account", plancheck.ResourceActionUpdate),
					plancheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyProfile),
						knownvalue.StringExact(testAWSProfile(t))),
				},
			},
		}},
	})
}

// TestAccAWSAccountResourceAssumeRole onboards the AWS account by assuming a
// role with the default profile, and updates the regions.
func TestAccAWSAccountResourceAssumeRole(t *testing.T) {
	name := testAWSCloudApplicationTargetAccountName(t)

	vars := config.Variables{
		"name":        config.StringVariable(name),
		"assume_role": config.StringVariable(testAWSCloudApplicationTargetRole(t)),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the account can be onboarded by assuming a role.
			Config: `
				variable "name" {
					type = string
				}
				variable "assume_role" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					assume_role = var.assume_role
					name        = var.name

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyDeleteSnapshotsOnDestroy),
					knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudDiscovery),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_account.account", tfjsonpath.New(keyID)),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyProfile),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyAssumeRole),
					knownvalue.StringExact(testAWSCloudApplicationTargetRole(t))),
			},
		}, {
			// Verify that adding a region updates the account in place.
			Config: `
				variable "name" {
					type = string
				}
				variable "assume_role" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					assume_role = var.assume_role
					name        = var.name

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2", "us-west-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2", "us-west-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_account.account", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudDiscovery),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
								knownvalue.StringExact("us-west-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
								knownvalue.StringExact("us-west-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
			},
		}, {
			// Verify that removing a region updates the account in place.
			Config: `
				variable "name" {
					type = string
				}
				variable "assume_role" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					assume_role = var.assume_role
					name        = var.name

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_account.account", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudDiscovery),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
			},
		}},
	})
}

// TestAccAWSAccountResource_FrameworkMigration verifies that state written by
// the SDKv2 resource of the last release is upgraded in place by the Framework
// resource, with an empty plan and the identity set.
func TestAccAWSAccountResource_FrameworkMigration(t *testing.T) {
	const conf = `
		variable "name" {
			type = string
		}
		variable "profile" {
			type = string
		}

		resource "rubrik_aws_account" "account" {
			name    = var.name
			profile = var.profile

			cloud_discovery {
				permission_groups = ["BASIC"]
				regions           = ["us-east-2"]
			}

			cloud_native_protection {
				permission_groups = ["BASIC"]
				regions           = ["us-east-2"]
			}
		}
	`
	vars := config.Variables{
		"name":    config.StringVariable(testAWSAccountName(t)),
		"profile": config.StringVariable(testAWSProfile(t)),
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard the account with the SDKv2 resource of the last release.
			ExternalProviders: map[string]resource.ExternalProvider{
				"rubrik": {
					Source:            "rubrikinc/rubrik",
					VersionConstraint: "1.12.0",
				},
			},
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			// Verify that the state upgraded in place by the Framework resource
			// gives an empty plan, with assume_role and role_chaining_account_id
			// still null and the identity set.
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables:          vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyAssumeRole),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_account.account", tfjsonpath.New(keyRoleChainingAccountID),
					knownvalue.Null()),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_account.account", tfjsonpath.New(keyID)),
			},
		}},
	})
}

// TestAccAWSAccountResource_MoveState verifies that state from a
// polaris_aws_account resource created by the rubrikinc/polaris provider can
// be moved to a rubrik_aws_account resource using the moved {} block.
func TestAccAWSAccountResource_MoveState(t *testing.T) {
	vars := config.Variables{
		"credentials": config.StringVariable(testCredentials(t)),
		"name":        config.StringVariable(testAWSAccountName(t)),
		"profile":     config.StringVariable(testAWSProfile(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard the account with the polaris provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "credentials" {
					type      = string
					sensitive = true
				}
				variable "name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_aws_account" "account" {
					name    = var.name
					profile = var.profile

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_account.account", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("polaris_aws_account.account", tfjsonpath.New(keyName),
					knownvalue.StringExact(testAWSAccountName(t))),
				statecheck.ExpectKnownValue("polaris_aws_account.account", tfjsonpath.New(keyDeleteSnapshotsOnDestroy),
					knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("polaris_aws_account.account", tfjsonpath.New(keyCloudDiscovery),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
				statecheck.ExpectKnownValue("polaris_aws_account.account", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyRegions: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("us-east-2"),
							}),
							keyStatus:   knownvalue.StringExact("connected"),
							keyStackARN: knownvalue.NotNull(),
						}),
					})),
			},
		}, {
			// Verify that the move gives an empty plan, then apply to save the
			// moved state.
			ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
				"rubrik": newMuxedProviderServer,
			},
			Config: `
				variable "credentials" {
					type      = string
					sensitive = true
				}
				variable "name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				moved {
					from = polaris_aws_account.account
					to   = rubrik_aws_account.account
				}

				resource "rubrik_aws_account" "account" {
					name    = var.name
					profile = var.profile

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
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
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_account.account", tfjsonpath.New(keyID)),
			},
		}},
	})
}

// TestAWSAccountBlockFeatures verifies that the feature blocks of the AWS
// account resource cover exactly the features which can be onboarded using the
// AWS CloudFormation workflow.
func TestAWSAccountBlockFeatures(t *testing.T) {
	var got []string
	for _, features := range awsAccountBlockFeatures {
		got = append(got, core.FeatureNames(features)...)
	}
	slices.Sort(got)

	if want := core.AWSCloudFormationFeatureNames(); !slices.Equal(got, want) {
		t.Errorf("unexpected features, got: %s want: %s", got, want)
	}
}

// TestAWSAccountBlockPermissionGroups verifies that the permission_groups
// field of every feature block of the AWS account resource has possible
// values, i.e. that the feature name of the block is known by the SDK, and
// that every block in the feature block map is in the schema.
func TestAWSAccountBlockPermissionGroups(t *testing.T) {
	s := awsAccountTestSchema(t)

	for key := range awsAccountBlockFeatures {
		block, ok := s.Blocks[key].(schema.ListNestedBlock)
		if !ok {
			t.Errorf("feature block %s missing from the schema", key)
			continue
		}
		groups, ok := block.NestedObject.Attributes[keyPermissionGroups]
		if !ok {
			t.Errorf("feature block %s has no permission_groups field", key)
			continue
		}
		if strings.Contains(groups.GetDescription(), possibleValues([]string{})) {
			t.Errorf("feature block %s has no possible permission groups", key)
		}
	}
}

// TestAWSAccountValidateConfig checks the feature block combinations rejected
// at plan time. Only the presence of a block matters, so the attributes of the
// blocks are left out.
func TestAWSAccountValidateConfig(t *testing.T) {
	ctx := context.Background()

	r := &awsAccountResource{prefix: keyRubrik}
	s := awsAccountTestSchema(t)

	// JSON can't hold an unknown values. Turns the Cloud Native Protection
	// feature block into an unknown value.
	cnpUnknown := func(path *tftypes.AttributePath, value tftypes.Value) (tftypes.Value, error) {
		if path.Equal(tftypes.NewAttributePath().WithAttributeName(keyCloudNativeProtection)) {
			return tftypes.NewValue(value.Type(), tftypes.UnknownValue), nil
		}
		return value, nil
	}
	unknownBlock, err := tftypes.Transform(awsAccountTestValue(t, `{}`), cnpUnknown)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		config  tftypes.Value
		wantErr string
	}{{
		name:   "Feature",
		config: awsAccountTestValue(t, `{"cloud_native_protection": [{}]}`),
	}, {
		name:    "NoFeature",
		config:  awsAccountTestValue(t, `{}`),
		wantErr: "Missing feature block",
	}, {
		name:   "RoleChaining",
		config: awsAccountTestValue(t, `{"role_chaining": [{}]}`),
	}, {
		name:    "RoleChainingWithFeature",
		config:  awsAccountTestValue(t, `{"cloud_native_protection": [{}], "role_chaining": [{}]}`),
		wantErr: "Invalid block combination",
	}, {
		name: "RoleChainingWithAccountID",
		config: awsAccountTestValue(t, `{
			"role_chaining": [{}],
			"role_chaining_account_id": "3f2a1b0c-9d8e-4f7a-8b6c-5d4e3f2a1b0c"
		}`),
		wantErr: "Invalid attribute combination",
	}, {
		name:   "Outpost",
		config: awsAccountTestValue(t, `{"outpost": [{}]}`),
	}, {
		name:    "OutpostAccountIDAlone",
		config:  awsAccountTestValue(t, `{"outpost": [{"outpost_account_id": "210987654321"}]}`),
		wantErr: "Invalid block combination",
	}, {
		name: "OutpostAccountIDWithFeature",
		config: awsAccountTestValue(t, `{
			"cloud_native_protection": [{}],
			"outpost": [{"outpost_account_id": "210987654321"}]
		}`),
	}, {
		name:   "UnknownBlock",
		config: unknownBlock,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := fwresource.ValidateConfigRequest{Config: tfsdk.Config{Schema: s, Raw: tc.config}}
			var res fwresource.ValidateConfigResponse
			r.ValidateConfig(ctx, req, &res)

			var gotErr bool
			for _, d := range res.Diagnostics.Errors() {
				if d.Summary() != tc.wantErr {
					t.Fatalf("%s: %s", d.Summary(), d.Detail())
				}
				gotErr = true
			}
			if tc.wantErr != "" && !gotErr {
				t.Fatalf("got no error, want %q", tc.wantErr)
			}
		})
	}
}

// TestAWSAccountModifyPlan checks the Cloud Discovery removal and the Role
// Chaining addition rules. Only the presence of a block matters, so the
// attributes of the blocks are left out.
func TestAWSAccountModifyPlan(t *testing.T) {
	ctx := context.Background()

	r := &awsAccountResource{prefix: keyRubrik}
	s := awsAccountTestSchema(t)

	tests := []struct {
		name    string
		state   string
		plan    string
		wantErr string
	}{{
		name:  "KeepFeatures",
		state: `{"cloud_discovery": [{}], "cloud_native_protection": [{}]}`,
		plan:  `{"cloud_discovery": [{}], "cloud_native_protection": [{}]}`,
	}, {
		name:    "RemoveCloudDiscoveryWithProtection",
		state:   `{"cloud_discovery": [{}], "cloud_native_protection": [{}]}`,
		plan:    `{"cloud_native_protection": [{}]}`,
		wantErr: "Invalid feature removal",
	}, {
		name:  "RemoveCloudDiscoveryWithoutProtection",
		state: `{"cloud_discovery": [{}], "exocompute": [{}]}`,
		plan:  `{"exocompute": [{}]}`,
	}, {
		name:    "AddRoleChainingWithFeatureInState",
		state:   `{"cloud_native_protection": [{}]}`,
		plan:    `{"role_chaining": [{}]}`,
		wantErr: "Invalid feature addition",
	}, {
		name:  "KeepRoleChaining",
		state: `{"role_chaining": [{}]}`,
		plan:  `{"role_chaining": [{}]}`,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := fwresource.ModifyPlanRequest{
				State: tfsdk.State{Schema: s, Raw: awsAccountTestValue(t, tc.state)},
				Plan:  tfsdk.Plan{Schema: s, Raw: awsAccountTestValue(t, tc.plan)},
			}
			res := fwresource.ModifyPlanResponse{Plan: req.Plan}
			r.ModifyPlan(ctx, req, &res)

			var gotErr bool
			for _, d := range res.Diagnostics.Errors() {
				if d.Summary() != tc.wantErr {
					t.Fatalf("%s: %s", d.Summary(), d.Detail())
				}
				gotErr = true
			}
			if tc.wantErr != "" && !gotErr {
				t.Fatalf("got no error, want %q", tc.wantErr)
			}
		})
	}
}

// awsAccountTestSchema returns the latest schema of the AWS account resource.
func awsAccountTestSchema(t *testing.T) schema.Schema {
	t.Helper()

	var res fwresource.SchemaResponse
	r := newAwsAccountResource()
	r.Schema(context.Background(), fwresource.SchemaRequest{}, &res)
	if res.Diagnostics.HasError() {
		t.Fatal(res.Diagnostics)
	}

	return res.Schema
}

// awsAccountTestValue returns the AWS account resource state, or configuration,
// in JSON as a value of the schema type. Attributes left out are null, and
// blocks left out are empty, since Terraform never stores a null block.
func awsAccountTestValue(t *testing.T, state string) tftypes.Value {
	t.Helper()

	var attrs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(state), &attrs); err != nil {
		t.Fatal(err)
	}

	s := awsAccountTestSchema(t)
	for key := range s.Blocks {
		if _, ok := attrs[key]; !ok {
			attrs[key] = json.RawMessage("[]")
		}
	}

	buf, err := json.Marshal(attrs)
	if err != nil {
		t.Fatal(err)
	}

	rawState := tfprotov6.RawState{JSON: buf}
	value, err := rawState.Unmarshal(s.Type().TerraformType(t.Context()))
	if err != nil {
		t.Fatal(err)
	}

	return value
}
