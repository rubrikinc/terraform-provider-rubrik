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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccGCPArchivalLocationDataSourceFrameworkMigration verifies that the
// migrated gcp_archival_location data source returns the same values as the
// SDKv2 implementation in the polaris provider, both when looked up by ID and
// when looked up by name. The name lookup uses the name in upper case to verify
// that the name is returned as stored in RSC, not as configured.
func TestAccGCPArchivalLocationDataSourceFrameworkMigration(t *testing.T) {
	skipUnlessGCPArchivalDefined(t)

	name := testUniqueName(t)

	var checks []statecheck.StateCheck
	for _, attr := range []string{
		keyID,
		keyBucketLabels,
		keyBucketPrefix,
		keyCloudAccountID,
		keyConnectionStatus,
		keyCustomerManagedKey,
		keyLocationTemplate,
		keyName,
		keyRegion,
		keyStorageClass,
	} {
		checks = append(checks,
			statecheck.CompareValuePairs(
				"data.polaris_gcp_archival_location.old_by_id", tfjsonpath.New(attr),
				"data.polaris_gcp_archival_location.new_by_id", tfjsonpath.New(attr),
				compare.ValuesSame()),
			statecheck.CompareValuePairs(
				"data.polaris_gcp_archival_location.old_by_name", tfjsonpath.New(attr),
				"data.polaris_gcp_archival_location.new_by_name", tfjsonpath.New(attr),
				compare.ValuesSame()))
	}

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.12.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpArchivalLocationCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Onboard the project, create the archival location and read it
			// with both data source implementations.
			Config: `
				variable "credentials" {
					type      = string
					sensitive = true
				}
				variable "project_credentials" {
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
				variable "name" {
					type = string
				}
				variable "lookup_name" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				resource "rubrik_gcp_project" "project" {
					credentials    = var.project_credentials
					project        = var.project_id
					project_name   = var.project_name
					project_number = var.project_number

					feature {
						name              = "CLOUD_NATIVE_ARCHIVAL"
						permission_groups = ["BASIC"]
					}

					# The organization name can't be read back for a project
					# without a protection feature.
					lifecycle {
						ignore_changes = [organization_name]
					}
				}

				resource "rubrik_gcp_archival_location" "location" {
					cloud_account_id = rubrik_gcp_project.project.id
					name             = var.name
					bucket_prefix    = "tf-acc-archival"
					region           = var.region
					storage_class    = "NEARLINE"
					bucket_labels = {
						environment = "test"
						owner       = "terraform"
					}
				}

				data "polaris_gcp_archival_location" "old_by_id" {
					provider = polaris-sdkv2
					id       = rubrik_gcp_archival_location.location.id
				}

				data "polaris_gcp_archival_location" "new_by_id" {
					id = rubrik_gcp_archival_location.location.id
				}

				data "polaris_gcp_archival_location" "old_by_name" {
					provider   = polaris-sdkv2
					name       = var.lookup_name
					depends_on = [rubrik_gcp_archival_location.location]
				}

				data "polaris_gcp_archival_location" "new_by_name" {
					name       = var.lookup_name
					depends_on = [rubrik_gcp_archival_location.location]
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":         config.StringVariable(testCredentials(t)),
				"project_credentials": config.StringVariable(testGCPCredentials(t)),
				"project_id":          config.StringVariable(testGCPProjectID(t)),
				"project_name":        config.StringVariable(testGCPProjectName(t)),
				"project_number":      config.StringVariable(testGCPProjectNumber(t)),
				"region":              config.StringVariable(testGCPArchivalRegion(t)),
				"name":                config.StringVariable(name),
				"lookup_name":         config.StringVariable(strings.ToUpper(name)),
			},
			// Verify that every attribute matches the SDKv2 implementation.
			ConfigStateChecks: checks,
		}},
	})
}
