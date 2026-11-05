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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

// gcpProjectResourceModelV0 is the model of the v0 and v1 state, the two
// versions have the same schema. The ID of v0 is the RSC cloud account ID and
// the GCP project ID separated by a colon, the ID of v1 is the RSC cloud
// account ID.
type gcpProjectResourceModelV0 struct {
	ID                       types.String `tfsdk:"id"`
	Credentials              types.String `tfsdk:"credentials"`
	DeleteSnapshotsOnDestroy types.Bool   `tfsdk:"delete_snapshots_on_destroy"`
	OrganizationName         types.String `tfsdk:"organization_name"`
	Project                  types.String `tfsdk:"project"`
	ProjectName              types.String `tfsdk:"project_name"`
	ProjectNumber            types.String `tfsdk:"project_number"`
}

// gcpProjectSchemaV0 is the SDKv2 schema of the v0 and v1 state.
var gcpProjectSchemaV0 = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:                       schema.StringAttribute{Computed: true},
		keyCredentials:              schema.StringAttribute{Optional: true},
		keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{Optional: true},
		keyOrganizationName:         schema.StringAttribute{Optional: true, Computed: true},
		keyProject:                  schema.StringAttribute{Optional: true, Computed: true},
		keyProjectName:              schema.StringAttribute{Optional: true, Computed: true},
		keyProjectNumber:            schema.StringAttribute{Optional: true, Computed: true},
	},
}

// gcpProjectSchemaV2 is the SDKv2 schema of the v2 state.
var gcpProjectSchemaV2 = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:                       schema.StringAttribute{Computed: true},
		keyCredentials:              schema.StringAttribute{Optional: true},
		keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{Optional: true},
		keyOrganizationName:         schema.StringAttribute{Optional: true},
		keyPermissionsHash:          schema.StringAttribute{Optional: true},
		keyProject:                  schema.StringAttribute{Required: true},
		keyProjectName:              schema.StringAttribute{Required: true},
		keyProjectNumber:            schema.StringAttribute{Required: true},
	},
	Blocks: map[string]schema.Block{
		keyCloudNativeProtection: schema.ListNestedBlock{
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					keyStatus: schema.StringAttribute{Optional: true, Computed: true},
				},
			},
		},
		keyFeature: schema.SetNestedBlock{
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					keyName:             schema.StringAttribute{Required: true},
					keyPermissionGroups: schema.SetAttribute{ElementType: types.StringType, Required: true},
					keyPermissions:      schema.StringAttribute{Optional: true},
					keyStatus:           schema.StringAttribute{Computed: true},
				},
			},
		},
	},
}

func (r *gcpProjectResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "gcpProjectResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
		1: r.upgradeStateV1(),
		2: r.upgradeStateV2(),
	}
}

// upgradeStateV0 upgrades v0 state to v3. The ID is reduced to the RSC cloud
// account ID, after checking that both parts refer to the same project, and
// the Cloud Native Protection block is added.
func (r *gcpProjectResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: gcpProjectSchemaV0,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "gcpProjectResource.upgradeStateV0")

			var prior gcpProjectResourceModelV0
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			polarisClient, err := r.client.polaris()
			if err != nil {
				res.Diagnostics.AddError("RSC client error", err.Error())
				return
			}

			parts := strings.Split(prior.ID.ValueString(), ":")
			if len(parts) != 2 {
				res.Diagnostics.AddError("State upgrade failed", "invalid id format for v0 resource state")
				return
			}
			id, err := uuid.Parse(parts[0])
			if err != nil {
				res.Diagnostics.AddError("State upgrade failed", err.Error())
				return
			}

			account, err := gcp.Wrap(polarisClient).ProjectByID(ctx, id)
			if err != nil {
				res.Diagnostics.AddError("Failed to read GCP project", err.Error())
				return
			}
			byNativeID, err := gcp.Wrap(polarisClient).ProjectByNativeID(ctx, parts[1])
			if err != nil {
				res.Diagnostics.AddError("Failed to read GCP project", err.Error())
				return
			}
			if account.ID != byNativeID.ID {
				res.Diagnostics.AddError("State upgrade failed", "v0 id refers to two different accounts")
				return
			}

			prior.ID = types.StringValue(account.ID.String())
			state, diags := gcpProjectStateFromV1(ctx, prior, &account)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeStateV1 upgrades v1 state to v3 by adding the Cloud Native Protection
// block.
//
// The SDKv2 state upgrader read the status from the Exocompute feature and
// failed for a project without it. This reads the status from the Cloud Native
// Protection feature, and a project without it gets an empty block.
func (r *gcpProjectResource) upgradeStateV1() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: gcpProjectSchemaV0,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "gcpProjectResource.upgradeStateV1")

			var prior gcpProjectResourceModelV0
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			polarisClient, err := r.client.polaris()
			if err != nil {
				res.Diagnostics.AddError("RSC client error", err.Error())
				return
			}

			id, err := uuid.Parse(prior.ID.ValueString())
			if err != nil {
				res.Diagnostics.AddError("State upgrade failed", err.Error())
				return
			}
			account, err := gcp.Wrap(polarisClient).ProjectByID(ctx, id)
			if err != nil {
				res.Diagnostics.AddError("Failed to read GCP project", err.Error())
				return
			}

			state, diags := gcpProjectStateFromV1(ctx, prior, &account)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeStateV2 upgrades v2 state, written by the SDKv2 resource, to v3. The
