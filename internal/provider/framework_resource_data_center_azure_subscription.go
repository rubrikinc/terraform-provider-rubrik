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
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
)

const resourceDataCenterAzureSubscriptionDescription = `
The ´rubrik_data_center_azure_subscription´ resource adds a data center Azure
subscription to RSC. A data center subscription can only be used with data
center archival.

~> **Note:** Due to technical issue in RSC, names of removed data center Azure
   subscriptions cannot be reused.

-> **Note:** Data center subscriptions and cloud native subscriptions are
   different and cannot be used interchangeably.
`

var (
	_ resource.Resource                = &dataCenterAzureSubscriptionResource{}
	_ resource.ResourceWithIdentity    = &dataCenterAzureSubscriptionResource{}
	_ resource.ResourceWithImportState = &dataCenterAzureSubscriptionResource{}
	_ resource.ResourceWithMoveState   = &dataCenterAzureSubscriptionResource{}
)

type dataCenterAzureSubscriptionResource struct {
	client *client
	prefix string
}

type dataCenterAzureSubscriptionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Description    types.String `tfsdk:"description"`
	Name           types.String `tfsdk:"name"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	TenantID       types.String `tfsdk:"tenant_id"`
}

type dataCenterAzureSubscriptionIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newDataCenterAzureSubscriptionResource() resource.Resource {
	return &dataCenterAzureSubscriptionResource{prefix: keyRubrik}
}

func newPolarisDataCenterAzureSubscriptionResource() resource.Resource {
	return &dataCenterAzureSubscriptionResource{prefix: keyPolaris}
}

func (r *dataCenterAzureSubscriptionResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.Metadata")

	res.TypeName = r.prefix + "_" + keyDataCenterAzureSubscription
}

func (r *dataCenterAzureSubscriptionResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceDataCenterAzureSubscriptionDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC data center cloud account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyDescription: schema.StringAttribute{
				Optional: true,
				Computed: true,
				// SDKv2 stores an unset description as the empty string.
				// Defaulting to the empty string keeps state written by SDKv2
				// from showing a diff against a configuration without a
				// description.
				Default:     stringdefault.StaticString(""),
				Description: "Data center subscription description.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Data center subscription name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keySubscriptionID: schema.StringAttribute{
				Required:    true,
				Description: "Azure subscription ID (UUID).",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyTenantID: schema.StringAttribute{
				Computed:    true,
				Description: "Azure tenant ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_data_center_azure_subscription` instead."
	}
}

func (r *dataCenterAzureSubscriptionResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC data center cloud account ID (UUID).",
			},
		},
	}
}

func (r *dataCenterAzureSubscriptionResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *dataCenterAzureSubscriptionResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.Create")

	var plan dataCenterAzureSubscriptionResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := archival.Wrap(polarisClient).CreateAzureCloudAccount(ctx, gqlarchival.CreateAzureCloudAccountParams{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		SubscriptionID: plan.SubscriptionID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to create data center Azure subscription", err.Error())
		return
	}

	// Save the ID to state and identity before the read-back so Terraform can
	// track the subscription even if the read fails.
	plan.ID = types.StringValue(id.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := dataCenterAzureSubscriptionIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Only the tenant ID is read back, the configured values are kept as
	// planned.
	cloudAccount, err := archival.Wrap(polarisClient).AzureCloudAccountByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read data center Azure subscription after create",
			fmt.Sprintf("The data center subscription was created successfully but the computed fields could not "+
				"be populated: %s", err.Error()))
		plan.TenantID = types.StringNull()
	} else {
		plan.TenantID = types.StringValue(cloudAccount.TenantID.String())
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *dataCenterAzureSubscriptionResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.Read")

	var state dataCenterAzureSubscriptionResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid data center subscription ID", err.Error())
		return
	}

	cloudAccount, err := archival.Wrap(polarisClient).AzureCloudAccountByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read data center Azure subscription", err.Error())
		return
	}

	state.Description = types.StringValue(cloudAccount.Description)
	state.Name = types.StringValue(cloudAccount.Name)
	state.SubscriptionID = types.StringValue(cloudAccount.SubscriptionID.String())
	state.TenantID = types.StringValue(cloudAccount.TenantID.String())

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := dataCenterAzureSubscriptionIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *dataCenterAzureSubscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.Update")

	var plan dataCenterAzureSubscriptionResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state dataCenterAzureSubscriptionResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid data center subscription ID", err.Error())
		return
	}

	err = archival.Wrap(polarisClient).UpdateAzureCloudAccount(ctx, id, gqlarchival.UpdateAzureCloudAccountParams{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		SubscriptionID: plan.SubscriptionID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to update data center Azure subscription", err.Error())
		return
	}

	// The update isn't read back, the next refresh reads it.
	plan.ID = state.ID
	plan.TenantID = state.TenantID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := dataCenterAzureSubscriptionIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *dataCenterAzureSubscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.Delete")

	var state dataCenterAzureSubscriptionResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid data center subscription ID", err.Error())
		return
	}

	err = archival.Wrap(polarisClient).DeleteAzureCloudAccount(ctx, id)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to delete data center Azure subscription", err.Error())
		return
	}
}

func (r *dataCenterAzureSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}
