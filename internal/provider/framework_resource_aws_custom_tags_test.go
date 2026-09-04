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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

func TestAccAwsCustomTagsResource(t *testing.T) {
	tagKey1 := testUniqueTagKey(t)
	tagKey2 := testUniqueTagKey(t)
	tagKey3 := testUniqueTagKey(t)

	exKey1 := testUniqueTagKey(t)
	exKey2 := testUniqueTagKey(t)
	exKey3 := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customTagsCheckDestroy(t, core.CloudVendorAWS),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created without excluded tags.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				resource "rubrik_aws_custom_tags" "tags" {
					custom_tags = {
						(var.key1) = "value1"
						(var.key2) = "value2"
					}
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey2),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(awsCustomTagsID)),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCloudAccountID),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("value1"),
						tagKey2: knownvalue.StringExact("value2"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyExcludedTags),
					knownvalue.Null()),
				// The override_resource_tags field defaults to true.
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyOverrideResourceTags),
					knownvalue.Bool(true)),
			},
		}, {
			// Verify that the resource can be updated to include excluded tags.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_aws_custom_tags" "tags" {
					custom_tags = {
						(var.key1) = "value1"
						(var.key2) = "value2"
					}

					excluded_tags = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey2),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(awsCustomTagsID)),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCloudAccountID),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("value1"),
						tagKey2: knownvalue.StringExact("value2"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey2),
					})),
				// The override_resource_tags field defaults to true.
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyOverrideResourceTags),
					knownvalue.Bool(true)),
			},
		}, {
			// Verify that the resource can be updated by changing its values.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_aws_custom_tags" "tags" {
					custom_tags = {
						(var.key1) = "updated1"
						(var.key2) = "value3"
					}

					excluded_tags = [
						var.exKey1,
						var.exKey2,
					]

					override_resource_tags = false
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("updated1"),
						tagKey3: knownvalue.StringExact("value3"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey3),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyOverrideResourceTags),
					knownvalue.Bool(false)),
			},
		}, {
			// Restore the override field to its default value. The field refers
			// to a single global value in RSC, so leaving it turned off would
			// affect other tests using the same RSC account.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_aws_custom_tags" "tags" {
					custom_tags = {
						(var.key1) = "updated1"
						(var.key2) = "value3"
					}

					excluded_tags = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey3),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyOverrideResourceTags),
					knownvalue.Bool(true)),
			},
		}, {
			// Terraform import. Note, the import will take over all custom tags
			// of the cloud vendor, not just the ones managed by this resource,
			// so the test will fail if there are existing tags.
			ResourceName:      "rubrik_aws_custom_tags.tags",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportStateId:     keyGlobal,
			ImportState:       true,
			ImportStateVerify: true,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
		}, {
			// import {} block with id attribute. Note, the import will take
			// over all custom tags of the cloud vendor, not just the ones
			// managed by this resource, so the test will fail if there are
			// existing tags.
			ResourceName:    "rubrik_aws_custom_tags.tags",
			ImportStateKind: resource.ImportBlockWithID,
			ImportStateId:   keyGlobal,
			ImportState:     true,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
		}, {
			// An import ID which is neither a cloud account ID nor global is
			// rejected, so that a malformed cloud account ID does not silently
			// import the global scope.
			ResourceName:    "rubrik_aws_custom_tags.tags",
			ImportStateKind: resource.ImportCommandWithID,
			ImportStateId:   "not-a-cloud-account-id",
			ImportState:     true,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey3),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ExpectError: regexp.MustCompile(`is not a valid import ID`),
		}},
	})
}

