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
)

func TestAccCDMClusterSettingsResource(t *testing.T) {
	clusterID := testClusterID(t)

	const rsc = "rubrik_cluster_settings.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Declaring the resource with only cluster_id set means reconcile
			// performs no download or upgrade and only refreshes computed state,
			// keeping the acceptance test non-destructive.
			Config: `
				variable "cluster_id" {
					type = string
				}

				resource "rubrik_cluster_settings" "test" {
					cluster_id = var.cluster_id
				}
			`,
			ConfigVariables: config.Variables{
				"cluster_id": config.StringVariable(clusterID),
			},
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(rsc, "id", clusterID),
				resource.TestCheckResourceAttr(rsc, "cluster_id", clusterID),
				resource.TestCheckResourceAttrSet(rsc, "name"),
				resource.TestCheckResourceAttrSet(rsc, "upgrade_mode"),
			),
		}, {
			ResourceName:      rsc,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateVerifyIgnore: []string{
				"package_url",
				"package_md5",
			},
			// The import step reuses the previous step's config, which declares
			// the cluster_id variable, so the value must be supplied again here.
			ConfigVariables: config.Variables{
				"cluster_id": config.StringVariable(clusterID),
			},
		}},
	})
}
