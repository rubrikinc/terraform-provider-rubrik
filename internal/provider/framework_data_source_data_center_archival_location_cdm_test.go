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

func TestAccCDMDataCenterArchivalLocationDataSource(t *testing.T) {
	_, dc := loadDataCenterTestConfig(t)
	if len(dc.ArchivalLocations) == 0 {
		t.Fatal("TEST_DATACENTER_FILE has no archival locations")
	}

	for _, loc := range dc.ArchivalLocations {
		t.Run(loc.LocationName, func(t *testing.T) {
			const ds = "data.rubrik_data_center_archival_location.test"
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories,
				Steps: []resource.TestStep{{
					// Verify that the archival location is found by cluster ID
					// and name, and that the computed attributes are populated.
					Config: `
						variable "cluster_id" {
							type = string
						}
						variable "name" {
							type = string
						}

						data "rubrik_data_center_archival_location" "test" {
							cluster_id = var.cluster_id
							name       = var.name
						}
					`,
					ConfigVariables: config.Variables{
						"cluster_id": config.StringVariable(dc.ClusterUUID.String()),
						"name":       config.StringVariable(loc.LocationName),
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyID), knownvalue.StringExact(loc.LocationID)),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyClusterID),
							knownvalue.StringExact(dc.ClusterUUID.String())),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyName), knownvalue.StringExact(loc.LocationName)),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyTargetType),
							knownvalue.StringExact(loc.LocationType)),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyStatus), knownvalue.NotNull()),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyActive), knownvalue.NotNull()),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyClusterName), knownvalue.NotNull()),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyClusterStatus), knownvalue.NotNull()),
						statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyClusterVersion), knownvalue.NotNull()),
					},
				}},
			})
		})
	}
}

// TestAccCDMDataCenterArchivalLocationDataSource_FrameworkMigration verifies
// that the migrated data_center_archival_location data source is backwards
// compatible with the SDKv2 provider.
func TestAccCDMDataCenterArchivalLocationDataSource_FrameworkMigration(t *testing.T) {
	_, dc := loadDataCenterTestConfig(t)
	if len(dc.ArchivalLocations) == 0 {
		t.Fatal("TEST_DATACENTER_FILE has no archival locations")
	}
	loc := dc.ArchivalLocations[0]

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
				variable "cluster_id" {
					type = string
				}
				variable "name" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				data "polaris_data_center_archival_location" "old" {
					provider = polaris-sdkv2

					cluster_id = var.cluster_id
					name       = var.name
				}

				data "polaris_data_center_archival_location" "new" {
					cluster_id = var.cluster_id
					name       = var.name
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(testCredentials(t)),
				"cluster_id":  config.StringVariable(dc.ClusterUUID.String()),
				"name":        config.StringVariable(loc.LocationName),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyID),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyName),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyStatus),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyStatus),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyTargetType),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyTargetType),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyActive),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyActive),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyClusterName),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyClusterName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyClusterStatus),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyClusterStatus),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"data.polaris_data_center_archival_location.old", tfjsonpath.New(keyClusterVersion),
					"data.polaris_data_center_archival_location.new", tfjsonpath.New(keyClusterVersion),
					compare.ValuesSame()),
			},
		}},
	})
}