// TestAccAwsCustomTagsResource_MultipleResources verifies that a resource only
// manages the custom tags in its own configuration. Destroying one resource
// must leave the custom tags managed by another resource untouched.
func TestAccAwsCustomTagsResource_MultipleResources(t *testing.T) {
	tagKey1 := testUniqueTagKey(t)
	tagKey2 := testUniqueTagKey(t)

	exKey1 := testUniqueTagKey(t)
	exKey2 := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customTagsCheckDestroy(t, core.CloudVendorAWS),
		Steps: []resource.TestStep{{
			// Two resources, each managing a disjoint set of custom tags and
			// excluded tags.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}

				resource "rubrik_aws_custom_tags" "tags1" {
					custom_tags = {
						(var.key1) = "value1"
					}

					excluded_tags = [
						var.exKey1,
					]
				}

				resource "rubrik_aws_custom_tags" "tags2" {
					custom_tags = {
						(var.key2) = "value2"
					}

					excluded_tags = [
						var.exKey2,
					]

					depends_on = [
						rubrik_aws_custom_tags.tags1,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey2),
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags1", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("value1"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags1", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags2", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey2: knownvalue.StringExact("value2"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags2", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey2),
					})),
			},
		}, {
			// Destroying the second resource must not remove the custom tag or
			// the excluded tag managed by the first resource.
			Config: `
				variable "key1" {
					type = string
				}
				variable "key2" {
					type = string
				}
				variable "exKey1" {
					type = string
				}

				resource "rubrik_aws_custom_tags" "tags1" {
					custom_tags = {
						(var.key1) = "value1"
					}

					excluded_tags = [
						var.exKey1,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"key1":   config.StringVariable(tagKey1),
				"key2":   config.StringVariable(tagKey2),
				"exKey1": config.StringVariable(exKey1),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags1", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey1: knownvalue.StringExact("value1"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags1", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
					})),
			},
		}},
	})
}

// TestAccAwsCustomTagsResource_ExcludedTagsOnly verifies that a resource can
// manage excluded tags without any custom tags.
func TestAccAwsCustomTagsResource_ExcludedTagsOnly(t *testing.T) {
	exKey1 := testUniqueTagKey(t)
	exKey2 := testUniqueTagKey(t)
	exKey3 := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             customTagsCheckDestroy(t, core.CloudVendorAWS),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with only excluded tags.
			Config: `
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_aws_custom_tags" "tags" {
					excluded_tags = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(awsCustomTagsID)),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCloudAccountID),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCustomTags),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey2),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyOverrideResourceTags),
					knownvalue.Bool(true)),
			},
		}, {
			// Verify that the resource can be updated. One excluded tag is
			// added and one is removed, still without any custom tags.
			Config: `
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}
				resource "rubrik_aws_custom_tags" "tags" {
					excluded_tags = [
						var.exKey1,
						var.exKey2,
					]
				}
			`,
			ConfigVariables: config.Variables{
				"exKey1": config.StringVariable(exKey1),
				"exKey2": config.StringVariable(exKey3),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyCustomTags),
					knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.tags", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
						knownvalue.StringExact(exKey3),
					})),
			},
		}},
	})
}

// TestAccAwsCustomTagsResource_CloudAccountScoped verifies that custom tags and
// excluded tags can be scoped to a single cloud account, and that the scoped
// and the global configurations are independent of each other.
func TestAccAwsCustomTagsResource_CloudAccountScoped(t *testing.T) {
	tagKey := testUniqueTagKey(t)
	exKey1 := testUniqueTagKey(t)
	exKey2 := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCnpAccountCheckDestroy(t),
			customTagsCheckDestroy(t, core.CloudVendorAWS),
		),
		Steps: []resource.TestStep{{
			// Verify that a scoped resource can be created alongside a global
			// resource sharing the same tag key.
			Config: `
				variable "aws_account_id" {
					type = string
				}
				variable "aws_account_name" {
					type = string
				}
				variable "key" {
					type = string
				}
				variable "exKey1" {
					type = string
				}
				variable "exKey2" {
					type = string
				}

				resource "rubrik_aws_cnp_account" "account" {
					name      = var.aws_account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "rubrik_aws_custom_tags" "global" {
					custom_tags = {
						(var.key) = "global"
					}

					excluded_tags = [var.exKey1]
				}

				resource "rubrik_aws_custom_tags" "account" {
					cloud_account_id = rubrik_aws_cnp_account.account.id

					custom_tags = {
						(var.key) = "account"
					}

					excluded_tags = [var.exKey2]
				}
			`,
			ConfigVariables: config.Variables{
				"aws_account_id":   config.StringVariable(testAWSAccountID(t)),
				"aws_account_name": config.StringVariable(testAWSAccountName(t)),
				"key":              config.StringVariable(tagKey),
				"exKey1":           config.StringVariable(exKey1),
				"exKey2":           config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// The global resource.
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.global", tfjsonpath.New(keyID),
					knownvalue.StringExact(awsCustomTagsID)),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.global", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey: knownvalue.StringExact("global"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.global", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey1),
					})),

				// The scoped resource.
				statecheck.CompareValuePairs(
					"rubrik_aws_custom_tags.account", tfjsonpath.New(keyID),
					"rubrik_aws_custom_tags.account", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.account", tfjsonpath.New(keyCloudAccountID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.account", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey: knownvalue.StringExact("account"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.account", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey2),
					})),
			},
		}, {
			// Destroying the global resource must leave the scoped
			// configuration untouched.
			Config: `
				variable "aws_account_id" {
					type = string
				}
				variable "aws_account_name" {
					type = string
				}
				variable "key" {
					type = string
				}
				variable "exKey2" {
					type = string
				}

				resource "rubrik_aws_cnp_account" "account" {
					name      = var.aws_account_name
					native_id = var.aws_account_id
					regions   = ["us-east-2"]

					feature {
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				resource "rubrik_aws_custom_tags" "account" {
					cloud_account_id = rubrik_aws_cnp_account.account.id

					custom_tags = {
						(var.key) = "account"
					}

					excluded_tags = [var.exKey2]
				}
			`,
			ConfigVariables: config.Variables{
				"aws_account_id":   config.StringVariable(testAWSAccountID(t)),
				"aws_account_name": config.StringVariable(testAWSAccountName(t)),
				"key":              config.StringVariable(tagKey),
				"exKey2":           config.StringVariable(exKey2),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.account", tfjsonpath.New(keyCustomTags),
					knownvalue.MapExact(map[string]knownvalue.Check{
						tagKey: knownvalue.StringExact("account"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_custom_tags.account", tfjsonpath.New(keyExcludedTags),
					knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact(exKey2),
					})),
			},
		}, {
			// Terraform import of the scoped resource, using the cloud account
			// ID as the import ID.
			ResourceName:      "rubrik_aws_custom_tags.account",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportStateIdFunc: customTagsImportID("rubrik_aws_custom_tags.account"),
			ImportState:       true,
			ImportStateVerify: true,
			ConfigVariables: config.Variables{
				"aws_account_id":   config.StringVariable(testAWSAccountID(t)),
				"aws_account_name": config.StringVariable(testAWSAccountName(t)),
				"key":              config.StringVariable(tagKey),
				"exKey2":           config.StringVariable(exKey2),
			},
		}},
	})
}

