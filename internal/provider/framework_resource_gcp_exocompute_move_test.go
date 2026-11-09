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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestGcpExocomputeMoveStateV0 moves hand-built v0 source states and checks
// the resulting target state.
func TestGcpExocomputeMoveStateV0(t *testing.T) {
	ctx := t.Context()
	r := &gcpExocomputeResource{prefix: keyRubrik}

	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	mover := r.moveStateV0()
	sourceType := mover.SourceSchema.Type().TerraformType(ctx).(tftypes.Object)
	configSetType := sourceType.AttributeTypes[keyRegionalConfig].(tftypes.Set)
	configType := configSetType.ElementType.(tftypes.Object)

	const id = "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b"
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

	// regionalConfig builds a source regional configuration, attributes left
	// out are null, as for a polaris provider version without them.
	regionalConfig := func(values map[string]tftypes.Value) tftypes.Value {
		for k, typ := range configType.AttributeTypes {
			if _, ok := values[k]; !ok {
				values[k] = tftypes.NewValue(typ, nil)
			}
		}
		return tftypes.NewValue(configType, values)
	}
	sourceState := func(configs ...tftypes.Value) tfsdk.State {
		return tfsdk.State{Schema: mover.SourceSchema, Raw: tftypes.NewValue(sourceType, map[string]tftypes.Value{
			keyID:                 str(id),
			keyCloudAccountID:     str(id),
			keyTriggerHealthCheck: tftypes.NewValue(tftypes.Bool, nil),
			keyRegionalConfig:     tftypes.NewValue(configSetType, configs),
		})}
	}
	wantConfig := func(hostProjectID, secondaryRangeName string) attr.Value {
		hostProject := types.StringNull()
		if hostProjectID != "" {
			hostProject = types.StringValue(hostProjectID)
		}
		return types.ObjectValueMust(gcpExocomputeRegionalConfigType.AttrTypes, map[string]attr.Value{
			keyHostProjectID:      hostProject,
			keyRegion:             types.StringValue("us-east1"),
			keySecondaryRangeName: types.StringValue(secondaryRangeName),
			keySubnetName:         types.StringValue("subnet"),
			keyVPCName:            types.StringValue("vpc"),
		})
	}

	tests := []struct {
		name     string
		address  string
		typeName string
		version  int64
		source   tfsdk.State
		want     types.Set
	}{{
		// SDKv2 state stores an unset host project as an empty string.
		name:     "Polaris",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_exocompute",
		source: sourceState(regionalConfig(map[string]tftypes.Value{
			keyHostProjectID:      str(""),
			keyRegion:             str("us-east1"),
			keySecondaryRangeName: str(gcpDefaultSecondaryRangeName),
			keySubnetName:         str("subnet"),
			keyVPCName:            str("vpc"),
		})),
		want: types.SetValueMust(gcpExocomputeRegionalConfigType, []attr.Value{
			wantConfig("", gcpDefaultSecondaryRangeName),
		}),
	}, {
		// Polaris provider versions before v1.11.0 have no host project and no
		// secondary range name.
		name:     "PolarisBeforeSharedVPC",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_exocompute",
		source: sourceState(regionalConfig(map[string]tftypes.Value{
			keyRegion:     str("us-east1"),
			keySubnetName: str("subnet"),
			keyVPCName:    str("vpc"),
		})),
		want: types.SetValueMust(gcpExocomputeRegionalConfigType, []attr.Value{
			wantConfig("", gcpDefaultSecondaryRangeName),
		}),
	}, {
		// SDKv2 state stores every unset string as an empty string.
		name:     "PolarisEmptyStrings",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_exocompute",
		source: sourceState(regionalConfig(map[string]tftypes.Value{
			keyHostProjectID:      str(""),
			keyRegion:             str(""),
			keySecondaryRangeName: str(""),
			keySubnetName:         str(""),
			keyVPCName:            str(""),
		})),
		want: types.SetValueMust(gcpExocomputeRegionalConfigType, []attr.Value{
			types.ObjectValueMust(gcpExocomputeRegionalConfigType.AttrTypes, map[string]attr.Value{
				keyHostProjectID:      types.StringNull(),
				keyRegion:             types.StringNull(),
				keySecondaryRangeName: types.StringValue(gcpDefaultSecondaryRangeName),
				keySubnetName:         types.StringNull(),
				keyVPCName:            types.StringNull(),
			}),
		}),
	}, {
		name:     "RubrikSharedVPC",
		address:  "registry.terraform.io/rubrikinc/rubrik",
		typeName: "polaris_gcp_exocompute",
		source: sourceState(regionalConfig(map[string]tftypes.Value{
			keyHostProjectID:      str("host-project"),
			keyRegion:             str("us-east1"),
			keySecondaryRangeName: str("pods"),
			keySubnetName:         str("subnet"),
			keyVPCName:            str("vpc"),
		})),
		want: types.SetValueMust(gcpExocomputeRegionalConfigType, []attr.Value{
			wantConfig("host-project", "pods"),
		}),
	}, {
		name:     "OtherProvider",
		address:  "registry.terraform.io/hashicorp/google",
		typeName: "polaris_gcp_exocompute",
		source:   sourceState(),
	}, {
		name:     "OtherType",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_exocompute",
		source:   sourceState(),
	}, {
		name:     "OtherVersion",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_exocompute",
		version:  1,
		source:   sourceState(),
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
			mover.StateMover(ctx, req, &res)
			if res.Diagnostics.HasError() {
				t.Fatal(res.Diagnostics)
			}

			if tc.want.IsNull() {
				if !res.TargetState.Raw.IsNull() {
					t.Fatalf("expected no move, got %v", res.TargetState.Raw)
				}
				return
			}

			var got gcpExocomputeModel
			if diags := res.TargetState.Get(ctx, &got); diags.HasError() {
				t.Fatal(diags)
			}
			if got.ID.ValueString() != id || got.CloudAccountID.ValueString() != id {
				t.Errorf("id or cloud_account_id: got %v, %v", got.ID, got.CloudAccountID)
			}
			if !got.TriggerHealthCheck.IsNull() {
				t.Errorf("trigger_health_check: got %v, want null", got.TriggerHealthCheck)
			}
			if !got.RegionalConfig.Equal(tc.want) {
				t.Errorf("regional_config: got %v, want %v", got.RegionalConfig, tc.want)
			}
		})
	}
}
