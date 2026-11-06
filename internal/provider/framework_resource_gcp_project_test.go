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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

// gcpProjectTestVariables are the variables shared by the GCP project test
// configurations.
const gcpProjectTestVariables = `
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
`

// gcpProjectTestVars returns the variables for the GCP project test
// configurations.
func gcpProjectTestVars(t *testing.T) config.Variables {
	t.Helper()

	return config.Variables{
		"credentials":    config.StringVariable(testGCPCredentials(t)),
		"project_id":     config.StringVariable(testGCPProjectID(t)),
		"project_name":   config.StringVariable(testGCPProjectName(t)),
		"project_number": config.StringVariable(testGCPProjectNumber(t)),
	}
}

// TestAccGcpProjectResource onboards the GCP project with the deprecated
// cloud_native_protection block, then with features, updates the features and
// imports the project.
func TestAccGcpProjectResource(t *testing.T) {
	// The organization name isn't configured, RSC returns it.
	const cnpConf = gcpProjectTestVariables + `
		resource "rubrik_gcp_project" "project" {
			credentials    = var.credentials
			project        = var.project_id
			project_name   = var.project_name
			project_number = var.project_number

			cloud_native_protection {
			}

			lifecycle {
				ignore_changes = [organization_name]
			}
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the project can be onboarded with the deprecated
			// cloud_native_protection block, and that the block holds the
			// status of the feature.
			Config:          cnpConf,
			ConfigVariables: gcpProjectTestVars(t),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyProject),
					knownvalue.StringExact(testGCPProjectID(t))),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyProjectName),
					knownvalue.StringExact(testGCPProjectName(t))),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyProjectNumber),
					knownvalue.StringExact(testGCPProjectNumber(t))),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project",
					tfjsonpath.New(keyDeleteSnapshotsOnDestroy), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyStatus: knownvalue.StringExact("CONNECTED"),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_gcp_project.project", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_project.project", tfjsonpath.New(keyID)),
			},
		}},
	})

	const featureConf = gcpProjectTestVariables + `
		variable "shared_vpc_groups" {
			type = list(string)
		}
		variable "with_credentials" {
			type    = bool
			default = true
		}

		resource "rubrik_gcp_project" "project" {
			credentials    = var.with_credentials ? var.credentials : null
			project        = var.project_id
			project_name   = var.project_name
			project_number = var.project_number

			feature {
				name              = "CLOUD_NATIVE_PROTECTION"
				permission_groups = ["BASIC", "EXPORT_AND_RESTORE", "FILE_LEVEL_RECOVERY"]
			}

			feature {
				name              = "GCP_SHARED_VPC_HOST"
				permission_groups = var.shared_vpc_groups
			}

			lifecycle {
				ignore_changes = [organization_name]
			}
		}
	`
	featureVars := func(sharedVPCGroups ...string) config.Variables {
		vars := gcpProjectTestVars(t)
		var groups []config.Variable
		for _, group := range sharedVPCGroups {
			groups = append(groups, config.StringVariable(group))
		}
		vars["shared_vpc_groups"] = config.ListVariable(groups...)
		return vars
	}
	importVars := featureVars("BASIC", "CLOUDSQL")
	importVars["with_credentials"] = config.BoolVariable(false)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the project can be onboarded with two features.
			Config:          featureConf,
			ConfigVariables: featureVars("BASIC"),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
								knownvalue.StringExact("EXPORT_AND_RESTORE"),
								knownvalue.StringExact("FILE_LEVEL_RECOVERY"),
							}),
							keyPermissions: knownvalue.Null(),
							keyStatus:      knownvalue.StringExact("CONNECTED"),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("GCP_SHARED_VPC_HOST"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
							}),
							keyPermissions: knownvalue.Null(),
							keyStatus:      knownvalue.StringExact("CONNECTED"),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_gcp_project.project", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_project.project", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that adding a permission group to a feature updates the
			// project in place.
			Config:          featureConf,
			ConfigVariables: featureVars("BASIC", "CLOUDSQL"),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_project.project", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetPartial([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("GCP_SHARED_VPC_HOST"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
								knownvalue.StringExact("CLOUDSQL"),
							}),
							keyPermissions: knownvalue.Null(),
							keyStatus:      knownvalue.StringExact("CONNECTED"),
						}),
					})),
				statecheck.ExpectIdentity("rubrik_gcp_project.project", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_project.project", tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import. The credentials are never returned by RSC.
			ResourceName:            "rubrik_gcp_project.project",
			Config:                  featureConf,
			ConfigVariables:         featureVars("BASIC", "CLOUDSQL"),
			ImportStateKind:         resource.ImportCommandWithID,
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{keyCredentials},
		}, {
			// import {} block with id attribute. RSC never returns the
			// credentials, so the project is imported without them, as an
			// already onboarded project would be.
			ResourceName:    "rubrik_gcp_project.project",
			Config:          featureConf,
			ConfigVariables: importVars,
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity, without credentials.
			ResourceName:    "rubrik_gcp_project.project",
			Config:          featureConf,
			ConfigVariables: importVars,
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

// TestAccGcpProjectResourceDefaultServiceAccount onboards the GCP project
// without credentials, using the default service account.
func TestAccGcpProjectResourceDefaultServiceAccount(t *testing.T) {
	vars := gcpProjectTestVars(t)
	vars["organization_name"] = config.StringVariable(testGCPOrganizationName(t))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the project can be onboarded using the default
			// service account and the given organization name.
			Config: gcpProjectTestVariables + `
				variable "organization_name" {
					type = string
				}

				resource "rubrik_gcp_service_account" "default" {
					credentials = var.credentials

					lifecycle {
						ignore_changes = [name]
					}
				}

				resource "rubrik_gcp_project" "project" {
					organization_name = var.organization_name
					project           = var.project_id
					project_name      = var.project_name
					project_number    = var.project_number

					cloud_native_protection {
					}

					depends_on = [rubrik_gcp_service_account.default]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyOrganizationName),
					knownvalue.StringExact(testGCPOrganizationName(t))),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyCredentials),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyStatus: knownvalue.StringExact("CONNECTED"),
						}),
					})),
			},
		}},
	})
}

