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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// slaDomainPlanConfig returns the configuration of an SLA domain resource with
// the given body. The daily schedule is added unless the body has a schedule.
func slaDomainPlanConfig(name, body string) string {
	return `
		resource "rubrik_sla_domain" "test" {
			name = "` + name + `"
` + body + `
		}`
}

const slaDomainPlanDaily = `
			daily_schedule {
				frequency = 1
				retention = 7
			}`

// TestAccSLADomainResource_PlanErrors checks the rules of the SLA domain which
// are reported when planning, through the provider server. Every configuration
// is rejected before anything is created in RSC. TestAccSLADomainResource_PlanOnly
// plans a valid configuration, so these can't pass because everything is
// rejected.
func TestAccSLADomainResource_PlanErrors(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	tests := []struct {
		name        string
		body        string
		expectError string
	}{{
		name:        "no snapshot schedule",
		body:        `object_types = ["VSPHERE_OBJECT_TYPE"]`,
		expectError: `(?s)At least one of these attributes must be configured.*daily_schedule`,
	}, {
		name:        "an object type which doesn't exist",
		body:        `object_types = ["NO_SUCH_OBJECT_TYPE"]` + slaDomainPlanDaily,
		expectError: `NO_SUCH_OBJECT_TYPE`,
	}, {
		name:        "two daily schedules",
		body:        `object_types = ["VSPHERE_OBJECT_TYPE"]` + slaDomainPlanDaily + slaDomainPlanDaily,
		expectError: `(?s)daily_schedule.*(at most 1|no more than 1)`,
	}, {
		name: "Azure Postgres config for another object type",
		body: `object_types = ["VSPHERE_OBJECT_TYPE"]` + slaDomainPlanDaily + `
			azure_postgres_flexible_server_config {
				backup_retention_in_days = 7
			}`,
		expectError: `azure_postgres_flexible_server_config is only valid when object_types is`,
	}, {
		name: "AWS Config with a minute schedule",
		body: `object_types = ["AWS_CONFIG_OBJECT_TYPE"]
			minute_schedule {
				frequency = 15
				retention = 7
			}`,
		expectError: `the AWS Config object type does not support a minute schedule`,
	}, {
		name:        "GCP BigQuery without a backup location",
		body:        `object_types = ["GCP_BIGQUERY_OBJECT_TYPE"]` + slaDomainPlanDaily,
		expectError: `the GCP BigQuery object type requires a backup_location`,
	}}

	var steps []resource.TestStep
	for _, tt := range tests {
		steps = append(steps, resource.TestStep{
			Config:      slaDomainPlanConfig("plan-errors", tt.body),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(tt.expectError),
		})
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps:                    steps,
	})
}

// TestAccSLADomainResource_PlanOnly plans a valid SLA domain. Nothing is created,
// since the step is plan only, and it expects the plan to have changes.
func TestAccSLADomainResource_PlanOnly(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:             slaDomainPlanConfig("plan-only", `object_types = ["VSPHERE_OBJECT_TYPE"]`+slaDomainPlanDaily),
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
		}},
	})
}

// TestAccSLADomainResource creates, updates and imports an SLA domain with a
// daily and a weekly schedule. The import steps expect an empty plan, which
// checks that the optional attributes which aren't configured, such as the day
// of the week of a weekly schedule, are null after an import.
func TestAccSLADomainResource(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	name := testUniqueName(t)
	conf := `
		variable "name" {
			type = string
		}
		variable "daily_retention" {
			type = number
		}
		resource "rubrik_sla_domain" "test" {
			name         = var.name
			description  = "Acceptance test SLA domain"
			object_types = ["VSPHERE_OBJECT_TYPE"]

			daily_schedule {
				frequency = 1
				retention = var.daily_retention
			}

			weekly_schedule {
				frequency = 1
				retention = 4
			}
		}
	`
	variables := func(dailyRetention int) config.Variables {
		return config.Variables{
			"name":            config.StringVariable(name),
			"daily_retention": config.IntegerVariable(dailyRetention),
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             slaDomainCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the SLA domain can be created.
			Config:          conf,
			ConfigVariables: variables(7),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_sla_domain.test", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_sla_domain.test", tfjsonpath.New(keyName), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_sla_domain.test", tfjsonpath.New(keyDailySchedule).AtSliceIndex(0).
					AtMapKey(keyRetention), knownvalue.Int64Exact(7)),
				statecheck.ExpectKnownValue("rubrik_sla_domain.test", tfjsonpath.New(keyWeeklySchedule), knownvalue.ListSizeExact(1)),
				statecheck.ExpectIdentity("rubrik_sla_domain.test", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_sla_domain.test", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the schedule is updated in place, and that the plan
			// after the update is empty.
			Config:          conf,
			ConfigVariables: variables(14),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_sla_domain.test", plancheck.ResourceActionUpdate),
				},
				PostApplyPostRefresh: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_sla_domain.test", tfjsonpath.New(keyDailySchedule).AtSliceIndex(0).
					AtMapKey(keyRetention), knownvalue.Int64Exact(14)),
				statecheck.ExpectIdentityValueMatchesState("rubrik_sla_domain.test", tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_sla_domain.test",
			ConfigVariables:   variables(14),
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_sla_domain.test",
			ConfigVariables: variables(14),
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_sla_domain.test",
			ConfigVariables: variables(14),
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
