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

//go:build cdm

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccCDMSLASourceClusterDataSource(t *testing.T) {
	clusterID, clusterName, clusterVersion := testClusterIdentity(t)

	const ds = "data.rubrik_sla_source_cluster.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the cluster is found by name, and that the ID and
			// version match the cluster in the Data Center test configuration.
			Config: `
				variable "name" {
					type = string
				}

				data "rubrik_sla_source_cluster" "test" {
					name = var.name
				}
			`,
			ConfigVariables: config.Variables{
				"name": config.StringVariable(clusterName),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyID), knownvalue.StringExact(clusterID)),
				statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyName), knownvalue.StringExact(clusterName)),
				statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyVersion), knownvalue.StringExact(clusterVersion)),
			},
		}},
	})
}

// TestAccCDMSLASourceClusterDataSource_FrameworkMigration verifies that the
// migrated sla_source_cluster data source is backwards compatible with the
// SDKv2 provider.
func TestAccCDMSLASourceClusterDataSource_FrameworkMigration(t *testing.T) {
	_, clusterName, _ := testClusterIdentity(t)

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
				variable "name" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				data "polaris_sla_source_cluster" "old" {
					provider = polaris-sdkv2

					name = var.name
				}

				data "polaris_sla_source_cluster" "new" {
					name = var.name
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"name":        config.StringVariable(clusterName),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.polaris_sla_source_cluster.old", tfjsonpath.New(keyID),
					"data.polaris_sla_source_cluster.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_sla_source_cluster.old", tfjsonpath.New(keyName),
					"data.polaris_sla_source_cluster.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_sla_source_cluster.old", tfjsonpath.New(keyVersion),
					"data.polaris_sla_source_cluster.new", tfjsonpath.New(keyVersion),
					compare.ValuesSame()),
			},
		}},
	})
}
