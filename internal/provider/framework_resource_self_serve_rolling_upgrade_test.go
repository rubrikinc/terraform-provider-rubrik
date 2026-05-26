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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSelfServeRollingUpgradeResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Enable the setting.
			Config: `
				resource "rubrik_self_serve_rolling_upgrade" "account" {
					enabled = true
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_self_serve_rolling_upgrade.account",
					tfjsonpath.New(keyID), knownvalue.StringExact(selfServeRollingUpgradeID)),
				statecheck.ExpectKnownValue("rubrik_self_serve_rolling_upgrade.account",
					tfjsonpath.New(keyEnabled), knownvalue.Bool(true)),
			},
		}, {
			// Toggle off via update.
			Config: `
				resource "rubrik_self_serve_rolling_upgrade" "account" {
					enabled = false
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_self_serve_rolling_upgrade.account",
					tfjsonpath.New(keyEnabled), knownvalue.Bool(false)),
			},
		}, {
			// Import: the import ID is discarded; the singleton ID is set
			// in state and Read populates enabled.
			ResourceName:      "rubrik_self_serve_rolling_upgrade.account",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportStateId:     "ignored",
			ImportState:       true,
			ImportStateVerify: true,
		}},
	})
}