// TestAccGcpProjectResourceCloudSQL verifies that the Cloud SQL protection
// feature can be onboarded on a GCP project, together with the CLOUDSQL
// permission group of the EXOCOMPUTE and GCP_SHARED_VPC_HOST features, which
// RSC requires for Cloud SQL archival and archived recovery.
//
// The test skips unless the CNP_GCP_SQL_ENABLED feature flag is enabled for the
// RSC account, since RSC rejects the feature without it.
func TestAccGcpProjectResourceCloudSQL(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagGCPSQL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the three features are onboarded.
			Config: gcpProjectTestVariables + `
				resource "rubrik_gcp_project" "project" {
					credentials    = var.credentials
					project        = var.project_id
					project_name   = var.project_name
					project_number = var.project_number

					feature {
						name              = "CLOUD_SQL_PROTECTION"
						permission_groups = ["BASIC", "EXPORT_AND_RESTORE"]
					}

					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "CLOUDSQL"]
					}

					feature {
						name              = "GCP_SHARED_VPC_HOST"
						permission_groups = ["BASIC", "CLOUDSQL"]
					}

					lifecycle {
						ignore_changes = [organization_name]
					}
				}
			`,
			ConfigVariables: gcpProjectTestVars(t),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("CLOUD_SQL_PROTECTION"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
								knownvalue.StringExact("EXPORT_AND_RESTORE"),
							}),
							keyPermissions: knownvalue.Null(),
							keyStatus:      knownvalue.StringExact("CONNECTED"),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("EXOCOMPUTE"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
								knownvalue.StringExact("CLOUDSQL"),
							}),
							keyPermissions: knownvalue.Null(),
							keyStatus:      knownvalue.StringExact("CONNECTED"),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName: knownvalue.StringExact("GCP_SHARED_VPC_HOST"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("BASIC"),
								knownvalue.StringExact("CLOUDSQL"),
							}),
							keyPermissions: knownvalue.Null(),
							keyStatus:      knownvalue.StringExact("CONNECTED"),
						}),
					})),
			},
		}},
	})
}

