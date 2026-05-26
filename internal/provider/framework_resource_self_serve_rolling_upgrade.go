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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
)

const resourceSelfServeRollingUpgradeDescription = `
The ´rubrik_self_serve_rolling_upgrade´ resource manages the account-wide
self-serve rolling upgrade setting in RSC.

This is a singleton resource. Only one instance per RSC tenant is meaningful;
managing the same setting in multiple Terraform workspaces will cause drift.

Deleting the resource removes it from Terraform state but leaves the underlying
RSC toggle unchanged. To disable the feature, apply with ´enabled = false´
before removing the resource block.
`

// selfServeRollingUpgradeID is the singleton identifier used in TF state.
const selfServeRollingUpgradeID = "self_serve_rolling_upgrade"

var (
	_ resource.Resource                = &selfServeRollingUpgradeResource{}
	_ resource.ResourceWithImportState = &selfServeRollingUpgradeResource{}
)

type selfServeRollingUpgradeResource struct {
	client *client
}

type selfServeRollingUpgradeModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func newSelfServeRollingUpgradeResource() resource.Resource {
	return &selfServeRollingUpgradeResource{}
}

func (r *selfServeRollingUpgradeResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.Metadata")

	res.TypeName = keyRubrik + "_" + keySelfServeRollingUpgrade
}

func (r *selfServeRollingUpgradeResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceSelfServeRollingUpgradeDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Singleton ID. Always `self_serve_rolling_upgrade`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyEnabled: schema.BoolAttribute{
				Required:    true,
				Description: "Whether self-serve rolling upgrade is enabled for the account.",
			},
		},
	}
}

func (r *selfServeRollingUpgradeResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *selfServeRollingUpgradeResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.Create")

	var plan selfServeRollingUpgradeModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	if err := cluster.Wrap(polarisClient).SetSelfServeRollingUpgrade(ctx, plan.Enabled.ValueBool()); err != nil {
		res.Diagnostics.AddError("Failed to set self-serve rolling upgrade", err.Error())
		return
	}

	state := selfServeRollingUpgradeModel{
		ID:      types.StringValue(selfServeRollingUpgradeID),
		Enabled: plan.Enabled,
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *selfServeRollingUpgradeResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.Read")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	enabled, err := cluster.Wrap(polarisClient).SelfServeRollingUpgrade(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to read self-serve rolling upgrade", err.Error())
		return
	}

	state := selfServeRollingUpgradeModel{
		ID:      types.StringValue(selfServeRollingUpgradeID),
		Enabled: types.BoolValue(enabled),
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *selfServeRollingUpgradeResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.Update")

	var plan selfServeRollingUpgradeModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	if err := cluster.Wrap(polarisClient).SetSelfServeRollingUpgrade(ctx, plan.Enabled.ValueBool()); err != nil {
		res.Diagnostics.AddError("Failed to set self-serve rolling upgrade", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *selfServeRollingUpgradeResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.Delete")

	// No-op: the underlying RSC setting is global and cannot be deleted.
	// Removing the resource only drops it from Terraform state; to disable
	// the feature, set enabled = false before removing the resource block.
}

func (r *selfServeRollingUpgradeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "selfServeRollingUpgradeResource.ImportState")

	// Singleton resource: the import ID is ignored and replaced with the
	// constant ID. Read populates enabled.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), selfServeRollingUpgradeID)...)
}