// schema is the same, only the values are cleaned up, see
// gcpProjectStateFromV2.
func (r *gcpProjectResource) upgradeStateV2() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: gcpProjectSchemaV2,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "gcpProjectResource.upgradeStateV2")

			var prior gcpProjectResourceModel
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, diags := gcpProjectStateFromV2(ctx, prior)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// gcpProjectStateFromV1 builds v3 state from v1 state, the v0 ID must already
// have been reduced to the RSC cloud account ID. The v1 configuration had no
// features, the SDKv2 resource onboarded the project with Cloud Native
// Protection only.
//
// The state upgraders read the project from RSC and pass it as the account.
// The Cloud Native Protection block then holds the status of the Cloud Native
// Protection feature, and is null if the project doesn't have the feature.
// The state movers pass a nil account, since Terraform doesn't configure the
// provider before moving state and RSC can't be reached. The block is then
// null, and Read fills in the features.
//
// The SDKv2 resource stored an unset string as an empty string, these are
// stored as null, matching a configuration leaving them out.
func gcpProjectStateFromV1(ctx context.Context, prior gcpProjectResourceModelV0, account *gcp.CloudAccount) (gcpProjectResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	// A nil slice gives a null list for the unused block.
	var cnp []gcpProjectCloudNativeProtectionModel
	if account != nil {
		if feature, ok := account.Feature(core.FeatureCloudNativeProtection); ok {
			cnp = append(cnp, gcpProjectCloudNativeProtectionModel{Status: types.StringValue(string(feature.Status))})
		}
	}
	list, d := types.ListValueFrom(ctx, gcpProjectCloudNativeProtectionType, cnp)
	diags.Append(d...)

	deleteSnapshots := prior.DeleteSnapshotsOnDestroy
	if deleteSnapshots.IsNull() {
		deleteSnapshots = types.BoolValue(false)
	}

	if _, err := uuid.Parse(prior.ID.ValueString()); err != nil {
		diags.AddError("State upgrade failed", fmt.Sprintf("invalid RSC cloud account ID %q: %s",
			prior.ID.ValueString(), err))
	}

	for _, v := range []*types.String{&prior.Credentials, &prior.OrganizationName} {
		if v.ValueString() == "" {
			*v = types.StringNull()
		}
	}

	return gcpProjectResourceModel{
		ID:                       prior.ID,
		CloudNativeProtection:    list,
		Credentials:              prior.Credentials,
		DeleteSnapshotsOnDestroy: deleteSnapshots,
		Feature:                  types.SetValueMust(gcpProjectFeatureType, nil),
		OrganizationName:         prior.OrganizationName,
		PermissionsHash:          types.StringNull(),
		Project:                  prior.Project,
		ProjectName:              prior.ProjectName,
		ProjectNumber:            prior.ProjectNumber,
	}, diags
}

// gcpProjectStateFromV2 builds v3 state from v2 state written by the SDKv2
// resource. It's used by the state upgrader and the state mover, and kept
// separate so it can be unit tested.
//
// The SDKv2 resource stored an unset string as an empty string, these are
// stored as null, matching a configuration leaving them out. Otherwise the
// first plan would show them changing to null, and an empty credentials
// change would make Update set the service account with an empty key.
//
// The SDKv2 resource also populated both blocks, only the block in use is
// kept. With features, the Cloud Native Protection block was read from the
// features and is dropped. A block which isn't in use is null, as in
// gcpProjectStateFromV1.
func gcpProjectStateFromV2(ctx context.Context, state gcpProjectResourceModel) (gcpProjectResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	for _, v := range []*types.String{&state.Credentials, &state.OrganizationName, &state.PermissionsHash} {
		if v.ValueString() == "" {
			*v = types.StringNull()
		}
	}
	if state.DeleteSnapshotsOnDestroy.IsNull() {
		state.DeleteSnapshotsOnDestroy = types.BoolValue(false)
	}

	features := []gcpProjectFeatureModel{}
	diags.Append(state.Feature.ElementsAs(ctx, &features, false)...)
	if diags.HasError() {
		return state, diags
	}
	for i := range features {
		if features[i].Permissions.ValueString() == "" {
			features[i].Permissions = types.StringNull()
		}
	}
	set, d := types.SetValueFrom(ctx, gcpProjectFeatureType, features)
	diags.Append(d...)
	if diags.HasError() {
		return state, diags
	}
	state.Feature = set

	if len(state.Feature.Elements()) > 0 || len(state.CloudNativeProtection.Elements()) == 0 {
		state.CloudNativeProtection = types.ListNull(gcpProjectCloudNativeProtectionType)
	}

	return state, diags
}
