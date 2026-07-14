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
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
)

const listResourceAWSCNPAccountDescription = `
The ´rubrik_aws_cnp_account´ list resource lists AWS accounts onboarded via
the AWS IAM roles workflow in RSC. Accounts onboarded through the AWS
CloudFormation workflow are skipped: they are managed by the
´rubrik_aws_account´ resource.

The ´trust_policies´ attribute is not populated in list results because RSC
does not return the external ID required to compute trust policies. To
manage trust policies for an account discovered through this list resource,
import the account first and supply the external ID in the import block's
identity, then manage it as a normal ´rubrik_aws_cnp_account´ resource.
`

var (
	_ list.ListResource              = &awsCnpAccountListResource{}
	_ list.ListResourceWithConfigure = &awsCnpAccountListResource{}
)

type awsCnpAccountListResource struct {
	client *client
	prefix string
}

type awsCnpAccountListConfigModel struct {
	Name     types.String `tfsdk:"name"`
	NativeID types.String `tfsdk:"native_id"`
}

func newAwsCnpAccountListResource() list.ListResource {
	return &awsCnpAccountListResource{prefix: keyRubrik}
}

func newPolarisAwsCnpAccountListResource() list.ListResource {
	return &awsCnpAccountListResource{prefix: keyPolaris}
}

func (r *awsCnpAccountListResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsCnpAccountListResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSCNPAccount
}

func (r *awsCnpAccountListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "awsCnpAccountListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResourceAWSCNPAccountDescription),
		Attributes: map[string]listschema.Attribute{
			keyName: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter accounts by name. Matches accounts whose name contains the given value (case-insensitive).",
			},
			keyNativeID: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter accounts by AWS account ID. Matches accounts whose native ID equals the given value.",
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_aws_cnp_account` list resource instead."
	}
}

func (r *awsCnpAccountListResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsCnpAccountListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsCnpAccountListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "awsCnpAccountListResource.List")

	var config awsCnpAccountListConfigModel
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
		if account.OnboardingMode() == aws.OnboardingModeCFT {
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

			identity := awsCnpAccountIdentityModel{
				ID:         types.StringValue(account.ID.String()),
				ExternalID: types.StringNull(),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				featureSet, featureDiags := awsFromFeatures(ctx, account.Features)
				result.Diagnostics.Append(featureDiags...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}

				regionSet, regionDiags := awsFromFeatureRegions(account.Features)
				result.Diagnostics.Append(regionDiags...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}

				roleChainingAccountID := types.StringNull()
				if account.RoleChainingAccountID != uuid.Nil {
					roleChainingAccountID = types.StringValue(account.RoleChainingAccountID.String())
				}

				model := awsCnpAccountModel{
					ID:                       types.StringValue(account.ID.String()),
					Cloud:                    types.StringValue(account.Cloud),
					DeleteSnapshotsOnDestroy: types.BoolNull(),
					ExternalID:               types.StringNull(),
					Feature:                  featureSet,
					Name:                     types.StringValue(account.Name),
					NativeID:                 types.StringValue(account.NativeID),
					RoleChainingAccountID:    roleChainingAccountID,
					Regions:                  regionSet,
					TrustPolicies:            types.SetNull(types.ObjectType{AttrTypes: awsTrustPolicyAttrTypes()}),
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
