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

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/aws"
)

const listResourceAWSAccountDescription = `
The ´rubrik_aws_account´ list resource lists AWS accounts onboarded via the AWS
CloudFormation workflow in RSC. Accounts onboarded through the AWS IAM roles
workflow are skipped, use the ´rubrik_aws_cnp_account´ list resource for them.
RSC-managed accounts are skipped too, use the ´rubrik_aws_account_managed´ list
resource for them.

RSC does not return the ´profile´ and ´assume_role´ fields, so they are not
populated in list results. One of them must be added to the generated
configuration before it can be applied. An Outpost feature onboarded in a
separate outpost account is not listed under the account.
`

var (
	_ list.ListResource              = &awsAccountListResource{}
	_ list.ListResourceWithConfigure = &awsAccountListResource{}
)

type awsAccountListResource struct {
	client *client
	prefix string
}

type awsAccountListConfigModel struct {
	Name     types.String `tfsdk:"name"`
	NativeID types.String `tfsdk:"native_id"`
}

func newAwsAccountListResource() list.ListResource {
	return &awsAccountListResource{prefix: keyRubrik}
}

func newPolarisAwsAccountListResource() list.ListResource {
	return &awsAccountListResource{prefix: keyPolaris}
}

func (r *awsAccountListResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsAccountListResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSAccount
}

func (r *awsAccountListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "awsAccountListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResourceAWSAccountDescription),
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
		res.Schema.DeprecationMessage = "use the `rubrik_aws_account` list resource instead."
	}
}

func (r *awsAccountListResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsAccountListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsAccountListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "awsAccountListResource.List")

	var config awsAccountListConfigModel
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
		if account.OnboardingMode() != aws.OnboardingModeCFT || account.ServiceType == gqlaws.ServiceTypeBaaS {
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

			id := types.StringValue(account.ID.String())
			result.Diagnostics.Append(result.Identity.Set(ctx, awsAccountIdentityModel{ID: id})...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				// Build the model the same way Read does, so the generated
				// configuration matches what an import puts in state. The
				// fields RSC doesn't return are left null.
				model := awsAccountResourceModel{ID: id}
				updateAWSAccountResourceModel(&model, account, nil)
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
