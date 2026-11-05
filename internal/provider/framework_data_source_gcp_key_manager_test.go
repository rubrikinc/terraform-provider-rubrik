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

// TestAccGCPKeyManagerDataSource onboards the GCP project with the UEM key
// management feature, registers a key manager and reads it with the data
// source by ID and by name.
func TestAccGCPKeyManagerDataSource(t *testing.T) {
	skipUnlessGCPKMSDefined(t)
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	name := testUniqueName(t)

	const conf = `
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
		variable "kms_region" {
			type = string
		}
		variable "key_ring" {
			type = string
		}
		variable "name" {
			type = string
		}

		resource "rubrik_gcp_project" "project" {
			credentials    = var.credentials
			project        = var.project_id
			project_name   = var.project_name
			project_number = var.project_number

			feature {
				name              = "CLOUD_NATIVE_UEM_KEY_MANAGEMENT"
				permission_groups = ["BASIC"]
			}

			# The organization name can't be read back for a project without a
			# protection feature.
			lifecycle {
				ignore_changes = [organization_name]
			}
		}

		resource "rubrik_gcp_key_manager" "key_manager" {
			name             = var.name
			cloud_account_id = rubrik_gcp_project.project.id
			region           = var.kms_region
			key_ring         = var.key_ring
		}

		data "rubrik_gcp_key_manager" "by_id" {
			id = rubrik_gcp_key_manager.key_manager.id
		}

		data "rubrik_gcp_key_manager" "by_name" {
			name = rubrik_gcp_key_manager.key_manager.name
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpKeyManagerCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the data source reads the key manager by ID and by
			// name.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"name":           config.StringVariable(name),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By ID.
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyID),
					"data.rubrik_gcp_key_manager.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyName),
					"data.rubrik_gcp_key_manager.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyDescription),
					"data.rubrik_gcp_key_manager.by_id", tfjsonpath.New(keyDescription),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_gcp_key_manager.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyRegion),
					"data.rubrik_gcp_key_manager.by_id", tfjsonpath.New(keyRegion),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyKeyRing),
					"data.rubrik_gcp_key_manager.by_id", tfjsonpath.New(keyKeyRing),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_gcp_key_manager.by_id",
					tfjsonpath.New(keyProvisioningMethod), knownvalue.StringExact("CLOUD_ACCOUNT_PROVISION")),
				// By name.
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyID),
					"data.rubrik_gcp_key_manager.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}},
	})
}
