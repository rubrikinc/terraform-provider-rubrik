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
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"google.golang.org/api/idtoken"
)

// gcpPCRAudience is the audience RSC requires in the service account ownership
// token.
const gcpPCRAudience = "https://rubrik.com/exocompute/pcr/gcp"

const gcpPrivateContainerRegistryTmpl = `
provider "rubrik" {
	credentials = "{{ .Provider.Credentials }}"
}

variable "service_account_email" {
	type = string
}

variable "service_account_ownership_token" {
	type      = string
	sensitive = true
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

resource "rubrik_gcp_private_container_registry" "default" {
	cloud_account_id                = rubrik_gcp_project.default.id
	service_account_email           = var.service_account_email
	service_account_ownership_token = var.service_account_ownership_token
	url                             = var.url
}
`

// gcpPCRToken returns the email of the service account in the GCP credentials
// file and a Google OIDC identity token for the service account, with the
// audience RSC requires.
func gcpPCRToken(t *testing.T) (email, token string) {
	t.Helper()

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

	source, err := idtoken.NewTokenSource(context.Background(), gcpPCRAudience,
		idtoken.WithCredentialsFile(credentialsFile))
	if err != nil {
		t.Fatalf("failed to create GCP identity token source: %v", err)
	}
	tok, err := source.Token()
	if err != nil {
		t.Fatalf("failed to create GCP identity token: %v", err)
	}

	return key.ClientEmail, tok.AccessToken
}

// TestAccGcpPrivateContainerRegistryResource_basic verifies that a GCP private
// container registry can be created, imported and updated.
//
// The test skips unless the REL_ENABLE_GCP_PRIVATE_CONTAINER_REGISTRY feature
// flag is enabled for the RSC account, since RSC rejects the registry without
// it. It also skips unless the GCP test project file holds the URL of an
// Artifact Registry repository in the pcr.registryUrl field. The service account
// in the GCP credentials is the service account registered with RSC.
func TestAccGcpPrivateContainerRegistryResource_basic(t *testing.T) {
	skipUnlessFeatureEnabled(t, core.FeatureFlagGCPPrivateContainerRegistry)

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

	email, token := gcpPCRToken(t)
	variables := func(url string) config.Variables {
		return config.Variables{
			"service_account_email":           config.StringVariable(email),
			"service_account_ownership_token": config.StringVariable(token),
			"url":                             config.StringVariable(url),
		}
	}

	const name = "rubrik_gcp_private_container_registry.default"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			// The service account ownership token is a write-only attribute.
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		Steps: []resource.TestStep{{
			Config:          tfConfig,
			ConfigVariables: variables(url),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair(name, keyID, "rubrik_gcp_project.default", keyID),
				resource.TestCheckResourceAttrPair(name, keyCloudAccountID, "rubrik_gcp_project.default", keyID),
				resource.TestCheckResourceAttr(name, keyServiceAccountEmail, email),
				resource.TestCheckResourceAttr(name, keyURL, url),
				// Write-only values are never stored.
				resource.TestCheckNoResourceAttr(name, keyServiceAccountOwnershipToken),
			),
		}, {
			Config:                  tfConfig,
			ConfigVariables:         variables(url),
			ResourceName:            name,
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{keyServiceAccountOwnershipToken},
		}, {
			Config:          tfConfig,
			ConfigVariables: variables(updatedURL),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(name, keyServiceAccountEmail, email),
				resource.TestCheckResourceAttr(name, keyURL, updatedURL),
			),
		}},
	})
}
