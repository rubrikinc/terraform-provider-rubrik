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

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccGcpArchivalLocationResource onboards the GCP project with the Cloud
// Native Archival feature, creates a specific region archival location,
// updates and imports it, reads it with the data source, and removes the bucket
// labels with an empty map.
func TestAccGcpArchivalLocationResource(t *testing.T) {
	skipUnlessGCPArchivalDefined(t)

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
		variable "region" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "storage_class" {
			type = string
		}
		variable "bucket_labels" {
			type = map(string)
		}

		resource "rubrik_gcp_project" "project" {
			credentials    = var.credentials
			project        = var.project_id
			project_name   = var.project_name
			project_number = var.project_number

			feature {
				name              = "CLOUD_NATIVE_ARCHIVAL"
				permission_groups = ["BASIC"]
			}

			# The organization name can't be read back for a project without a
			# protection feature.
			lifecycle {
				ignore_changes = [organization_name]
			}
		}

		resource "rubrik_gcp_archival_location" "location" {
			cloud_account_id = rubrik_gcp_project.project.id
			name             = var.name
			bucket_prefix    = "tf-acc-archival"
			region           = var.region
			storage_class    = var.storage_class
			bucket_labels    = var.bucket_labels
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpArchivalLocationCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the project can be onboarded with the Cloud Native
			// Archival feature and that an archival location can be created.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name),
				"storage_class":  config.StringVariable("STANDARD"),
				"bucket_labels": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
				}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_project.project", tfjsonpath.New(keyFeature),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyName:   knownvalue.StringExact("CLOUD_NATIVE_ARCHIVAL"),
							keyStatus: knownvalue.StringExact("CONNECTED"),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectIdentity("rubrik_gcp_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyID)),
				statecheck.CompareValuePairs(
					"rubrik_gcp_project.project", tfjsonpath.New(keyID),
					"rubrik_gcp_archival_location.location", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyRegion),
					knownvalue.StringExact(testGCPArchivalRegion(t))),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyStorageClass),
					knownvalue.StringExact("STANDARD")),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyBucketLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						"environment": knownvalue.StringExact("test"),
					})),
			},
		}, {
			// Verify that the name, the storage class and the bucket labels are
			// updated in place.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("NEARLINE"),
				"bucket_labels": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectIdentity("rubrik_gcp_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyID)),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyStorageClass),
					knownvalue.StringExact("NEARLINE")),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyBucketLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{
						"environment": knownvalue.StringExact("test"),
						"owner":       knownvalue.StringExact("terraform"),
					})),
			},
		}, {
			// Verify that the data source reads the archival location by ID
			// and by name.
			Config: conf + `
				data "rubrik_gcp_archival_location" "by_id" {
					id = rubrik_gcp_archival_location.location.id
				}
				data "rubrik_gcp_archival_location" "by_name" {
					name = rubrik_gcp_archival_location.location.name
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("NEARLINE"),
				"bucket_labels": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_gcp_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_gcp_archival_location.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_archival_location.location", tfjsonpath.New(keyName),
					"data.rubrik_gcp_archival_location.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_archival_location.location", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_gcp_archival_location.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_gcp_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_gcp_archival_location.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_gcp_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("NEARLINE"),
				"bucket_labels": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_gcp_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("NEARLINE"),
				"bucket_labels": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity.
			ResourceName: "rubrik_gcp_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("NEARLINE"),
				"bucket_labels": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// Verify that configuring an empty map removes the bucket labels and
			// that the empty map is kept in state, not read back as null, so the
			// plan after the apply is empty. Last, since an import can't tell an
			// empty map from no labels.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("NEARLINE"),
				"bucket_labels":  config.MapVariable(map[string]config.Variable{}),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
				PostApplyPostRefresh: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyBucketLabels),
					knownvalue.MapExact(map[string]knownvalue.Check{})),
			},
		}},
	})
}

// TestAccGcpArchivalLocationKMSKeyResource onboards the GCP project with the
// Cloud Native Archival and UEM key management features, registers a key
// manager, creates a client-side encrypted archival location, renames it,
// rekeys it and imports it.
func TestAccGcpArchivalLocationKMSKeyResource(t *testing.T) {
	skipUnlessGCPArchivalDefined(t)
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
		variable "region" {
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
				name              = "CLOUD_NATIVE_ARCHIVAL"
				permission_groups = ["BASIC"]
			}
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

		resource "rubrik_gcp_archival_location" "location" {
			cloud_account_id = rubrik_gcp_project.project.id
			name             = var.name
			bucket_prefix    = "tf-acc-kms"
			region           = var.region

			kms_key {
				key_manager_id = rubrik_gcp_key_manager.key_manager.id
				key_name       = var.key_name
				key_version    = var.key_version
			}
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpArchivalLocationCheckDestroy(t),
			gcpKeyManagerCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that a client-side encrypted archival location can be
			// created with a key from the key manager.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[0].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[0].Version),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectIdentity("rubrik_gcp_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyID)),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testGCPKMSKeys(t)[0].Name)),
				statecheck.CompareValuePairs(
					"rubrik_gcp_key_manager.key_manager", tfjsonpath.New(keyID),
					"rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyManagerID),
					compare.ValuesSame()),
			},
		}, {
			// Verify that a client-side encrypted archival location can be
			// renamed, an update must not send customer managed keys.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[0].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[0].Version),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectIdentity("rubrik_gcp_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that changing the key name and the key version rekeys the
			// archival location in place.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[1].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[1].Version),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_gcp_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testGCPKMSKeys(t)[1].Name)),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyVersion),
					knownvalue.StringExact(testGCPKMSKeys(t)[1].Version)),
				statecheck.ExpectIdentity("rubrik_gcp_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import, verifying that the KMS key is read back.
			ResourceName: "rubrik_gcp_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
				"key_name":       config.StringVariable(testGCPKMSKeys(t)[1].Name),
				"key_version":    config.StringVariable(testGCPKMSKeys(t)[1].Version),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_gcp_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
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
			// import {} block with identity.
			ResourceName: "rubrik_gcp_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":    config.StringVariable(testGCPCredentials(t)),
				"project_id":     config.StringVariable(testGCPProjectID(t)),
				"project_name":   config.StringVariable(testGCPProjectName(t)),
				"project_number": config.StringVariable(testGCPProjectNumber(t)),
				"region":         config.StringVariable(testGCPArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"kms_region":     config.StringVariable(testGCPKMSRegion(t)),
				"key_ring":       config.StringVariable(testGCPKMSKeyRing(t)),
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

// TestAccGcpArchivalLocationResource_MoveState verifies that state from a
// polaris_gcp_archival_location resource created by the rubrikinc/polaris
// provider can be moved to a rubrik_gcp_archival_location resource using the
// moved {} block. The source region archival location has no region and no
// labels, which the SDKv2 provider stores as an empty string and an empty map.
func TestAccGcpArchivalLocationResource_MoveState(t *testing.T) {
	skipUnlessGCPArchivalDefined(t)

	vars := config.Variables{
		"rsc_credentials": config.StringVariable(testCredentials(t)),
		"credentials":     config.StringVariable(testGCPCredentials(t)),
		"project_id":      config.StringVariable(testGCPProjectID(t)),
		"project_name":    config.StringVariable(testGCPProjectName(t)),
		"project_number":  config.StringVariable(testGCPProjectNumber(t)),
		"name":            config.StringVariable(testUniqueName(t)),
	}

	// The project stays with the polaris provider, so the polaris name can't
	// also be served by the provider under test.
	const project = `
		variable "rsc_credentials" {
			type      = string
			sensitive = true
		}
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

		provider "polaris" {
			credentials = var.rsc_credentials
		}

		resource "polaris_gcp_project" "project" {
			credentials    = var.credentials
			project        = var.project_id
			project_name   = var.project_name
			project_number = var.project_number

			feature {
				name              = "CLOUD_NATIVE_ARCHIVAL"
				permission_groups = ["BASIC"]
			}

			lifecycle {
				ignore_changes = [organization_name]
			}
		}
	`

	polarisProvider := map[string]resource.ExternalProvider{
		"polaris": {
			Source:            "rubrikinc/polaris",
			VersionConstraint: "1.11.2",
		},
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpArchivalLocationCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Create the archival location with the polaris provider.
			ExternalProviders: polarisProvider,
			Config: project + `
				resource "polaris_gcp_archival_location" "location" {
					cloud_account_id = polaris_gcp_project.project.id
					name             = var.name
					bucket_prefix    = "tf-acc-move"
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
			},
		}, {
			// Verify that the move gives an empty plan, then apply to save the
			// moved state, without it destroy can fail on the dependencies.
			ExternalProviders: polarisProvider,
			ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
				"rubrik": newMuxedProviderServer,
			},
			Config: project + `
				moved {
					from = polaris_gcp_archival_location.location
					to   = rubrik_gcp_archival_location.location
				}

				resource "rubrik_gcp_archival_location" "location" {
					cloud_account_id = polaris_gcp_project.project.id
					name             = var.name
					bucket_prefix    = "tf-acc-move"
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location", tfjsonpath.New(keyRegion),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SOURCE_REGION")),
				statecheck.ExpectIdentity("rubrik_gcp_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
			},
		}},
	})
}
