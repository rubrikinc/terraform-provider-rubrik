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

// TestAccGCPRCVArchivalLocationResource creates, updates and imports an RCV
// GCP archival location encrypted with a key RCV creates and manages.
func TestAccGCPRCVArchivalLocationResource(t *testing.T) {
	skipUnlessGCPRCVDefined(t)

	name := testUniqueName(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpRCVArchivalLocationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				resource "rubrik_gcp_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
					tier   = "BACKUP"
				}
			`,
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name),
				"region": config.StringVariable(testGCPRCVRegion(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location", tfjsonpath.New(keyRegion),
					knownvalue.StringExact(testGCPRCVRegion(t))),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location", tfjsonpath.New(keyTier),
					knownvalue.StringExact("BACKUP")),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location", tfjsonpath.New(keyKMSKey),
					knownvalue.ListSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_gcp_rcv_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_rcv_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name is updated in place. RSC only supports
			// updating the name, the tier and the region force replacement.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				resource "rubrik_gcp_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
					tier   = "BACKUP"
				}
			`,
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testGCPRCVRegion(t)),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_rcv_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_rcv_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_gcp_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testGCPRCVRegion(t)),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_gcp_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testGCPRCVRegion(t)),
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
			ResourceName: "rubrik_gcp_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testGCPRCVRegion(t)),
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

// TestAccGCPRCVArchivalLocationKMSKeyResource onboards the GCP project with the
// UEM key management feature, registers a key manager, creates an RCV GCP
// archival location encrypted with the first key and rekeys it to the second
// key.
func TestAccGCPRCVArchivalLocationKMSKeyResource(t *testing.T) {
	skipUnlessGCPRCVDefined(t)
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
		variable "name" {
			type = string
		}
		variable "kms_region" {
			type = string
		}
		variable "key_ring" {
			type = string
		}
		variable "rcv_region" {
			type = string
		}
		variable "key_name" {
			type = string
		}
		variable "key_version" {
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
			cloud_account_id = rubrik_gcp_project.project.id
			region           = var.kms_region
			key_ring         = var.key_ring
		}

		resource "rubrik_gcp_rcv_archival_location" "location" {
			name   = var.name
			region = var.rcv_region
			tier   = "BACKUP"

			kms_key {
				key_manager_id = rubrik_gcp_kms_key_manager.key_manager.id
				key_name       = var.key_name
				key_version    = var.key_version
			}
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpRCVArchivalLocationCheckDestroy(t),
			gcpKMSKeyManagerCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the project can be onboarded with the UEM key
			// management feature, that a key manager can be registered, and
			// that an archival location can be created with the first key.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"name":           config.StringVariable(name),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"rcv_region":     config.StringVariable(testGCPRCVRegion(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[0].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[0].Version),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyName:   knownvalue.StringExact("CLOUD_NATIVE_UEM_KEY_MANAGEMENT"),
							keyStatus: knownvalue.StringExact("CONNECTED"),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testGCPKMSKeys(t)[0].Name)),
				statecheck.CompareValuePairs(
					"rubrik_gcp_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"rubrik_gcp_rcv_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyManagerID),
					compare.ValuesSame()),
			},
		}, {
			// Verify that changing the key rekeys the archival location in
			// place, keeping its ID.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"name":           config.StringVariable(name),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"rcv_region":     config.StringVariable(testGCPRCVRegion(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[1].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[1].Version),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_rcv_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testGCPKMSKeys(t)[1].Name)),
				statecheck.ExpectKnownValue("rubrik_gcp_rcv_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyVersion),
					knownvalue.StringExact(testGCPKMSKeys(t)[1].Version)),
			},
		}, {
			// Terraform import, verifying that the KMS key is read back.
			ResourceName: "rubrik_gcp_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"name":           config.StringVariable(name),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"rcv_region":     config.StringVariable(testGCPRCVRegion(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[1].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[1].Version),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_gcp_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"name":           config.StringVariable(name),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"rcv_region":     config.StringVariable(testGCPRCVRegion(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[1].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[1].Version),
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
			ResourceName: "rubrik_gcp_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"name":           config.StringVariable(name),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"rcv_region":     config.StringVariable(testGCPRCVRegion(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[1].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[1].Version),
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
