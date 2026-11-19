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
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
)

func (r *awsAccountResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "awsAccountResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
		1: r.upgradeStateV1(),
		2: r.upgradeStateV2(),
	}
}

// upgradeStateV0 upgrades v0 state to v3, after checking with RSC that both
// parts of the v0 ID refer to the same account.
func (r *awsAccountResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: awsAccountSchemaV0,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "awsAccountResource.upgradeStateV0")

			var prior awsAccountResourceModelV0
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, d := upgradeAccountStateV0(ctx, prior)
			res.Diagnostics.Append(d...)
			if res.Diagnostics.HasError() {
				return
			}

			polarisClient, err := r.client.polaris()
			if err != nil {
				res.Diagnostics.AddError("RSC client error", err.Error())
				return
			}

			// Check that the cloud account ID refers to the exact same resource
			// as the old composite ID.
			cloudAccountID, err := uuid.Parse(state.ID.ValueString())
			if err != nil {
				res.Diagnostics.AddError("State upgrade failed", err.Error())
				return
			}
			account, err := aws.Wrap(polarisClient).AccountByID(ctx, cloudAccountID)
			if err != nil {
				res.Diagnostics.AddError("Failed to read AWS account", err.Error())
				return
			}
			if compositeID := prior.ID.ValueString(); compositeID != fmt.Sprintf("%s:%s", account.ID, account.NativeID) {
				res.Diagnostics.AddError("State upgrade failed", "v0 id refers to a different account")
				return
			}

			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeStateV1 upgrades v1 state to v3.
