// Copyright 2021 Rubrik, Inc.
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
	"maps"
	"strconv"
	"testing"

	fwdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

var awsExocomputeTmpl = `
provider "polaris" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "polaris_aws_account" "default" {
	name    = "{{ .Resource.AccountName }}"
	profile = "{{ .Resource.Profile }}"

	cloud_native_protection {
		permission_groups = [
			"BASIC",
		]
		regions = [
			"us-east-2",
		]
	}
  
	exocompute {
		permission_groups = [
			"BASIC",
			"RSC_MANAGED_CLUSTER",
		]

		regions = [
			"us-east-2",
		]
	}
}

resource "polaris_aws_exocompute" "default" {
	account_id = polaris_aws_account.default.id
	region     = "us-east-2"
	vpc_id     = "{{ .Resource.Exocompute.VPCID }}"

	subnets = [
		{{ range slice .Resource.Exocompute.Subnets 0 2 }}
		"{{ .ID }}",
		{{ end }}
	]
}
`

func TestAccPolarisAWSExocompute_basic(t *testing.T) {
	config, account := loadAWSTestConfig(t)
	exocompute, err := makeTerraformConfig(config, awsExocomputeTmpl)
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{{
			Config: exocompute,
			Check: resource.ComposeTestCheckFunc(
				// Account resource
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.AccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "profile", account.Profile),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "delete_snapshots_on_destroy", "false"),

				// Cloud Native Protection feature
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-east-2"),

				// Exocompute feature
				resource.TestCheckResourceAttr("polaris_aws_account.default", "exocompute.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "exocompute.0.regions.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "exocompute.0.regions.*", "us-east-2"),

				// Exocompute resource
				resource.TestCheckResourceAttrPair("polaris_aws_exocompute.default", "account_id", "polaris_aws_account.default", "id"),
				resource.TestCheckResourceAttr("polaris_aws_exocompute.default", "region", "us-east-2"),
				resource.TestCheckResourceAttr("polaris_aws_exocompute.default", "vpc_id", account.Exocompute.VPCID),
				resource.TestCheckResourceAttr("polaris_aws_exocompute.default", "polaris_managed", "true"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_exocompute.default", "subnets.*", account.Exocompute.Subnets[0].ID),
				resource.TestCheckTypeSetElemAttr("polaris_aws_exocompute.default", "subnets.*", account.Exocompute.Subnets[1].ID),
			),
		}},
	})
}

// TestExocomputeRunHealthCheckCustomerManaged verifies that no health check is
// triggered for a customer managed Exocompute configuration, and that this is
// reported as a warning. The client is nil, so any RSC call would panic.
func TestExocomputeRunHealthCheckCustomerManaged(t *testing.T) {
	diags := exocomputeRunHealthCheck(context.Background(), nil, core.CloudVendorAzure,
		"550e8400-e29b-41d4-a716-446655440000", true)
	if len(diags) != 1 || diags[0].Severity() != fwdiag.SeverityWarning {
		t.Fatalf("expected a single warning, got: %v", diags)
	}
}

// TestExocomputeTriggerHealthCheckDiff verifies that trigger_health_check on
// the AWS Exocompute resource is updated in place, so that changing it never
// replaces the Exocompute configuration, and that state created before the
// field existed has no diff when the field is not set.
func TestExocomputeTriggerHealthCheckDiff(t *testing.T) {
	const id = "550e8400-e29b-41d4-a716-446655440000"
	const cloudAccountID = "550e8400-e29b-41d4-a716-446655440001"

	resources := []struct {
		name     string
		resource *schema.Resource
		state    map[string]string
		config   map[string]any
	}{{
		name:     "AWS",
		resource: resourceAwsExocompute(),
		state: map[string]string{
			"id":                        id,
			"account_id":                cloudAccountID,
			"region":                    "us-east-2",
			"vpc_id":                    "vpc",
			"cluster_access":            "",
			"cluster_security_group_id": "",
			"node_security_group_id":    "",
			"polaris_managed":           "true",
			"subnet.#":                  "0",
			"subnets.#":                 "2",
			"subnets." + strconv.Itoa(schema.HashString("subnet-1")): "subnet-1",
			"subnets." + strconv.Itoa(schema.HashString("subnet-2")): "subnet-2",
		},
		config: map[string]any{
			"account_id": cloudAccountID,
			"region":     "us-east-2",
			"vpc_id":     "vpc",
			"subnets":    []any{"subnet-1", "subnet-2"},
		},
	}}

	testCases := []struct {
		name        string
		stateValue  string
		configValue any
		wantUpdate  bool
	}{{
		name: "UnsetInStateAndConfigIsNoChange",
	}, {
		name:        "UnsetToTrueUpdatesInPlace",
		configValue: true,
		wantUpdate:  true,
	}, {
		name:        "TrueToFalseUpdatesInPlace",
		stateValue:  "true",
		configValue: false,
		wantUpdate:  true,
	}}
	for _, r := range resources {
		for _, tc := range testCases {
			t.Run(r.name+tc.name, func(t *testing.T) {
				state := maps.Clone(r.state)
				if tc.stateValue != "" {
					state["trigger_health_check"] = tc.stateValue
				}
				config := maps.Clone(r.config)
				if tc.configValue != nil {
					config["trigger_health_check"] = tc.configValue
				}

				diff, err := r.resource.Diff(context.Background(), &terraform.InstanceState{ID: id, Attributes: state},
					terraform.NewResourceConfigRaw(config), nil)
				if err != nil {
					t.Fatal(err)
				}

				if !tc.wantUpdate {
					if diff != nil && !diff.Empty() {
						t.Fatalf("expected no diff, got: %v", diff)
					}
					return
				}
				if diff == nil || diff.Attributes["trigger_health_check"] == nil {
					t.Fatalf("expected trigger_health_check to change, diff: %v", diff)
				}
				if diff.RequiresNew() {
					t.Fatalf("expected an in-place update, diff: %v", diff)
				}
			})
		}
	}
}
