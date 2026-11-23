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
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const (
	testGCPProjectAccountID = "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b"
	testGCPProjectNativeID  = "my-project"
)

// TestGCPProjectMoveState moves hand-built v0, v1, v2 and v3 source states and
// checks the resulting target state.
func TestGCPProjectMoveState(t *testing.T) {
	ctx := context.Background()
	r := &gcpProjectResource{prefix: keyRubrik}

	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	movers := r.MoveState(ctx)
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

	// v0 and v1 share a schema.
	v0Type := gcpProjectSchemaV0.Type().TerraformType(ctx)
	v0StateWith := func(id, project, credentials, orgName string) tfsdk.State {
		return tfsdk.State{Schema: gcpProjectSchemaV0, Raw: tftypes.NewValue(v0Type, map[string]tftypes.Value{
			keyID:                       str(id),
			keyCredentials:              str(credentials),
			keyDeleteSnapshotsOnDestroy: tftypes.NewValue(tftypes.Bool, false),
			keyOrganizationName:         str(orgName),
			keyProject:                  str(project),
			keyProjectName:              str("name"),
			keyProjectNumber:            str("123"),
		})}
	}
	v0State := func(id, project string) tfsdk.State { return v0StateWith(id, project, "key.json", "org") }

	// v2 and v3 share a schema.
	v2Schema := movers[2].SourceSchema
	v2Type := v2Schema.Type().TerraformType(ctx).(tftypes.Object)
	featureSetType := v2Type.AttributeTypes[keyFeature].(tftypes.Set)
	featureType := featureSetType.ElementType.(tftypes.Object)
	cnpListType := v2Type.AttributeTypes[keyCloudNativeProtection].(tftypes.List)
	cnpType := cnpListType.ElementType.(tftypes.Object)
	v2State := func(features, cnp []tftypes.Value) tfsdk.State {
		return tfsdk.State{Schema: v2Schema, Raw: tftypes.NewValue(v2Type, map[string]tftypes.Value{
			keyID:                       str(testGCPProjectAccountID),
			keyCredentials:              str(""),
			keyDeleteSnapshotsOnDestroy: tftypes.NewValue(tftypes.Bool, false),
			keyOrganizationName:         str(""),
			keyPermissionsHash:          str(""),
			keyProject:                  str(testGCPProjectNativeID),
			keyProjectName:              str("name"),
			keyProjectNumber:            str("123"),
			keyFeature:                  tftypes.NewValue(featureSetType, features),
			keyCloudNativeProtection:    tftypes.NewValue(cnpListType, cnp),
		})}
	}
	cnpFeature := tftypes.NewValue(featureType, map[string]tftypes.Value{
		keyName:             str("CLOUD_NATIVE_PROTECTION"),
		keyPermissionGroups: tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{str("BASIC")}),
		keyPermissions:      str(""),
		keyStatus:           str("CONNECTED"),
	})
	cnpBlock := tftypes.NewValue(cnpType, map[string]tftypes.Value{keyStatus: str("CONNECTED")})

	emptyFeatures := types.SetValueMust(gcpProjectFeatureType, nil)
	nullCNP := types.ListNull(gcpProjectCloudNativeProtectionType)

	tests := []struct {
		name        string
		version     int64
		address     string
		typeName    string
		source      tfsdk.State
		wantErr     string
		wantMoved   bool
		wantFeature types.Set
		wantCNP     types.List
		wantCreds   types.String
		wantOrg     types.String
	}{{
		name:        "V0",
		version:     0,
		source:      v0State(testGCPProjectAccountID+":"+testGCPProjectNativeID, testGCPProjectNativeID),
		wantMoved:   true,
		wantFeature: emptyFeatures,
		wantCNP:     nullCNP,
		wantCreds:   types.StringValue("key.json"),
		wantOrg:     types.StringValue("org"),
	}, {
		name:    "V0InvalidID",
		version: 0,
		source:  v0State(testGCPProjectAccountID, testGCPProjectNativeID),
		wantErr: "invalid id format",
	}, {
		name:    "V0ProjectMismatch",
		version: 0,
		source:  v0State(testGCPProjectAccountID+":other-project", testGCPProjectNativeID),
		wantErr: "other-project",
	}, {
		name:        "V1",
		version:     1,
		source:      v0State(testGCPProjectAccountID, testGCPProjectNativeID),
		wantMoved:   true,
		wantFeature: emptyFeatures,
		wantCNP:     nullCNP,
		wantCreds:   types.StringValue("key.json"),
		wantOrg:     types.StringValue("org"),
	}, {
		// The SDKv2 resource stored unset strings as empty strings.
		name:        "V1EmptyStrings",
		version:     1,
		source:      v0StateWith(testGCPProjectAccountID, testGCPProjectNativeID, "", ""),
		wantMoved:   true,
		wantFeature: emptyFeatures,
		wantCNP:     nullCNP,
		wantCreds:   types.StringNull(),
		wantOrg:     types.StringNull(),
	}, {
		// SDKv2 v2 state with features also holds the Cloud Native Protection
		// block read from the features. Only the features are kept, and empty
		// strings become null.
		name:      "V2Features",
		version:   2,
		source:    v2State([]tftypes.Value{cnpFeature}, []tftypes.Value{cnpBlock}),
		wantMoved: true,
		wantFeature: types.SetValueMust(gcpProjectFeatureType, []attr.Value{
			types.ObjectValueMust(gcpProjectFeatureType.AttrTypes, map[string]attr.Value{
				keyName: types.StringValue("CLOUD_NATIVE_PROTECTION"),
				keyPermissionGroups: types.SetValueMust(types.StringType, []attr.Value{
					types.StringValue("BASIC"),
				}),
				keyPermissions: types.StringNull(),
				keyStatus:      types.StringValue("CONNECTED"),
			}),
		}),
		wantCNP:   nullCNP,
		wantCreds: types.StringNull(),
		wantOrg:   types.StringNull(),
	}, {
		name:        "V2CloudNativeProtection",
		version:     2,
		source:      v2State(nil, []tftypes.Value{cnpBlock}),
		wantMoved:   true,
		wantFeature: emptyFeatures,
		wantCNP: types.ListValueMust(gcpProjectCloudNativeProtectionType, []attr.Value{
			types.ObjectValueMust(gcpProjectCloudNativeProtectionType.AttrTypes, map[string]attr.Value{
				keyStatus: types.StringValue("CONNECTED"),
			}),
		}),
		wantCreds: types.StringNull(),
		wantOrg:   types.StringNull(),
	}, {
		// v3 state from the polaris_gcp_project alias of this provider.
		name:        "V3",
		version:     3,
		address:     "registry.terraform.io/rubrikinc/rubrik",
		source:      v2State(nil, []tftypes.Value{cnpBlock}),
		wantMoved:   true,
		wantFeature: emptyFeatures,
		wantCNP: types.ListValueMust(gcpProjectCloudNativeProtectionType, []attr.Value{
			types.ObjectValueMust(gcpProjectCloudNativeProtectionType.AttrTypes, map[string]attr.Value{
				keyStatus: types.StringValue("CONNECTED"),
			}),
		}),
		wantCreds: types.StringNull(),
		wantOrg:   types.StringNull(),
	}, {
		name:     "OtherProvider",
		version:  2,
		address:  "registry.terraform.io/hashicorp/google",
		source:   v2State(nil, nil),
		typeName: "polaris_gcp_project",
	}, {
		name:     "OtherType",
		version:  2,
		typeName: "polaris_gcp_service_account",
		source:   v2State(nil, nil),
	}, {
		name:    "OtherVersion",
		version: 4,
		source:  v2State(nil, nil),
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.address == "" {
				tc.address = "registry.terraform.io/rubrikinc/polaris"
			}
			if tc.typeName == "" {
				tc.typeName = "polaris_gcp_project"
			}
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

			// The framework runs each mover until one sets the target state or
			// reports an error.
			for _, mover := range movers {
				mover.StateMover(ctx, req, &res)
				if res.Diagnostics.HasError() || !res.TargetState.Raw.IsNull() {
					break
				}
			}

			if tc.wantErr != "" {
				if !res.Diagnostics.HasError() {
					t.Fatalf("expected error containing %q", tc.wantErr)
				}
				if detail := res.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, tc.wantErr) {
					t.Fatalf("error %q does not contain %q", detail, tc.wantErr)
				}
				return
			}
			if res.Diagnostics.HasError() {
				t.Fatal(res.Diagnostics)
			}
			if !tc.wantMoved {
				if !res.TargetState.Raw.IsNull() {
					t.Fatalf("expected no move, got %v", res.TargetState.Raw)
				}
				return
			}

			var got gcpProjectResourceModel
			if diags := res.TargetState.Get(ctx, &got); diags.HasError() {
				t.Fatal(diags)
			}
			if got.ID.ValueString() != testGCPProjectAccountID {
				t.Errorf("id: got %v", got.ID)
			}
			if got.Project.ValueString() != testGCPProjectNativeID || got.ProjectName.ValueString() != "name" ||
				got.ProjectNumber.ValueString() != "123" {
				t.Errorf("unexpected project, project name or project number: %+v", got)
			}
			if !got.DeleteSnapshotsOnDestroy.Equal(types.BoolValue(false)) {
				t.Errorf("delete_snapshots_on_destroy: got %v", got.DeleteSnapshotsOnDestroy)
			}
			if !got.PermissionsHash.IsNull() {
				t.Errorf("permissions_hash: got %v, want null", got.PermissionsHash)
			}
			if !got.Credentials.Equal(tc.wantCreds) {
				t.Errorf("credentials: got %v, want %v", got.Credentials, tc.wantCreds)
			}
			if !got.OrganizationName.Equal(tc.wantOrg) {
				t.Errorf("organization_name: got %v, want %v", got.OrganizationName, tc.wantOrg)
			}
			if !got.Feature.Equal(tc.wantFeature) {
				t.Errorf("feature: got %v, want %v", got.Feature, tc.wantFeature)
			}
			if !got.CloudNativeProtection.Equal(tc.wantCNP) {
				t.Errorf("cloud_native_protection: got %v, want %v", got.CloudNativeProtection, tc.wantCNP)
			}
		})
	}
}