func (r *awsAccountResource) upgradeStateV1() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: awsAccountSchemaV1,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "awsAccountResource.upgradeStateV1")

			var prior awsAccountResourceModelV1
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, d := upgradeAccountStateV1(ctx, prior)
			res.Diagnostics.Append(d...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeStateV2 upgrades v2 state to v3.
func (r *awsAccountResource) upgradeStateV2() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: awsAccountSchemaV2,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "awsAccountResource.upgradeStateV2")

			var prior awsAccountResourceModelV2
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, d := upgradeAccountStateV2(ctx, prior)
			res.Diagnostics.Append(d...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeAccountStateV0 upgrades v0 state to v3. The ID is reduced to the
// RSC cloud account ID.
func upgradeAccountStateV0(ctx context.Context, prior awsAccountResourceModelV0) (awsAccountResourceModel, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	parts := strings.Split(prior.ID.ValueString(), ":")
	if len(parts) != 2 {
		diags.AddError("State upgrade failed", "invalid id format for v0 resource state")
		return awsAccountResourceModel{}, diags
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		diags.AddError("State upgrade failed", err.Error())
		return awsAccountResourceModel{}, diags
	}

	state, d := upgradeAccountStateV1(ctx, awsAccountResourceModelV1{
		ID:                       types.StringValue(id.String()),
		DeleteSnapshotsOnDestroy: prior.DeleteSnapshotsOnDestroy,
		Name:                     prior.Name,
		Profile:                  prior.Profile,
		Regions:                  prior.Regions,
	})
	diags.Append(d...)

	return state, diags
}

// upgradeAccountStateV1 upgrades v1 state to v3. The top-level regions become
// the cloud_native_protection block. The permission groups, status and stack
// ARN are left null, Read fills them in.
func upgradeAccountStateV1(ctx context.Context, prior awsAccountResourceModelV1) (awsAccountResourceModel, diag.Diagnostics) {
	var exocompute []awsAccountFeatureModelV2
	for _, block := range prior.Exocompute {
		exocompute = append(exocompute, awsAccountFeatureModelV2{
			PermissionGroups: types.SetNull(types.StringType),
			Regions:          block.Regions,
		})
	}

	// Cloud Native Protection always exists in v1 state.
	return upgradeAccountStateV2(ctx, awsAccountResourceModelV2{
		ID:         prior.ID,
		AssumeRole: types.StringNull(),
		CloudNativeProtection: []awsAccountFeatureModelV2{{
			PermissionGroups: types.SetNull(types.StringType),
			Regions:          prior.Regions,
		}},
		DeleteSnapshotsOnDestroy: prior.DeleteSnapshotsOnDestroy,
		Exocompute:               exocompute,
		Name:                     prior.Name,
		Permissions:              types.StringNull(),
		Profile:                  prior.Profile,
		RoleChainingAccountID:    types.StringNull(),
	})
}

// upgradeAccountStateV2 upgrades v2 state to v3. The resource stored unset
// strings as empty strings. They are stored as null, matching a configuration
// that leaves them out, so the first plan doesn't show them changing.
func upgradeAccountStateV2(ctx context.Context, prior awsAccountResourceModelV2) (awsAccountResourceModel, diag.Diagnostics) {
	nullEmpty := func(s types.String) types.String {
		if s.ValueString() == "" {
			return types.StringNull()
		}
		return s
	}
	feature := func(prior []awsAccountFeatureModelV2) []awsAccountFeatureModel {
		var model []awsAccountFeatureModel
		for _, block := range prior {
			model = append(model, awsAccountFeatureModel(block))
		}
		return model
	}

	var outpost []awsAccountOutpostModel
	for _, block := range prior.Outpost {
		outpost = append(outpost, awsAccountOutpostModel{
			OutpostAccountID:      nullEmpty(block.OutpostAccountID),
			OutpostAccountProfile: nullEmpty(block.OutpostAccountProfile),
			PermissionGroups:      block.PermissionGroups,
			StackARN:              block.StackARN,
			Status:                block.Status,
		})
	}
	var roleChaining []awsAccountRoleChainingModel
	for _, block := range prior.RoleChaining {
		roleChaining = append(roleChaining, awsAccountRoleChainingModel(block))
	}

	return awsAccountResourceModel{
		ID:                            prior.ID,
		AssumeRole:                    nullEmpty(prior.AssumeRole),
		CloudDiscovery:                feature(prior.CloudDiscovery),
		CloudNativeArchival:           feature(prior.CloudNativeArchival),
		CloudNativeConfigProtection:   feature(prior.CloudNativeConfigProtection),
		CloudNativeDynamoDBProtection: feature(prior.CloudNativeDynamoDBProtection),
		CloudNativeProtection:         feature(prior.CloudNativeProtection),
		CloudNativeS3Protection:       feature(prior.CloudNativeS3Protection),
		CloudNativeUEMKeyManagement:   feature(prior.CloudNativeUEMKeyManagement),
		CyberRecoveryDataScanning:     feature(prior.CyberRecoveryDataScanning),
		DataScanning:                  feature(prior.DataScanning),
		DeleteSnapshotsOnDestroy:      prior.DeleteSnapshotsOnDestroy,
		DSPM:                          feature(prior.DSPM),
		Exocompute:                    feature(prior.Exocompute),
		KubernetesProtection:          feature(prior.KubernetesProtection),
		Name:                          nullEmpty(prior.Name),
		Outpost:                       outpost,
		Permissions:                   nullEmpty(prior.Permissions),
		Profile:                       nullEmpty(prior.Profile),
		RDSProtection:                 feature(prior.RDSProtection),
		RoleChaining:                  roleChaining,
		RoleChainingAccountID:         nullEmpty(prior.RoleChainingAccountID),
		ServersAndApps:                feature(prior.ServersAndApps),
	}, nil
}

// awsAccountSchemaV0 is the schema of the resource in the v0 state.
var awsAccountSchemaV0 = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:                       schema.StringAttribute{Optional: true, Computed: true},
		keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{Optional: true},
		keyName:                     schema.StringAttribute{Optional: true},
		keyProfile:                  schema.StringAttribute{Required: true},
		keyRegions:                  schema.SetAttribute{ElementType: types.StringType, Required: true},
	},
}

// awsAccountResourceModelV0 is the model of the v0 state. The ID is the RSC
// cloud account ID and the AWS account ID separated by a colon.
type awsAccountResourceModelV0 struct {
	ID                       types.String `tfsdk:"id"`
	DeleteSnapshotsOnDestroy types.Bool   `tfsdk:"delete_snapshots_on_destroy"`
	Name                     types.String `tfsdk:"name"`
	Profile                  types.String `tfsdk:"profile"`
	Regions                  types.Set    `tfsdk:"regions"`
}

