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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// TestNCDArchivalLocationDataSourceSchema verifies that the migrated data
// source keeps the SDKv2 schema: the cluster ID and the name are the required
// lookup keys, everything else is computed, and only the polaris alias is
// deprecated.
func TestNCDArchivalLocationDataSourceSchema(t *testing.T) {
	tests := []struct {
		name           string
		newDataSource  func() datasource.DataSource
		wantTypeName   string
		wantDeprecated bool
	}{
		{
			name:          "rubrik",
			newDataSource: newNCDArchivalLocationDataSource,
			wantTypeName:  "rubrik_ncd_archival_location",
		},
		{
			name:           "polaris alias",
			newDataSource:  newPolarisNCDArchivalLocationDataSource,
			wantTypeName:   "polaris_ncd_archival_location",
			wantDeprecated: true,
		},
	}

	required := map[string]bool{keyClusterID: true, keyName: true}
	computed := []string{keyID, keyStatus, keyTargetType, keyActive, keyClusterName, keyClusterStatus,
		keyClusterVersion}

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
			if len(res.Schema.Attributes) != len(required)+len(computed) {
				t.Errorf("got %d attributes, want %d", len(res.Schema.Attributes), len(required)+len(computed))
			}
			for key := range required {
				attr, ok := res.Schema.Attributes[key]
				if !ok || !attr.IsRequired() || attr.IsComputed() {
					t.Errorf("attribute %q must be required and not computed", key)
				}
			}
			for _, key := range computed {
				attr, ok := res.Schema.Attributes[key]
				if !ok || !attr.IsComputed() || attr.IsRequired() || attr.IsOptional() {
					t.Errorf("attribute %q must be computed only", key)
				}
			}
		})
	}
}
