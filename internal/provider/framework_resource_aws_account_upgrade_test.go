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

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAWSAccountStateV0 is v0 state. The ID is the RSC cloud account ID and
// the AWS account ID separated by a colon.
const testAWSAccountStateV0 = `{
	"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b:123456789012",
	"delete_snapshots_on_destroy": false,
	"name": "name",
	"profile": "default",
	"regions": ["us-east-2"]
}`

// testAWSAccountStateV0Upgraded is testAWSAccountStateV0 upgraded to the latest
// version. Read fills in the permission groups, stack ARN and status.
const testAWSAccountStateV0Upgraded = `{
	"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
	"cloud_native_protection": [{"regions": ["us-east-2"]}],
	"delete_snapshots_on_destroy": false,
	"name": "name",
	"profile": "default"
}`

// testAWSAccountStateV1 is v1 state.
const testAWSAccountStateV1 = `{
	"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
	"delete_snapshots_on_destroy": false,
	"exocompute": [{"regions": ["us-west-2"]}],
	"name": "name",
	"profile": "default",
	"regions": ["us-east-2"]
}`

// testAWSAccountStateV1Upgraded is testAWSAccountStateV1 upgraded to the latest
// version.
const testAWSAccountStateV1Upgraded = `{
	"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
	"cloud_native_protection": [{"regions": ["us-east-2"]}],
	"delete_snapshots_on_destroy": false,
	"exocompute": [{"regions": ["us-west-2"]}],
	"name": "name",
	"profile": "default"
}`

// testAWSAccountStateV2 is v2 state, which stored unset strings as empty
// strings. Blocks left out are null.
const testAWSAccountStateV2 = `{
	"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
	"assume_role": "",
	"cloud_native_protection": [{
		"permission_groups": ["BASIC"],
		"regions": ["us-east-2"],
		"stack_arn": "arn:aws:cloudformation:us-east-2:123456789012:stack/rubrik/1",
		"status": "connected"
	}],
	"delete_snapshots_on_destroy": false,
	"name": "name",
	"outpost": [{
		"outpost_account_id": "",
		"outpost_account_profile": "",
		"permission_groups": ["BASIC"],
		"stack_arn": "arn:aws:cloudformation:us-east-1:123456789012:stack/rubrik-outpost/1",
		"status": "connected"
	}],
	"permissions": "",
	"profile": "default",
	"role_chaining_account_id": ""
}`

// testAWSAccountStateV2Upgraded is testAWSAccountStateV2 upgraded to the latest
// version, with the empty strings null.
const testAWSAccountStateV2Upgraded = `{
	"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
	"cloud_native_protection": [{
		"permission_groups": ["BASIC"],
		"regions": ["us-east-2"],
		"stack_arn": "arn:aws:cloudformation:us-east-2:123456789012:stack/rubrik/1",
		"status": "connected"
	}],
	"delete_snapshots_on_destroy": false,
	"name": "name",
	"outpost": [{
		"permission_groups": ["BASIC"],
		"stack_arn": "arn:aws:cloudformation:us-east-1:123456789012:stack/rubrik-outpost/1",
		"status": "connected"
	}],
	"profile": "default"
}`