// TestGCPProjectUpgradeStateV2 upgrades hand-built v2 state written by the
// SDKv2 resource, and checks that the empty strings become null and that only
// the block in use is kept.
func TestGCPProjectUpgradeStateV2(t *testing.T) {
	ctx := context.Background()
	r := &gcpProjectResource{prefix: keyRubrik}

	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	upgrader := r.UpgradeState(ctx)[2]
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }
	v2Type := gcpProjectSchemaV2.Type().TerraformType(ctx).(tftypes.Object)
	featureSetType := v2Type.AttributeTypes[keyFeature].(tftypes.Set)
	featureType := featureSetType.ElementType.(tftypes.Object)
	cnpListType := v2Type.AttributeTypes[keyCloudNativeProtection].(tftypes.List)
	cnpType := cnpListType.ElementType.(tftypes.Object)

	// SDKv2 state onboarded with a feature, holding empty strings for the
	// unset strings and both blocks.
	prior := tfsdk.State{Schema: gcpProjectSchemaV2, Raw: tftypes.NewValue(v2Type, map[string]tftypes.Value{
		keyID:                       str(testGCPProjectAccountID),
		keyCredentials:              str(""),
		keyDeleteSnapshotsOnDestroy: tftypes.NewValue(tftypes.Bool, false),
		keyOrganizationName:         str(""),
		keyPermissionsHash:          str(""),
		keyProject:                  str(testGCPProjectNativeID),
		keyProjectName:              str("name"),
		keyProjectNumber:            str("123"),
		keyFeature: tftypes.NewValue(featureSetType, []tftypes.Value{
			tftypes.NewValue(featureType, map[string]tftypes.Value{
				keyName:             str("CLOUD_NATIVE_PROTECTION"),
				keyPermissionGroups: tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{str("BASIC")}),
				keyPermissions:      str(""),
				keyStatus:           str("CONNECTED"),
			}),
		}),
		keyCloudNativeProtection: tftypes.NewValue(cnpListType, []tftypes.Value{
			tftypes.NewValue(cnpType, map[string]tftypes.Value{keyStatus: str("CONNECTED")}),
		}),
	})}

	req := resource.UpgradeStateRequest{State: &prior}
	res := resource.UpgradeStateResponse{State: tfsdk.State{
		Schema: schemaRes.Schema,
		Raw:    tftypes.NewValue(schemaRes.Schema.Type().TerraformType(ctx), nil),
	}}
	upgrader.StateUpgrader(ctx, req, &res)
	if res.Diagnostics.HasError() {
		t.Fatal(res.Diagnostics)
	}

	var got gcpProjectResourceModel
	if diags := res.State.Get(ctx, &got); diags.HasError() {
		t.Fatal(diags)
	}
	if !got.Credentials.IsNull() || !got.OrganizationName.IsNull() || !got.PermissionsHash.IsNull() {
		t.Errorf("credentials, organization_name and permissions_hash: got %v, %v, %v, want null",
			got.Credentials, got.OrganizationName, got.PermissionsHash)
	}
	wantFeature := types.SetValueMust(gcpProjectFeatureType, []attr.Value{
		types.ObjectValueMust(gcpProjectFeatureType.AttrTypes, map[string]attr.Value{
			keyName:             types.StringValue("CLOUD_NATIVE_PROTECTION"),
			keyPermissionGroups: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("BASIC")}),
			keyPermissions:      types.StringNull(),
			keyStatus:           types.StringValue("CONNECTED"),
		}),
	})
	if !got.Feature.Equal(wantFeature) {
		t.Errorf("feature: got %v, want %v", got.Feature, wantFeature)
	}
	if !got.CloudNativeProtection.Equal(types.ListNull(gcpProjectCloudNativeProtectionType)) {
		t.Errorf("cloud_native_protection: got %v, want null", got.CloudNativeProtection)
	}
	if got.ID.ValueString() != testGCPProjectAccountID || got.ProjectNumber.ValueString() != "123" {
		t.Errorf("unexpected id or project number: %+v", got)
	}
}

