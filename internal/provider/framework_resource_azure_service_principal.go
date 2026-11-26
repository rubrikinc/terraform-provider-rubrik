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
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
)

const resourceAzureServicePrincipalDescription = `
The ´rubrik_azure_service_principal´ resource adds an Azure service principal
to RSC. A service principal must be added for each Azure tenant before
subscriptions for the tenants can be added to RSC.

There are 3 ways to create a ´rubrik_azure_service principal´ resource:
  1. Using the ´app_id´, ´app_name´, ´app_secret´, ´tenant_id´ and
     ´tenant_domain´ fields.
  2. Using the ´credentials´ field which is the path to a custom service
     principal file. A description of the custom format can be found
     [here](https://github.com/rubrikinc/rubrik-polaris-sdk-for-go?tab=readme-ov-file#azure-credentials).
  3. Using the ´sdk_auth´ field which is the path to an Azure service principal
     created with the Azure SDK using the ´--sdk-auth´ parameter.

Prefer to use option 1, as the ´app_name´ and the ´app_secret´ can be updated
without replacing the service principal.

~> **Note:** Removing the last subscription from an RSC tenant will
   automatically remove the tenant, which also removes the service principal.
   If this happens, the service principal can be replaced using
   ´terraform apply -replace=<address-of-service-principal>´.

~> **Note:** Destroying the ´rubrik_azure_service_principal´ resource only
   updates the local state, it does not remove the service principal from RSC.
   However, creating another ´rubrik_azure_service_principal´ resource for the
   same Azure tenant will overwrite the old service principal in RSC.

-> **Note:** There is no way to verify if a service principal has been added
   to RSC using the UI. RSC tenants don't show up in the UI until the first
   subscription is added.

-> **Note:** A tenant that needs both cloud native protection and Azure DevOps
   protection declares two ´rubrik_azure_service_principal´ resources with the
   same ´tenant_domain´ but different ´use_case´ values. The credentials are
   stored in a separate location per use case.
`

// Azure service principal use cases. These are the provider-facing values for
// the use_case field; they are mapped to azure.AppUseCase before being sent to
// RSC.
const (
	useCaseCloudNativeProtection = "CLOUD_NATIVE_PROTECTION"
	useCaseAzureDevOps           = "AZURE_DEVOPS"
)

// azureAppUseCase maps the provider-facing use_case value to the SDK use case.
func azureAppUseCase(useCase string) azure.AppUseCase {
	if useCase == useCaseAzureDevOps {
		return azure.AppUseCaseDevOps
	}
	return azure.AppUseCaseCNP
}

var (
	_ resource.Resource                     = &azureServicePrincipalResource{}
	_ resource.ResourceWithConfigure        = &azureServicePrincipalResource{}
	_ resource.ResourceWithConfigValidators = &azureServicePrincipalResource{}
	_ resource.ResourceWithMoveState        = &azureServicePrincipalResource{}
	_ resource.ResourceWithUpgradeState     = &azureServicePrincipalResource{}
)

// azureServicePrincipalResource adds an Azure service principal to RSC. Note
// that the delete function cannot remove the service principal since there is
// no delete operation in the RSC API.
type azureServicePrincipalResource struct {
	client *client
	prefix string
}

type azureServicePrincipalResourceModel struct {
	ID              types.String `tfsdk:"id"`
	AppID           types.String `tfsdk:"app_id"`
	AppName         types.String `tfsdk:"app_name"`
	AppSecret       types.String `tfsdk:"app_secret"`
	Credentials     types.String `tfsdk:"credentials"`
	SDKAuth         types.String `tfsdk:"sdk_auth"`
	Permissions     types.String `tfsdk:"permissions"`
	PermissionsHash types.String `tfsdk:"permissions_hash"`
	TenantDomain    types.String `tfsdk:"tenant_domain"`
	TenantID        types.String `tfsdk:"tenant_id"`
	UseCase         types.String `tfsdk:"use_case"`
}

func newAzureServicePrincipalResource() resource.Resource {
	return &azureServicePrincipalResource{prefix: keyRubrik}
}

func newPolarisAzureServicePrincipalResource() resource.Resource {
	return &azureServicePrincipalResource{prefix: keyPolaris}
}

func (r *azureServicePrincipalResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureServicePrincipalResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureServicePrincipal
}

