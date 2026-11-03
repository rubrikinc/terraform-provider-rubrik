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
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestDeploymentDataSourceSchema verifies that the migrated data source keeps
// the SDKv2 schema: every attribute is computed, there are no lookup keys, and
// only the polaris alias is deprecated.
func TestDeploymentDataSourceSchema(t *testing.T) {
	tests := []struct {
		name           string
		newDataSource  func() datasource.DataSource
		wantTypeName   string
		wantDeprecated bool
	}{
		{
			name:          "rubrik",
			newDataSource: newDeploymentDataSource,
			wantTypeName:  "rubrik_deployment",
		},
		{
			name:           "polaris alias",
			newDataSource:  newPolarisDeploymentDataSource,
			wantTypeName:   "polaris_deployment",
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

			computed := []string{keyID, keyIPAddresses, keyVersion}
			if len(res.Schema.Attributes) != len(computed) {
				t.Errorf("got %d attributes, want %d", len(res.Schema.Attributes), len(computed))
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

func TestAccDeploymentDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the data source returns the deployment IP addresses
			// and version, and that the ID is a SHA-256 hash.
			Config: `
				data "rubrik_deployment" "current" {}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_deployment.current", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{64}$`))),
				statecheck.ExpectKnownValue("data.rubrik_deployment.current", tfjsonpath.New(keyVersion),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.rubrik_deployment.current", tfjsonpath.New(keyIPAddresses),
					knownvalue.NotNull()),
			},
		}},
	})
}

// TestAccDeploymentDataSource_FrameworkMigration verifies that the migrated
// deployment data source is backwards compatible with the SDKv2 provider.
func TestAccDeploymentDataSource_FrameworkMigration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.11.2",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the SDKv2 and Framework data sources return
			// identical values for every attribute.
			Config: `
				variable "credentials" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				data "polaris_deployment" "old" {
					provider = polaris-sdkv2
				}

				data "polaris_deployment" "new" {}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.polaris_deployment.old", tfjsonpath.New(keyID),
					"data.polaris_deployment.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_deployment.old", tfjsonpath.New(keyIPAddresses),
					"data.polaris_deployment.new", tfjsonpath.New(keyIPAddresses),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_deployment.old", tfjsonpath.New(keyVersion),
					"data.polaris_deployment.new", tfjsonpath.New(keyVersion),
					compare.ValuesSame()),
			},
		}},
	})
}
