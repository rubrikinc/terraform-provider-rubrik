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

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccSSOGroupListResource(t *testing.T) {
	checkTestSSOGroup(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			customRoleCheckDestroy(t.Context()),
			ssoGroupCheckDestroy(t.Context()),
		),
		Steps: []resource.TestStep{{
			// Create the SSO group so the list resource has something to
			// return. The list step below uses the same group.
			Config: `
				variable "sso_group_name" {
					type = string
				}
				variable "auth_domain_id" {
					type = string
				}

				resource "polaris_custom_role" "role" {
					name        = "Test Role for SSO Group List"
					description = "Test Role: Delete Me!"

					permission {
						operation = "VIEW_CLUSTER"
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
			ConfigVariables: config.Variables{
				"auth_domain_id": config.StringVariable(testAuthDomainID(t)),
				"sso_group_name": config.StringVariable(testSSOGroupName(t)),
			},
		}, {
			Query: true,
			Config: `
				variable "sso_group_name" {
					type = string
				}
				variable "auth_domain_id" {
					type = string
				}

				resource "polaris_custom_role" "role" {
					name        = "Test Role for SSO Group List"
					description = "Test Role: Delete Me!"

					permission {
						operation = "VIEW_CLUSTER"
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

				provider "polaris" {}

				list "polaris_sso_group" "all" {
					provider = polaris
				}
			`,
			ConfigVariables: config.Variables{
				"auth_domain_id": config.StringVariable(testAuthDomainID(t)),
				"sso_group_name": config.StringVariable(testSSOGroupName(t)),
			},
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("polaris_sso_group.all", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
			},
		}, {
			Query: true,
			Config: `
				variable "sso_group_name" {
					type = string
				}
				variable "auth_domain_id" {
					type = string
				}

				resource "polaris_custom_role" "role" {
					name        = "Test Role for SSO Group List"
					description = "Test Role: Delete Me!"

					permission {
						operation = "VIEW_CLUSTER"
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

				provider "polaris" {}

				list "polaris_sso_group" "filtered" {
					provider = polaris

					config {
						name = var.sso_group_name
					}
				}
			`,
			ConfigVariables: config.Variables{
				"auth_domain_id": config.StringVariable(testAuthDomainID(t)),
				"sso_group_name": config.StringVariable(testSSOGroupName(t)),
			},
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("polaris_sso_group.filtered", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				querycheck.ExpectLength("polaris_sso_group.filtered", 1),
			},
		}},
	})
}
