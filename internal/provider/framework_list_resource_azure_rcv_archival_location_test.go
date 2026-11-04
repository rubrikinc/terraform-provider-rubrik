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

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAzureRCVArchivalLocationListResource lists RCV Azure archival
// locations, filtered by the name of an archival location created by the test.
func TestAccAzureRCVArchivalLocationListResource(t *testing.T) {
	skipUnlessAzureRCVDefined(t)

	name := testUniqueName(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureRCVArchivalLocationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Create an archival location so the list resource has something
			// deterministic to return.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				resource "rubrik_azure_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
				}
			`,
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name),
				"region": config.StringVariable(testAzureRCVRegion(t)),
			},
		}, {
			// Verify that filtering by the unique name returns the archival
			// location.
			Query: true,
			Config: `
				provider "rubrik" {}

				list "rubrik_azure_rcv_archival_location" "filtered" {
					provider = rubrik

					config {
						name = var.name
					}
				}
			`,
			ConfigVariables: config.Variables{
				"name":   config.StringVariable(name),
				"region": config.StringVariable(testAzureRCVRegion(t)),
			},
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_azure_rcv_archival_location.filtered", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				querycheck.ExpectLength("rubrik_azure_rcv_archival_location.filtered", 1),
			},
		}},
	})
}
