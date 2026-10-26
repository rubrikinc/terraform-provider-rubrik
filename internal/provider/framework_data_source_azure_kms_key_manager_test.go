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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAzureKMSKeyManagerDataSource(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				variable "key_manager_id" {
					type = string
				}
				data "rubrik_azure_kms_key_manager" "by_id" {
					id = var.key_manager_id
				}

				variable "key_manager_name" {
					type = string
				}
				data "rubrik_azure_kms_key_manager" "by_name" {
					name = var.key_manager_name
				}
			`,
			ConfigVariables: config.Variables{
				"key_manager_id":   config.StringVariable(testAzureKeyManagerID(t)),
				"key_manager_name": config.StringVariable(testAzureKeyManagerName(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// By name.
				statecheck.ExpectKnownValue("data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyID),
					knownvalue.StringExact(testAzureKeyManagerID(t))),
				statecheck.ExpectKnownValue("data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyName),
					knownvalue.StringExact(testAzureKeyManagerName(t))),
				statecheck.ExpectKnownValue("data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyProvisioningMethod),
					knownvalue.StringExact("CLOUD_ACCOUNT_PROVISION")),
				statecheck.ExpectKnownValue("data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyRegion),
					knownvalue.StringExact(testAzureKeyManagerRegion(t))),
				// By ID.
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyID),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyName),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyDescription),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyDescription),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyProvisioningMethod),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyProvisioningMethod),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyNaturalID),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyNaturalID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyDataLocationsCount),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyDataLocationsCount),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyCloudAccountID),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyRegion),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyRegion),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyKeyVaultURL),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyKeyVaultURL),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyTenantID),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyTenantID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.rubrik_azure_kms_key_manager.by_name", tfjsonpath.New(keyClientID),
					"data.rubrik_azure_kms_key_manager.by_id", tfjsonpath.New(keyClientID),
					compare.ValuesSame()),
			},
		}},
	})
}

func TestAccAzureKMSKeyManagerDataSource_OtherCloud(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				variable "key_manager_name" {
					type = string
				}
				data "rubrik_aws_kms_key_manager" "key_manager" {
					name = var.key_manager_name
				}
			`,
			ConfigVariables: config.Variables{
				"key_manager_name": config.StringVariable(testAzureKeyManagerName(t)),
			},
			ExpectError: regexp.MustCompile("KMS key manager not found"),
		}},
	})
}

func TestAccAzureKMSKeyManagerDataSource_NotFound(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_azure_kms_key_manager" "key_manager" {
				  name = "terraform-acceptance-test-missing-key-manager"
				}
			`,
			ExpectError: regexp.MustCompile("KMS key manager not found"),
		}},
	})
}