// TestAWSAccountMoveState moves state to the rubrik_aws_account resource
// through the provider server, and checks that only state from the
// polaris_aws_account resource is moved.
func TestAWSAccountMoveState(t *testing.T) {
	server := providerserver.NewProtocol6(&FrameworkProvider{Version: "test"})()
	stateType := awsAccountTestSchema(t).Type().TerraformType(t.Context())

	tests := []struct {
		name     string
		provider string
		typeName string
		version  int64
		source   string
		want     string
		wantErr  string
	}{{
		name:     "V0",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  0,
		source:   testAWSAccountStateV0,
		want:     testAWSAccountStateV0Upgraded,
	}, {
		name:     "V0MissingAWSAccountID",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  0,
		source:   `{"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b"}`,
		wantErr:  "State upgrade failed",
	}, {
		name:     "V0InvalidCloudAccountID",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  0,
		source:   `{"id": "default:123456789012"}`,
		wantErr:  "State upgrade failed",
	}, {
		name:     "V1",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  1,
		source:   testAWSAccountStateV1,
		want:     testAWSAccountStateV1Upgraded,
	}, {
		name:     "V1NoExocompute",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  1,
		source:   `{"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b", "exocompute": [], "regions": ["us-east-2"]}`,
		want:     `{"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b", "cloud_native_protection": [{"regions": ["us-east-2"]}]}`,
	}, {
		name:     "V2",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  2,
		source:   testAWSAccountStateV2,
		want:     testAWSAccountStateV2Upgraded,
	}, {
		// State of the polaris_aws_account alias, written by a version of this
		// provider before the resource was migrated to the framework.
		name:     "V2FromRubrik",
		provider: "registry.terraform.io/rubrikinc/rubrik",
		typeName: "polaris_aws_account",
		version:  2,
		source:   testAWSAccountStateV2,
		want:     testAWSAccountStateV2Upgraded,
	}, {
		// State of the polaris_aws_account alias of this provider, which is
		// moved as it is.
		name:     "LatestFromRubrik",
		provider: "registry.terraform.io/rubrikinc/rubrik",
		typeName: "polaris_aws_account",
		version:  3,
		source:   testAWSAccountStateV2Upgraded,
		want:     testAWSAccountStateV2Upgraded,
	}, {
		name:     "OtherProvider",
		provider: "registry.terraform.io/hashicorp/aws",
		typeName: "polaris_aws_account",
		version:  2,
		source:   testAWSAccountStateV2,
		wantErr:  "Unable to Move Resource State",
	}, {
		name:     "OtherType",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_cnp_account",
		version:  2,
		source:   testAWSAccountStateV2,
		wantErr:  "Unable to Move Resource State",
	}, {
		name:     "OtherVersion",
		provider: "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  4,
		source:   testAWSAccountStateV2,
		wantErr:  "Unable to Move Resource State",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := server.MoveResourceState(t.Context(), &tfprotov6.MoveResourceStateRequest{
				SourceProviderAddress: tc.provider,
				SourceTypeName:        tc.typeName,
				SourceSchemaVersion:   tc.version,
				SourceState:           &tfprotov6.RawState{JSON: []byte(tc.source)},
				TargetTypeName:        "rubrik_aws_account",
			})
			if err != nil {
				t.Fatal(err)
			}

			var gotErr bool
			for _, d := range res.Diagnostics {
				if d.Severity != tfprotov6.DiagnosticSeverityError {
					continue
				}
				if d.Summary != tc.wantErr {
					t.Fatalf("%s: %s", d.Summary, d.Detail)
				}
				gotErr = true
			}
			if tc.wantErr != "" && !gotErr {
				t.Fatalf("got no error, want %q", tc.wantErr)
			}
			if gotErr {
				return
			}

			got, err := res.TargetState.Unmarshal(stateType)
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := got.Diff(awsAccountTestValue(t, tc.want))
			if err != nil {
				t.Fatal(err)
			}
			for _, diff := range diffs {
				t.Errorf("%s: got %v, want %v", diff.Path, diff.Value1, diff.Value2)
			}
		})
	}
}

// TestAWSAccountUpgradeState upgrades state through the provider server. The
// v0 state upgrader looks the account up in RSC, so the v0 conversion is
// covered by the v0 move.
func TestAWSAccountUpgradeState(t *testing.T) {
	server := providerserver.NewProtocol6(&FrameworkProvider{Version: "test"})()
	stateType := awsAccountTestSchema(t).Type().TerraformType(t.Context())

	tests := []struct {
		name    string
		version int64
		prior   string
		want    string
	}{{
		name:    "V1",
		version: 1,
		prior:   testAWSAccountStateV1,
		want:    testAWSAccountStateV1Upgraded,
	}, {
		name:    "V2",
		version: 2,
		prior:   testAWSAccountStateV2,
		want:    testAWSAccountStateV2Upgraded,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := server.UpgradeResourceState(t.Context(), &tfprotov6.UpgradeResourceStateRequest{
				TypeName: "rubrik_aws_account",
				Version:  tc.version,
				RawState: &tfprotov6.RawState{JSON: []byte(tc.prior)},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range res.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					t.Fatal(d.Summary + ": " + d.Detail)
				}
			}

			got, err := res.UpgradedState.Unmarshal(stateType)
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := got.Diff(awsAccountTestValue(t, tc.want))
			if err != nil {
				t.Fatal(err)
			}
			for _, diff := range diffs {
				t.Errorf("%s: got %v, want %v", diff.Path, diff.Value1, diff.Value2)
			}
		})
	}
}