// awsAccountSchemaV1 is the schema of the resource in the v1 state.
var awsAccountSchemaV1 = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:                       schema.StringAttribute{Optional: true, Computed: true},
		keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{Optional: true},
		keyName:                     schema.StringAttribute{Optional: true, Computed: true},
		keyProfile:                  schema.StringAttribute{Required: true},
		keyRegions:                  schema.SetAttribute{ElementType: types.StringType, Required: true},
	},
	Blocks: map[string]schema.Block{
		keyExocompute: awsAccountExocomputeBlockV1,
	},
}

// awsAccountExocomputeBlockV1 is the schema of nested exocompute blocks in the
// v1 state.
var awsAccountExocomputeBlockV1 = schema.ListNestedBlock{
	NestedObject: schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			keyRegions: schema.SetAttribute{ElementType: types.StringType, Required: true},
		},
	},
}

// awsAccountResourceModelV1 is the model of the v1 state. The ID is the RSC
// cloud account ID.
type awsAccountResourceModelV1 struct {
	ID                       types.String                  `tfsdk:"id"`
	DeleteSnapshotsOnDestroy types.Bool                    `tfsdk:"delete_snapshots_on_destroy"`
	Exocompute               []awsAccountExocomputeModelV1 `tfsdk:"exocompute"`
	Name                     types.String                  `tfsdk:"name"`
	Profile                  types.String                  `tfsdk:"profile"`
	Regions                  types.Set                     `tfsdk:"regions"`
}

// awsAccountExocomputeModelV1 is the model of nested exocompute blocks in the
// v1 state.
type awsAccountExocomputeModelV1 struct {
	Regions types.Set `tfsdk:"regions"`
}

// awsAccountSchemaV2 is the schema of the resource in the v2 state.
var awsAccountSchemaV2 = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:                       schema.StringAttribute{Computed: true},
		keyAssumeRole:               schema.StringAttribute{Optional: true},
		keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{Optional: true},
		keyName:                     schema.StringAttribute{Optional: true, Computed: true},
		keyPermissions:              schema.StringAttribute{Optional: true, Computed: true},
		keyProfile:                  schema.StringAttribute{Optional: true},
		keyRoleChainingAccountID:    schema.StringAttribute{Optional: true},
	},
	Blocks: map[string]schema.Block{
		keyCloudDiscovery:                awsAccountFeatureBlockV2,
		keyCloudNativeArchival:           awsAccountFeatureBlockV2,
		keyCloudNativeConfigProtection:   awsAccountFeatureBlockV2,
		keyCloudNativeDynamoDBProtection: awsAccountFeatureBlockV2,
		keyCloudNativeProtection:         awsAccountFeatureBlockV2,
		keyCloudNativeS3Protection:       awsAccountFeatureBlockV2,
		keyCloudNativeUEMKeyManagement:   awsAccountFeatureBlockV2,
		keyCyberRecoveryDataScanning:     awsAccountFeatureBlockV2,
		keyDataScanning:                  awsAccountFeatureBlockV2,
		keyDSPM:                          awsAccountFeatureBlockV2,
		keyExocompute:                    awsAccountFeatureBlockV2,
		keyKubernetesProtection:          awsAccountFeatureBlockV2,
		keyOutpost:                       awsAccountOutpostBlockV2,
		keyRDSProtection:                 awsAccountFeatureBlockV2,
		keyRoleChaining:                  awsAccountRoleChainingBlockV2,
		keyServersAndApps:                awsAccountFeatureBlockV2,
	},
}

// awsAccountFeatureBlockV2 is the schema of nested regular feature blocks in
// the v2 state.
var awsAccountFeatureBlockV2 = schema.ListNestedBlock{
	NestedObject: schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			keyPermissionGroups: schema.SetAttribute{ElementType: types.StringType, Required: true},
			keyRegions:          schema.SetAttribute{ElementType: types.StringType, Required: true},
			keyStackARN:         schema.StringAttribute{Computed: true},
			keyStatus:           schema.StringAttribute{Computed: true},
		},
	},
}

// awsAccountOutpostBlockV2 is the schema of a nested outpost block in the v2
// state.
var awsAccountOutpostBlockV2 = schema.ListNestedBlock{
	NestedObject: schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			keyOutpostAccountID:      schema.StringAttribute{Optional: true},
			keyOutpostAccountProfile: schema.StringAttribute{Optional: true},
			keyPermissionGroups:      schema.SetAttribute{ElementType: types.StringType, Required: true},
			keyStackARN:              schema.StringAttribute{Computed: true},
			keyStatus:                schema.StringAttribute{Computed: true},
		},
	},
}

