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
	"context"
	"regexp"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	gqlcloudapp "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cloudapp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
)

func TestValidateAWSCloudApplication(t *testing.T) {
	sourceID := types.StringValue("22bf439e-9922-486e-a32c-ae9009987924")
	targetID := types.StringValue("ce09d0e4-d0a5-4201-9db2-97f0b6644514")
	tags := types.MapValueMust(types.StringType, map[string]attr.Value{"Application": types.StringValue("web")})
	arns := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("arn:aws:lambda:us-east-1:123456789012:function:my-function"),
	})

	tests := []struct {
		name    string
		config  awsCloudApplicationModel
		wantErr bool
	}{{
		name: "TagsOnly",
		config: awsCloudApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: tags, ResourceARNs: types.SetNull(types.StringType)},
	}, {
		name: "ARNsOnly",
		config: awsCloudApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: types.MapNull(types.StringType), ResourceARNs: arns},
	}, {
		name: "NoSelection",
		config: awsCloudApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: types.MapNull(types.StringType), ResourceARNs: types.SetNull(types.StringType)},
		wantErr: true,
	}, {
		name: "UnknownSelection",
		config: awsCloudApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: types.MapUnknown(types.StringType), ResourceARNs: types.SetNull(types.StringType)},
	}, {
		name: "TargetIsSource",
		config: awsCloudApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: sourceID,
			MatchingTags: tags, ResourceARNs: types.SetNull(types.StringType)},
		wantErr: true,
	}, {
		name: "UnknownTarget",
		config: awsCloudApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: types.StringUnknown(),
			MatchingTags: tags, ResourceARNs: types.SetNull(types.StringType)},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateAWSCloudApplication(tc.config)
			if diags.HasError() != tc.wantErr {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
		})
	}
}

// TestAWSCloudApplicationConversion verifies that the resource selection
// and services of the model convert to the SDK parameters, with tags sorted by
// key and every service of the set enabled, and that the application read back
// from RSC converts to the same model, keeping unset collections null.
func TestAWSCloudApplicationConversion(t *testing.T) {
	ctx := context.Background()

	model := awsCloudApplicationModel{
		MatchingTags: types.MapValueMust(types.StringType, map[string]attr.Value{
			"Version":     types.StringValue("1.0"),
			"Application": types.StringValue("web"),
		}),
		ResourceARNs: types.SetValueMust(types.StringType, []attr.Value{
			types.StringValue("arn:aws:lambda:us-east-1:123456789012:function:my-function"),
		}),
		ExcludedResourceARNs: types.SetNull(types.StringType),
		Services: types.SetValueMust(types.StringType, []attr.Value{
			types.StringValue("EC2"), types.StringValue("LAMBDA"), types.StringValue("SECRETS_MANAGER"),
		}),
	}

	var tags []core.Tag
	var arns, excludedARNs []string
	var services gqlcloudapp.AWSServices
	if diags := model.toAWSCloudApplicationParams(ctx, &tags, &arns, &excludedARNs, &services); diags.HasError() {
		t.Fatal(diags)
	}
	wantTags := []core.Tag{{Key: "Application", Value: "web"}, {Key: "Version", Value: "1.0"}}
	if !slices.Equal(tags, wantTags) {
		t.Errorf("invalid tags: %v", tags)
	}
	if len(arns) != 1 || len(excludedARNs) != 0 {
		t.Errorf("invalid ARNs: %v %v", arns, excludedARNs)
	}
	if services != (gqlcloudapp.AWSServices{EC2: true, Lambda: true, Secrets: true}) {
		t.Errorf("invalid services: %+v", services)
	}

	app := gqlcloudapp.Application{
		ID:                   uuid.MustParse("c7ec46ba-d419-4a9d-88e2-26665e1479db"),
		Name:                 "my-application",
		CloudAccountID:       uuid.MustParse("22bf439e-9922-486e-a32c-ae9009987924"),
		TargetCloudAccountID: uuid.MustParse("ce09d0e4-d0a5-4201-9db2-97f0b6644514"),
		MatchingTags:         gqlcloudapp.MatchingTags{Tags: tags},
		ResourceARNs:         arns,
		ServiceConfig:        gqlcloudapp.NewServiceConfig(services),
		RefreshStatus:        gqlcloudapp.RefreshStatusSuccess,
		ProtectionStatus:     gqlcloudapp.ProtectionStatusUnprotected,
	}
	app.Region.Region.AWSRegion = aws.RegionUsEast1.ToRegionEnum()
	got := awsCloudApplicationModel{
		MatchingTags:         types.MapNull(types.StringType),
		ResourceARNs:         types.SetNull(types.StringType),
		ExcludedResourceARNs: types.SetNull(types.StringType),
	}
	if diags := got.fromAWSCloudApplication(app); diags.HasError() {
		t.Fatal(diags)
	}
	if got.Region.ValueString() != "us-east-1" || got.Name.ValueString() != "my-application" {
		t.Errorf("invalid name or region: %s %s", got.Name, got.Region)
	}
	if !got.MatchingTags.Equal(model.MatchingTags) {
		t.Errorf("invalid matching tags: %s", got.MatchingTags)
	}
	if !got.ResourceARNs.Equal(model.ResourceARNs) {
		t.Errorf("invalid resource ARNs: %s", got.ResourceARNs)
	}
	if !got.ExcludedResourceARNs.IsNull() {
		t.Errorf("excluded resource ARNs should be null: %s", got.ExcludedResourceARNs)
	}
	if !got.Services.Equal(model.Services) {
		t.Errorf("invalid services: %s", got.Services)
	}
	if got.DeleteSnapshotsOnDestroy.ValueBool() {
		t.Error("delete_snapshots_on_destroy should default to false")
	}
}

