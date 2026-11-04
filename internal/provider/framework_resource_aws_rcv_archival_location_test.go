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
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAWSRCVArchivalLocationResource creates, updates and imports an RCV
// AWS archival location encrypted with a key RCV creates and manages.
func TestAccAWSRCVArchivalLocationResource(t *testing.T) {
	skipUnlessAWSRCVDefined(t)

	name := testUniqueName(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsRCVArchivalLocationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "tier" {
					type = string
				}
				resource "rubrik_aws_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
					tier   = var.tier
				}
			`,
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name),
				"region": config.StringVariable(testAWSRCVRegion(t)),
				"tier":   config.StringVariable("BACKUP"),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyRegion),
					knownvalue.StringExact(testAWSRCVRegion(t))),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyTier),
					knownvalue.StringExact("BACKUP")),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyKMSKey),
					knownvalue.ListSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_aws_rcv_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name is updated in place. The tier is left
			// unchanged, RSC converts it asynchronously.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "tier" {
					type = string
				}
				resource "rubrik_aws_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
					tier   = var.tier
				}
			`,
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testAWSRCVRegion(t)),
				"tier":   config.StringVariable("BACKUP"),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_rcv_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyTier),
					knownvalue.StringExact("BACKUP")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_aws_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testAWSRCVRegion(t)),
				"tier":   config.StringVariable("BACKUP"),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_aws_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testAWSRCVRegion(t)),
				"tier":   config.StringVariable("BACKUP"),
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
			ResourceName: "rubrik_aws_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name + " Update"),
				"region": config.StringVariable(testAWSRCVRegion(t)),
				"tier":   config.StringVariable("BACKUP"),
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

// TestAccAWSRCVArchivalLocationCustomerProvidedKeyResource creates an RCV AWS
// archival location encrypted with a customer provided AES key, generated by
// the test, and verifies that changing the key version replaces the archival
// location.
func TestAccAWSRCVArchivalLocationCustomerProvidedKeyResource(t *testing.T) {
	skipUnlessAWSRCVDefined(t)

	name := testUniqueName(t)

	// Generate a random customer provided key.
	key1 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatal(err)
	}
	key2 := make([]byte, 32)
	if _, err := rand.Read(key2); err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsRCVArchivalLocationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with a customer provided
			// key, and that the write-only key never reaches the state.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "key" {
					type      = string
					sensitive = true
				}
				variable "key_version" {
					type = string
				}
				resource "rubrik_aws_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
		
					customer_provided_key {
						key     = var.key
						version = var.key_version
					}
				}
			`,
			ConfigVariables: config.Variables{
				"name":        config.StringVariable(name),
				"region":      config.StringVariable(testAWSRCVRegion(t)),
				"key":         config.StringVariable(base64.StdEncoding.EncodeToString(key1)),
				"key_version": config.StringVariable("1"),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyEncryptionType), knownvalue.StringExact("SSE_CPK")),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyCustomerProvidedKey).AtSliceIndex(0).AtMapKey(keyKey), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyCustomerProvidedKey).AtSliceIndex(0).AtMapKey(keyVersion),
					knownvalue.StringExact("1")),
			},
		}, {
			// Verify that a new key version replaces the archival location,
			// since RSC only accepts the key when the location is created.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "key" {
					type      = string
					sensitive = true
				}
				variable "key_version" {
					type = string
				}
				resource "rubrik_aws_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
		
					customer_provided_key {
						key     = var.key
						version = var.key_version
					}
				}
			`,
			ConfigVariables: config.Variables{
				"name":        config.StringVariable(name),
				"region":      config.StringVariable(testAWSRCVRegion(t)),
				"key":         config.StringVariable(base64.StdEncoding.EncodeToString(key2)),
				"key_version": config.StringVariable("2"),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_rcv_archival_location.location",
						plancheck.ResourceActionReplace),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyCustomerProvidedKey).AtSliceIndex(0).AtMapKey(keyVersion),
					knownvalue.StringExact("2")),
			},
		}},
	})
}

