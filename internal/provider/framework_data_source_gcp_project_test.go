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

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const gcpProjectDataSourceTmpl = `
provider "rubrik" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "rubrik_gcp_project" "default" {
	credentials    = "{{ .Resource.Credentials }}"
	project        = "{{ .Resource.ProjectID }}"
	project_name   = "{{ .Resource.ProjectName }}"
	project_number = {{ .Resource.ProjectNumber }}

	feature {
		name              = "CLOUD_NATIVE_PROTECTION"
		permission_groups = ["BASIC"]
	}
}

data "rubrik_gcp_project" "by_id" {
	id = rubrik_gcp_project.default.id
}

data "rubrik_gcp_project" "by_name" {
	name = rubrik_gcp_project.default.project_name
}

data "rubrik_gcp_project" "by_project_id" {
	project_id = rubrik_gcp_project.default.project
}

data "rubrik_gcp_project" "by_project_number" {
	project_number = rubrik_gcp_project.default.project_number
}
`

func TestAccGCPProjectDataSource(t *testing.T) {
	testConfig, _ := loadGCPTestConfig(t)
	tfConfig, err := makeTerraformConfig(testConfig, gcpProjectDataSourceTmpl)
	if err != nil {
		t.Fatal(err)
	}

	var checks []statecheck.StateCheck
	for _, name := range []string{"by_id", "by_name", "by_project_id", "by_project_number"} {
		ds := "data.rubrik_gcp_project." + name
		checks = append(checks,
			statecheck.CompareValuePairs("rubrik_gcp_project.default", tfjsonpath.New(keyID),
				ds, tfjsonpath.New(keyID), compare.ValuesSame()),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyName),
				knownvalue.StringExact(testGCPProjectName(t))),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyProjectID),
				knownvalue.StringExact(testGCPProjectID(t))),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyProjectNumber),
				knownvalue.StringExact(testGCPProjectNumber(t))),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyOrganizationName),
				knownvalue.StringExact(testGCPOrganizationName(t))),
			statecheck.ExpectKnownValue(ds, tfjsonpath.New(keyFeature), knownvalue.SetPartial([]knownvalue.Check{
				knownvalue.ObjectExact(map[string]knownvalue.Check{
					keyName:             knownvalue.StringExact("CLOUD_NATIVE_PROTECTION"),
					keyPermissionGroups: knownvalue.SetExact([]knownvalue.Check{knownvalue.StringExact("BASIC")}),
					keyStatus:           knownvalue.StringExact("CONNECTED"),
				}),
			})),
		)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the project is found by the RSC cloud account ID,
			// the name, the GCP project ID and the GCP project number, and
			// that every lookup returns the same project.
			Config:            tfConfig,
			ConfigStateChecks: checks,
		}},
	})
}

// TestAccGCPProjectDataSourceFrameworkMigration verifies that the migrated
// gcp_project data source returns the same values as the SDKv2 implementation
// in the polaris provider. The feature attribute is not compared, since the
// status of the features changed from lower case to the RSC value in v1.12.0
// of the rubrik provider.
func TestAccGCPProjectDataSourceFrameworkMigration(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	var checks []statecheck.StateCheck
	for _, attr := range []string{keyID, keyName, keyOrganizationName, keyProjectID, keyProjectNumber} {
		checks = append(checks, statecheck.CompareValuePairs(
			"data.polaris_gcp_project.old", tfjsonpath.New(attr),
			"data.polaris_gcp_project.new", tfjsonpath.New(attr),
			compare.ValuesSame()))
	}

	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"polaris-sdkv2": {
				Source:            "rubrikinc/polaris",
				VersionConstraint: "1.12.0",
			},
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             gcpProjectCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Onboard the project.
			Config: `
				variable "credentials" {
					type = string
				}
				variable "project_credentials" {
					type = string
				}
				variable "project_id" {
					type = string
				}
				variable "project_name" {
					type = string
				}
				variable "project_number" {
					type = string
				}

				provider "polaris-sdkv2" {
					credentials = var.credentials
				}

				resource "rubrik_gcp_project" "default" {
					credentials    = var.project_credentials
					project        = var.project_id
					project_name   = var.project_name
					project_number = var.project_number

					feature {
						name              = "CLOUD_NATIVE_PROTECTION"
						permission_groups = ["BASIC"]
					}
				}

				data "polaris_gcp_project" "old" {
					provider   = polaris-sdkv2
					project_id = rubrik_gcp_project.default.project
				}

				data "polaris_gcp_project" "new" {
					project_id = rubrik_gcp_project.default.project
				}
			`,
			ConfigVariables: config.Variables{
				"credentials":         config.StringVariable(testCredentials(t)),
				"project_credentials": config.StringVariable(testGCPCredentials(t)),
				"project_id":          config.StringVariable(testGCPProjectID(t)),
				"project_name":        config.StringVariable(testGCPProjectName(t)),
				"project_number":      config.StringVariable(testGCPProjectNumber(t)),
			},
			// Verify that every attribute matches the SDKv2 implementation.
			ConfigStateChecks: checks,
		}},
	})
}
