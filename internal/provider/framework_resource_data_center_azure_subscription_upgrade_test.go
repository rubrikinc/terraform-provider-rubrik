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
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestDataCenterAzureSubscriptionUpgradeState upgrades state through the
// provider server. Attributes left out of the JSON state are null.
func TestDataCenterAzureSubscriptionUpgradeState(t *testing.T) {
	server := providerserver.NewProtocol6(&FrameworkProvider{Version: "test"})()

	var schemaRes resource.SchemaResponse
	newDataCenterAzureSubscriptionResource().Schema(t.Context(), resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}
	stateType := schemaRes.Schema.Type().TerraformType(t.Context())

	tests := []struct {
		name    string
		version int64
		prior   string
		want    string
	}{{
		// The resource stored an unset description as an empty string.
		name:    "V0WithoutDescription",
		version: 0,
		prior: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"description": "",
			"name": "name",
			"subscription_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"tenant_id": "c4d5e6f7-a8b9-4c0d-8e1f-2a3b4c5d6e7f"
		}`,
		want: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"name": "name",
			"subscription_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"tenant_id": "c4d5e6f7-a8b9-4c0d-8e1f-2a3b4c5d6e7f"
		}`,
	}, {
		name:    "V0WithDescription",
		version: 0,
		prior: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"description": "description",
			"name": "name",
			"subscription_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"tenant_id": "c4d5e6f7-a8b9-4c0d-8e1f-2a3b4c5d6e7f"
		}`,
		want: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"description": "description",
			"name": "name",
			"subscription_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"tenant_id": "c4d5e6f7-a8b9-4c0d-8e1f-2a3b4c5d6e7f"
		}`,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := server.UpgradeResourceState(t.Context(), &tfprotov6.UpgradeResourceStateRequest{
				TypeName: "rubrik_data_center_azure_subscription",
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
			wantState := tfprotov6.RawState{JSON: []byte(tc.want)}
			want, err := wantState.Unmarshal(stateType)
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := got.Diff(want)
			if err != nil {
				t.Fatal(err)
			}
			for _, diff := range diffs {
				t.Errorf("%s: got %v, want %v", diff.Path, diff.Value1, diff.Value2)
			}
		})
	}
}