// awsAccountRoleChainingBlockV2 is the schema of nested role chaining blocks in
// the v2 state.
var awsAccountRoleChainingBlockV2 = schema.ListNestedBlock{
	NestedObject: schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			keyPermissionGroups: schema.SetAttribute{ElementType: types.StringType, Required: true},
			keyStackARN:         schema.StringAttribute{Computed: true},
			keyStatus:           schema.StringAttribute{Computed: true},
		},
	},
}

// awsAccountResourceModelV2 is the model of the v2 state.
type awsAccountResourceModelV2 struct {
	ID                            types.String                    `tfsdk:"id"`
	AssumeRole                    types.String                    `tfsdk:"assume_role"`
	CloudDiscovery                []awsAccountFeatureModelV2      `tfsdk:"cloud_discovery"`
	CloudNativeArchival           []awsAccountFeatureModelV2      `tfsdk:"cloud_native_archival"`
	CloudNativeConfigProtection   []awsAccountFeatureModelV2      `tfsdk:"cloud_native_config_protection"`
	CloudNativeDynamoDBProtection []awsAccountFeatureModelV2      `tfsdk:"cloud_native_dynamodb_protection"`
	CloudNativeProtection         []awsAccountFeatureModelV2      `tfsdk:"cloud_native_protection"`
	CloudNativeS3Protection       []awsAccountFeatureModelV2      `tfsdk:"cloud_native_s3_protection"`
	CloudNativeUEMKeyManagement   []awsAccountFeatureModelV2      `tfsdk:"cloud_native_uem_key_management"`
	CyberRecoveryDataScanning     []awsAccountFeatureModelV2      `tfsdk:"cyber_recovery_data_scanning"`
	DataScanning                  []awsAccountFeatureModelV2      `tfsdk:"data_scanning"`
	DeleteSnapshotsOnDestroy      types.Bool                      `tfsdk:"delete_snapshots_on_destroy"`
	DSPM                          []awsAccountFeatureModelV2      `tfsdk:"dspm"`
	Exocompute                    []awsAccountFeatureModelV2      `tfsdk:"exocompute"`
	KubernetesProtection          []awsAccountFeatureModelV2      `tfsdk:"kubernetes_protection"`
	Name                          types.String                    `tfsdk:"name"`
	Outpost                       []awsAccountOutpostModelV2      `tfsdk:"outpost"`
	Permissions                   types.String                    `tfsdk:"permissions"`
	Profile                       types.String                    `tfsdk:"profile"`
	RDSProtection                 []awsAccountFeatureModelV2      `tfsdk:"rds_protection"`
	RoleChaining                  []awsAccountRoleChainingModelV2 `tfsdk:"role_chaining"`
	RoleChainingAccountID         types.String                    `tfsdk:"role_chaining_account_id"`
	ServersAndApps                []awsAccountFeatureModelV2      `tfsdk:"servers_and_apps"`
}

// awsAccountFeatureModelV2 is the model of nested regular feature blocks in the
// v2 state.
type awsAccountFeatureModelV2 struct {
	PermissionGroups types.Set    `tfsdk:"permission_groups"`
	Regions          types.Set    `tfsdk:"regions"`
	StackARN         types.String `tfsdk:"stack_arn"`
	Status           types.String `tfsdk:"status"`
}

// awsAccountOutpostModelV2 is the model of a nested outpost block in the v2
// state.
type awsAccountOutpostModelV2 struct {
	OutpostAccountID      types.String `tfsdk:"outpost_account_id"`
	OutpostAccountProfile types.String `tfsdk:"outpost_account_profile"`
	PermissionGroups      types.Set    `tfsdk:"permission_groups"`
	StackARN              types.String `tfsdk:"stack_arn"`
	Status                types.String `tfsdk:"status"`
}

// awsAccountRoleChainingModelV2 is the model of nested role chaining blocks in
// the v2 state.
type awsAccountRoleChainingModelV2 struct {
	PermissionGroups types.Set    `tfsdk:"permission_groups"`
	StackARN         types.String `tfsdk:"stack_arn"`
	Status           types.String `tfsdk:"status"`
}
