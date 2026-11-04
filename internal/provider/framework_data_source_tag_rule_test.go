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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestTagRuleSchemas verifies that the migrated resource and data source keep
// the SDKv2 type names and deprecation, and that only the polaris aliases are
// deprecated.
func TestTagRuleSchemas(t *testing.T) {
	tests := []struct {
		name           string
		newResource    func() fwresource.Resource
		newDataSource  func() datasource.DataSource
		wantTypeName   string
		wantDeprecated bool
	}{
		{
			name:          "rubrik",
			newResource:   newTagRuleResource,
			newDataSource: newTagRuleDataSource,
			wantTypeName:  "rubrik_tag_rule",
		},
		{
			name:           "polaris alias",
			newResource:    newPolarisTagRuleResource,
			newDataSource:  newPolarisTagRuleDataSource,
			wantTypeName:   "polaris_tag_rule",
			wantDeprecated: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.newResource()
			var rMeta fwresource.MetadataResponse
			r.Metadata(t.Context(), fwresource.MetadataRequest{}, &rMeta)
			if rMeta.TypeName != tc.wantTypeName {
				t.Errorf("resource type name = %q, want %q", rMeta.TypeName, tc.wantTypeName)
			}
			var rSchema fwresource.SchemaResponse
			r.Schema(t.Context(), fwresource.SchemaRequest{}, &rSchema)
			if diags := rSchema.Schema.ValidateImplementation(t.Context()); diags.HasError() {
				t.Fatalf("invalid resource schema: %v", diags)
			}
			if got := rSchema.Schema.DeprecationMessage != ""; got != tc.wantDeprecated {
				t.Errorf("resource deprecated = %t, want %t", got, tc.wantDeprecated)
			}

			// The tag condition must be a block, not an attribute, so that
			// existing configurations using `tag { ... }` keep parsing.
			if _, ok := rSchema.Schema.Blocks[keyTag]; !ok {
				t.Errorf("resource %q must be a block", keyTag)
			}
			if _, ok := rSchema.Schema.Attributes[keyTag]; ok {
				t.Errorf("resource %q must not be an attribute", keyTag)
			}

			d := tc.newDataSource()
			var dMeta datasource.MetadataResponse
			d.Metadata(t.Context(), datasource.MetadataRequest{}, &dMeta)
			if dMeta.TypeName != tc.wantTypeName {
				t.Errorf("data source type name = %q, want %q", dMeta.TypeName, tc.wantTypeName)
			}
			var dSchema datasource.SchemaResponse
			d.Schema(t.Context(), datasource.SchemaRequest{}, &dSchema)
			if diags := dSchema.Schema.ValidateImplementation(t.Context()); diags.HasError() {
				t.Fatalf("invalid data source schema: %v", diags)
			}
			if got := dSchema.Schema.DeprecationMessage != ""; got != tc.wantDeprecated {
				t.Errorf("data source deprecated = %t, want %t", got, tc.wantDeprecated)
			}
		})
	}
}

// TestAccTagRuleDataSource_PlanValidation verifies that the lookup keys of the
// data source are validated at plan time. The steps only plan, so they create
// nothing in RSC.
func TestAccTagRuleDataSource_PlanValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Both id and name are specified.
			Config: `
				data "rubrik_tag_rule" "rule" {
					id   = "00000000-0000-0000-0000-000000000000"
					name = "both set"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
		}, {
			// Neither id nor name is specified.
			Config: `
				data "rubrik_tag_rule" "rule" {}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`No attribute specified when one \(and only one\) of`),
		}, {
			// The ID is not a UUID.
			Config: `
				data "rubrik_tag_rule" "rule" {
					id = "not-a-uuid"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid UUID`),
		}},
	})
}

// TestAccTagRuleDataSource looks up a tag rule by ID and by name. It creates a
// tag rule, and needs an account which is allowed to delete tag rules.
func TestAccTagRuleDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             tagRuleCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the data source finds the tag rule by ID and by name,
			// and that both return the tag condition in both styles.
			Config: `
				resource "rubrik_tag_rule" "rule" {
					name        = "Terraform Test Tag Rule DS"
					object_type = "AWS_EC2_INSTANCE"
					tag_key     = "Environment"
					tag_value   = "Production"
				}

				data "rubrik_tag_rule" "by_id" {
					id = rubrik_tag_rule.rule.id
				}

				data "rubrik_tag_rule" "by_name" {
					name = rubrik_tag_rule.rule.name
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs("data.rubrik_tag_rule.by_id", tfjsonpath.New(keyID),
					"rubrik_tag_rule.rule", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.CompareValuePairs("data.rubrik_tag_rule.by_name", tfjsonpath.New(keyID),
					"rubrik_tag_rule.rule", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_tag_rule.by_id", tfjsonpath.New(keyTagKey),
					knownvalue.StringExact("Environment")),
				statecheck.ExpectKnownValue("data.rubrik_tag_rule.by_id", tfjsonpath.New(keyTag),
					knownvalue.ListSizeExact(1)),
			},
		}},
	})
}