// TestGCPProjectStateFromV1 checks the v3 state built by the state upgraders
// from v1 state and the GCP project read from RSC. The SDKv2 upgrader read the
// status from the Exocompute feature and failed without it.
func TestGCPProjectStateFromV1(t *testing.T) {
	ctx := context.Background()

	prior := gcpProjectResourceModelV0{
		ID:                       types.StringValue(testGCPProjectAccountID),
		Credentials:              types.StringValue("key.json"),
		DeleteSnapshotsOnDestroy: types.BoolNull(),
		OrganizationName:         types.StringValue("org"),
		Project:                  types.StringValue(testGCPProjectNativeID),
		ProjectName:              types.StringValue("name"),
		ProjectNumber:            types.StringValue("123"),
	}
	cnpStatus := func(status string) types.List {
		return types.ListValueMust(gcpProjectCloudNativeProtectionType, []attr.Value{
			types.ObjectValueMust(gcpProjectCloudNativeProtectionType.AttrTypes, map[string]attr.Value{
				keyStatus: types.StringValue(status),
			}),
		})
	}

	tests := []struct {
		name    string
		account *gcp.CloudAccount
		wantCNP types.List
	}{{
		name: "CloudNativeProtection",
		account: &gcp.CloudAccount{Features: []gcp.Feature{
			{Feature: core.FeatureExocompute, Status: "DISCONNECTED"},
			{Feature: core.FeatureCloudNativeProtection, Status: "CONNECTED"},
		}},
		wantCNP: cnpStatus("CONNECTED"),
	}, {
		name: "NoCloudNativeProtection",
		account: &gcp.CloudAccount{Features: []gcp.Feature{
			{Feature: core.FeatureExocompute, Status: "CONNECTED"},
		}},
		wantCNP: types.ListNull(gcpProjectCloudNativeProtectionType),
	}, {
		name:    "NoAccount",
		wantCNP: types.ListNull(gcpProjectCloudNativeProtectionType),
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := gcpProjectStateFromV1(ctx, prior, tc.account)
			if diags.HasError() {
				t.Fatal(diags)
			}
			if !got.CloudNativeProtection.Equal(tc.wantCNP) {
				t.Errorf("cloud_native_protection: got %v, want %v", got.CloudNativeProtection, tc.wantCNP)
			}
			if !got.Feature.Equal(types.SetValueMust(gcpProjectFeatureType, nil)) {
				t.Errorf("feature: got %v, want empty", got.Feature)
			}
			if !got.DeleteSnapshotsOnDestroy.Equal(types.BoolValue(false)) {
				t.Errorf("delete_snapshots_on_destroy: got %v, want false", got.DeleteSnapshotsOnDestroy)
			}
			if !got.Credentials.Equal(prior.Credentials) || !got.OrganizationName.Equal(prior.OrganizationName) {
				t.Errorf("unexpected credentials or organization name: %+v", got)
			}
		})
	}
}

