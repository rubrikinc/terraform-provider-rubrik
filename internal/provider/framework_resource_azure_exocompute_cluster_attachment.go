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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/exocompute"
)

const resourceAzureExocomputeClusterAttachmentDescription = `
The ´rubrik_azure_exocompute_cluster_attachment´ resource attaches an Azure AKS
cluster to a customer managed host Exocompute configuration, allowing RSC to use
the cluster for Exocompute operations.

The cluster name must be specified as ´<resource-group>/<cluster-name>´, e.g.
´my-resource-group/my-cluster´.
`

var (
	_ resource.Resource              = &azureExocomputeClusterAttachmentResource{}
	_ resource.ResourceWithConfigure = &azureExocomputeClusterAttachmentResource{}
	_ resource.ResourceWithIdentity  = &azureExocomputeClusterAttachmentResource{}
)

type azureExocomputeClusterAttachmentResource struct {
	client *client
	prefix string
}

type azureExocomputeClusterAttachmentModel struct {
	ID           types.String `tfsdk:"id"`
	ClusterName  types.String `tfsdk:"cluster_name"`
	ExocomputeID types.String `tfsdk:"exocompute_id"`
	Manifest     types.String `tfsdk:"manifest"`
	TokenRefresh types.Int64  `tfsdk:"token_refresh"`
}

// azureExocomputeClusterAttachmentIdentityModel is the resource identity.
type azureExocomputeClusterAttachmentIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAzureExocomputeClusterAttachmentResource() resource.Resource {
	return &azureExocomputeClusterAttachmentResource{prefix: keyRubrik}
}

func newPolarisAzureExocomputeClusterAttachmentResource() resource.Resource {
	return &azureExocomputeClusterAttachmentResource{prefix: keyPolaris}
}

func (r *azureExocomputeClusterAttachmentResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureExocomputeClusterAttachment
}

// Schema returns the schema of the resource. The resource is deprecated, and
// the documentation generator skips deprecated resources without a template,
// so the template must not be removed.
func (r *azureExocomputeClusterAttachmentResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.Schema")

	res.Schema = schema.Schema{
		Description:        description(resourceAzureExocomputeClusterAttachmentDescription),
		DeprecationMessage: "customer managed Exocompute is no longer supported by RSC.",
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cluster ID (UUID).",
			},
			keyClusterName: schema.StringAttribute{
				Required:    true,
				Description: "Azure AKS cluster name. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyExocomputeID: schema.StringAttribute{
				Required: true,
				Description: "RSC exocompute configuration ID (UUID). Changing this forces a new resource to be " +
					"created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyManifest: schema.StringAttribute{
				Computed: true,
				Description: "Kubernetes manifest which can be passed to `kubectl apply` to create a connection " +
					"between the cluster and RSC.",
			},
			keyTokenRefresh: schema.Int64Attribute{
				Optional: true,
				Description: "To force a refresh of the authentication token, part of the manifest, increase the " +
					"value of this field. The token is valid for 24 hours.",
			},
		},
	}
}

func (r *azureExocomputeClusterAttachmentResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC cluster ID (UUID).",
			},
		},
	}
}

func (r *azureExocomputeClusterAttachmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureExocomputeClusterAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.Create")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan azureExocomputeClusterAttachmentModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	configID, err := uuid.Parse(plan.ExocomputeID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyExocomputeID), "Invalid Exocompute ID", err.Error())
		return
	}

	// The cluster connection isn't read back, reading it fails until the
	// manifest has been applied to the cluster.
	clusterID, info, err := exocompute.Wrap(polarisClient).ConnectAzureCluster(ctx, plan.ClusterName.ValueString(), configID)
	if err != nil {
		res.Diagnostics.AddError("Failed to connect Azure cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID.String())
	plan.Manifest = types.StringValue(info.Manifest)
	res.Diagnostics.Append(res.State.Set(ctx, plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azureExocomputeClusterAttachmentIdentityModel{ID: plan.ID})...)
}

func (r *azureExocomputeClusterAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.Read")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state azureExocomputeClusterAttachmentModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	configID, err := uuid.Parse(state.ExocomputeID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid Exocompute ID", err.Error())
		return
	}

	// A failed read keeps the state as is, same as the SDKv2 resource.
	info, err := exocompute.Wrap(polarisClient).AzureClusterConnection(ctx, state.ClusterName.ValueString(), configID)
	if err != nil {
		tflog.Warn(ctx, "failed to read cluster attachment", map[string]any{"err": err.Error()})
		return
	}

	state.Manifest = types.StringValue(info.Manifest)
	res.Diagnostics.Append(res.State.Set(ctx, state)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azureExocomputeClusterAttachmentIdentityModel{ID: state.ID})...)
}

// Update connects the cluster again, which refreshes the authentication token
// in the manifest. The token_refresh field is the only field that can be
// updated in place.
func (r *azureExocomputeClusterAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.Update")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan azureExocomputeClusterAttachmentModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	configID, err := uuid.Parse(plan.ExocomputeID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyExocomputeID), "Invalid Exocompute ID", err.Error())
		return
	}

	clusterID, info, err := exocompute.Wrap(polarisClient).ConnectAzureCluster(ctx, plan.ClusterName.ValueString(), configID)
	if err != nil {
		res.Diagnostics.AddError("Failed to connect Azure cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID.String())
	plan.Manifest = types.StringValue(info.Manifest)
	res.Diagnostics.Append(res.State.Set(ctx, plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azureExocomputeClusterAttachmentIdentityModel{ID: plan.ID})...)
}

func (r *azureExocomputeClusterAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureExocomputeClusterAttachmentResource.Delete")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state azureExocomputeClusterAttachmentModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	if err := exocompute.Wrap(polarisClient).DisconnectAzureCluster(ctx, id); err != nil {
		res.Diagnostics.AddError("Failed to disconnect Azure cluster", err.Error())
	}
}
