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
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAwsCnpPermissionsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				locals {
					features = {
						CLOUD_DISCOVERY = {
							permission_groups = ["BASIC"]
						}
						CLOUD_NATIVE_ARCHIVAL = {
							permission_groups = ["BASIC"]
						}
						CLOUD_NATIVE_DYNAMODB_PROTECTION = {
							permission_groups = ["BASIC"]
						}
						CLOUD_NATIVE_PROTECTION = {
							permission_groups = ["BASIC"]
						}
						CLOUD_NATIVE_S3_PROTECTION = {
							permission_groups = ["BASIC"]
						}
						EXOCOMPUTE = {
							permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
						}
						KUBERNETES_PROTECTION = {
							permission_groups = ["BASIC"]
						}
						RDS_PROTECTION = {
							permission_groups = ["BASIC"]
						}
						SERVERS_AND_APPS = {
							permission_groups = ["CLOUD_CLUSTER_ES"]
						}
					}
				}

				data "polaris_aws_cnp_permissions" "crossaccount" {
					role_key = "CROSSACCOUNT"

					dynamic "feature" {
						for_each = local.features
						content {
							name              = feature.key
							permission_groups = feature.value.permission_groups
						}
					}
				}

				data "polaris_aws_cnp_permissions" "masternode" {
					role_key = "EXOCOMPUTE_EKS_MASTERNODE"

					dynamic "feature" {
						for_each = local.features
						content {
							name              = feature.key
							permission_groups = feature.value.permission_groups
						}
					}
				}

				data "polaris_aws_cnp_permissions" "workernode" {
					role_key = "EXOCOMPUTE_EKS_WORKERNODE"

					dynamic "feature" {
						for_each = local.features
						content {
							name              = feature.key
							permission_groups = feature.value["permission_groups"]
						}
					}
				}

				data "polaris_aws_cnp_permissions" "lambda" {
					role_key = "EXOCOMPUTE_EKS_LAMBDA"

					dynamic "feature" {
						for_each = local.features
						content {
							name              = feature.key
							permission_groups = feature.value.permission_groups
						}
					}
				}

				data "polaris_aws_cnp_permissions" "role_chaining" {
					role_key = "ROLE_CHAINING"
					feature {
						name              = "ROLE_CHAINING"
						permission_groups = ["BASIC"]
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// CROSSACCOUNT.
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.crossaccount",
					tfjsonpath.New(keyID), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.crossaccount",
					tfjsonpath.New(keyCustomerManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("CLOUDACCOUNTS"),
							keyName:    knownvalue.StringExact("CloudAccountsPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("CLOUD_DISCOVERY"),
							keyName:    knownvalue.StringExact("CloudDiscoveryPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("CLOUD_NATIVE_ARCHIVAL"),
							keyName:    knownvalue.StringExact("CloudNativeArchivalLocationPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("CLOUD_NATIVE_DYNAMODB_PROTECTION"),
							keyName:    knownvalue.StringExact("DynamoDBProtectionPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
							keyName:    knownvalue.StringExact("EC2ProtectionPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("CLOUD_NATIVE_S3_PROTECTION"),
							keyName:    knownvalue.StringExact("S3ProtectionPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("EXOCOMPUTE"),
							keyName:    knownvalue.StringExact("ExocomputeInlinePolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("EXOCOMPUTE"),
							keyName:    knownvalue.StringExact("ExocomputePolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("KUBERNETES_PROTECTION"),
							keyName:    knownvalue.StringExact("KubernetesProtectionPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("RDS_PROTECTION"),
							keyName:    knownvalue.StringExact("RDSProtectionPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("SERVERS_AND_APPS"),
							keyName:    knownvalue.StringExact("ServersAndAppFeaturePolicy"),
						}),
					})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.crossaccount",
					tfjsonpath.New(keyManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{})),

				// EXOCOMPUTE_EKS_MASTERNODE.
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.masternode",
					tfjsonpath.New(keyID), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.masternode",
					tfjsonpath.New(keyCustomerManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.masternode",
					tfjsonpath.New(keyManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
					})),

				// EXOCOMPUTE_EKS_WORKERNODE.
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.workernode",
					tfjsonpath.New(keyID), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.workernode",
					tfjsonpath.New(keyCustomerManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("EXOCOMPUTE"),
							keyName:    knownvalue.StringExact("EBSCSIDriverPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("EXOCOMPUTE"),
							keyName:    knownvalue.StringExact("EKSWorkerNodePolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("EXOCOMPUTE"),
							keyName:    knownvalue.StringExact("NodeRoleAutoscalingPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("EXOCOMPUTE"),
							keyName:    knownvalue.StringExact("NodeRoleKMSPolicy"),
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("EXOCOMPUTE"),
							keyName:    knownvalue.StringExact("NodeRoleSSMPolicy"),
						}),
					})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.workernode",
					tfjsonpath.New(keyManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"),
						knownvalue.StringExact("arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"),
					})),

				// EXOCOMPUTE_EKS_LAMBDA.
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.lambda",
					tfjsonpath.New(keyID), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.lambda",
					tfjsonpath.New(keyCustomerManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.lambda",
					tfjsonpath.New(keyManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"),
						knownvalue.StringExact("arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"),
					})),

				// ROLE_CHAINING.
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.role_chaining",
					tfjsonpath.New(keyID), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.role_chaining",
					tfjsonpath.New(keyCustomerManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyFeature: knownvalue.StringExact("ROLE_CHAINING"),
							keyName:    knownvalue.StringExact("RoleChainingPolicy"),
							keyPolicy: knownvalue.StringExact(
								`{"Statement":[{"Sid":"RoleChainingPolicySid","Effect":"Allow",` +
									`"Action":["sts:AssumeRole"],"Resource":["*"]}],"Version":"2012-10-17"}`),
						}),
					})),
				statecheck.ExpectKnownValue("data.polaris_aws_cnp_permissions.role_chaining",
					tfjsonpath.New(keyManagedPolicies),
					knownvalue.ListExact([]knownvalue.Check{})),
			},
		}},
	})
}

