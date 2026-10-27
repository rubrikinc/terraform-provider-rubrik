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
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

const gcpPrivateContainerRegistryTmpl = `
provider "rubrik" {
	credentials = "{{ .Provider.Credentials }}"
}

provider "google" {
	credentials = "{{ .Resource.Credentials }}"
}

variable "service_account_email" {
	type = string
}

variable "url" {
	type = string
}

resource "rubrik_gcp_project" "default" {
	credentials    = "{{ .Resource.Credentials }}"
	project        = "{{ .Resource.ProjectID }}"
	project_name   = "{{ .Resource.ProjectName }}"
	project_number = {{ .Resource.ProjectNumber }}

	feature {
		name = "EXOCOMPUTE"
		permission_groups = [
			"BASIC",
		]
	}

	# RSC reports the organization name as "<DISABLED>" until the
	# CLOUD_NATIVE_PROTECTION feature is onboarded, which this test doesn't do.
	lifecycle {
		ignore_changes = [
			organization_name,
		]
	}
}

# Google OIDC identity token proving ownership of the service account. Ephemeral
# values are never stored in the plan or the state and are created on every run,
# so the token is always fresh. The audience must match the one RSC verifies.
ephemeral "google_service_account_id_token" "pcr" {
	target_audience = "https://rubrik.com/exocompute/pcr/gcp"
}

resource "rubrik_gcp_private_container_registry" "default" {
	cloud_account_id                = rubrik_gcp_project.default.id
	service_account_email           = var.service_account_email
	service_account_ownership_token = ephemeral.google_service_account_id_token.pcr.id_token
	url                             = var.url
}
`

// TestAccGcpPrivateContainerRegistryResource verifies that a GCP private
// container registry can be created, updated and imported.
//
// The test skips unless the REL_ENABLE_GCP_PRIVATE_CONTAINER_REGISTRY feature
// flag is enabled for the RSC account, since RSC rejects the registry without
// it. It also skips unless the GCP test project file holds the URL of an
// Artifact Registry repository in the pcr.registryUrl field. The service account
// in the GCP credentials is the service account registered with RSC.
func TestAccGcpPrivateContainerRegistryResource(t *testing.T) {
	skipUnlessFeatureEnabled(t, FeatureFlagGCPPrivateContainerRegistry)

	testConfig, project := loadGCPTestConfig(t)
	if project.PCR.RegistryURL == "" {
		t.Skip("GCP test project file has no pcr.registryUrl")
	}

	tfConfig, err := makeTerraformConfig(testConfig, gcpPrivateContainerRegistryTmpl)
	if err != nil {
		t.Fatal(err)
	}

	// RSC does not check that the repository exists, so the update step points
	// the registry at a differently named repository.
	url := project.PCR.RegistryURL
	updatedURL := url + "-updated"

	// The service account in the GCP credentials file is the service account
	// registered with RSC, and the one the Google provider mints the ownership
	// token for.
	credentialsFile := os.Getenv(gcpCredentialsEnv)
	if credentialsFile == "" {
		t.Skipf("Environment variable %s not set", gcpCredentialsEnv)
	}
	buf, err := os.ReadFile(credentialsFile)
	if err != nil {
		t.Fatalf("failed to read GCP credentials file: %v", err)
	}
	var key struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(buf, &key); err != nil {
		t.Fatalf("failed to parse GCP credentials file: %v", err)
	}
	if key.ClientEmail == "" {
		t.Fatal("GCP credentials file is not a service account key")
	}
	email := key.ClientEmail

	variables := func(email, url string) config.Variables {
		return config.Variables{
			"service_account_email": config.StringVariable(email),
			"url":                   config.StringVariable(url),
		}
	}

	const name = "rubrik_gcp_private_container_registry.default"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		// The ownership token is created by an ephemeral resource of the Google
		// provider, which is downloaded when the test runs.
		ExternalProviders: map[string]resource.ExternalProvider{
			"google": {Source: "hashicorp/google"},
		},
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			// The service account ownership token is a write-only attribute,
			// which requires v1.11.0, and resource identity requires v1.12.0.
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			gcpPrivateContainerRegistryCheckDestroy(t),
			gcpProjectCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that an invalid registry URL is rejected at plan time. Steps
			// that expect an error go first: the final destroy reuses the last
			// step's configuration, which must be valid.
			Config:          tfConfig,
			ConfigVariables: variables(email, "docker.io/my-project/my-repo"),
			PlanOnly:        true,
			ExpectError:     regexp.MustCompile(`must be an Artifact Registry repository URL`),
		}, {
			// Verify that an invalid service account email is rejected at plan
			// time.
			Config:          tfConfig,
			ConfigVariables: variables("not-an-email", url),
			PlanOnly:        true,
			ExpectError:     regexp.MustCompile(`must be a service account email`),
		}, {
			// Verify that the resource can be created, and that the write-only
			// ownership token is not stored.
			Config:          tfConfig,
			ConfigVariables: variables(email, url),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(name, tfjsonpath.New(keyID),
					"rubrik_gcp_project.default", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.CompareValuePairs(name, tfjsonpath.New(keyCloudAccountID),
					"rubrik_gcp_project.default", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.ExpectKnownValue(name, tfjsonpath.New(keyServiceAccountEmail),
					knownvalue.StringExact(email)),
				statecheck.ExpectKnownValue(name, tfjsonpath.New(keyURL), knownvalue.StringExact(url)),
				statecheck.ExpectKnownValue(name, tfjsonpath.New(keyServiceAccountOwnershipToken),
					knownvalue.Null()),
				statecheck.ExpectIdentity(name, map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState(name, tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the url can be updated in place, and that the ID does
			// not change.
			Config:          tfConfig,
			ConfigVariables: variables(email, updatedURL),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs(name, tfjsonpath.New(keyID),
					"rubrik_gcp_project.default", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.ExpectKnownValue(name, tfjsonpath.New(keyURL), knownvalue.StringExact(updatedURL)),
				statecheck.ExpectIdentityValueMatchesState(name, tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			Config:            tfConfig,
			ConfigVariables:   variables(email, updatedURL),
			ResourceName:      name,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			Config:          tfConfig,
			ConfigVariables: variables(email, updatedURL),
			ResourceName:    name,
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			Config:          tfConfig,
			ConfigVariables: variables(email, updatedURL),
			ResourceName:    name,
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
