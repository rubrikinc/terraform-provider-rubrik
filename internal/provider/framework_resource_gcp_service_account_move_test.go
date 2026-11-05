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
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestGcpServiceAccountMoveState moves hand-built v0 and v1 source states and
// checks the resulting target state.
func TestGcpServiceAccountMoveState(t *testing.T) {
	ctx := context.Background()
	r := &gcpServiceAccountResource{prefix: keyRubrik}

	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	sourceType := gcpServiceAccountSourceSchema.Type().TerraformType(ctx)
	sourceState := func(id string, name *string) tfsdk.State {
		var nameValue any
		if name != nil {
			nameValue = *name
		}
		return tfsdk.State{
			Schema: gcpServiceAccountSourceSchema,
			Raw: tftypes.NewValue(sourceType, map[string]tftypes.Value{
				keyID:              tftypes.NewValue(tftypes.String, id),
				keyCredentials:     tftypes.NewValue(tftypes.String, "key"),
				keyName:            tftypes.NewValue(tftypes.String, nameValue),
				keyPermissionsHash: tftypes.NewValue(tftypes.String, nil),
			}),
		}
	}
	name := "test-name"
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))

	tests := []struct {
		name     string
		address  string
		typeName string
		version  int64
		source   tfsdk.State
		wantID   string
		wantName types.String
	}{{
		// In v0 the ID is the service account name.
		name:     "V0",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_service_account",
		version:  0,
		source:   sourceState(name, &name),
		wantID:   hash,
		wantName: types.StringValue(name),
	}, {
		// A v0 state with a null name falls back to the ID.
		name:     "V0NullName",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_service_account",
		version:  0,
		source:   sourceState(name, nil),
		wantID:   hash,
		wantName: types.StringNull(),
	}, {
		name:     "V1",
		address:  "registry.terraform.io/rubrikinc/rubrik",
		typeName: "polaris_gcp_service_account",
		version:  1,
		source:   sourceState(hash, &name),
		wantID:   hash,
		wantName: types.StringValue(name),
	}, {
		name:     "OtherProvider",
		address:  "registry.terraform.io/hashicorp/google",
		typeName: "polaris_gcp_service_account",
		version:  1,
		source:   sourceState(hash, &name),
	}, {
		name:     "OtherType",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_account",
		version:  1,
		source:   sourceState(hash, &name),
	}, {
		name:     "OtherVersion",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_service_account",
		version:  2,
		source:   sourceState(hash, &name),
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := resource.MoveStateRequest{
				SourceProviderAddress: tc.address,
				SourceTypeName:        tc.typeName,
				SourceSchemaVersion:   tc.version,
				SourceState:           &tc.source,
			}
			res := resource.MoveStateResponse{
				TargetState: tfsdk.State{
					Schema: schemaRes.Schema,
					Raw:    tftypes.NewValue(schemaRes.Schema.Type().TerraformType(ctx), nil),
				},
			}

			// The framework runs each mover until one sets the target state.
			for _, mover := range r.MoveState(ctx) {
				mover.StateMover(ctx, req, &res)
				if res.Diagnostics.HasError() {
					t.Fatal(res.Diagnostics)
				}
				if !res.TargetState.Raw.IsNull() {
					break
				}
			}

			if tc.wantID == "" {
				if !res.TargetState.Raw.IsNull() {
					t.Fatalf("expected no move, got %v", res.TargetState.Raw)
				}
				return
			}

			var got gcpServiceAccountModel
			if diags := res.TargetState.Get(ctx, &got); diags.HasError() {
				t.Fatal(diags)
			}
			if got.ID.ValueString() != tc.wantID {
				t.Errorf("id: got %v, want %s", got.ID, tc.wantID)
			}
			if !got.Name.Equal(tc.wantName) {
				t.Errorf("name: got %v, want %v", got.Name, tc.wantName)
			}
			if got.Credentials.ValueString() != "key" {
				t.Errorf("credentials: got %v", got.Credentials)
			}
		})
	}
}
