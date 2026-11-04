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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func TestAccTagRuleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             tagRuleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with a single tag block,
			// and that match_all defaults to false.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyName),
					knownvalue.StringExact("Terraform Test Tag Rule")),
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyObjectType),
					knownvalue.StringExact("AWS_EC2_INSTANCE")),
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyTag),
					knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							keyKey:         knownvalue.StringExact("Environment"),
							keyValues:      knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("Production")}),
							keyTagMatchAll: knownvalue.Bool(false),
						}),
					})),
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyTagKey), knownvalue.Null()),
				statecheck.ExpectIdentity("rubrik_tag_rule.rule", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_tag_rule.rule", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name can be updated in place, and that the ID
			// does not change.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Updated"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}
			`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_tag_rule.rule", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyName),
					knownvalue.StringExact("Terraform Test Tag Rule Updated")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_tag_rule.rule", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that changing the object type replaces the resource.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Updated"
					object_type = "AWS_EBS_VOLUME"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}
			`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_tag_rule.rule", plancheck.ResourceActionReplace),
				},
			},
		}, {
			// Verify that changing the tag replaces the resource.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Updated"
					object_type = "AWS_EBS_VOLUME"

					tag {
						key    = "Environment"
						values = ["Staging"]
					}
				}
			`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_tag_rule.rule", plancheck.ResourceActionReplace),
				},
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_tag_rule.rule",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_tag_rule.rule",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_tag_rule.rule",
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

func TestAccTagRuleResource_MultiTag(t *testing.T) {
	skipUnlessFeatureEnabled(t, core.FeatureFlagMultipleKeyValuePairsInTagRules)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             tagRuleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that a tag rule with multiple tag blocks is created, and
			// that the state is sorted by key.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Multi"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key       = "Owner"
						match_all = true
					}

					tag {
						key    = "Environment"
						values = ["Production", "Staging"]
					}
				}
			`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_tag_rule.rule", plancheck.ResourceActionCreate),
				},
			},
		}, {
			// The configuration lists the tags in a different order than the
			// sorted state, so the re-plan being empty shows that the order is
			// not a difference. This mirrors the SDKv2 behaviour, which sorted
			// the tags on read.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Multi"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key    = "Environment"
						values = ["Production", "Staging"]
					}

					tag {
						key       = "Owner"
						match_all = true
					}
				}
			`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

func TestAccTagRuleResource_Deprecated(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             tagRuleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the deprecated tag_key and tag_value fields still work,
			// and that only the deprecated fields are populated, not the tag
			// block, to avoid a plan diff.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Deprecated"
					object_type = "AWS_EC2_INSTANCE"
					tag_key     = "Environment"
					tag_value   = "Production"
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyTagKey),
					knownvalue.StringExact("Environment")),
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyTagValue),
					knownvalue.StringExact("Production")),
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyTag),
					knownvalue.ListSizeExact(0)),
			},
		}, {
			// Verify that the deprecated tag_all_values field still works.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name           = "Terraform Test Tag Rule Deprecated"
					object_type    = "AWS_EC2_INSTANCE"
					tag_key        = "Environment"
					tag_all_values = true
				}
			`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_tag_rule.rule", plancheck.ResourceActionReplace),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_tag_rule.rule", tfjsonpath.New(keyTagAllValues),
					knownvalue.Bool(true)),
			},
		}},
	})
}

// TestAccTagRuleResource_PlanValidation verifies that the plan-time rules of the
// SDKv2 resource are kept. The steps only plan, so they create nothing in RSC.
func TestAccTagRuleResource_PlanValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Neither tag nor tag_key is specified.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Invalid"
					object_type = "AWS_EC2_INSTANCE"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`one of .tag. or .tag_key. must be specified`),
		}, {
			// tag_key is specified without tag_value or tag_all_values.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Invalid"
					object_type = "AWS_EC2_INSTANCE"
					tag_key     = "Environment"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`one of .tag_value. or .tag_all_values. must be specified`),
		}, {
			// tag and tag_key are specified together.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Invalid"
					object_type = "AWS_EC2_INSTANCE"
					tag_key     = "Environment"
					tag_value   = "Production"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
		}, {
			// values is specified when match_all is true.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Invalid"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key       = "Environment"
						values    = ["Production"]
						match_all = true
					}
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`must be empty when .match_all. is true`),
		}, {
			// The object type is not a valid tag rule object type.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule Invalid"
					object_type = "NOT_AN_OBJECT_TYPE"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		}},
	})
}

