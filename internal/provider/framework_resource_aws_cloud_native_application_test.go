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

func TestValidateAWSCloudNativeApplication(t *testing.T) {
	sourceID := types.StringValue("22bf439e-9922-486e-a32c-ae9009987924")
	targetID := types.StringValue("ce09d0e4-d0a5-4201-9db2-97f0b6644514")
	tags := types.MapValueMust(types.StringType, map[string]attr.Value{"Application": types.StringValue("web")})
	arns := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("arn:aws:lambda:us-east-1:123456789012:function:my-function"),
	})

	tests := []struct {
		name    string
		config  awsCloudNativeApplicationModel
		wantErr bool
	}{{
		name: "TagsOnly",
		config: awsCloudNativeApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: tags, ResourceARNs: types.SetNull(types.StringType)},
	}, {
		name: "ARNsOnly",
		config: awsCloudNativeApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: types.MapNull(types.StringType), ResourceARNs: arns},
	}, {
		name: "NoSelection",
		config: awsCloudNativeApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: types.MapNull(types.StringType), ResourceARNs: types.SetNull(types.StringType)},
		wantErr: true,
	}, {
		name: "UnknownSelection",
		config: awsCloudNativeApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: targetID,
			MatchingTags: types.MapUnknown(types.StringType), ResourceARNs: types.SetNull(types.StringType)},
	}, {
		name: "TargetIsSource",
		config: awsCloudNativeApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: sourceID,
			MatchingTags: tags, ResourceARNs: types.SetNull(types.StringType)},
		wantErr: true,
	}, {
		name: "UnknownTarget",
		config: awsCloudNativeApplicationModel{CloudAccountID: sourceID, TargetCloudAccountID: types.StringUnknown(),
			MatchingTags: tags, ResourceARNs: types.SetNull(types.StringType)},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateAWSCloudNativeApplication(tc.config)
			if diags.HasError() != tc.wantErr {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
		})
	}
}

// TestAWSCloudNativeApplicationConversion verifies that the resource selection
// and services of the model convert to the SDK parameters, with tags sorted by
// key and every service of the set enabled, and that the application read back
// from RSC converts to the same model, keeping unset collections null.
func TestAWSCloudNativeApplicationConversion(t *testing.T) {
	ctx := context.Background()

	model := awsCloudNativeApplicationModel{
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
	if diags := model.toAWSCloudNativeApplicationParams(ctx, &tags, &arns, &excludedARNs, &services); diags.HasError() {
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
	got := awsCloudNativeApplicationModel{
		MatchingTags:         types.MapNull(types.StringType),
		ResourceARNs:         types.SetNull(types.StringType),
		ExcludedResourceARNs: types.SetNull(types.StringType),
	}
	if diags := got.fromAWSCloudNativeApplication(app); diags.HasError() {
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

// TestAccAwsCloudNativeApplicationResource manages an application between two
// AWS accounts already onboarded with the CLOUD_NATIVE_CONFIG_PROTECTION
// feature, see testCloudApplicationAccounts, and assigns it an SLA Domain.
func TestAccAwsCloudNativeApplicationResource(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagAWSAppResilience)
	accountID, targetAccountID, region := testCloudApplicationAccounts(t)

	vars := config.Variables{
		// Application names may only hold letters, digits, - and _.
		"name":              config.StringVariable("terraform-test-" + uuid.NewString()),
		"account_id":        config.StringVariable(accountID),
		"target_account_id": config.StringVariable(targetAccountID),
		"region":            config.StringVariable(region),
	}

	const variables = `
		variable "name" {
			type = string
		}
		variable "account_id" {
			type = string
		}
		variable "target_account_id" {
			type = string
		}
		variable "region" {
			type = string
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsCloudNativeApplicationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the target account must differ from the source
			// account. Runs first, before anything is applied.
			Config: variables + `
				resource "rubrik_aws_cloud_native_application" "app" {
					name                    = var.name
					cloud_account_id        = var.account_id
					target_cloud_account_id = var.account_id
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
			Config: variables + `
				resource "rubrik_aws_cloud_native_application" "app" {
					name                    = var.name
					cloud_account_id        = var.account_id
					target_cloud_account_id = var.target_account_id
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
					object_ids    = [rubrik_aws_cloud_native_application.app.id]
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyMatchingTags), knownvalue.MapExact(map[string]knownvalue.Check{
						"Application": knownvalue.StringExact("terraform-test"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyResourceARNs), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyServices), knownvalue.SetSizeExact(len(awsCloudNativeApplicationServices))),
				statecheck.ExpectKnownValue("rubrik_sla_domain_assignment.app",
					tfjsonpath.New(keyObjectIDs), knownvalue.SetSizeExact(1)),
				statecheck.ExpectIdentity("rubrik_aws_cloud_native_application.app", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name, the tags and the services can be updated
			// in place.
			Config: variables + `
				resource "rubrik_aws_cloud_native_application" "app" {
					name                    = "${var.name}-updated"
					cloud_account_id        = var.account_id
					target_cloud_account_id = var.target_account_id
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
					plancheck.ExpectResourceAction("rubrik_aws_cloud_native_application.app",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyName), knownvalue.StringRegexp(regexp.MustCompile(`-updated$`))),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyServices), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("EC2"),
						knownvalue.StringExact("LAMBDA"),
					})),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyMatchingTags), knownvalue.MapSizeExact(2)),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_cloud_native_application.app",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that removing all the matching tags is rejected at plan
			// time, since RSC ignores an empty tag list on update.
			Config: variables + `
				resource "rubrik_aws_cloud_native_application" "app" {
					name                    = "${var.name}-updated"
					cloud_account_id        = var.account_id
					target_cloud_account_id = var.target_account_id
					region                  = var.region
					services                = ["EC2", "LAMBDA"]
					resource_arns           = ["arn:aws:lambda:us-east-1:123456789012:function:my-function"]

					delete_snapshots_on_destroy = true
				}
			`,
			ConfigVariables: vars,
			PlanOnly:        true,
			ExpectError:     regexp.MustCompile(`Matching tags can't be removed`),
		}, {
			// Re-apply the updated configuration with
			// delete_snapshots_on_destroy back at its default, false, which is
			// what an import yields. The import steps below reuse this
			// configuration.
			Config: variables + `
				resource "rubrik_aws_cloud_native_application" "app" {
					name                    = "${var.name}-updated"
					cloud_account_id        = var.account_id
					target_cloud_account_id = var.target_account_id
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
					plancheck.ExpectResourceAction("rubrik_aws_cloud_native_application.app",
						plancheck.ResourceActionUpdate),
				},
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_aws_cloud_native_application.app",
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
			ResourceName:    "rubrik_aws_cloud_native_application.app",
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
			ResourceName:    "rubrik_aws_cloud_native_application.app",
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
