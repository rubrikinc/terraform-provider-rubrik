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

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccSLADomainDataSource_PlanErrors checks the rules of the lookup of the SLA
// domain data source, through the provider server. Every configuration is
// rejected before anything is read from RSC.
func TestAccSLADomainDataSource_PlanErrors(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	tests := []struct {
		name        string
		body        string
		expectError string
	}{{
		name:        "neither the id nor the name",
		body:        ``,
		expectError: `No attribute specified when one \(and only one\) of \[id,name\] is required`,
	}, {
		name:        "both the id and the name",
		body:        `id = "0e55e625-b78d-4e83-87f3-90313a980211"` + "\n" + `name = "Gold"`,
		expectError: `2 attributes specified when one \(and only one\) of \[id,name\] is required`,
	}, {
		name:        "an id which isn't a UUID",
		body:        `id = "not-a-uuid"`,
		expectError: `"not-a-uuid" is not a valid UUID`,
	}, {
		name:        "a name which is white space",
		body:        `name = "  "`,
		expectError: `(?s)Invalid Value.*name`,
	}}

	var steps []resource.TestStep
	for _, tt := range tests {
		steps = append(steps, resource.TestStep{
			Config: `
				data "rubrik_sla_domain" "test" {
					` + tt.body + `
				}`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(tt.expectError),
		})
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps:                    steps,
	})
}

// TestAccSLADomainDataSource reads a predefined SLA domain, which every RSC
// account has, by name and by ID. It only reads, and creates nothing.
func TestAccSLADomainDataSource(t *testing.T) {
	skipUnlessAcceptanceTest(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
				data "rubrik_sla_domain" "by_name" {
					name = "Gold"
				}

				data "rubrik_sla_domain" "by_id" {
					id = data.rubrik_sla_domain.by_name.id
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				// The predefined SLA domains have fixed IDs, and the ID of Gold is
				// the nil UUID, which NonNullUUID rejects.
				statecheck.ExpectKnownValue("data.rubrik_sla_domain.by_name", tfjsonpath.New(keyID),
					knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))),
				statecheck.ExpectKnownValue("data.rubrik_sla_domain.by_name", tfjsonpath.New(keyName), knownvalue.StringExact("Gold")),
				statecheck.ExpectKnownValue("data.rubrik_sla_domain.by_id", tfjsonpath.New(keyName), knownvalue.StringExact("Gold")),
				statecheck.CompareValuePairs("data.rubrik_sla_domain.by_name", tfjsonpath.New(keyID),
					"data.rubrik_sla_domain.by_id", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.CompareValuePairs("data.rubrik_sla_domain.by_name", tfjsonpath.New(keyObjectTypes),
					"data.rubrik_sla_domain.by_id", tfjsonpath.New(keyObjectTypes), compare.ValuesSame()),
				// The blocks which the SDKv2 data source never filled in are
				// empty lists, and not null, so length() of them is 0.
				statecheck.ExpectKnownValue("data.rubrik_sla_domain.by_name", tfjsonpath.New(keyRetentionLock), knownvalue.ListSizeExact(0)),
				statecheck.ExpectKnownValue("data.rubrik_sla_domain.by_name", tfjsonpath.New(keyOracleConfig), knownvalue.ListSizeExact(0)),
			},
		}},
	})
}
