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

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccCDMClusterSettingsListResource(t *testing.T) {
	clusterID, clusterName, clusterVersion := testClusterIdentity(t)

	expectCluster := func(addr string) []querycheck.QueryResultCheck {
		return []querycheck.QueryResultCheck{
			querycheck.ExpectIdentity(addr, map[string]knownvalue.Check{
				keyClusterID: knownvalue.StringExact(clusterID),
			}),
		}
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Query: true,
			// Credentials default to the RUBRIK_* environment, so the provider
			// block needs no configuration.
			Config: `
				provider "rubrik" {}

				list "rubrik_cluster_settings" "all" {
					provider = rubrik
				}
			`,
			QueryResultChecks: expectCluster("rubrik_cluster_settings.all"),
		}, {
			Query: true,
			// Filtering by name returns the same cluster picked up by the
			// unconstrained list above.
			Config: `
				variable "cluster_name" {
					type = string
				}

				provider "rubrik" {}

				list "rubrik_cluster_settings" "by_name" {
					provider = rubrik

					config {
						name = var.cluster_name
					}
				}
			`,
			ConfigVariables: config.Variables{
				"cluster_name": config.StringVariable(clusterName),
			},
			QueryResultChecks: expectCluster("rubrik_cluster_settings.by_name"),
		}, {
			Query: true,
			// Filtering by installed version returns the same cluster.
			Config: `
				variable "cluster_version" {
					type = string
				}

				provider "rubrik" {}

				list "rubrik_cluster_settings" "by_version" {
					provider = rubrik

					config {
						version = var.cluster_version
					}
				}
			`,
			ConfigVariables: config.Variables{
				"cluster_version": config.StringVariable(clusterVersion),
			},
			QueryResultChecks: expectCluster("rubrik_cluster_settings.by_version"),
		}},
	})
}
