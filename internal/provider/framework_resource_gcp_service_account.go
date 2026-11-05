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
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
)

const resourceGCPServiceAccountDescription = `
The ´rubrik_gcp_service_account´ resource adds the GCP service account to RSC
as the default service account. The default service account will be used by RSC
to authenticate to the GCP for projects added to RSC without a service account.

~> **Note:** RSC always uses the client email of the service account key as
   the name of the default service account, the ´name´ field is not sent to
   RSC. If ´name´ is set to anything other than the client email, every plan
   shows a change. Use the ´ignore_changes´ field of the ´lifecycle´ block if
   it becomes an issue.

~> **Note:** Destroying the ´rubrik_gcp_service_account´ resource only updates
   the local state, it does not remove the service account from RSC. However,
   it's possible to overwrite the RSC global service account with new service
   accounts.

-> **Note:** There is no way to verify if an default GCP service account has
   been added to RSC using the UI.
`

// The resource has no identity and no import. RSC holds a single default
// service account and never returns its key, so an imported resource could
// not be planned without a change.
var (
	_ resource.Resource                 = &gcpServiceAccountResource{}
	_ resource.ResourceWithMoveState    = &gcpServiceAccountResource{}
	_ resource.ResourceWithUpgradeState = &gcpServiceAccountResource{}
)

type gcpServiceAccountResource struct {
	client *client
	prefix string
}

type gcpServiceAccountModel struct {
	ID              types.String `tfsdk:"id"`
	Credentials     types.String `tfsdk:"credentials"`
	Name            types.String `tfsdk:"name"`
	PermissionsHash types.String `tfsdk:"permissions_hash"`
}

func newGcpServiceAccountResource() resource.Resource {
	return &gcpServiceAccountResource{prefix: keyRubrik}
}

func newPolarisGcpServiceAccountResource() resource.Resource {
	return &gcpServiceAccountResource{prefix: keyPolaris}
}

func (r *gcpServiceAccountResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpServiceAccountResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPServiceAccount
}

func (r *gcpServiceAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpServiceAccountResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPServiceAccountDescription),
		Version:     1,
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed: true,
				Description: "SHA-256 hash of the `name` field when the resource was created, or of a generated " +
					"name if `name` isn't set. It is not derived from the name in RSC.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCredentials: schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Base64 encoded GCP service account private key or path to GCP service account key " +
					"file.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyName: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Service account name in RSC. RSC always uses the client email of the service account " +
					"key as the name, the value is not sent to RSC.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyPermissionsHash: schema.StringAttribute{
				Optional: true,
				Description: "Signals that the permissions has been updated. **Deprecated:** use the " +
					"`permissions` field of the `feature` block of the `rubrik_gcp_project` resource instead.",
				DeprecationMessage: "Use the `permissions` field of the `feature` block of `rubrik_gcp_project` " +
					"instead.",
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_gcp_service_account` instead."
	}
}

func (r *gcpServiceAccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpServiceAccountResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpServiceAccountResource.Create")

	var plan gcpServiceAccountModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// A name that isn't configured is unknown in the plan, and is the only
	// case where the name read back from RSC can be saved to state. Terraform
	// rejects a state value that differs from a configured value.
	readBackName := plan.Name.IsUnknown()
	name := plan.Name.ValueString()
	if name == "" {
		name = fmt.Sprintf("service-account-%d", time.Now().UnixMicro())
	}

	err = gcp.Wrap(polarisClient).SetServiceAccount(ctx, gcp.Key(plan.Credentials.ValueString()), gcp.Name(name))
	if err != nil {
		res.Diagnostics.AddError("Failed to set GCP default service account", err.Error())
		return
	}

	// The ID is derived from the name in the plan, not the name RSC returns,
	// as in the SDKv2 resource. The state is saved before the read-back, so a
	// failed read doesn't lose the resource.
	plan.ID = types.StringValue(fmt.Sprintf("%x", sha256.Sum256([]byte(name))))
	plan.Name = types.StringValue(name)
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() || !readBackName {
		return
	}

	rscName, err := gcp.Wrap(polarisClient).ServiceAccount(ctx)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read GCP default service account after create",
			fmt.Sprintf("The GCP default service account was set but could not be read back: %s", err))
		return
	}
	plan.Name = types.StringValue(rscName)
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *gcpServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpServiceAccountResource.Read")

	var state gcpServiceAccountModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// RSC has no way to remove the default service account, so there is no
	// not found case.
	name, err := gcp.Wrap(polarisClient).ServiceAccount(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP default service account", err.Error())
		return
	}
	state.Name = types.StringValue(name)

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *gcpServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpServiceAccountResource.Update")

	var plan gcpServiceAccountModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state gcpServiceAccountModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	if !plan.Credentials.Equal(state.Credentials) || !plan.Name.Equal(state.Name) {
		err := gcp.Wrap(polarisClient).SetServiceAccount(ctx, gcp.Key(plan.Credentials.ValueString()),
			gcp.Name(plan.Name.ValueString()))
		if err != nil {
			res.Diagnostics.AddError("Failed to set GCP default service account", err.Error())
			return
		}
	}

	if !plan.PermissionsHash.Equal(state.PermissionsHash) {
		if err := gcp.Wrap(polarisClient).PermissionsUpdatedForDefault(ctx, nil); err != nil {
			res.Diagnostics.AddError("Failed to signal GCP default service account permissions updated",
				err.Error())
			return
		}
	}

	// As in the SDKv2 resource, the ID is not changed when the name changes.
	plan.ID = state.ID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

// Delete only removes the resource from state. The default service account
// can't be removed using the RSC API.
func (r *gcpServiceAccountResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpServiceAccountResource.Delete")
}