func (r *azureServicePrincipalResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureServicePrincipalResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAzureServicePrincipalDescription),
		Version:     1,
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed: true,
				Description: "Azure app registration application ID (UUID). Also known as the client ID. " +
					"Note, this might change in the future, use the `app_id` field to reference the application ID " +
					"in configurations.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyAppID: schema.StringAttribute{
				Optional: true,
				Description: "Azure app registration application ID. Also known as the client ID. Changing this " +
					"forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyAppName: schema.StringAttribute{
				Optional:    true,
				Description: "Azure app registration display name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyAppSecret: schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Azure app registration client secret.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyCredentials: schema.StringAttribute{
				Optional: true,
				Description: "Path to a custom service principal file. Changing this forces a new resource to be " +
					"created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isExistingFile(),
				},
			},
			keySDKAuth: schema.StringAttribute{
				Optional: true,
				Description: "Path to an Azure service principal created with the Azure SDK using the `--sdk-auth` " +
					"parameter. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isExistingFile(),
				},
			},
			keyPermissions: schema.StringAttribute{
				Optional: true,
				Description: "Permissions updated signal. When this field is updated, the provider will notify RSC " +
					"that permissions has been updated. Use this field with the `rubrik_azure_permissions` data " +
					"source. **Deprecated:** use the `rubrik_azure_subscription` resource's `permissions` fields " +
					"instead.",
				DeprecationMessage: "use the `rubrik_azure_subscription` resource's `permissions` fields instead.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyPermissionsHash: schema.StringAttribute{
				Optional: true,
				Description: "Permissions updated signal. **Deprecated:** use the `rubrik_azure_subscription` " +
					"resource's `permissions` fields instead.",
				DeprecationMessage: "use the `rubrik_azure_subscription` resource's `permissions` fields instead.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyTenantDomain: schema.StringAttribute{
				Required:    true,
				Description: "Azure tenant primary domain. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyTenantID: schema.StringAttribute{
				Optional: true,
				Description: "Azure tenant ID. Also known as the directory ID. Changing this forces a new resource to " +
					"be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyUseCase: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(useCaseCloudNativeProtection),
				Description: "What the service principal is registered for. One of `CLOUD_NATIVE_PROTECTION` " +
					"(default) or `AZURE_DEVOPS`. The credentials are stored in a separate location per use case, " +
					"so a tenant can have one service principal per use case. Changing this forces a new resource " +
					"to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(useCaseCloudNativeProtection, useCaseAzureDevOps),
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_azure_service_principal` instead."
	}
}

func (r *azureServicePrincipalResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "azureServicePrincipalResource.ConfigValidators")

	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot(keyAppID),
			path.MatchRoot(keyCredentials),
			path.MatchRoot(keySDKAuth),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot(keyAppID),
			path.MatchRoot(keyAppName),
			path.MatchRoot(keyAppSecret),
			path.MatchRoot(keyTenantID),
		),
	}
}

func (r *azureServicePrincipalResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureServicePrincipalResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureServicePrincipalResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureServicePrincipalResource.Create")

	var plan azureServicePrincipalResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	tenantDomain := plan.TenantDomain.ValueString()
	var principal azure.ServicePrincipalFunc
	switch {
	case plan.Credentials.ValueString() != "":
		principal = azure.KeyFile(plan.Credentials.ValueString(), tenantDomain)
	case plan.SDKAuth.ValueString() != "":
		principal = azure.SDKAuthFile(plan.SDKAuth.ValueString(), tenantDomain)
	default:
		appID, err := uuid.Parse(plan.AppID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid app ID", err.Error())
			return
		}
		tenantID, err := uuid.Parse(plan.TenantID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid tenant ID", err.Error())
			return
		}

		principal = azure.ServicePrincipal(appID, plan.AppName.ValueString(), plan.AppSecret.ValueString(), tenantID,
			tenantDomain)
	}

	useCase := azureAppUseCase(plan.UseCase.ValueString())
	appID, err := azure.Wrap(polarisClient).SetServicePrincipalForUseCase(ctx, principal, useCase)
	if err != nil {
		res.Diagnostics.AddError("Failed to set service principal", err.Error())
		return
	}

	plan.ID = types.StringValue(appID.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

// Read is intentionally a no-op. RSC does not expose the service principal, so
// there is nothing to read back and no way to detect drift.
func (r *azureServicePrincipalResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureServicePrincipalResource.Read")

	var state azureServicePrincipalResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *azureServicePrincipalResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureServicePrincipalResource.Update")

	var plan azureServicePrincipalResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state azureServicePrincipalResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	if !plan.AppName.Equal(state.AppName) || !plan.AppSecret.Equal(state.AppSecret) {
		id, err := uuid.Parse(state.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid app ID", err.Error())
			return
		}
		tenantID, err := uuid.Parse(plan.TenantID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid tenant ID", err.Error())
			return
		}

		principal := azure.ServicePrincipal(id, plan.AppName.ValueString(), plan.AppSecret.ValueString(), tenantID,
			plan.TenantDomain.ValueString())
		useCase := azureAppUseCase(plan.UseCase.ValueString())
		if _, err := azure.Wrap(polarisClient).SetServicePrincipalForUseCase(ctx, principal, useCase); err != nil {
			res.Diagnostics.AddError("Failed to set service principal", err.Error())
			return
		}
	}

	if !plan.Permissions.Equal(state.Permissions) || !plan.PermissionsHash.Equal(state.PermissionsHash) {
		err := azure.Wrap(polarisClient).PermissionsUpdatedForTenantDomain(ctx, plan.TenantDomain.ValueString(), nil)
		if err != nil {
			res.Diagnostics.AddError("Failed to update permissions", err.Error())
			return
		}
	}

	plan.ID = state.ID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

// Delete only removes the resource from the Terraform state, since the service
// principal cannot be removed using the RSC API.
func (r *azureServicePrincipalResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureServicePrincipalResource.Delete")
}