// TestAccTagRuleResource_FrameworkMigration verifies that the migrated tag_rule
// resource and data source are backwards compatible with the SDKv2 provider.
func TestAccTagRuleResource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.11.2",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             tagRuleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Create a tag rule with the SDKv2 resource, and verify that the
			// SDKv2 and Framework data sources return identical values.
			Config: `
				variable "credentials" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				resource "polaris_tag_rule" "rule" {
					provider = polaris-sdkv2

					name        = "Terraform Test Tag Rule Migration"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}

				data "polaris_tag_rule" "old" {
					provider = polaris-sdkv2

					id = polaris_tag_rule.rule.id
				}

				data "polaris_tag_rule" "new" {
					id = polaris_tag_rule.rule.id
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.polaris_tag_rule.old", tfjsonpath.New(keyID),
					"data.polaris_tag_rule.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_tag_rule.old", tfjsonpath.New(keyName),
					"data.polaris_tag_rule.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_tag_rule.old", tfjsonpath.New(keyObjectType),
					"data.polaris_tag_rule.new", tfjsonpath.New(keyObjectType),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_tag_rule.old", tfjsonpath.New(keyTag),
					"data.polaris_tag_rule.new", tfjsonpath.New(keyTag),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_tag_rule.old", tfjsonpath.New(keyTagKey),
					"data.polaris_tag_rule.new", tfjsonpath.New(keyTagKey),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_tag_rule.old", tfjsonpath.New(keyTagValue),
					"data.polaris_tag_rule.new", tfjsonpath.New(keyTagValue),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_tag_rule.old", tfjsonpath.New(keyTagAllValues),
					"data.polaris_tag_rule.new", tfjsonpath.New(keyTagAllValues),
					compare.ValuesSame()),
			},
		}},
	})
}

// TestAccTagRuleResource_MoveState verifies that state can be moved from the
// polaris_tag_rule resource in the polaris provider, for both the tag block and
// the deprecated tag fields.
func TestAccTagRuleResource_MoveState(t *testing.T) {
	vars := config.Variables{
		"credentials": config.StringVariable(testCredentials(t)),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		CheckDestroy: tagRuleCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "credentials" {
					type = string
				}

				provider "polaris" {
					credentials = var.credentials
				}

				resource "polaris_tag_rule" "block" {
					name        = "Terraform Test Tag Rule Move Block"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}

				resource "polaris_tag_rule" "legacy" {
					name        = "Terraform Test Tag Rule Move Legacy"
					object_type = "AWS_EC2_INSTANCE"
					tag_key     = "Environment"
					tag_value   = "Production"
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_tag_rule.block", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("polaris_tag_rule.legacy", tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				variable "credentials" {
					type = string
				}

				moved {
					from = polaris_tag_rule.block
					to   = rubrik_tag_rule.block
				}

				moved {
					from = polaris_tag_rule.legacy
					to   = rubrik_tag_rule.legacy
				}

				resource "rubrik_tag_rule" "block" {
					name        = "Terraform Test Tag Rule Move Block"
					object_type = "AWS_EC2_INSTANCE"

					tag {
						key    = "Environment"
						values = ["Production"]
					}
				}

				resource "rubrik_tag_rule" "legacy" {
					name        = "Terraform Test Tag Rule Move Legacy"
					object_type = "AWS_EC2_INSTANCE"
					tag_key     = "Environment"
					tag_value   = "Production"
				}
			`,
			ConfigVariables: vars,
			// Verify the plan is empty, so the move succeeded without drift, and
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