// TestGCPProjectUpdateModel checks which block Read populates from the GCP
// project in RSC, and that the permissions signal is kept.
func TestGCPProjectUpdateModel(t *testing.T) {
	ctx := context.Background()

	groups := func(names ...string) types.Set {
		var values []attr.Value
		for _, name := range names {
			values = append(values, types.StringValue(name))
		}
		return types.SetValueMust(types.StringType, values)
	}
	feature := func(name, permissions, status string, groupNames ...string) attr.Value {
		p := types.StringNull()
		if permissions != "" {
			p = types.StringValue(permissions)
		}
		s := types.StringNull()
		if status != "" {
			s = types.StringValue(status)
		}
		return types.ObjectValueMust(gcpProjectFeatureType.AttrTypes, map[string]attr.Value{
			keyName:             types.StringValue(name),
			keyPermissionGroups: groups(groupNames...),
			keyPermissions:      p,
			keyStatus:           s,
		})
	}
	cnp := func(status string) types.List {
		return types.ListValueMust(gcpProjectCloudNativeProtectionType, []attr.Value{
			types.ObjectValueMust(gcpProjectCloudNativeProtectionType.AttrTypes, map[string]attr.Value{
				keyStatus: types.StringValue(status),
			}),
		})
	}
	emptyFeatures := types.SetValueMust(gcpProjectFeatureType, nil)
	emptyCNP := types.ListValueMust(gcpProjectCloudNativeProtectionType, nil)

	account := gcp.CloudAccount{
		ID:            uuid.MustParse(testGCPProjectAccountID),
		NativeID:      testGCPProjectNativeID,
		Name:          "name",
		ProjectNumber: 123,
		Features: []gcp.Feature{{
			Feature: core.Feature{Name: "CLOUD_NATIVE_PROTECTION", PermissionGroups: []core.PermissionGroup{"BASIC"}},
			Status:  "CONNECTED",
		}},
	}

	tests := []struct {
		name        string
		feature     types.Set
		cnp         types.List
		wantFeature types.Set
		wantCNP     types.List
	}{{
		name:        "Features",
		feature:     types.SetValueMust(gcpProjectFeatureType, []attr.Value{feature("CLOUD_NATIVE_PROTECTION", "v1", "", "BASIC")}),
		cnp:         emptyCNP,
		wantFeature: types.SetValueMust(gcpProjectFeatureType, []attr.Value{feature("CLOUD_NATIVE_PROTECTION", "v1", "CONNECTED", "BASIC")}),
		wantCNP:     emptyCNP,
	}, {
		name:        "CloudNativeProtection",
		feature:     emptyFeatures,
		cnp:         cnp("DISCONNECTED"),
		wantFeature: emptyFeatures,
		wantCNP:     cnp("CONNECTED"),
	}, {
		// State written by the SDKv2 resource holds both blocks, the
		// features are used.
		name:        "Both",
		feature:     types.SetValueMust(gcpProjectFeatureType, []attr.Value{feature("CLOUD_NATIVE_PROTECTION", "", "CONNECTED", "BASIC")}),
		cnp:         cnp("CONNECTED"),
		wantFeature: types.SetValueMust(gcpProjectFeatureType, []attr.Value{feature("CLOUD_NATIVE_PROTECTION", "", "CONNECTED", "BASIC")}),
		wantCNP:     emptyCNP,
	}, {
		// After an import neither block is populated, the features are used.
		name:        "Import",
		feature:     types.SetNull(gcpProjectFeatureType),
		cnp:         types.ListNull(gcpProjectCloudNativeProtectionType),
		wantFeature: types.SetValueMust(gcpProjectFeatureType, []attr.Value{feature("CLOUD_NATIVE_PROTECTION", "", "CONNECTED", "BASIC")}),
		wantCNP:     emptyCNP,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := gcpProjectResourceModel{
				ID:                    types.StringValue(testGCPProjectAccountID),
				Feature:               tc.feature,
				CloudNativeProtection: tc.cnp,
			}
			if diags := updateGCPProjectResourceModel(ctx, &m, account); diags.HasError() {
				t.Fatal(diags)
			}
			if !m.Feature.Equal(tc.wantFeature) {
				t.Errorf("feature: got %v, want %v", m.Feature, tc.wantFeature)
			}
			if !m.CloudNativeProtection.Equal(tc.wantCNP) {
				t.Errorf("cloud_native_protection: got %v, want %v", m.CloudNativeProtection, tc.wantCNP)
			}
			if !m.OrganizationName.IsNull() {
				t.Errorf("organization_name: got %v, want null", m.OrganizationName)
			}
			if !m.DeleteSnapshotsOnDestroy.Equal(types.BoolValue(false)) {
				t.Errorf("delete_snapshots_on_destroy: got %v, want false", m.DeleteSnapshotsOnDestroy)
			}
		})
	}
}

