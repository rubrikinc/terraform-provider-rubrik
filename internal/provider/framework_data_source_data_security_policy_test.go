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
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccDataSecurityPolicyDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             dataSecurityPolicyCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the data source can look up the policy by ID and
			// name.
			Config: `
				resource "rubrik_data_security_policy" "test" {
					name        = "Terraform Test DS Policy"
					description = "Data source acceptance test: Delete Me!"
					category    = "OVEREXPOSED"
					severity    = "MEDIUM"

					object_filter {
						op = "AND"
						condition {
							filter_type  = "SECURITY_DOCUMENT_SENSITIVITY"
							relationship = "IS"
							values       = ["HIGH"]
						}
					}

					identity_filter {
						op = "OR"
						condition {
							filter_type  = "SECURITY_IDENTITY_NAME"
							relationship = "CONTAINS"
							values       = ["svc-"]
						}
					}

					threshold_filter {
						filter_type  = "SECURITY_DOCUMENT_HIT_COUNT"
						relationship = "GREATER_THAN"
						values       = ["5"]
					}
				}

				data "rubrik_data_security_policy" "by_id" {
					policy_id = rubrik_data_security_policy.test.id
				}

				data "rubrik_data_security_policy" "by_name" {
					name = rubrik_data_security_policy.test.name
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// Resource.
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyID),
					NonNullUUID()),

				// By ID.
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyID),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyID),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyPolicyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyName),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyDescription),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyDescription),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyCategory),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyCategory),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keySeverity),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keySeverity),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyEnabled),
					knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyPredefined),
					knownvalue.Bool(false)),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyObjectFilter),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyObjectFilter),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyIdentityFilter),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyIdentityFilter),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyThresholdFilter),
					"data.rubrik_data_security_policy.by_id", tfjsonpath.New(keyThresholdFilter),
					compare.ValuesSame()),

				// By Name.
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyID),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyID),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyName),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyName),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyDescription),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyDescription),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyCategory),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyCategory),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keySeverity),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keySeverity),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyEnabled),
					knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyPredefined),
					knownvalue.Bool(false)),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyObjectFilter),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyObjectFilter),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyIdentityFilter),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyIdentityFilter),
					compare.ValuesSame()),
				statecheck.CompareValuePairs(
					"rubrik_data_security_policy.test", tfjsonpath.New(keyThresholdFilter),
					"data.rubrik_data_security_policy.by_name", tfjsonpath.New(keyThresholdFilter),
					compare.ValuesSame()),
			},
		}},
	})
}
