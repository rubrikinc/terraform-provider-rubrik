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
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProviderCredentialsInEnv(t *testing.T) {
	credentials := testCredentials(t)

	// Clear the legacy RUBRIK_POLARIS_ prefixed service account variables so
	// that the provider cannot fall back on these.
	t.Setenv("RUBRIK_POLARIS_SERVICEACCOUNT_FILE", "")
	t.Setenv("RUBRIK_POLARIS_SERVICEACCOUNT_CREDENTIALS", "")
	t.Setenv("RUBRIK_POLARIS_SERVICEACCOUNT_NAME", "")

	// Valid service account in RUBRIK_SERVICEACCOUNT_FILE.
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyID),
					knownvalue.StringExact("00000000-0000-0000-0000-000000000000")),
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyName),
					knownvalue.StringExact("Administrator")),
			},
		}},
	})

	// Non-existing service account in RUBRIK_SERVICEACCOUNT_FILE.
	t.Setenv("RUBRIK_SERVICEACCOUNT_FILE", "03147711-359c-40fd-b635-69619fcf374d")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ExpectError: regexp.MustCompile("(?s)^.*Error: RSC client error.*service account file and env.*$"),
		}},
	})

	// Valid service account in RUBRIK_SERVICEACCOUNT_CREDENTIALS.
	t.Setenv("RUBRIK_SERVICEACCOUNT_FILE", "")
	t.Setenv("RUBRIK_SERVICEACCOUNT_CREDENTIALS", credentials)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyID),
					knownvalue.StringExact("00000000-0000-0000-0000-000000000000")),
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyName),
					knownvalue.StringExact("Administrator")),
			},
		}},
	})

	// Invalid service account in RUBRIK_SERVICEACCOUNT_CREDENTIALS.
	t.Setenv("RUBRIK_SERVICEACCOUNT_CREDENTIALS", "invalid")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ExpectError: regexp.MustCompile("(?s)^.*Error: RSC client error.*service account file and env.*$"),
		}},
	})

	// Partial service account in env. This could happen if the service account
	// is given in parts and one of the parts is missing.
	t.Setenv("RUBRIK_SERVICEACCOUNT_CREDENTIALS", "")
	t.Setenv("RUBRIK_SERVICEACCOUNT_NAME", "name")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ExpectError: regexp.MustCompile("(?s)^.*Error: Failed to configure provider.*invalid service account client id.*$"),
		}},
	})

	// No service account in env. This could happen if the provider is used to
	// bootstrap a CDM cluster without RSC credentials, but an RSC resource is
	// used.
	t.Setenv("RUBRIK_SERVICEACCOUNT_NAME", "")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ExpectError: regexp.MustCompile("(?s)^.*Error: RSC client error.*service account file and env.*$"),
		}},
	})
}

// TestAccProviderCredentialsInEnvFallback verifies that the provider falls back
// to the legacy RUBRIK_POLARIS_ prefixed environment variables when the RUBRIK_
// prefixed variables are not defined.
func TestAccProviderCredentialsInEnvFallback(t *testing.T) {
	credentials := testCredentials(t)
	file := os.Getenv("RUBRIK_SERVICEACCOUNT_FILE")

	// Clear the RUBRIK_ prefixed service account variables, forcing the
	// provider to fall back on the legacy RUBRIK_POLARIS_ prefixed variables.
	t.Setenv("RUBRIK_SERVICEACCOUNT_FILE", "")
	t.Setenv("RUBRIK_SERVICEACCOUNT_CREDENTIALS", "")

	// Valid service account via the RUBRIK_POLARIS_SERVICEACCOUNT_FILE fallback.
	t.Setenv("RUBRIK_POLARIS_SERVICEACCOUNT_FILE", file)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyID),
					knownvalue.StringExact("00000000-0000-0000-0000-000000000000")),
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyName),
					knownvalue.StringExact("Administrator")),
			},
		}},
	})

	// Valid service account via the RUBRIK_POLARIS_SERVICEACCOUNT_CREDENTIALS
	// fallback.
	t.Setenv("RUBRIK_POLARIS_SERVICEACCOUNT_FILE", "")
	t.Setenv("RUBRIK_POLARIS_SERVICEACCOUNT_CREDENTIALS", credentials)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_role" "admin" {
					name = "Administrator"
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyID),
					knownvalue.StringExact("00000000-0000-0000-0000-000000000000")),
				statecheck.ExpectKnownValue("data.rubrik_role.admin", tfjsonpath.New(keyName),
					knownvalue.StringExact("Administrator")),
			},
		}},
	})
}