// TestGCPProjectValidateConfig checks that exactly one of the feature and the
// cloud_native_protection blocks is specified.
func TestGCPProjectValidateConfig(t *testing.T) {
	ctx := context.Background()
	r := &gcpProjectResource{prefix: keyRubrik}

	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	configType := schemaRes.Schema.Type().TerraformType(ctx).(tftypes.Object)
	featureSetType := configType.AttributeTypes[keyFeature].(tftypes.Set)
	featureType := featureSetType.ElementType.(tftypes.Object)
	cnpListType := configType.AttributeTypes[keyCloudNativeProtection].(tftypes.List)
	cnpType := cnpListType.ElementType.(tftypes.Object)
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

	features := tftypes.NewValue(featureSetType, []tftypes.Value{
		tftypes.NewValue(featureType, map[string]tftypes.Value{
			keyName:             str("CLOUD_NATIVE_PROTECTION"),
			keyPermissionGroups: tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{str("BASIC")}),
			keyPermissions:      tftypes.NewValue(tftypes.String, nil),
			keyStatus:           tftypes.NewValue(tftypes.String, nil),
		}),
	})
	cnp := tftypes.NewValue(cnpListType, []tftypes.Value{
		tftypes.NewValue(cnpType, map[string]tftypes.Value{keyStatus: tftypes.NewValue(tftypes.String, nil)}),
	})
	noFeatures := tftypes.NewValue(featureSetType, []tftypes.Value{})
	noCNP := tftypes.NewValue(cnpListType, []tftypes.Value{})

	tests := []struct {
		name    string
		feature tftypes.Value
		cnp     tftypes.Value
		wantErr bool
	}{
		{name: "Feature", feature: features, cnp: noCNP},
		{name: "CloudNativeProtection", feature: noFeatures, cnp: cnp},
		{name: "Neither", feature: noFeatures, cnp: noCNP, wantErr: true},
		{name: "Both", feature: features, cnp: cnp, wantErr: true},
		{name: "UnknownFeature", feature: tftypes.NewValue(featureSetType, tftypes.UnknownValue), cnp: noCNP},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]tftypes.Value{keyFeature: tc.feature, keyCloudNativeProtection: tc.cnp}
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
			if res.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("expected error %t, got diagnostics %v", tc.wantErr, res.Diagnostics)
			}
		})
	}
}

