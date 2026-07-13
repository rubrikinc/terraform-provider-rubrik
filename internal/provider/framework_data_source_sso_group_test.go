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

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSSOGroupDataSource(t *testing.T) {
	skipUnlessSSOGroupDefined(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			customRoleCheckDestroy(t),
			ssoGroupCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the data source can look up the SSO group
			// by ID and name.
			Config: `
				variable "auth_domain_id" {
					type = string
				}
				variable "sso_group_name" {
					type = string
				}

				resource "rubrik_custom_role" "role" {
					name        = "Test Role for SSO Group Data Source"
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

				data "rubrik_sso_group" "by_name" {
					name = rubrik_sso_group.group.group_name
				}

				data "rubrik_sso_group" "by_id" {
					sso_group_id = rubrik_sso_group.group.id
				}
			`,
			ConfigVariables: config.Variables{
				"auth_domain_id": config.StringVariable(testAuthDomainID(t)),
				"sso_group_name": config.StringVariable(testSSOGroupName(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By Name.
				statecheck.ExpectKnownValue("data.rubrik_sso_group.by_name", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
				statecheck.CompareValuePairs(
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keyID),
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keySSOGroupID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_sso_group.by_name", tfjsonpath.New(keyDomainName),
					knownvalue.NotNull()),
				// By ID.
				statecheck.CompareValuePairs(
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keyID),
					"data.rubrik_sso_group.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keySSOGroupID),
					"data.rubrik_sso_group.by_id", tfjsonpath.New(keySSOGroupID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keyName),
					"data.rubrik_sso_group.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keyDomainName),
					"data.rubrik_sso_group.by_id", tfjsonpath.New(keyDomainName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keyRoles),
					"data.rubrik_sso_group.by_id", tfjsonpath.New(keyRoles),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_sso_group.by_name", tfjsonpath.New(keyUsers),
					"data.rubrik_sso_group.by_id", tfjsonpath.New(keyUsers),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccSSOGroupDataSource_FrameworkMigration verifies that the migrated SSO
// group data source is backwards compatible with the SDKv2 provider.
func TestAccSSOGroupDataSource_FrameworkMigration(t *testing.T) {
	skipUnlessSSOGroupDefined(t)

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.5.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			customRoleCheckDestroy(t),
			ssoGroupCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the two data sources are equal.
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

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				resource "polaris_custom_role" "role" {
					name        = "Test Role for SSO Group Data Source Migration"
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

				data "polaris_sso_group" "old" {
					provider = polaris-sdkv2

					name = polaris_sso_group.group.group_name
				}

				data "polaris_sso_group" "new" {
					name = polaris_sso_group.group.group_name
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testCredentials(t)),
				"auth_domain_id": config.StringVariable(testAuthDomainID(t)),
				"sso_group_name": config.StringVariable(testSSOGroupName(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.polaris_sso_group.new", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
				statecheck.CompareValuePairs(
					"data.polaris_sso_group.old", tfjsonpath.New(keyID),
					"data.polaris_sso_group.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_sso_group.old", tfjsonpath.New(keySSOGroupID),
					"data.polaris_sso_group.new", tfjsonpath.New(keySSOGroupID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_sso_group.old", tfjsonpath.New(keyName),
					"data.polaris_sso_group.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_sso_group.old", tfjsonpath.New(keyDomainName),
					"data.polaris_sso_group.new", tfjsonpath.New(keyDomainName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_sso_group.old", tfjsonpath.New(keyRoles),
					"data.polaris_sso_group.new", tfjsonpath.New(keyRoles),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_sso_group.old", tfjsonpath.New(keyUsers),
					"data.polaris_sso_group.new", tfjsonpath.New(keyUsers),
					compare.ValuesSame()),
			},
		}},
	})
}