func TestValidateTagRuleConfig(t *testing.T) {
	tagList := func(match bool, values ...string) types.List {
		valueElems := make([]attr.Value, 0, len(values))
		for _, v := range values {
			valueElems = append(valueElems, types.StringValue(v))
		}
		obj := types.ObjectValueMust(tagRuleTagAttrTypes(), map[string]attr.Value{
			keyKey:         types.StringValue("Environment"),
			keyValues:      types.ListValueMust(types.StringType, valueElems),
			keyTagMatchAll: types.BoolValue(match),
		})
		return types.ListValueMust(types.ObjectType{AttrTypes: tagRuleTagAttrTypes()}, []attr.Value{obj})
	}
	noTags := types.ListNull(types.ObjectType{AttrTypes: tagRuleTagAttrTypes()})

	tests := []struct {
		name    string
		config  tagRuleModel
		wantErr string
	}{
		{
			name: "TagBlock",
			config: tagRuleModel{
				Tag: tagList(false, "Production"), TagKey: types.StringNull(), TagValue: types.StringNull(),
				TagAllValues: types.BoolNull(),
			},
		},
		{
			name: "TagBlockMatchAll",
			config: tagRuleModel{
				Tag: tagList(true), TagKey: types.StringNull(), TagValue: types.StringNull(),
				TagAllValues: types.BoolNull(),
			},
		},
		{
			name: "TagBlockMatchAllWithValues",
			config: tagRuleModel{
				Tag: tagList(true, "Production"), TagKey: types.StringNull(), TagValue: types.StringNull(),
				TagAllValues: types.BoolNull(),
			},
			wantErr: "must be empty when `match_all` is true",
		},
		{
			name: "TagKeyAndTagValue",
			config: tagRuleModel{
				Tag: noTags, TagKey: types.StringValue("Environment"), TagValue: types.StringValue("Production"),
				TagAllValues: types.BoolNull(),
			},
		},
		{
			name: "TagKeyAndTagAllValues",
			config: tagRuleModel{
				Tag: noTags, TagKey: types.StringValue("Environment"), TagValue: types.StringNull(),
				TagAllValues: types.BoolValue(true),
			},
		},
		{
			name: "TagKeyWithoutValue",
			config: tagRuleModel{
				Tag: noTags, TagKey: types.StringValue("Environment"), TagValue: types.StringNull(),
				TagAllValues: types.BoolNull(),
			},
			wantErr: "one of `tag_value` or `tag_all_values` must be specified when using `tag_key`",
		},
		{
			name: "NeitherTagNorTagKey",
			config: tagRuleModel{
				Tag: noTags, TagKey: types.StringNull(), TagValue: types.StringNull(), TagAllValues: types.BoolNull(),
			},
			wantErr: "one of `tag` or `tag_key` must be specified",
		},
		{
			name: "UnknownTagKeyIsNotChecked",
			config: tagRuleModel{
				Tag: noTags, TagKey: types.StringUnknown(), TagValue: types.StringNull(), TagAllValues: types.BoolNull(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateTagRuleConfig(t.Context(), tc.config)
			if tc.wantErr == "" {
				if diags.HasError() {
					t.Fatalf("unexpected error: %v", diags)
				}
				return
			}
			if !diags.HasError() {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			found := false
			for _, d := range diags.Errors() {
				if regexp.MustCompile(regexp.QuoteMeta(tc.wantErr)).MatchString(d.Summary() + " " + d.Detail()) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected an error containing %q, got %v", tc.wantErr, diags)
			}
		})
	}
}

// TestTagRuleTagBlockPlan verifies that a plan which only changes the name does
// not replace the resource. The tag block requires replacement when it changes,
// and match_all is computed with a default of false, so the planned value of an
// unset match_all must equal the state value and not be unknown.
func TestTagRuleTagBlockPlan(t *testing.T) {
	r := newTagRuleResource()
	var res fwresource.SchemaResponse
	r.Schema(t.Context(), fwresource.SchemaRequest{}, &res)

	block, ok := res.Schema.Blocks[keyTag].(schema.ListNestedBlock)
	if !ok {
		t.Fatalf("%q is not a list nested block", keyTag)
	}
	matchAll, ok := block.NestedObject.Attributes[keyTagMatchAll].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("%q is not a bool attribute", keyTagMatchAll)
	}

	if !matchAll.Computed {
		t.Errorf("%q must be computed, which a default requires", keyTagMatchAll)
	}
	if matchAll.Default == nil {
		t.Fatalf("%q must have a default, so its planned value is false and not unknown", keyTagMatchAll)
	}

	var defaultRes defaults.BoolResponse
	matchAll.Default.DefaultBool(t.Context(), defaults.BoolRequest{}, &defaultRes)
	if defaultRes.PlanValue.IsNull() || defaultRes.PlanValue.IsUnknown() || defaultRes.PlanValue.ValueBool() {
		t.Errorf("default = %s, want false", defaultRes.PlanValue)
	}
}

func TestTagRuleTagValues(t *testing.T) {
	null := types.ListNull(types.StringType)
	empty := types.ListValueMust(types.StringType, []attr.Value{})
	unknown := types.ListUnknown(types.StringType)
	prod := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("Production")})

	tests := []struct {
		name   string
		values []string
		prior  types.List
		want   types.List
	}{
		{name: "NoValuesAndNullPrior", values: nil, prior: null, want: null},
		{name: "NoValuesAndUnknownPrior", values: []string{}, prior: unknown, want: null},
		{name: "NoValuesAndEmptyPrior", values: nil, prior: empty, want: empty},
		{name: "ValuesAreSorted", values: []string{"Staging", "Production"}, prior: null, want: types.ListValueMust(
			types.StringType, []attr.Value{types.StringValue("Production"), types.StringValue("Staging")})},
		{name: "ValuesWithPrior", values: []string{"Production"}, prior: prod, want: prod},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := tagRuleTagValues(tc.values, tc.prior)
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if !got.Equal(tc.want) {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}