// testAWSCloudApplicationAccountsConfig onboards the source and the target
// AWS accounts of the cloud application tests with the
// CLOUD_NATIVE_CONFIG_PROTECTION feature, including the RECOVERY permission
// groups, using a single cross account IAM role per account. The target
// account is managed with the aws.target provider, which assumes the
// target_role IAM role. Both accounts are onboarded in var.region.
//
// Note, the Terraform Plugin Testing package does not support for_each on data
// sources or resources, only count.
const testAWSCloudApplicationAccountsConfig = `
	variable "name" {
		type = string
	}
	variable "region" {
		type = string
	}
	variable "source_account_id" {
		type = string
	}
	variable "source_account_name" {
		type = string
	}
	variable "target_account_id" {
		type = string
	}
	variable "target_account_name" {
		type = string
	}
	variable "target_role" {
		type = string
	}

	provider "aws" {
		region = var.region
	}

	provider "aws" {
		alias  = "target"
		region = var.region

		assume_role {
			role_arn = var.target_role
		}
	}

	locals {
		features = [{
			name              = "CLOUD_DISCOVERY"
			permission_groups = ["BASIC"]
		}, {
			name              = "CLOUD_NATIVE_CONFIG_PROTECTION"
			permission_groups = ["BASIC", "BASIC_2", "RECOVERY", "RECOVERY_2", "RECOVERY_3", "RECOVERY_4"]
		}]
	}

	data "rubrik_aws_cnp_permissions" "crossaccount" {
		role_key = "CROSSACCOUNT"

		dynamic "feature" {
			for_each = local.features
			content {
				name              = feature.value["name"]
				permission_groups = feature.value["permission_groups"]
			}
		}
	}

	# Source account.
	resource "rubrik_aws_cnp_account" "source" {
		name      = var.source_account_name
		native_id = var.source_account_id
		regions   = [var.region]

		dynamic "feature" {
			for_each = local.features
			content {
				name              = feature.value["name"]
				permission_groups = feature.value["permission_groups"]
			}
		}
	}

	resource "aws_iam_role" "source" {
		name_prefix        = "tfacc-crossaccount-"
		assume_role_policy = one(rubrik_aws_cnp_account.source.trust_policies).policy
	}

	resource "aws_iam_policy" "source" {
		count       = length(data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies)
		name_prefix = "tfacc-crossaccount-${data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].name}-"
		policy      = data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].policy
	}

	resource "aws_iam_role_policy_attachments_exclusive" "source" {
		role_name   = aws_iam_role.source.name
		policy_arns = concat(data.rubrik_aws_cnp_permissions.crossaccount.managed_policies, aws_iam_policy.source[*].arn)
	}

	resource "rubrik_aws_cnp_account_attachments" "source" {
		account_id = rubrik_aws_cnp_account.source.id

		role {
			key         = "CROSSACCOUNT"
			arn         = aws_iam_role.source.arn
			permissions = data.rubrik_aws_cnp_permissions.crossaccount.id
		}

		depends_on = [aws_iam_role_policy_attachments_exclusive.source]
	}

	# Target account.
	resource "rubrik_aws_cnp_account" "target" {
		name      = var.target_account_name
		native_id = var.target_account_id
		regions   = [var.region]

		dynamic "feature" {
			for_each = local.features
			content {
				name              = feature.value["name"]
				permission_groups = feature.value["permission_groups"]
			}
		}
	}

	resource "aws_iam_role" "target" {
		provider           = aws.target
		name_prefix        = "tfacc-crossaccount-"
		assume_role_policy = one(rubrik_aws_cnp_account.target.trust_policies).policy
	}

	resource "aws_iam_policy" "target" {
		provider    = aws.target
		count       = length(data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies)
		name_prefix = "tfacc-crossaccount-${data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].name}-"
		policy      = data.rubrik_aws_cnp_permissions.crossaccount.customer_managed_policies[count.index].policy
	}

	resource "aws_iam_role_policy_attachments_exclusive" "target" {
		provider    = aws.target
		role_name   = aws_iam_role.target.name
		policy_arns = concat(data.rubrik_aws_cnp_permissions.crossaccount.managed_policies, aws_iam_policy.target[*].arn)
	}

	resource "rubrik_aws_cnp_account_attachments" "target" {
		account_id = rubrik_aws_cnp_account.target.id

		role {
			key         = "CROSSACCOUNT"
			arn         = aws_iam_role.target.arn
			permissions = data.rubrik_aws_cnp_permissions.crossaccount.id
		}

		depends_on = [aws_iam_role_policy_attachments_exclusive.target]
	}
`

