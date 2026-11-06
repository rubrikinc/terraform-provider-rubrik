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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestGcpArchivalLocationMoveStateV0 moves hand-built v0 source states, as
// written by the polaris provider and by the SDKv2 rubrik provider, and checks
// the resulting target state.
func TestGcpArchivalLocationMoveStateV0(t *testing.T) {
	ctx := context.Background()
	r := &gcpArchivalLocationResource{prefix: keyRubrik}

	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	mover := r.moveStateV0(ctx)
	sourceType := mover.SourceSchema.Type().TerraformType(ctx).(tftypes.Object)
	cmkType := sourceType.AttributeTypes[keyCustomerManagedKey].(tftypes.Set).ElementType
	kmsKeyType := sourceType.AttributeTypes[keyKMSKey].(tftypes.List).ElementType

	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

	// sourceState builds a source state with the given attribute overrides,
	// all other attributes are null.
	sourceState := func(overrides map[string]tftypes.Value) tfsdk.State {
		values := map[string]tftypes.Value{
			keyID:               str("4f2c1f43-3a47-4a54-9c3d-5c1f3a2f6b7e"),
			keyCloudAccountID:   str("8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b"),
			keyBucketPrefix:     str("prefix"),
			keyConnectionStatus: str("CONNECTED"),
			keyLocationTemplate: str("SOURCE_REGION"),
			keyName:             str("name"),
			keyStorageClass:     str("STANDARD"),
		}
		for k, v := range overrides {
			values[k] = v
		}
		for k, typ := range sourceType.AttributeTypes {
			if _, ok := values[k]; !ok {
				values[k] = tftypes.NewValue(typ, nil)
			}
		}
		return tfsdk.State{Schema: mover.SourceSchema, Raw: tftypes.NewValue(sourceType, values)}
	}

	emptyCMK := types.SetValueMust(gcpArchivalLocationCustomerManagedKeyType, nil)
	emptyKMSKey := types.ListValueMust(gcpArchivalLocationKMSKeyType, nil)

	tests := []struct {
		name         string
		address      string
		typeName     string
		version      int64
		source       tfsdk.State
		moved        bool
		wantRegion   types.String
		wantLabels   types.Map
		wantCMK      types.Set
		wantKMSKey   types.List
		wantTemplate string
	}{{
		// The polaris provider stores no region as an empty string, no labels
		// as an empty map and has no kms_key block.
		name:     "PolarisSourceRegion",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_archival_location",
		source: sourceState(map[string]tftypes.Value{
			keyRegion:       str(""),
			keyBucketLabels: tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{}),
			keyCustomerManagedKey: tftypes.NewValue(tftypes.Set{ElementType: cmkType},
				[]tftypes.Value{}),
		}),
		moved:        true,
		wantRegion:   types.StringNull(),
		wantLabels:   types.MapNull(types.StringType),
		wantCMK:      emptyCMK,
		wantKMSKey:   emptyKMSKey,
		wantTemplate: "SOURCE_REGION",
	}, {
		// The SDKv2 rubrik provider, with a region, labels, a customer managed
		// key and a KMS key.
		name:     "RubrikSpecificRegion",
		address:  "registry.terraform.io/rubrikinc/rubrik",
		typeName: "polaris_gcp_archival_location",
		source: sourceState(map[string]tftypes.Value{
			keyRegion:           str("us-east1"),
			keyLocationTemplate: str("SPECIFIC_REGION"),
			keyBucketLabels: tftypes.NewValue(tftypes.Map{ElementType: tftypes.String},
				map[string]tftypes.Value{"env": str("test")}),
			keyCustomerManagedKey: tftypes.NewValue(tftypes.Set{ElementType: cmkType}, []tftypes.Value{
				tftypes.NewValue(cmkType, map[string]tftypes.Value{
					keyName: str("key"), keyRegion: str("us-east1"), keyRingName: str("ring"),
				}),
			}),
			keyKMSKey: tftypes.NewValue(tftypes.List{ElementType: kmsKeyType}, []tftypes.Value{
				tftypes.NewValue(kmsKeyType, map[string]tftypes.Value{
					keyKeyManagerID: str("e9895414-64b4-4e25-a10d-c7a76983953b"),
					keyKeyName:      str("key"),
					keyKeyVersion:   str("1"),
				}),
			}),
		}),
		moved:      true,
		wantRegion: types.StringValue("us-east1"),
		wantLabels: types.MapValueMust(types.StringType, map[string]attr.Value{"env": types.StringValue("test")}),
		wantCMK: types.SetValueMust(gcpArchivalLocationCustomerManagedKeyType, []attr.Value{
			types.ObjectValueMust(gcpArchivalLocationCustomerManagedKeyType.AttrTypes, map[string]attr.Value{
				keyName: types.StringValue("key"), keyRegion: types.StringValue("us-east1"),
				keyRingName: types.StringValue("ring"),
			}),
		}),
		wantKMSKey: types.ListValueMust(gcpArchivalLocationKMSKeyType, []attr.Value{
			types.ObjectValueMust(gcpArchivalLocationKMSKeyType.AttrTypes, map[string]attr.Value{
				keyKeyManagerID: types.StringValue("e9895414-64b4-4e25-a10d-c7a76983953b"),
				keyKeyName:      types.StringValue("key"),
				keyKeyVersion:   types.StringValue("1"),
			}),
		}),
		wantTemplate: "SPECIFIC_REGION",
	}, {
		name:     "OtherProvider",
		address:  "registry.terraform.io/hashicorp/google",
		typeName: "polaris_gcp_archival_location",
		source:   sourceState(nil),
	}, {
		name:     "OtherType",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_aws_archival_location",
		source:   sourceState(nil),
	}, {
		name:     "OtherVersion",
		address:  "registry.terraform.io/rubrikinc/polaris",
		typeName: "polaris_gcp_archival_location",
		version:  1,
		source:   sourceState(nil),
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

			if !tc.moved {
				if !res.TargetState.Raw.IsNull() {
					t.Fatalf("expected no move, got %v", res.TargetState.Raw)
				}
				return
			}

			var got gcpArchivalLocationResourceModel
			if diags := res.TargetState.Get(ctx, &got); diags.HasError() {
				t.Fatal(diags)
			}
			if got.ID.ValueString() != "4f2c1f43-3a47-4a54-9c3d-5c1f3a2f6b7e" {
				t.Errorf("id: got %v", got.ID)
			}
			if got.Name.ValueString() != "name" || got.BucketPrefix.ValueString() != "prefix" ||
				got.StorageClass.ValueString() != "STANDARD" {
				t.Errorf("unexpected name, bucket prefix or storage class: %+v", got)
			}
			if got.LocationTemplate.ValueString() != tc.wantTemplate {
				t.Errorf("location_template: got %v, want %s", got.LocationTemplate, tc.wantTemplate)
			}
			if !got.Region.Equal(tc.wantRegion) {
				t.Errorf("region: got %v, want %v", got.Region, tc.wantRegion)
			}
			if !got.BucketLabels.Equal(tc.wantLabels) {
				t.Errorf("bucket_labels: got %v, want %v", got.BucketLabels, tc.wantLabels)
			}
			if !got.CustomerManagedKey.Equal(tc.wantCMK) {
				t.Errorf("customer_managed_key: got %v, want %v", got.CustomerManagedKey, tc.wantCMK)
			}
			if !got.KMSKey.Equal(tc.wantKMSKey) {
				t.Errorf("kms_key: got %v, want %v", got.KMSKey, tc.wantKMSKey)
			}
			if !got.Timeouts.IsNull() {
				t.Errorf("timeouts: got %v, want null", got.Timeouts)
			}
		})
	}
}

