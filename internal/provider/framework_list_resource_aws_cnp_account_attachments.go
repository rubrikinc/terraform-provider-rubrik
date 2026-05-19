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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
)

const listResourceAWSCNPAccountAttachmentsDescription = `
The ´rubrik_aws_cnp_account_attachments´ list resource lists the artifact
attachments of AWS accounts onboarded via the AWS IAM roles workflow in
RSC. One result is emitted per onboarded account, identified by the RSC
cloud account ID.

Accounts onboarded through the AWS CloudFormation workflow are skipped:
they are managed by the ´rubrik_aws_account´ resource and have no
artifact attachments to list.

The ´permissions´ field on each ´role´ block is not populated in list
results because RSC does not return the sentinel value. After importing
through this list resource, configure the field as usual on the imported
´rubrik_aws_cnp_account_attachments´ resource if you want to track
permission updates.

## Bulk Import

The list resource can be combined with an ´import´ block to bulk-import
existing attachments:

´´´hcl
import {
  for_each = list.rubrik_aws_cnp_account_attachments.all.results
  to       = rubrik_aws_cnp_account_attachments.attachments[each.value.identity.id]
  identity = {
    id = each.value.identity.id
  }
}
´´´
`

var (
	_ list.ListResource              = &awsCnpAccountAttachmentsListResource{}
	_ list.ListResourceWithConfigure = &awsCnpAccountAttachmentsListResource{}
)

type awsCnpAccountAttachmentsListResource struct {
	client *client
	prefix string
}

type awsCnpAccountAttachmentsListConfigModel struct {
	Name     types.String `tfsdk:"name"`
	NativeID types.String `tfsdk:"native_id"`
}

func newAwsCnpAccountAttachmentsListResource() list.ListResource {
	return &awsCnpAccountAttachmentsListResource{prefix: keyRubrik}
}

func newPolarisAwsCnpAccountAttachmentsListResource() list.ListResource {
	return &awsCnpAccountAttachmentsListResource{prefix: keyPolaris}
}

func (r *awsCnpAccountAttachmentsListResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsListResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSCNPAccountAttachments
}

func (r *awsCnpAccountAttachmentsListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResourceAWSCNPAccountAttachmentsDescription),
		Attributes: map[string]listschema.Attribute{
			keyName: listschema.StringAttribute{
				Optional: true,
				Description: "Filter by the parent AWS account name. Matches attachments whose account name " +
					"contains the given value (case-insensitive).",
			},
			keyNativeID: listschema.StringAttribute{
				Optional: true,
				Description: "Filter by the parent AWS account ID. Matches attachments whose account native " +
					"ID equals the given value.",
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_aws_cnp_account_attachments` list resource instead."
	}
}

func (r *awsCnpAccountAttachmentsListResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsCnpAccountAttachmentsListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsListResource.List")

	var config awsCnpAccountAttachmentsListConfigModel
	diags := req.Config.Get(ctx, &config)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		diags.AddError("RSC client error", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	accounts, err := aws.Wrap(polarisClient).Accounts(ctx, "")
	if err != nil {
		diags.AddError("Failed to list AWS accounts", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	nameFilter := strings.ToLower(config.Name.ValueString())
	nativeIDFilter := config.NativeID.ValueString()
	filtered := make([]aws.CloudAccount, 0, len(accounts))
	for _, account := range accounts {
		if account.OnboardingMode() != aws.OnboardingModeIAM {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(account.Name), nameFilter) {
			continue
		}
		if nativeIDFilter != "" && account.NativeID != nativeIDFilter {
			continue
		}
		filtered = append(filtered, account)
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, account := range filtered {
			if int64(i) >= req.Limit {
				return
			}

			result := req.NewListResult(ctx)
			result.DisplayName = account.Name

			identity := awsCnpAccountAttachmentsIdentityModel{
				ID: types.StringValue(account.ID.String()),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				featureValues := make([]attr.Value, 0, len(account.Features))
				for _, feature := range account.Features {
					featureValues = append(featureValues, types.StringValue(feature.Feature.Name))
				}
				featureSet, diags := types.SetValue(types.StringType, featureValues)
				result.Diagnostics.Append(diags...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}

				instanceProfiles, roles, err := aws.Wrap(polarisClient).AccountArtifacts(ctx, account.ID)
				if err != nil {
					result.Diagnostics.AddError("Failed to read AWS account artifacts", err.Error())
					push(result)
					return
				}
				delete(roles, "ROLE_CHAINING")

				profileSet, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: awsInstanceProfileAttrTypes()}, awsFromInstanceProfiles(instanceProfiles))
				result.Diagnostics.Append(diags...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}

				roleSet, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: awsRoleAttrTypes()}, awsFromRoles(roles))
				result.Diagnostics.Append(diags...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}

				roleChainingAccountID := types.StringNull()
				if account.RoleChainingAccountID != uuid.Nil {
					roleChainingAccountID = types.StringValue(account.RoleChainingAccountID.String())
				}

				model := awsCnpAccountAttachmentsModel{
					ID:                    types.StringValue(account.ID.String()),
					AccountID:             types.StringValue(account.ID.String()),
					Features:              featureSet,
					InstanceProfile:       profileSet,
					Role:                  roleSet,
					RoleChainingAccountID: roleChainingAccountID,
				}
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}
			}

			if !push(result) {
				return
			}
		}
	}
}