// testAWSCloudApplicationAccountsVariables returns the variables of
// testAWSCloudApplicationAccountsConfig, with name as the application name.
// The test is skipped if the target account isn't set in the AWS test
// configuration.
func testAWSCloudApplicationAccountsVariables(t *testing.T, name string) config.Variables {
	t.Helper()

	return config.Variables{
		"name":                config.StringVariable(name),
		"region":              config.StringVariable(testAWSCloudApplicationRegion),
		"source_account_id":   config.StringVariable(testAWSAccountID(t)),
		"source_account_name": config.StringVariable(testAWSAccountName(t)),
		"target_account_id":   config.StringVariable(testAWSCloudApplicationTargetAccountID(t)),
		"target_account_name": config.StringVariable(testAWSCloudApplicationTargetAccountName(t)),
		"target_role":         config.StringVariable(testAWSCloudApplicationTargetRole(t)),
	}
}

// testAWSCloudApplicationRegion is the AWS region the cloud application tests
// onboard the accounts in and create the applications in.
const testAWSCloudApplicationRegion = "us-east-2"

// testAWSCloudApplicationExternalProviders are the external providers needed
// by testAWSCloudApplicationAccountsConfig.
var testAWSCloudApplicationExternalProviders = map[string]resource.ExternalProvider{
	"aws": {
		Source:            "hashicorp/aws",
		VersionConstraint: ">=6.0.0",
	},
}