// TestAccGcpProjectResourceCloudDiscovery verifies that the Cloud Discovery
// feature can be onboarded together with a protection feature, that the plan
// refuses to remove Cloud Discovery while the protection feature is enabled,
// and that removing the protection feature first, then Cloud Discovery, works.
//
// The test skips unless the REL_ENABLE_CNP_GCP_CLOUD_DISCOVERY_MODE feature
// flag is enabled for the RSC account, since RSC rejects the feature without it.
func TestAccGcpProjectResourceCloudDiscovery(t *testing.T) {
	skipUnlessFeatureEnabled(t, core.FeatureFlagName("REL_ENABLE_CNP_GCP_CLOUD_DISCOVERY_MODE"))

	const conf = gcpProjectTestVariables + `
		variable "cloud_discovery" {
			type = bool
		}
		variable "protection" {
			type = bool
		}

		resource "rubrik_gcp_project" "project" {
			credentials    = var.credentials
			project        = var.project_id
			project_name   = var.project_name
			project_number = var.project_number

			dynamic "feature" {
				for_each = var.cloud_discovery ? [1] : []
				content {
					name              = "CLOUD_DISCOVERY"
					permission_groups = ["BASIC"]
				}
			}

			dynamic "feature" {
				for_each = var.protection ? [1] : []
				content {
					name              = "CLOUD_NATIVE_PROTECTION"
					permission_groups = ["BASIC"]
				}
			}

			lifecycle {
				ignore_changes = [organization_name]
			}
		}
	`
	vars := func(cloudDiscovery, protection bool) config.Variables {
		v := gcpProjectTestVars(t)
		v["cloud_discovery"] = config.BoolVariable(cloudDiscovery)
		v["protection"] = config.BoolVariable(protection)
		return v
	}
	cloudDiscovery := knownvalue.ObjectExact(map[string]knownvalue.Check{
		keyName:             knownvalue.StringExact("CLOUD_DISCOVERY"),
		keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{knownvalue.StringExact("BASIC")}),
		keyPermissions:      knownvalue.Null(),
		keyStatus:           knownvalue.StringExact("CONNECTED"),
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard Cloud Discovery together with a protection feature.
			Config:          conf,
			ConfigVariables: vars(true, true),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						cloudDiscovery,
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyName:             knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{knownvalue.StringExact("BASIC")}),
							keyPermissions:      knownvalue.Null(),
							keyStatus:           knownvalue.StringExact("CONNECTED"),
						}),
					})),
			},
		}, {
			// Removing Cloud Discovery while the protection feature remains is
			// rejected at plan time, before anything is sent to RSC.
			Config:          conf,
			ConfigVariables: vars(false, true),
			PlanOnly:        true,
			ExpectError: regexp.MustCompile(`CLOUD_DISCOVERY cannot be removed while the protection feature\s+` +
				`CLOUD_NATIVE_PROTECTION is enabled`),
		}, {
			// Remove the protection feature first, keeping Cloud Discovery.
			Config:          conf,
			ConfigVariables: vars(true, false),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{cloudDiscovery})),
			},
		}},
		// Destroy removes the remaining Cloud Discovery feature.
	})
}

// TestAccGcpProjectResource_MoveState verifies that state from a
// polaris_gcp_project resource created by the rubrikinc/polaris provider can
// be moved to a rubrik_gcp_project resource using the moved {} block.
func TestAccGcpProjectResource_MoveState(t *testing.T) {
	vars := gcpProjectTestVars(t)
	vars["rsc_credentials"] = config.StringVariable(testCredentials(t))

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard the project with the polaris provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			Config: gcpProjectTestVariables + `
				variable "rsc_credentials" {
					type      = string
					sensitive = true
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
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}

					lifecycle {
						ignore_changes = [organization_name]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_project.project", tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			// Verify that the move gives an empty plan, then apply to save the
			// moved state, without it destroy can fail on the dependencies.
			ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
				"rubrik": newMuxedProviderServer,
			},
			Config: gcpProjectTestVariables + `
				variable "rsc_credentials" {
					type      = string
					sensitive = true
				}

				moved {
					from = polaris_gcp_project.project
					to   = rubrik_gcp_project.project
				}

				resource "rubrik_gcp_project" "project" {
					credentials    = var.credentials
					project        = var.project_id
					project_name   = var.project_name
					project_number = var.project_number

					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}

					lifecycle {
						ignore_changes = [organization_name]
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
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyCloudNativeProtection),
					knownvalue.ListSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_gcp_project.project", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
			},
		}},
	})
}