// TestGCPProjectModifyPlan checks the plan-time feature checks that need no
// RSC client: the Cloud Discovery removal check against features in state, and
// the warning when the state holds no features and no client is available, so
// the features in RSC can't be read.
func TestGCPProjectModifyPlan(t *testing.T) {
	ctx := context.Background()

	var schemaRes resource.SchemaResponse
	(&gcpProjectResource{prefix: keyRubrik}).Schema(ctx, resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}

	objType := schemaRes.Schema.Type().TerraformType(ctx).(tftypes.Object)
	featureSetType := objType.AttributeTypes[keyFeature].(tftypes.Set)
	featureType := featureSetType.ElementType.(tftypes.Object)
	cnpListType := objType.AttributeTypes[keyCloudNativeProtection].(tftypes.List)
	cnpType := cnpListType.ElementType.(tftypes.Object)
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

	features := func(names ...string) tftypes.Value {
		var elems []tftypes.Value
		for _, name := range names {
			elems = append(elems, tftypes.NewValue(featureType, map[string]tftypes.Value{
				keyName:             str(name),
				keyPermissionGroups: tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{str("BASIC")}),
				keyPermissions:      tftypes.NewValue(tftypes.String, nil),
				keyStatus:           tftypes.NewValue(tftypes.String, nil),
			}))
		}
		return tftypes.NewValue(featureSetType, elems)
	}
	cnp := tftypes.NewValue(cnpListType, []tftypes.Value{
		tftypes.NewValue(cnpType, map[string]tftypes.Value{keyStatus: str("CONNECTED")}),
	})
	noCNP := tftypes.NewValue(cnpListType, []tftypes.Value{})
	project := func(feature, cnp tftypes.Value) tftypes.Value {
		values := map[string]tftypes.Value{
			keyID:                       str(testGCPProjectAccountID),
			keyDeleteSnapshotsOnDestroy: tftypes.NewValue(tftypes.Bool, false),
			keyFeature:                  feature,
			keyCloudNativeProtection:    cnp,
		}
		for k, typ := range objType.AttributeTypes {
			if _, ok := values[k]; !ok {
				values[k] = tftypes.NewValue(typ, nil)
			}
		}
		return tftypes.NewValue(objType, values)
	}

	tests := []struct {
		name        string
		state       tftypes.Value
		plan        tftypes.Value
		wantErr     bool
		wantWarning bool
	}{{
		name:  "KeepFeatures",
		state: project(features("CLOUD_DISCOVERY", "CLOUD_NATIVE_PROTECTION"), noCNP),
		plan:  project(features("CLOUD_DISCOVERY", "CLOUD_NATIVE_PROTECTION"), noCNP),
	}, {
		name:    "RemoveCloudDiscoveryWithProtection",
		state:   project(features("CLOUD_DISCOVERY", "CLOUD_NATIVE_PROTECTION"), noCNP),
		plan:    project(features("CLOUD_NATIVE_PROTECTION"), noCNP),
		wantErr: true,
	}, {
		// The deprecated block in both, Update leaves the features in RSC as
		// they are, so nothing is read.
		name:  "CloudNativeProtection",
		state: project(features(), cnp),
		plan:  project(features(), cnp),
	}, {
		// Switching from the deprecated block without a client, the features
		// in RSC can't be read, so the plan warns instead.
		name:        "SwitchToFeatureWithoutClient",
		state:       project(features(), cnp),
		plan:        project(features("CLOUD_NATIVE_PROTECTION"), noCNP),
		wantWarning: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := resource.ModifyPlanRequest{
				State: tfsdk.State{Schema: schemaRes.Schema, Raw: tc.state},
				Plan:  tfsdk.Plan{Schema: schemaRes.Schema, Raw: tc.plan},
			}
			res := resource.ModifyPlanResponse{Plan: req.Plan}
			(&gcpProjectResource{prefix: keyRubrik}).ModifyPlan(ctx, req, &res)
			if res.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("expected error %t, got diagnostics %v", tc.wantErr, res.Diagnostics)
			}
			if gotWarning := res.Diagnostics.WarningsCount() > 0; gotWarning != tc.wantWarning {
				t.Fatalf("expected warning %t, got diagnostics %v", tc.wantWarning, res.Diagnostics)
			}
		})
	}
}