// TestAccAwsCloudApplicationResource onboards a source and a target AWS
// account with the CLOUD_NATIVE_CONFIG_PROTECTION feature, then manages an
// application between them, assigns it an SLA Domain, updates and imports it.
func TestAccAwsCloudApplicationResource(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagAWSAppResilience)

	// Application names may only hold letters, digits, - and _.
	name := "terraform-test-" + uuid.NewString()
	vars := testAWSCloudApplicationAccountsVariables(t, name)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		ExternalProviders:        testAWSCloudApplicationExternalProviders,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			awsCloudApplicationCheckDestroy(t),
			awsCnpAccountCheckDestroy(t),
			awsCnpAccountAttachmentsCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the target account must differ from the source
			// account. Runs first, before anything is applied.
			Config: testAWSCloudApplicationAccountsConfig + `
				resource "rubrik_aws_cloud_application" "app" {
					name                    = var.name
					cloud_account_id        = rubrik_aws_cnp_account_attachments.source.account_id
					target_cloud_account_id = rubrik_aws_cnp_account_attachments.source.account_id
					region                  = var.region
					matching_tags           = { Application = "terraform-test" }
				}
			`,
			ConfigVariables: vars,
			PlanOnly:        true,
			ExpectError:     regexp.MustCompile(`Invalid target cloud account`),
		}, {
			// Verify that the application can be created with tags only, that
			// the services default to all of them, and that the SLA Domain
			// assignment accepts the application ID.
			Config: testAWSCloudApplicationAccountsConfig + `
				resource "rubrik_aws_cloud_application" "app" {
					name                    = var.name
					cloud_account_id        = rubrik_aws_cnp_account_attachments.source.account_id
					target_cloud_account_id = rubrik_aws_cnp_account_attachments.target.account_id
					region                  = var.region
					matching_tags           = { Application = "terraform-test" }

					delete_snapshots_on_destroy = true
				}
				resource "rubrik_sla_domain" "sla" {
					name         = var.name
					description  = "Acceptance test: Delete Me!"
					object_types = ["AWS_CONFIG_OBJECT_TYPE"]

					hourly_schedule {
						frequency      = 6
						retention      = 1
						retention_unit = "DAYS"
					}
				}
				resource "rubrik_sla_domain_assignment" "app" {
					sla_domain_id = rubrik_sla_domain.sla.id
					object_ids    = [rubrik_aws_cloud_application.app.id]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyName), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyRegion), knownvalue.StringExact(testAWSCloudApplicationRegion)),
				statecheck.CompareValuePairs(
					"rubrik_aws_cnp_account.source", tfjsonpath.New(keyID),
					"rubrik_aws_cloud_application.app", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_aws_cnp_account.target", tfjsonpath.New(keyID),
					"rubrik_aws_cloud_application.app", tfjsonpath.New(keyTargetCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyMatchingTags), knownvalue.MapExact(map[string]knownvalue.Check{
						"Application": knownvalue.StringExact("terraform-test"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyResourceARNs), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyServices), knownvalue.SetSizeExact(len(awsCloudApplicationServices))),
				statecheck.ExpectKnownValue("rubrik_sla_domain_assignment.app",
					tfjsonpath.New(keyObjectIDs), knownvalue.SetSizeExact(1)),
				statecheck.ExpectIdentity("rubrik_aws_cloud_application.app", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name, the tags and the services can be updated
			// in place. Removing the SLA Domain assignment leaves the
			// application unprotected.
			Config: testAWSCloudApplicationAccountsConfig + `
				resource "rubrik_aws_cloud_application" "app" {
					name                    = "${var.name}-updated"
					cloud_account_id        = rubrik_aws_cnp_account_attachments.source.account_id
					target_cloud_account_id = rubrik_aws_cnp_account_attachments.target.account_id
					region                  = var.region
					services                = ["EC2", "LAMBDA"]
					matching_tags = {
						Application = "terraform-test"
						Environment = "test"
					}

					delete_snapshots_on_destroy = true
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_cloud_application.app",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyName), knownvalue.StringExact(name+"-updated")),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyServices), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("EC2"),
						knownvalue.StringExact("LAMBDA"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyMatchingTags), knownvalue.MapExact(map[string]knownvalue.Check{
						"Application": knownvalue.StringExact("terraform-test"),
						"Environment": knownvalue.StringExact("test"),
					})),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_cloud_application.app",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that removing all the matching tags is rejected at plan
			// time, since RSC ignores an empty tag list on update.
			Config: testAWSCloudApplicationAccountsConfig + `
				resource "rubrik_aws_cloud_application" "app" {
					name                    = "${var.name}-updated"
					cloud_account_id        = rubrik_aws_cnp_account_attachments.source.account_id
					target_cloud_account_id = rubrik_aws_cnp_account_attachments.target.account_id
					region                  = var.region
					services                = ["EC2", "LAMBDA"]
					resource_arns           = ["arn:aws:lambda:us-east-2:123456789012:function:my-function"]

					delete_snapshots_on_destroy = true
				}
			`,
			ConfigVariables: vars,
			PlanOnly:        true,
			ExpectError:     regexp.MustCompile(`Matching tags can't be removed`),
		}, {
			// Re-apply the updated configuration with
			// delete_snapshots_on_destroy back at its default, false, which is
			// what an import yields. The import block steps below reuse this
			// configuration.
			Config: testAWSCloudApplicationAccountsConfig + `
				resource "rubrik_aws_cloud_application" "app" {
					name                    = "${var.name}-updated"
					cloud_account_id        = rubrik_aws_cnp_account_attachments.source.account_id
					target_cloud_account_id = rubrik_aws_cnp_account_attachments.target.account_id
					region                  = var.region
					services                = ["EC2", "LAMBDA"]
					matching_tags = {
						Application = "terraform-test"
						Environment = "test"
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_cloud_application.app",
						plancheck.ResourceActionUpdate),
				},
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_aws_cloud_application.app",
			ConfigVariables:   vars,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateVerifyIgnore: []string{
				// Both are updated by RSC in the background, e.g., when the
				// application refresh finishes, so they can change between the
				// apply and the import.
				keyRefreshStatus,
				keyProtectionStatus,
			},
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_aws_cloud_application.app",
			ConfigVariables: vars,
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_aws_cloud_application.app",
			ConfigVariables: vars,
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
