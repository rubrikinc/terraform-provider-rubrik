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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/exocompute"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlexocompute "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/exocompute"
)

const resourceAWSExocomputeKMSConfirmationDescription = `
The ´rubrik_aws_exocompute_kms_confirmation´ resource confirms KMS key sharing
for a manually onboarded AWS application account, after the IAM policy from
´rubrik_aws_exocompute.kms_role_policy´ has been attached to the application
account's IAM role. Use ´depends_on´ to order confirmation after policy
attachment. RSC does not verify the attachment during confirmation.

Deleting this confirmation does not revoke KMS key sharing. The
´rubrik_aws_exocompute´ resource controls the mapping and finalizes a manual
revoke on deletion. Remove the IAM policy before deleting the mapping;
RSC does not verify removal. See the
[rubrik_aws_exocompute docs](aws_exocompute.md#automated-kms-key-sharing).
`

var (
	_ resource.Resource                = &awsExocomputeKMSConfirmationResource{}
	_ resource.ResourceWithConfigure   = &awsExocomputeKMSConfirmationResource{}
	_ resource.ResourceWithIdentity    = &awsExocomputeKMSConfirmationResource{}
	_ resource.ResourceWithImportState = &awsExocomputeKMSConfirmationResource{}
)

type awsExocomputeKMSConfirmationResource struct {
	client *client
}

type awsExocomputeKMSConfirmationModel struct {
	ID        types.String `tfsdk:"id"`
	AccountID types.String `tfsdk:"account_id"`
}

func newAWSExocomputeKMSConfirmationResource() resource.Resource {
	return &awsExocomputeKMSConfirmationResource{}
}

func (r *awsExocomputeKMSConfirmationResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyAWSExocomputeKMSConfirmation
}

func (r *awsExocomputeKMSConfirmationResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAWSExocomputeKMSConfirmationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC application cloud account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyAccountID: schema.StringAttribute{
				Required:    true,
				Description: "RSC application cloud account ID (UUID) for the manual Exocompute KMS mapping.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{isUUID()},
			},
		},
	}
}

func (r *awsExocomputeKMSConfirmationResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC application cloud account ID (UUID).",
			},
		},
	}
}

func (r *awsExocomputeKMSConfirmationResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.Configure")

	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client)
	}
}

func (r *awsExocomputeKMSConfirmationResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.Create")

	var plan awsExocomputeKMSConfirmationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	accountID, err := uuid.Parse(plan.AccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid application cloud account ID", err.Error())
		return
	}
	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	api := exocompute.Wrap(polarisClient)
	status, err := api.AWSKMSMappingStatus(ctx, accountID)
	if err != nil {
		res.Diagnostics.AddError("Could not read Exocompute KMS mapping status", err.Error())
		return
	}

	// The mutation also finalizes a pending revoke, so do not call it for a
	// DISABLING mapping. Gateway mappings use a separate workflow.
	if status.SharingMode != gqlexocompute.KMSSharingModeSharingOnly || status.StackARN != "" ||
		(status.ProvisioningStatus != gqlexocompute.KMSMappingProvisioningStatusConnecting &&
			status.ProvisioningStatus != gqlexocompute.KMSMappingProvisioningStatusConnected) {
		res.Diagnostics.AddError("Exocompute KMS mapping cannot be confirmed",
			fmt.Sprintf("Application account %s must have a pending or connected manual KMS sharing-only mapping; got mode %s and status %s.",
				accountID, status.SharingMode, status.ProvisioningStatus))
		return
	}
	if status.ProvisioningStatus == gqlexocompute.KMSMappingProvisioningStatusConnecting {
		if err := api.ConfirmAWSManualKMSMapping(ctx, accountID); err != nil {
			res.Diagnostics.AddError("Could not confirm Exocompute KMS mapping", err.Error())
			return
		}

		// RSC returns success without changing non-manual mappings, including
		// StackSet accounts (which do not have an individual stack ARN).
		status, err = api.AWSKMSMappingStatus(ctx, accountID)
		if err != nil {
			res.Diagnostics.AddWarning("Could not verify Exocompute KMS confirmation", err.Error())
		} else if status.ProvisioningStatus != gqlexocompute.KMSMappingProvisioningStatusConnected {
			res.Diagnostics.AddError("Exocompute KMS mapping was not confirmed",
				fmt.Sprintf("Application account %s is still in status %s. Verify that it was onboarded manually.",
					accountID, status.ProvisioningStatus))
			return
		}
	}

	// There is no separate confirmation object to orphan: retries after a
	// successful mutation observe CONNECTED and do not confirm again.
	plan.ID = types.StringValue(accountID.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), plan.ID)...)
}

func (r *awsExocomputeKMSConfirmationResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.Read")

	var state awsExocomputeKMSConfirmationModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	accountID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid application cloud account ID", err.Error())
		return
	}
	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	status, err := exocompute.Wrap(polarisClient).AWSKMSMappingStatus(ctx, accountID)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Could not read Exocompute KMS mapping status", err.Error())
		return
	}
	if status.SharingMode != gqlexocompute.KMSSharingModeSharingOnly ||
		status.ProvisioningStatus != gqlexocompute.KMSMappingProvisioningStatusConnected || status.StackARN != "" {
		res.State.RemoveResource(ctx)
		return
	}

	if state.AccountID.IsNull() {
		state.AccountID = types.StringValue(accountID.String())
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), state.ID)...)
}

func (r *awsExocomputeKMSConfirmationResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.Update")

	res.State.Raw = req.Plan.Raw
	res.Identity = req.Identity
}

func (r *awsExocomputeKMSConfirmationResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.Delete")

	// The IAM policy must be detached after this step but before the mapping
	// is deleted, so only the mapping resource can finalize the revoke.
}

func (r *awsExocomputeKMSConfirmationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsExocomputeKMSConfirmationResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}