// TestAccAwsCnpPermissionsDataSource_FrameworkMigration verifies that the
// migrated aws_cnp_permissions data source is backwards compatible with the
// SDKv2 provider.
func TestAccAwsCnpPermissionsDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.6.3",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "polaris_aws_cnp_permissions" "old" {
					provider = polaris-sdkv2

					role_key = "CROSSACCOUNT"
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_aws_cnp_permissions" "new" {
					role_key = "CROSSACCOUNT"
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_aws_cnp_permissions" "old_multi" {
					provider = polaris-sdkv2

					cloud    = "STANDARD"
					role_key = "CROSSACCOUNT"
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
				}

				data "polaris_aws_cnp_permissions" "new_multi" {
					cloud    = "STANDARD"
					role_key = "CROSSACCOUNT"
					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
					feature {
						name              = "EXOCOMPUTE"
						permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// Default-cloud (cloud unset) pair.
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_permissions.old", tfjsonpath.New(keyID),
					"data.polaris_aws_cnp_permissions.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_permissions.old", tfjsonpath.New(keyCustomerManagedPolicies),
					"data.polaris_aws_cnp_permissions.new", tfjsonpath.New(keyCustomerManagedPolicies),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_permissions.old", tfjsonpath.New(keyManagedPolicies),
					"data.polaris_aws_cnp_permissions.new", tfjsonpath.New(keyManagedPolicies),
					compare.ValuesSame()),

				// Multi-feature, explicit-cloud pair.
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_permissions.old_multi", tfjsonpath.New(keyID),
					"data.polaris_aws_cnp_permissions.new_multi", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_permissions.old_multi", tfjsonpath.New(keyCustomerManagedPolicies),
					"data.polaris_aws_cnp_permissions.new_multi", tfjsonpath.New(keyCustomerManagedPolicies),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_aws_cnp_permissions.old_multi", tfjsonpath.New(keyManagedPolicies),
					"data.polaris_aws_cnp_permissions.new_multi", tfjsonpath.New(keyManagedPolicies),
					compare.ValuesSame()),
			},
		}},
	})
}
