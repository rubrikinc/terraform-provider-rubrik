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
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAwsAccountObject(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			Config: `
				variable "account_name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				resource "rubrik_aws_account" "account" {
					name    = var.account_name
					profile = var.profile

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}

				data "rubrik_object" "aws_account" {
					name        = rubrik_aws_account.account.name
					object_type = "AwsNativeAccount"
				}
			`,
			ConfigVariables: config.Variables{
				"account_name": config.StringVariable(testAWSAccountName(t)),
				"profile":      config.StringVariable(testAWSProfile(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_object.aws_account",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("data.rubrik_object.aws_account",
					tfjsonpath.New(keyName), knownvalue.StringExact(testAWSAccountName(t))),
				statecheck.ExpectKnownValue("data.rubrik_object.aws_account",
					tfjsonpath.New(keyObjectType), knownvalue.StringExact("AwsNativeAccount")),
			},
		}},
	})
}

func TestAccAzureSubscriptionObject(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				Source:            "hashicorp/time",
				VersionConstraint: ">=0.14.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			Config: `
				variable "azure_credentials" {
					type = string
				}
				variable "tenant_domain" {
					type = string
				}
				variable "subscription_id" {
					type = string
				}
				variable "subscription_name" {
					type = string
				}
				variable "resource_group_name" {
					type = string
				}
				variable "resource_group_region" {
					type = string
				}

				resource "time_static" "timestamp" {}

				resource "rubrik_azure_service_principal" "principal" {
					credentials   = var.azure_credentials
					tenant_domain = var.tenant_domain
				}

				resource "rubrik_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["eastus2"]
					}

					cloud_native_protection {
						permission_groups     = ["BASIC"]
						regions               = ["eastus2"]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					depends_on = [rubrik_azure_service_principal.principal]
				}

				data "rubrik_object" "subscription" {
					name        = rubrik_azure_subscription.subscription.subscription_name
					object_type = "AzureNativeSubscription"
				}

				# Wait for the refresh to make sure the resource groups are
				# available.
				resource "rubrik_refresh" "subscription" {
					object_id   = data.rubrik_object.subscription.id
					object_type = "AzureNativeSubscription"
					timestamp   = time_static.timestamp.rfc3339
				}

				data "rubrik_object" "resource_group" {
					name            = var.resource_group_name
					object_type     = "AzureNativeResourceGroup"

					depends_on = [rubrik_refresh.subscription]
				}

				data "rubrik_object" "resource_group_by_subscription" {
					name            = var.resource_group_name
					object_type     = "AzureNativeResourceGroup"
					subscription_id = rubrik_azure_subscription.subscription.id

					depends_on = [rubrik_refresh.subscription]
				}
			`,
			ConfigVariables: config.Variables{
				"azure_credentials":     config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":         config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":       config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":     config.StringVariable(testAzureSubscriptionName(t)),
				"resource_group_name":   config.StringVariable(testAzureResourceGroupName(t)),
				"resource_group_region": config.StringVariable(testAzureResourceGroupRegion(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// Subscription.
				statecheck.ExpectKnownValue("data.rubrik_object.subscription",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("data.rubrik_object.subscription",
					tfjsonpath.New(keyName), knownvalue.StringExact(testAzureSubscriptionName(t))),
				statecheck.ExpectKnownValue("data.rubrik_object.subscription",
					tfjsonpath.New(keyObjectType), knownvalue.StringExact("AzureNativeSubscription")),

				// Resource group.
				statecheck.ExpectKnownValue("data.rubrik_object.resource_group",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("data.rubrik_object.resource_group",
					tfjsonpath.New(keyName), knownvalue.StringExact(testAzureResourceGroupName(t))),
				statecheck.ExpectKnownValue("data.rubrik_object.resource_group",
					tfjsonpath.New(keyObjectType), knownvalue.StringExact("AzureNativeResourceGroup")),

				// Resource group scoped to subscription.
				statecheck.ExpectKnownValue("data.rubrik_object.resource_group_by_subscription",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("data.rubrik_object.resource_group_by_subscription",
					tfjsonpath.New(keyName), knownvalue.StringExact(testAzureResourceGroupName(t))),
				statecheck.ExpectKnownValue("data.rubrik_object.resource_group_by_subscription",
					tfjsonpath.New(keyObjectType), knownvalue.StringExact("AzureNativeResourceGroup")),
			},
		}},
	})
}

func TestAccTagRuleObject(t *testing.T) {
	tagRuleName := testUniqueName(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             tagRuleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// A tag rule surfaces in the inventory hierarchy immediately after
			// creation and passes the active-object filters, so a depends_on is
			// enough to look it up as a CloudNativeTagRule object.
			Config: `
				variable "tag_rule_name" {
					type = string
				}

				resource "rubrik_tag_rule" "rule" {
					name        = var.tag_rule_name
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key    = "Test"
						values = ["true"]
					}
				}

				data "rubrik_object" "tag_rule" {
					name        = rubrik_tag_rule.rule.name
					object_type = "CloudNativeTagRule"

					depends_on = [rubrik_tag_rule.rule]
				}
			`,
			ConfigVariables: config.Variables{
				"tag_rule_name": config.StringVariable(tagRuleName),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_object.tag_rule",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("data.rubrik_object.tag_rule",
					tfjsonpath.New(keyName), knownvalue.StringExact(tagRuleName)),
				statecheck.ExpectKnownValue("data.rubrik_object.tag_rule",
					tfjsonpath.New(keyObjectType), knownvalue.StringExact("CloudNativeTagRule")),
				statecheck.CompareValuePairs(
					"data.rubrik_object.tag_rule", tfjsonpath.New(keyID),
					"rubrik_tag_rule.rule", tfjsonpath.New(keyID),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccAwsAccountObject_FrameworkMigration verifies that the migrated
// object data source is backwards compatible with the SDKv2 provider when
// looking up an AWS native account.
func TestAccAwsAccountObject_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.9.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsAccountCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard an AWS account and verify the SDKv2 and Framework object
			// data sources resolve it to identical values.
			Config: `
				variable "credentials" {
					type = string
				}
				variable "account_name" {
					type = string
				}
				variable "profile" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				resource "polaris_aws_account" "account" {
					name    = var.account_name
					profile = var.profile

					cloud_native_protection {
						permission_groups = ["BASIC"]
						regions           = ["us-east-2"]
					}
				}

				data "polaris_object" "old" {
					provider = polaris-sdkv2

					name        = polaris_aws_account.account.name
					object_type = "AwsNativeAccount"
				}

				data "polaris_object" "new" {
					name        = polaris_aws_account.account.name
					object_type = "AwsNativeAccount"
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":  config.StringVariable(testCredentials(t)),
				"account_name": config.StringVariable(testAWSAccountName(t)),
				"profile":      config.StringVariable(testAWSProfile(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.polaris_object.old", tfjsonpath.New(keyID),
					"data.polaris_object.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_object.old", tfjsonpath.New(keyName),
					"data.polaris_object.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_object.old", tfjsonpath.New(keyObjectType),
					"data.polaris_object.new", tfjsonpath.New(keyObjectType),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccAzureSubscriptionObject_FrameworkMigration verifies that the
// migrated object data source is backwards compatible with the SDKv2 provider
// when looking up an Azure native subscription.
func TestAccAzureSubscriptionObject_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.9.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureSubscriptionCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard an Azure subscription and verify the SDKv2 and Framework
			// object data sources resolve it to identical values.
			Config: `
				variable "credentials" {
					type = string
				}
				variable "azure_credentials" {
					type = string
				}
				variable "tenant_domain" {
					type = string
				}
				variable "subscription_id" {
					type = string
				}
				variable "subscription_name" {
					type = string
				}
				variable "resource_group_name" {
					type = string
				}
				variable "resource_group_region" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				resource "polaris_azure_service_principal" "principal" {
					credentials   = var.azure_credentials
					tenant_domain = var.tenant_domain
				}

				resource "polaris_azure_subscription" "subscription" {
					subscription_id   = var.subscription_id
					subscription_name = var.subscription_name
					tenant_domain     = var.tenant_domain

					cloud_discovery {
						permission_groups = ["BASIC"]
						regions           = ["eastus2"]
					}

					cloud_native_protection {
						permission_groups     = ["BASIC"]
						regions               = ["eastus2"]
						resource_group_name   = var.resource_group_name
						resource_group_region = var.resource_group_region
					}

					depends_on = [polaris_azure_service_principal.principal]
				}

				data "polaris_object" "subscription_old" {
					provider = polaris-sdkv2

					name        = polaris_azure_subscription.subscription.subscription_name
					object_type = "AzureNativeSubscription"
				}

				data "polaris_object" "subscription_new" {
					name        = polaris_azure_subscription.subscription.subscription_name
					object_type = "AzureNativeSubscription"
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":           config.StringVariable(testCredentials(t)),
				"azure_credentials":     config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":         config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":       config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name":     config.StringVariable(testAzureSubscriptionName(t)),
				"resource_group_name":   config.StringVariable(testAzureResourceGroupName(t)),
				"resource_group_region": config.StringVariable(testAzureResourceGroupRegion(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				// Subscription.
				statecheck.CompareValuePairs(
					"data.polaris_object.subscription_old", tfjsonpath.New(keyID),
					"data.polaris_object.subscription_new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_object.subscription_old", tfjsonpath.New(keyName),
					"data.polaris_object.subscription_new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_object.subscription_old", tfjsonpath.New(keyObjectType),
					"data.polaris_object.subscription_new", tfjsonpath.New(keyObjectType),
					compare.ValuesSame()),
			},
		}},
	})
}

func TestValidateObjectConfig(t *testing.T) {
	tests := []struct {
		name   string
		config objectModel
		// wantErrPaths lists the attribute paths that should produce a
		// validation error, in any order. Empty means the config is valid.
		wantErrPaths []string
	}{
		{
			name: "NoParentIDs",
			config: objectModel{
				ObjectType: types.StringValue("AwsNativeEc2Instance"),
			},
		},
		{
			name: "UnknownObjectTypeSkipsValidation",
			config: objectModel{
				ObjectType:     types.StringUnknown(),
				SubscriptionID: types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
				OrgID:          types.StringValue("550e8400-e29b-41d4-a716-446655440001"),
				ProjectID:      types.StringValue("550e8400-e29b-41d4-a716-446655440002"),
			},
		},
		{
			name: "NullObjectTypeSkipsValidation",
			config: objectModel{
				ObjectType:     types.StringNull(),
				SubscriptionID: types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
		},
		{
			name: "SubscriptionIDWithResourceGroup",
			config: objectModel{
				ObjectType:     types.StringValue("AzureNativeResourceGroup"),
				SubscriptionID: types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
		},
		{
			name: "SubscriptionIDWithWrongType",
			config: objectModel{
				ObjectType:     types.StringValue("AwsNativeEc2Instance"),
				SubscriptionID: types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
			wantErrPaths: []string{keySubscriptionID},
		},
		{
			name: "OrgIDWithAzureDevOpsProject",
			config: objectModel{
				ObjectType: types.StringValue("AzureDevOpsProject"),
				OrgID:      types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
		},
		{
			name: "OrgIDWithGitHubRepository",
			config: objectModel{
				ObjectType: types.StringValue("GitHubRepository"),
				OrgID:      types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
		},
		{
			name: "OrgIDWithWrongType",
			config: objectModel{
				ObjectType: types.StringValue("GitHubOrganization"),
				OrgID:      types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
			wantErrPaths: []string{keyOrgID},
		},
		{
			name: "ProjectIDWithAzureDevOpsRepository",
			config: objectModel{
				ObjectType: types.StringValue("AzureDevOpsRepository"),
				ProjectID:  types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
		},
		{
			name: "ProjectIDWithWrongType",
			config: objectModel{
				ObjectType: types.StringValue("AzureDevOpsProject"),
				ProjectID:  types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			},
			wantErrPaths: []string{keyProjectID},
		},
		{
			// A resource group accepts subscription_id but not org_id, so
			// setting both flags only org_id.
			name: "ResourceGroupWithSubscriptionAndOrgID",
			config: objectModel{
				ObjectType:     types.StringValue("AzureNativeResourceGroup"),
				SubscriptionID: types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
				OrgID:          types.StringValue("550e8400-e29b-41d4-a716-446655440001"),
			},
			wantErrPaths: []string{keyOrgID},
		},
		{
			name: "UnknownParentIDSkipped",
			config: objectModel{
				ObjectType:     types.StringValue("AwsNativeEc2Instance"),
				SubscriptionID: types.StringUnknown(),
			},
		},
		{
			name: "MultipleViolations",
			config: objectModel{
				ObjectType:     types.StringValue("AwsNativeEc2Instance"),
				SubscriptionID: types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
				OrgID:          types.StringValue("550e8400-e29b-41d4-a716-446655440001"),
				ProjectID:      types.StringValue("550e8400-e29b-41d4-a716-446655440002"),
			},
			wantErrPaths: []string{keySubscriptionID, keyOrgID, keyProjectID},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateObjectConfig(tc.config)

			var gotErrPaths []string
			for _, d := range diags.Errors() {
				dp, ok := d.(diag.DiagnosticWithPath)
				if !ok {
					t.Errorf("error diagnostic has no path: %s: %s", d.Summary(), d.Detail())
					continue
				}
				gotErrPaths = append(gotErrPaths, dp.Path().String())
			}

			var wantErrPaths []string
			for _, key := range tc.wantErrPaths {
				wantErrPaths = append(wantErrPaths, path.Root(key).String())
			}

			slices.Sort(gotErrPaths)
			slices.Sort(wantErrPaths)
			if !slices.Equal(gotErrPaths, wantErrPaths) {
				t.Errorf("error paths mismatch:\n got: %v\nwant: %v", gotErrPaths, wantErrPaths)
			}
		})
	}
}