// TestGCPCheckCloudDiscoveryRemoval verifies the plan-time rule that Cloud
// Discovery can only be removed from a project once no protection feature is
// left on it.
func TestGCPCheckCloudDiscoveryRemoval(t *testing.T) {
	tests := []struct {
		name        string
		oldFeatures []string
		newFeatures []string
		wantErr     string
	}{{
		name:        "RemoveCloudDiscoveryWithProtectionLeft",
		oldFeatures: []string{"CLOUD_DISCOVERY", "CLOUD_NATIVE_PROTECTION"},
		newFeatures: []string{"CLOUD_NATIVE_PROTECTION"},
		wantErr:     "CLOUD_DISCOVERY cannot be removed while the protection feature CLOUD_NATIVE_PROTECTION is enabled",
	}, {
		name:        "RemoveCloudDiscoveryWithCloudSQLLeft",
		oldFeatures: []string{"CLOUD_DISCOVERY", "CLOUD_SQL_PROTECTION", "EXOCOMPUTE"},
		newFeatures: []string{"CLOUD_SQL_PROTECTION", "EXOCOMPUTE"},
		wantErr:     "protection feature CLOUD_SQL_PROTECTION is enabled",
	}, {
		name:        "RemoveCloudDiscoveryAndProtectionTogether",
		oldFeatures: []string{"CLOUD_DISCOVERY", "CLOUD_NATIVE_PROTECTION"},
		newFeatures: []string{"EXOCOMPUTE"},
	}, {
		name:        "RemoveCloudDiscoveryWithOnlyNonProtectionLeft",
		oldFeatures: []string{"CLOUD_DISCOVERY", "EXOCOMPUTE", "GCP_SHARED_VPC_HOST"},
		newFeatures: []string{"EXOCOMPUTE", "GCP_SHARED_VPC_HOST"},
	}, {
		name:        "KeepCloudDiscovery",
		oldFeatures: []string{"CLOUD_DISCOVERY", "CLOUD_NATIVE_PROTECTION"},
		newFeatures: []string{"CLOUD_DISCOVERY"},
	}, {
		name:        "CloudDiscoveryNeverPresent",
		oldFeatures: []string{"CLOUD_NATIVE_PROTECTION"},
		newFeatures: []string{"CLOUD_NATIVE_PROTECTION", "CLOUD_SQL_PROTECTION"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gcpCheckCloudDiscoveryRemoval(tt.oldFeatures, tt.newFeatures)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("expected error containing %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// TestGCPProjectSetStatus checks that setting the status after an apply keeps
// an empty block empty. A null block would differ from the plan, and Read would
// then show a diff.
func TestGCPProjectSetStatus(t *testing.T) {
	ctx := context.Background()

	account := gcp.CloudAccount{Features: []gcp.Feature{{
		Feature: core.FeatureCloudNativeProtection,
		Status:  "CONNECTED",
	}}}
	unknownCNP := types.ListValueMust(gcpProjectCloudNativeProtectionType, []attr.Value{
		types.ObjectValueMust(gcpProjectCloudNativeProtectionType.AttrTypes, map[string]attr.Value{
			keyStatus: types.StringUnknown(),
		}),
	})

	for _, tc := range []struct {
		name    string
		account *gcp.CloudAccount
		want    types.String
	}{
		{name: "Account", account: &account, want: types.StringValue("CONNECTED")},
		{name: "NoAccount", want: types.StringNull()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := gcpProjectResourceModel{
				Feature:               types.SetValueMust(gcpProjectFeatureType, nil),
				CloudNativeProtection: unknownCNP,
			}
			if diags := setGCPProjectStatus(ctx, &m, tc.account); diags.HasError() {
				t.Fatal(diags)
			}
			if m.Feature.IsNull() || len(m.Feature.Elements()) != 0 {
				t.Errorf("feature: got %v, want empty", m.Feature)
			}
			var cnp []gcpProjectCloudNativeProtectionModel
			if diags := m.CloudNativeProtection.ElementsAs(ctx, &cnp, false); diags.HasError() {
				t.Fatal(diags)
			}
			if len(cnp) != 1 || !cnp[0].Status.Equal(tc.want) {
				t.Errorf("cloud_native_protection: got %v, want status %v", m.CloudNativeProtection, tc.want)
			}
		})
	}
}

// TestGCPProjectUpdateModelOrganizationName checks that Read keeps a
// configured empty organization name when RSC has none, and stores RSC's value
// otherwise.
func TestGCPProjectUpdateModelOrganizationName(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name  string
		prior types.String
		rsc   string
		want  types.String
	}{
		{name: "NullPriorNoneInRSC", prior: types.StringNull(), rsc: "", want: types.StringNull()},
		{name: "EmptyPriorNoneInRSC", prior: types.StringValue(""), rsc: "", want: types.StringValue("")},
		{name: "PriorRemovedInRSC", prior: types.StringValue("org"), rsc: "", want: types.StringNull()},
		{name: "NullPriorInRSC", prior: types.StringNull(), rsc: "org", want: types.StringValue("org")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := gcpProjectResourceModel{
				OrganizationName:      tc.prior,
				Feature:               types.SetValueMust(gcpProjectFeatureType, nil),
				CloudNativeProtection: types.ListValueMust(gcpProjectCloudNativeProtectionType, nil),
			}
			account := gcp.CloudAccount{NativeID: testGCPProjectNativeID, OrganizationName: tc.rsc}
			if diags := updateGCPProjectResourceModel(ctx, &m, account); diags.HasError() {
				t.Fatal(diags)
			}
			if !m.OrganizationName.Equal(tc.want) {
				t.Errorf("organization_name: got %v, want %v", m.OrganizationName, tc.want)
			}
		})
	}
}
