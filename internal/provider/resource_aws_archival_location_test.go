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

// TestAccAWSArchivalLocationResource onboards the AWS account with the Cloud
// Native Archival feature, creates a source region archival location encrypted
// with the AWS managed S3 key, updates and imports it, and reads it with the
// data source. A specific region archival location needs a customer managed
// key, which TestAccAWSArchivalLocationKMSKeyResource covers instead.
func TestAccAWSArchivalLocationResource(t *testing.T) {
	skipUnlessAWSArchivalDefined(t)

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
		variable "region" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "storage_class" {
			type = string
		}
		variable "bucket_tags" {
			type = map(string)
		}

		data "rubrik_aws_cnp_artifacts" "artifacts" {
			feature {
				name              = "CLOUD_NATIVE_ARCHIVAL"
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
			regions   = [var.region]

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

		resource "rubrik_aws_archival_location" "location" {
			account_id    = rubrik_aws_cnp_account.account.id
			name          = var.name
			bucket_prefix = "tf-acc-archival"
			storage_class = var.storage_class
			bucket_tags   = var.bucket_tags
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
			awsArchivalLocationCheckDestroy(t),
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the account can be onboarded with the Cloud Native
			// Archival feature and that an archival location can be created.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"region":         config.StringVariable(testAWSArchivalRegion(t)),
				"name":           config.StringVariable(name),
				"storage_class":  config.StringVariable("STANDARD_IA"),
				"bucket_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
				}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.CompareValuePairs(
					"rubrik_aws_cnp_account.account", tfjsonpath.New(keyID),
					"rubrik_aws_archival_location.location", tfjsonpath.New(keyAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SOURCE_REGION")),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyKMSMasterKey),
					knownvalue.StringExact("aws/s3")),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyStorageClass),
					knownvalue.StringExact("STANDARD_IA")),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyBucketTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						"environment": knownvalue.StringExact("test"),
					})),
			},
		}, {
			// Verify that the name, the storage class and the bucket tags are
			// updated in place.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"region":         config.StringVariable(testAWSArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("ONEZONE_IA"),
				"bucket_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyStorageClass),
					knownvalue.StringExact("ONEZONE_IA")),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyBucketTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						"environment": knownvalue.StringExact("test"),
						"owner":       knownvalue.StringExact("terraform"),
					})),
			},
		}, {
			// Verify that the data source reads the archival location by ID
			// and by name.
			Config: conf + `
				data "rubrik_aws_archival_location" "by_id" {
					id = rubrik_aws_archival_location.location.id
				}
				data "rubrik_aws_archival_location" "by_name" {
					name = rubrik_aws_archival_location.location.name
				}
			`,
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"region":         config.StringVariable(testAWSArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("ONEZONE_IA"),
				"bucket_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"rubrik_aws_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_aws_archival_location.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_archival_location.location", tfjsonpath.New(keyName),
					"data.rubrik_aws_archival_location.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_archival_location.location", tfjsonpath.New(keyAccountID),
					"data.rubrik_aws_archival_location.by_id", tfjsonpath.New(keyAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_archival_location.location", tfjsonpath.New(keyID),
					"data.rubrik_aws_archival_location.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_aws_archival_location.location",
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"region":         config.StringVariable(testAWSArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("ONEZONE_IA"),
				"bucket_tags": config.MapVariable(map[string]config.Variable{
					"environment": config.StringVariable("test"),
					"owner":       config.StringVariable("terraform"),
				}),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute. The resource has no identity,
			// so it can't be imported with an identity.
			ResourceName: "rubrik_aws_archival_location.location",
			ConfigVariables: config.Variables{
				"account_name":   config.StringVariable(testAWSAccountName(t)),
				"aws_account_id": config.StringVariable(testAWSAccountID(t)),
				"region":         config.StringVariable(testAWSArchivalRegion(t)),
				"name":           config.StringVariable(name + " Update"),
				"storage_class":  config.StringVariable("ONEZONE_IA"),
				"bucket_tags": config.MapVariable(map[string]config.Variable{
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
		}},
	})
}

// TestAccAWSArchivalLocationKMSKeyResource onboards the AWS account with the
// Cloud Native Archival and UEM key management features, registers a key
// manager, creates a client-side encrypted archival location, renames it,
// rekeys it and imports it.
func TestAccAWSArchivalLocationKMSKeyResource(t *testing.T) {
	skipUnlessAWSArchivalDefined(t)
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
		variable "archival_region" {
			type = string
		}
		variable "kms_region" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "key_id" {
			type = string
		}

		data "rubrik_aws_cnp_artifacts" "artifacts" {
			feature {
				name              = "CLOUD_NATIVE_ARCHIVAL"
				permission_groups = ["BASIC"]
			}
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
			regions   = [var.archival_region, var.kms_region]

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

		resource "rubrik_aws_archival_location" "location" {
			account_id    = rubrik_aws_cnp_account.account.id
			name          = var.name
			bucket_prefix = "tf-acc-kms"
			region        = var.archival_region

			kms_key {
				key_manager_id = rubrik_aws_kms_key_manager.key_manager.id
				key_id         = var.key_id
			}

			depends_on = [rubrik_aws_cnp_account_attachments.attachments]
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
			awsArchivalLocationCheckDestroy(t),
			awsKMSKeyManagerCheckDestroy(t),
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that a client-side encrypted archival location can be
			// created with a key from the key manager.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":    config.StringVariable(testAWSAccountName(t)),
				"aws_account_id":  config.StringVariable(testAWSAccountID(t)),
				"archival_region": config.StringVariable(testAWSArchivalRegion(t)),
				"kms_region":      config.StringVariable(testAWSKMSRegion(t)),
				"name":            config.StringVariable(name),
				"key_id":          config.StringVariable(testAWSKMSKeyIDs(t)[0]),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyID),
					knownvalue.StringExact(testAWSKMSKeyIDs(t)[0])),
				statecheck.CompareValuePairs(
					"rubrik_aws_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"rubrik_aws_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyManagerID),
					compare.ValuesSame()),
			},
		}, {
			// Verify that a client-side encrypted archival location can be
			// renamed, an update must not send a KMS master key.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":    config.StringVariable(testAWSAccountName(t)),
				"aws_account_id":  config.StringVariable(testAWSAccountID(t)),
				"archival_region": config.StringVariable(testAWSArchivalRegion(t)),
				"kms_region":      config.StringVariable(testAWSKMSRegion(t)),
				"name":            config.StringVariable(name + " Update"),
				"key_id":          config.StringVariable(testAWSKMSKeyIDs(t)[0]),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
			},
		}, {
			// Verify that changing the key ID rekeys the archival location in
			// place.
			Config: conf,
			ConfigVariables: config.Variables{
				"account_name":    config.StringVariable(testAWSAccountName(t)),
				"aws_account_id":  config.StringVariable(testAWSAccountID(t)),
				"archival_region": config.StringVariable(testAWSArchivalRegion(t)),
				"kms_region":      config.StringVariable(testAWSKMSRegion(t)),
				"name":            config.StringVariable(name + " Update"),
				"key_id":          config.StringVariable(testAWSKMSKeyIDs(t)[1]),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyID),
					knownvalue.StringExact(testAWSKMSKeyIDs(t)[1])),
			},
		}, {
			// Terraform import, verifying that the KMS key is read back.
			ResourceName: "rubrik_aws_archival_location.location",
			ConfigVariables: config.Variables{
				"account_name":    config.StringVariable(testAWSAccountName(t)),
				"aws_account_id":  config.StringVariable(testAWSAccountID(t)),
				"archival_region": config.StringVariable(testAWSArchivalRegion(t)),
				"kms_region":      config.StringVariable(testAWSKMSRegion(t)),
				"name":            config.StringVariable(name + " Update"),
				"key_id":          config.StringVariable(testAWSKMSKeyIDs(t)[1]),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute. The resource has no identity,
			// so it can't be imported with an identity.
			ResourceName: "rubrik_aws_archival_location.location",
			ConfigVariables: config.Variables{
				"account_name":    config.StringVariable(testAWSAccountName(t)),
				"aws_account_id":  config.StringVariable(testAWSAccountID(t)),
				"archival_region": config.StringVariable(testAWSArchivalRegion(t)),
				"kms_region":      config.StringVariable(testAWSKMSRegion(t)),
				"name":            config.StringVariable(name + " Update"),
				"key_id":          config.StringVariable(testAWSKMSKeyIDs(t)[1]),
			},
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