// TestAccAWSRCVArchivalLocationKMSKeyResource onboards the AWS account with the
// UEM key management feature, registers a key manager, creates an RCV AWS
// archival location encrypted with the first KMS key and rekeys it to the
// second key.
func TestAccAWSRCVArchivalLocationKMSKeyResource(t *testing.T) {
	skipUnlessAWSRCVDefined(t)
	skipUnlessAWSKMSDefined(t)
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	name := testUniqueName(t)

	// Note, the Terraform Plugin Testing package does not support for_each on
	// data source or resources, only count.
	const conf = `
		variable "account_name" {
			type = string
		}
		variable "aws_account_id" {
			type = string
		}
		variable "kms_region" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "rcv_region" {
			type = string
		}
		variable "key_id" {
			type = string
		}

		data "rubrik_aws_cnp_artifacts" "artifacts" {
			feature {
				name              = "CLOUD_NATIVE_UEM_KEY_MANAGEMENT"
				permission_groups = ["BASIC"]
			}
		}

		data "rubrik_aws_cnp_permissions" "crossaccount" {
			role_key = "CROSSACCOUNT"

			dynamic "feature" {
				for_each = data.rubrik_aws_cnp_artifacts.artifacts.feature
				content {
					name              = feature.value["name"]
					permission_groups = feature.value["permission_groups"]
				}
			}
		}

		resource "rubrik_aws_cnp_account" "account" {
			name      = var.account_name
			native_id = var.aws_account_id
			regions   = [var.kms_region]

			dynamic "feature" {
				for_each = data.rubrik_aws_cnp_artifacts.artifacts.feature
				content {
					name              = feature.value["name"]
					permission_groups = feature.value["permission_groups"]
				}
			}
		}

		resource "aws_iam_role" "crossaccount" {
			name_prefix        = "tfacc-crossaccount-"
			assume_role_policy = one(rubrik_aws_cnp_account.account.trust_policies).policy
		}

		resource "aws_iam_policy" "crossaccount" {
			count       = length(data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies)
			name_prefix = "tfacc-crossaccount-${data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].name}-"
			policy      = data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].policy
		}

		resource "aws_iam_role_policy_attachment" "crossaccount" {
			count      = length(data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies)
			role       = aws_iam_role.crossaccount.name
			policy_arn = aws_iam_policy.crossaccount[count.index].arn
		}

		resource "aws_iam_role_policy_attachments_exclusive" "crossaccount" {
			role_name   = aws_iam_role.crossaccount.name
			policy_arns = concat(data.rubrik_aws_cnp_permissions.crossaccount.managed_policies, aws_iam_policy.crossaccount[*].arn)
		}

		resource "rubrik_aws_cnp_account_attachments" "attachments" {
			account_id = rubrik_aws_cnp_account.account.id
			features   = data.rubrik_aws_cnp_artifacts.artifacts.feature.*.name

			role {
				key         = "CROSSACCOUNT"
				arn         = aws_iam_role.crossaccount.arn
				permissions = data.rubrik_aws_cnp_permissions.crossaccount.id
			}
		}

		resource "rubrik_aws_kms_key_manager" "key_manager" {
			name             = var.name
			cloud_account_id = rubrik_aws_cnp_account.account.id
			region           = var.kms_region

			depends_on = [rubrik_aws_cnp_account_attachments.attachments]
		}

		resource "rubrik_aws_rcv_archival_location" "location" {
			name   = var.name
			region = var.rcv_region

			kms_key {
				key_manager_id = rubrik_aws_kms_key_manager.key_manager.id
				key_id         = var.key_id
			}
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: ">=6.0.0",
			},
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsRCVArchivalLocationCheckDestroy(t),
			awsKMSKeyManagerCheckDestroy(t),
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the account can be onboarded with the UEM key
			// management feature, that a key manager can be registered, and
			// that an archival location can be created with the first key.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"kms_region":     config.StringVariable(testAWSKMSRegion(t)),
				"name":           config.StringVariable(name),
				"rcv_region":     config.StringVariable(testAWSRCVRegion(t)),
				"key_id":         config.StringVariable(testAWSKMSKeyIDs(t)[0]),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.CompareValuePairs(
					"rubrik_aws_cnp_account.account", tfjsonpath.New(keyID),
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyID),
					knownvalue.StringExact(testAWSKMSKeyIDs(t)[0])),
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyManagerID),
					compare.ValuesSame()),
			},
		}, {
			// Verify that changing the key rekeys the archival location in
			// place, keeping its ID.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"kms_region":     config.StringVariable(testAWSKMSRegion(t)),
				"name":           config.StringVariable(name),
				"rcv_region":     config.StringVariable(testAWSRCVRegion(t)),
				"key_id":         config.StringVariable(testAWSKMSKeyIDs(t)[1]),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_rcv_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_rcv_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyID),
					knownvalue.StringExact(testAWSKMSKeyIDs(t)[1])),
			},
		}, {
			// Terraform import, verifying that the KMS key is read back.
			ResourceName: "rubrik_aws_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"kms_region":     config.StringVariable(testAWSKMSRegion(t)),
				"name":           config.StringVariable(name),
				"rcv_region":     config.StringVariable(testAWSRCVRegion(t)),
				"key_id":         config.StringVariable(testAWSKMSKeyIDs(t)[1]),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_aws_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"kms_region":     config.StringVariable(testAWSKMSRegion(t)),
				"name":           config.StringVariable(name),
				"rcv_region":     config.StringVariable(testAWSRCVRegion(t)),
				"key_id":         config.StringVariable(testAWSKMSKeyIDs(t)[1]),
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
			ResourceName: "rubrik_aws_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"kms_region":     config.StringVariable(testAWSKMSRegion(t)),
				"name":           config.StringVariable(name),
				"rcv_region":     config.StringVariable(testAWSRCVRegion(t)),
				"key_id":         config.StringVariable(testAWSKMSKeyIDs(t)[1]),
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
