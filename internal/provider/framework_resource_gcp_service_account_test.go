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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccGcpServiceAccountResource sets the GCP default service account with
// the generated default name.
//
// The RSC GraphQL API appears to cache the service account name in the API
// server, so the wrong name might be returned depending on which API server
// instance handles the request. Therefore, the name in RSC isn't verified, and
// changes to it are ignored. The resource has no import, see the resource.
//
// None of the tests in this file have a CheckDestroy. Destroying the resource
// only removes it from state, the default service account can't be removed
// from RSC, so there is nothing to check.
func TestAccGcpServiceAccountResource(t *testing.T) {
	credentials := testGCPCredentials(t)

	// Verify that the ID is a SHA-256 hash when the generated default name is
	// used, the generated name can't be read back reliably.
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				variable "credentials" {
					type      = string
					sensitive = true
				}

				resource "rubrik_gcp_service_account" "default" {
					credentials = var.credentials

					lifecycle {
						ignore_changes = [name]
					}
				}
			`,
			ConfigVariables: config.Variables{
				"credentials": config.StringVariable(credentials),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_service_account.default", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(sha256Hex)),
				statecheck.ExpectKnownValue("rubrik_gcp_service_account.default", tfjsonpath.New(keyCredentials),
					knownvalue.StringExact(credentials)),
			},
		}},
	})
}

// TestAccGcpServiceAccountResourceStateUpgrade verifies that v0 state, written
// by polaris provider versions before v1.3.0 where the ID is the service
// account name, is upgraded to v1 state, where the ID is the SHA-256 hash of
// the service account name.
//
// The v0 resource only accepts the path to a service account key file, and it
// sends the name to RSC, which only accepts the client email of the key as the
// name, so the client email is used.
func TestAccGcpServiceAccountResourceStateUpgrade(t *testing.T) {
	credentials := testGCPCredentials(t)

	// The credentials are a base64 encoded key or the path to a key file, as
	// for the provider. The v0 resource needs a key file.
	keyFile := credentials
	keyData, err := base64.StdEncoding.DecodeString(credentials)
	if err == nil {
		keyFile = filepath.Join(t.TempDir(), "key.json")
		if err := os.WriteFile(keyFile, keyData, 0o600); err != nil {
			t.Fatalf("failed to write GCP service account key file: %v", err)
		}
	} else {
		keyData, err = os.ReadFile(credentials)
		if err != nil {
			t.Fatalf("GCP credentials are neither a base64 encoded key nor a readable key file: %v", err)
		}
	}
	var key struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(keyData, &key); err != nil {
		t.Fatalf("failed to parse GCP service account key: %v", err)
	}
	if key.ClientEmail == "" {
		t.Fatal("GCP credentials are not a service account key, client_email is missing")
	}

	const conf = `
		variable "rsc_credentials" {
			type      = string
			sensitive = true
		}
		variable "credentials" {
			type      = string
			sensitive = true
		}
		variable "name" {
			type = string
		}

		provider "polaris" {
			credentials = var.rsc_credentials
		}

		resource "polaris_gcp_service_account" "default" {
			credentials = var.credentials
			name        = var.name

			lifecycle {
				ignore_changes = [name]
			}
		}
	`
	vars := config.Variables{
		"rsc_credentials": config.StringVariable(testCredentials(t)),
		"credentials":     config.StringVariable(keyFile),
		"name":            config.StringVariable(key.ClientEmail),
	}

	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{{
			// Set the default service account with the last polaris provider
			// version writing v0 state. The ID is the service account name.
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.2.1",
				},
			},
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_service_account.default", tfjsonpath.New(keyID),
					knownvalue.StringExact(key.ClientEmail)),
			},
		}, {
			// Verify that the upgrade gives an empty plan and changes the ID to
			// a SHA-256 hash. The hash is of the name read from RSC, which
			// can't be verified, see TestAccGcpServiceAccountResource.
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables:          vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_service_account.default", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(sha256Hex)),
			},
		}},
	})
}

// TestAccGcpServiceAccountResourceMoveState verifies that state from a
// polaris_gcp_service_account resource created by the rubrikinc/polaris
// provider can be moved to a rubrik_gcp_service_account resource using the
// moved {} block.
//
// The polaris provider sends the name to RSC, and RSC only accepts the client
// email of the service account key as the name, so the client email is used.
func TestAccGcpServiceAccountResourceMoveState(t *testing.T) {
	credentials := testGCPCredentials(t)

	// The credentials are a base64 encoded key or the path to a key file, as
	// for the provider.
	keyData, err := base64.StdEncoding.DecodeString(credentials)
	if err != nil {
		keyData, err = os.ReadFile(credentials)
		if err != nil {
			t.Fatalf("GCP credentials are neither a base64 encoded key nor a readable key file: %v", err)
		}
	}
	var key struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(keyData, &key); err != nil {
		t.Fatalf("failed to parse GCP service account key: %v", err)
	}
	if key.ClientEmail == "" {
		t.Fatal("GCP credentials are not a service account key, client_email is missing")
	}

	vars := config.Variables{
		"rsc_credentials": config.StringVariable(testCredentials(t)),
		"credentials":     config.StringVariable(credentials),
		"name":            config.StringVariable(key.ClientEmail),
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{{
			// Set the default service account with the polaris provider.
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.11.2",
				},
			},
			Config: `
				variable "rsc_credentials" {
					type      = string
					sensitive = true
				}
				variable "credentials" {
					type      = string
					sensitive = true
				}
				variable "name" {
					type = string
				}

				provider "polaris" {
					credentials = var.rsc_credentials
				}

				resource "polaris_gcp_service_account" "default" {
					credentials = var.credentials
					name        = var.name

					lifecycle {
						ignore_changes = [name]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_gcp_service_account.default", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(sha256Hex)),
			},
		}, {
			// Verify that the move gives an empty plan and keeps the ID.
			ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
				"rubrik": newMuxedProviderServer,
			},
			Config: `
				variable "rsc_credentials" {
					type      = string
					sensitive = true
				}
				variable "credentials" {
					type      = string
					sensitive = true
				}
				variable "name" {
					type = string
				}

				moved {
					from = polaris_gcp_service_account.default
					to   = rubrik_gcp_service_account.default
				}

				resource "rubrik_gcp_service_account" "default" {
					credentials = var.credentials
					name        = var.name

					lifecycle {
						ignore_changes = [name]
					}
				}
			`,
			ConfigVariables: vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_gcp_service_account.default", tfjsonpath.New(keyID),
					knownvalue.StringExact(fmt.Sprintf("%x", sha256.Sum256([]byte(key.ClientEmail))))),
			},
		}},
	})
}
