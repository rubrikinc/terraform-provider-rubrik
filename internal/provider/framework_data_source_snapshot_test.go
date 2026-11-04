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
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestSnapshotDataSourceSchema verifies that the migrated data source keeps the
// SDKv2 schema: the workload ID is required, the time bounds are optional, the
// exclude flags are optional and computed so that Read can default them to
// true, and only the polaris alias is deprecated.
func TestSnapshotDataSourceSchema(t *testing.T) {
	tests := []struct {
		name           string
		newDataSource  func() datasource.DataSource
		wantTypeName   string
		wantDeprecated bool
	}{
		{
			name:          "rubrik",
			newDataSource: newSnapshotDataSource,
			wantTypeName:  "rubrik_snapshot",
		},
		{
			name:           "polaris alias",
			newDataSource:  newPolarisSnapshotDataSource,
			wantTypeName:   "polaris_snapshot",
			wantDeprecated: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := tc.newDataSource()

			var meta datasource.MetadataResponse
			ds.Metadata(t.Context(), datasource.MetadataRequest{}, &meta)
			if meta.TypeName != tc.wantTypeName {
				t.Errorf("type name = %q, want %q", meta.TypeName, tc.wantTypeName)
			}

			var res datasource.SchemaResponse
			ds.Schema(t.Context(), datasource.SchemaRequest{}, &res)
			if res.Diagnostics.HasError() {
				t.Fatalf("unexpected schema diagnostics: %v", res.Diagnostics)
			}
			if diags := res.Schema.ValidateImplementation(t.Context()); diags.HasError() {
				t.Fatalf("invalid schema: %v", diags)
			}

			if got := res.Schema.DeprecationMessage != ""; got != tc.wantDeprecated {
				t.Errorf("deprecated = %t, want %t", got, tc.wantDeprecated)
			}
			if len(res.Schema.Attributes) != 7 {
				t.Errorf("got %d attributes, want 7", len(res.Schema.Attributes))
			}

			attributes := []struct {
				key                          string
				required, optional, computed bool
			}{
				{key: keyID, computed: true},
				{key: keyWorkloadID, required: true},
				{key: keyBeforeTime, optional: true},
				{key: keyAfterTime, optional: true},
				{key: keyExcludeQuarantined, optional: true, computed: true},
				{key: keyExcludeAnomalous, optional: true, computed: true},
				{key: keyDate, computed: true},
			}
			for _, want := range attributes {
				attr, ok := res.Schema.Attributes[want.key]
				if !ok {
					t.Errorf("attribute %q is missing", want.key)
					continue
				}
				if attr.IsRequired() != want.required || attr.IsOptional() != want.optional ||
					attr.IsComputed() != want.computed {
					t.Errorf("attribute %q: required=%t optional=%t computed=%t, want required=%t optional=%t computed=%t",
						want.key, attr.IsRequired(), attr.IsOptional(), attr.IsComputed(),
						want.required, want.optional, want.computed)
				}
			}
		})
	}
}

// TestAccSnapshotDataSource_PlanValidation verifies that the plan-time rules of
// the SDKv2 data source are kept. The steps only plan, so they need no workload
// and create nothing in RSC.
func TestAccSnapshotDataSource_PlanValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Neither before_time nor after_time is set.
			Config: `
				data "rubrik_snapshot" "test" {
					workload_id = "00000000-0000-0000-0000-000000000000"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)No attribute specified when one \(and only one\) of`),
		}, {
			// Both before_time and after_time are set.
			Config: `
				data "rubrik_snapshot" "test" {
					workload_id = "00000000-0000-0000-0000-000000000000"
					before_time = "2025-01-01T00:00:00Z"
					after_time  = "2025-01-02T00:00:00Z"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)2 attributes specified when one \(and only one\) of`),
		}, {
			// The timestamp is not RFC 3339.
			Config: `
				data "rubrik_snapshot" "test" {
					workload_id = "00000000-0000-0000-0000-000000000000"
					before_time = "2025-01-01"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid Timestamp`),
		}, {
			// The workload ID is not a UUID.
			Config: `
				data "rubrik_snapshot" "test" {
					workload_id = "not-a-uuid"
					before_time = "2025-01-01T00:00:00Z"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid UUID`),
		}},
	})
}