// TestGcpArchivalLocationValidateConfig checks that the kms_key block
// conflicts with the customer_managed_key block.
func TestGcpArchivalLocationValidateConfig(t *testing.T) {
	ctx := context.Background()
	r := &gcpArchivalLocationResource{prefix: keyRubrik}

	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	configType := schemaRes.Schema.Type().TerraformType(ctx).(tftypes.Object)
	cmkSetType := configType.AttributeTypes[keyCustomerManagedKey].(tftypes.Set)
	kmsKeyListType := configType.AttributeTypes[keyKMSKey].(tftypes.List)
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

	cmk := tftypes.NewValue(cmkSetType, []tftypes.Value{
		tftypes.NewValue(cmkSetType.ElementType, map[string]tftypes.Value{
			keyName: str("key"), keyRegion: str("us-east1"), keyRingName: str("ring"),
		}),
	})
	kmsKey := tftypes.NewValue(kmsKeyListType, []tftypes.Value{
		tftypes.NewValue(kmsKeyListType.ElementType, map[string]tftypes.Value{
			keyKeyManagerID: str("e9895414-64b4-4e25-a10d-c7a76983953b"),
			keyKeyName:      str("key"),
			keyKeyVersion:   str("1"),
		}),
	})
	emptyCMK := tftypes.NewValue(cmkSetType, []tftypes.Value{})
	emptyKMSKey := tftypes.NewValue(kmsKeyListType, []tftypes.Value{})

	tests := []struct {
		name     string
		cmk      tftypes.Value
		kmsKey   tftypes.Value
		conflict bool
	}{{
		name:   "Neither",
		cmk:    emptyCMK,
		kmsKey: emptyKMSKey,
	}, {
		name:   "CustomerManagedKey",
		cmk:    cmk,
		kmsKey: emptyKMSKey,
	}, {
		name:   "KMSKey",
		cmk:    emptyCMK,
		kmsKey: kmsKey,
	}, {
		name:   "KMSKeyNullCustomerManagedKey",
		cmk:    tftypes.NewValue(cmkSetType, nil),
		kmsKey: kmsKey,
	}, {
		name:     "Both",
		cmk:      cmk,
		kmsKey:   kmsKey,
		conflict: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]tftypes.Value{
				keyCustomerManagedKey: tc.cmk,
				keyKMSKey:             tc.kmsKey,
			}
			for k, typ := range configType.AttributeTypes {
				if _, ok := values[k]; !ok {
					values[k] = tftypes.NewValue(typ, nil)
				}
			}
			req := resource.ValidateConfigRequest{
				Config: tfsdk.Config{Schema: schemaRes.Schema, Raw: tftypes.NewValue(configType, values)},
			}
			var res resource.ValidateConfigResponse
			r.ValidateConfig(ctx, req, &res)
			if res.Diagnostics.HasError() != tc.conflict {
				t.Fatalf("expected conflict %t, got diagnostics %v", tc.conflict, res.Diagnostics)
			}
		})
	}
}
