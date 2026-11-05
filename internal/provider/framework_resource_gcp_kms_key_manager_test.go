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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccGCPKMSKeyManagerResource onboards the GCP project with the UEM key
// management feature, registers a key manager, updates and imports it.
func TestAccGCPKMSKeyManagerResource(t *testing.T) {
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
		variable "description" {
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

		resource "rubrik_gcp_kms_key_manager" "key_manager" {
			name             = var.name
			description      = var.description
			cloud_account_id = rubrik_gcp_project.project.id
			region           = var.kms_region
			key_ring         = var.key_ring
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpKMSKeyManagerCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that a key manager can be registered for the project.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"name":           config.StringVariable(name),
				"description":    config.StringVariable("Test key manager."),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Test key manager.")),
				statecheck.ExpectKnownValue("rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyRegion),
					knownvalue.StringExact(testGCPKMSRegion(t))),
				statecheck.ExpectKnownValue("rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyKeyRing),
					knownvalue.StringExact(testGCPKMSKeyRing(t))),
				statecheck.CompareValuePairs(
					"rubrik_gcp_project.project", tfjsonpath.New(keyID),
					"rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectIdentity("rubrik_gcp_kms_key_manager.key_manager", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_kms_key_manager.key_manager",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name and the description are updated in place.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"name":           config.StringVariable(name + " Update"),
				"description":    config.StringVariable("Updated test key manager."),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_kms_key_manager.key_manager",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectKnownValue("rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Updated test key manager.")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_kms_key_manager.key_manager",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_gcp_kms_key_manager.key_manager",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"name":           config.StringVariable(name + " Update"),
				"description":    config.StringVariable("Updated test key manager."),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_gcp_kms_key_manager.key_manager",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"name":           config.StringVariable(name + " Update"),
				"description":    config.StringVariable("Updated test key manager."),
			},
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName: "rubrik_gcp_kms_key_manager.key_manager",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"name":           config.StringVariable(name + " Update"),
				"description":    config.StringVariable("Updated test key manager."),
			},
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