// TestAccAwsCustomTagsResource_InvalidConfigs verifies that the custom_tags and
// excluded_tags fields reject an explicitly empty collection, and that at least
// one of them must be specified. Terraform distinguishes an unset field from an
// empty one, and only the unset form is meaningful for these fields.
func TestAccAwsCustomTagsResource_InvalidConfigs(t *testing.T) {
	tagKey := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				resource "rubrik_aws_custom_tags" "tags" {
					custom_tags = {}
				}
			`,
			ExpectError: regexp.MustCompile(`custom_tags map must contain at least 1 elements`),
		}, {
			Config: `
				variable "key" {
					type = string
				}

				resource "rubrik_aws_custom_tags" "tags" {
					custom_tags = {
						(var.key) = "value"
					}

					excluded_tags = []
				}
			`,
			ConfigVariables: config.Variables{
				"key": config.StringVariable(tagKey),
			},
			ExpectError: regexp.MustCompile(`excluded_tags set must contain at least 1 elements`),
		}, {
			Config: `
				resource "rubrik_aws_custom_tags" "tags" {
				}
			`,
			ExpectError: regexp.MustCompile(`At least one of these attributes must be configured:\s+\[custom_tags,excluded_tags\]`),
		}},
	})
}

// TestAccAwsCustomTagsResource_FrameworkMigration verifies that existing state
// created by the SDKv2 provider (v1.5.0) can be read by the Framework provider
// without drift. Step 1 creates the resource using the published SDKv2
// provider, step 2 refreshes state using the local Framework provider and
// asserts the plan is empty.
func TestAccAwsCustomTagsResource_FrameworkMigration(t *testing.T) {
	conf := `
		variable "credentials" {
			type = string
		}
		variable "key" {
			type = string
		}

		provider "polaris" {
			credentials = var.credentials
		}

		resource "polaris_aws_custom_tags" "tags" {
			custom_tags = {
				(var.key) = "value"
			}
		}
	`
	tagKey := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		CheckDestroy: customTagsCheckDestroy(t, core.CloudVendorAWS),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_custom_tags.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(awsCustomTagsID)),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			PlanOnly: true,
		}},
	})
}

// TestAccAwsCustomTagsResource_MoveState verifies that state from a
// polaris_aws_custom_tags resource created by the rubrikinc/polaris provider
// can be moved to a rubrik_aws_custom_tags resource using the moved {} block.
func TestAccAwsCustomTagsResource_MoveState(t *testing.T) {
	tagKey := testUniqueTagKey(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: customTagsCheckDestroy(t, core.CloudVendorAWS),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.5.0",
				},
			},
			Config: `
				variable "credentials" {
					type = string
				}
				variable "key" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_aws_custom_tags" "tags" {
					custom_tags = {
						(var.key) = "value"
					}
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_custom_tags.tags", tfjsonpath.New(keyID),
					knownvalue.StringExact(awsCustomTagsID)),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "credentials" {
					type = string
				}
				variable "key" {
					type = string
				}

				moved {
					from = polaris_aws_custom_tags.tags
					to   = rubrik_aws_custom_tags.tags
				}

				resource "rubrik_aws_custom_tags" "tags" {
					custom_tags = {
						(var.key) = "value"
					}
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"key":         config.StringVariable(tagKey),
			},
			// Verify the plan is empty, move succeeded without drift, and
			// apply to update the state. Without the apply step, destroy can
			// fail due to resource dependency issues.
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
