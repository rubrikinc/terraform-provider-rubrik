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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// conditionGroupCheck builds the state check for a condition group block.
func conditionGroupCheck(op string, conditions ...knownvalue.Check) knownvalue.Check {
	return knownvalue.ListExact([]knownvalue.Check{
		knownvalue.ObjectExact(map[string]knownvalue.Check{
			keyOp:        knownvalue.StringExact(op),
			keyCondition: knownvalue.ListExact(conditions),
		}),
	})
}

// conditionCheck builds the state check for a single filter condition.
func conditionCheck(filterType, relationship string, values ...string) knownvalue.Check {
	valueChecks := make([]knownvalue.Check, 0, len(values))
	for _, value := range values {
		valueChecks = append(valueChecks, knownvalue.StringExact(value))
	}

	return knownvalue.ObjectExact(map[string]knownvalue.Check{
		keyFilterType:   knownvalue.StringExact(filterType),
		keyRelationship: knownvalue.StringExact(relationship),
		keyValues:       knownvalue.ListExact(valueChecks),
	})
}

func TestAccDataSecurityPolicyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             dataSecurityPolicyCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with a single object
			// condition group.
			Config: `
				resource "rubrik_data_security_policy" "test" {
					name        = "Terraform Test Policy"
					description = "Acceptance test: Delete Me!"
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
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyName),
					knownvalue.StringExact("Terraform Test Policy")),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Acceptance test: Delete Me!")),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyCategory),
					knownvalue.StringExact("OVEREXPOSED")),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keySeverity),
					knownvalue.StringExact("MEDIUM")),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyEnabled),
					knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyPredefined),
					knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyObjectFilter),
					conditionGroupCheck("AND",
						conditionCheck("SECURITY_DOCUMENT_SENSITIVITY", "IS", "HIGH"))),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyIdentityFilter),
					knownvalue.ListExact([]knownvalue.Check{})),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyThresholdFilter),
					knownvalue.ListExact([]knownvalue.Check{})),
				statecheck.ExpectIdentity("rubrik_data_security_policy.test", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_data_security_policy.test", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that an identity condition group and a threshold filter
			// can be added, and that the object group can be extended.
			Config: `
				resource "rubrik_data_security_policy" "test" {
					name        = "Terraform Test Policy Updated"
					description = "Acceptance test updated: Delete Me!"
					category    = "OVEREXPOSED"
					severity    = "HIGH"

					object_filter {
						op = "OR"
						condition {
							filter_type  = "SECURITY_DOCUMENT_SENSITIVITY"
							relationship = "IS"
							values       = ["HIGH", "MEDIUM"]
						}
						condition {
							filter_type  = "SECURITY_SNAPPABLE_NAME"
							relationship = "CONTAINS"
							values       = ["prod"]
						}
					}

					identity_filter {
						op = "AND"
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
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyName),
					knownvalue.StringExact("Terraform Test Policy Updated")),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Acceptance test updated: Delete Me!")),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keySeverity),
					knownvalue.StringExact("HIGH")),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyObjectFilter),
					conditionGroupCheck("OR",
						conditionCheck("SECURITY_DOCUMENT_SENSITIVITY", "IS", "HIGH", "MEDIUM"),
						conditionCheck("SECURITY_SNAPPABLE_NAME", "CONTAINS", "prod"))),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyIdentityFilter),
					conditionGroupCheck("AND",
						conditionCheck("SECURITY_IDENTITY_NAME", "CONTAINS", "svc-"))),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyThresholdFilter),
					knownvalue.ListExact([]knownvalue.Check{
						conditionCheck("SECURITY_DOCUMENT_HIT_COUNT", "GREATER_THAN", "5"),
					})),
				statecheck.ExpectIdentity("rubrik_data_security_policy.test", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_data_security_policy.test", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the threshold filter and the identity condition group
			// can be removed. Clearing the threshold filter requires the update
			// to ask RSC to honor the nil value.
			Config: `
				resource "rubrik_data_security_policy" "test" {
					name        = "Terraform Test Policy Updated"
					description = "Acceptance test updated: Delete Me!"
					category    = "OVEREXPOSED"
					severity    = "HIGH"

					object_filter {
						op = "OR"
						condition {
							filter_type  = "SECURITY_DOCUMENT_SENSITIVITY"
							relationship = "IS"
							values       = ["HIGH", "MEDIUM"]
						}
						condition {
							filter_type  = "SECURITY_SNAPPABLE_NAME"
							relationship = "CONTAINS"
							values       = ["prod"]
						}
					}
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyIdentityFilter),
					knownvalue.ListExact([]knownvalue.Check{})),
				statecheck.ExpectKnownValue("rubrik_data_security_policy.test", tfjsonpath.New(keyThresholdFilter),
					knownvalue.ListExact([]knownvalue.Check{})),
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_data_security_policy.test",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_data_security_policy.test",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_data_security_policy.test",
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
