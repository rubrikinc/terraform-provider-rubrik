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
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAWSArchivalLocation_kmsKey creates a client-side encrypted AWS
// archival location in a specific region and renames it, since an update must
// not send a KMS master key for a client-side encrypted location.
func TestAccAWSArchivalLocation_kmsKey(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)
	_, conf := loadRSCTestConfig(t)
	if conf.AWSArchivalCloudAccountID == "" || conf.AWSKeyID == "" {
		t.Skip("AWS archival KMS fixture not available: awsArchivalCloudAccountId or awsKeyId not set")
	}

	const configTemplate = `
		variable "name" {
			type = string
		}
		variable "cloud_account_id" {
			type = string
		}
		variable "key_manager_id" {
			type = string
		}
		variable "key_id" {
			type = string
		}
		resource "rubrik_aws_archival_location" "archival_location" {
			account_id    = var.cloud_account_id
			name          = var.name
			bucket_prefix = "tf-acc-kms"
			region        = "us-east-2"

			kms_key {
				key_manager_id = var.key_manager_id
				key_id         = var.key_id
			}
		}
	`
	variables := func(name string) config.Variables {
		return config.Variables{
			"name":             config.StringVariable(name),
			"cloud_account_id": config.StringVariable(conf.AWSArchivalCloudAccountID),
			"key_manager_id":   config.StringVariable(testAWSKeyManagerID(t)),
			"key_id":           config.StringVariable(conf.AWSKeyID),
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:          configTemplate,
			ConfigVariables: variables("terraform-acceptance-test-kms"),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.archival_location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SPECIFIC_REGION")),
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.archival_location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyID), knownvalue.StringExact(conf.AWSKeyID)),
			},
		}, {
			Config:          configTemplate,
			ConfigVariables: variables("terraform-acceptance-test-kms-renamed"),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_archival_location.archival_location",
					tfjsonpath.New(keyName), knownvalue.StringExact("terraform-acceptance-test-kms-renamed")),
			},
		}},
	})
}

// TestAccAzureArchivalLocation_keyVaultKey creates a client-side encrypted
// Azure archival location and renames it, since an update must not send
// customer managed keys for a client-side encrypted location.
func TestAccAzureArchivalLocation_keyVaultKey(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)
	_, conf := loadRSCTestConfig(t)
	if conf.AzureArchivalCloudAccountID == "" || conf.AzureKeyName == "" || conf.AzureKeyVersion == "" {
		t.Skip("Azure archival KMS fixture not available: azureArchivalCloudAccountId, azureKeyName or " +
			"azureKeyVersion not set")
	}

	const configTemplate = `
		variable "name" {
			type = string
		}
		variable "cloud_account_id" {
			type = string
		}
		variable "key_manager_id" {
			type = string
		}
		variable "key_name" {
			type = string
		}
		variable "key_version" {
			type = string
		}
		resource "rubrik_azure_archival_location" "archival_location" {
			cloud_account_id            = var.cloud_account_id
			name                        = var.name
			storage_account_name_prefix = "tfacckms"

			key_vault_key {
				key_manager_id = var.key_manager_id
				key_name       = var.key_name
				key_version    = var.key_version
			}
		}
	`
	variables := func(name string) config.Variables {
		return config.Variables{
			"name":             config.StringVariable(name),
			"cloud_account_id": config.StringVariable(conf.AzureArchivalCloudAccountID),
			"key_manager_id":   config.StringVariable(testAzureKeyManagerID(t)),
			"key_name":         config.StringVariable(conf.AzureKeyName),
			"key_version":      config.StringVariable(conf.AzureKeyVersion),
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:          configTemplate,
			ConfigVariables: variables("terraform-acceptance-test-kms"),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.archival_location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SOURCE_REGION")),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.archival_location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(conf.AzureKeyName)),
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.archival_location",
					tfjsonpath.New(keyCustomerManagedKey), knownvalue.SetSizeExact(0)),
			},
		}, {
			Config:          configTemplate,
			ConfigVariables: variables("terraform-acceptance-test-kms-renamed"),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_archival_location.archival_location",
					tfjsonpath.New(keyName), knownvalue.StringExact("terraform-acceptance-test-kms-renamed")),
			},
		}},
	})
}

// TestAccGCPArchivalLocation_kmsKey creates a client-side encrypted GCP
// archival location and renames it, since an update must not send customer
// managed keys for a client-side encrypted location.
func TestAccGCPArchivalLocation_kmsKey(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)
	_, conf := loadRSCTestConfig(t)
	if conf.GCPArchivalCloudAccountID == "" || conf.GCPKeyName == "" || conf.GCPKeyVersion == "" {
		t.Skip("GCP archival KMS fixture not available: gcpArchivalCloudAccountId, gcpKeyName or gcpKeyVersion " +
			"not set")
	}

	const configTemplate = `
		variable "name" {
			type = string
		}
		variable "cloud_account_id" {
			type = string
		}
		variable "key_manager_id" {
			type = string
		}
		variable "key_name" {
			type = string
		}
		variable "key_version" {
			type = string
		}
		resource "rubrik_gcp_archival_location" "archival_location" {
			cloud_account_id = var.cloud_account_id
			name             = var.name
			bucket_prefix    = "tf-acc-kms"

			kms_key {
				key_manager_id = var.key_manager_id
				key_name       = var.key_name
				key_version    = var.key_version
			}
		}
	`
	variables := func(name string) config.Variables {
		return config.Variables{
			"name":             config.StringVariable(name),
			"cloud_account_id": config.StringVariable(conf.GCPArchivalCloudAccountID),
			"key_manager_id":   config.StringVariable(testGCPKeyManagerID(t)),
			"key_name":         config.StringVariable(conf.GCPKeyName),
			"key_version":      config.StringVariable(conf.GCPKeyVersion),
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:          configTemplate,
			ConfigVariables: variables("terraform-acceptance-test-kms"),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.archival_location",
					tfjsonpath.New(keyLocationTemplate), knownvalue.StringExact("SOURCE_REGION")),
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.archival_location",
					tfjsonpath.New(keyKMSKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(conf.GCPKeyName)),
			},
		}, {
			Config:          configTemplate,
			ConfigVariables: variables("terraform-acceptance-test-kms-renamed"),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_archival_location.archival_location",
					tfjsonpath.New(keyName), knownvalue.StringExact("terraform-acceptance-test-kms-renamed")),
			},
		}},
	})
}
