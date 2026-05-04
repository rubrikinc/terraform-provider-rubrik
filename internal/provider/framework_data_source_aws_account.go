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
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
)

const dataSourceAWSAccountDescription = `
The ´rubrik_aws_account´ data source is used to access information about an AWS
account added to RSC. An AWS account is looked up using either the AWS account
ID, the RSC cloud account ID or the name.

-> **Note:** The account name is the name of the AWS account as it appears in
   RSC.
`

var _ datasource.DataSource = &awsAccountDataSource{}

type awsAccountDataSource struct {
	client *client
	prefix string
}

type awsAccountModel struct {
	ID             types.String `tfsdk:"id"`
	AccountID      types.String `tfsdk:"account_id"`
	CloudAccountID types.String `tfsdk:"cloud_account_id"`
	Name           types.String `tfsdk:"name"`
	Feature        types.Set    `tfsdk:"feature"`
}

func awsAccountFeatureAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyName:             types.StringType,
		keyPermissionGroups: types.SetType{ElemType: types.StringType},
	}
}

func newAwsAccountDataSource() datasource.DataSource {
	return &awsAccountDataSource{prefix: keyRubrik}
}

func newPolarisAwsAccountDataSource() datasource.DataSource {
	return &awsAccountDataSource{prefix: keyPolaris}
}

func (d *awsAccountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "awsAccountDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAWSAccount
}

func (d *awsAccountDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "awsAccountDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAWSAccountDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
			},
			keyAccountID: schema.StringAttribute{
				Optional:    true,
				Description: "AWS account ID.",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot(keyAccountID),
						path.MatchRoot(keyCloudAccountID),
						path.MatchRoot(keyName),
					),
					isNotWhiteSpace(),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Optional:    true,
				Description: "RSC cloud account ID (UUID).",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Description: "AWS account name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyFeature: schema.SetNestedAttribute{
				Computed:    true,
				Description: "RSC feature with permission groups.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Computed:    true,
							Description: "RSC feature name.",
						},
						keyPermissionGroups: schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Permission groups for the RSC feature.",
						},
					},
				},
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_aws_account` data source instead."
	}
}

func (d *awsAccountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "awsAccountDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *awsAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "awsAccountDataSource.Read")

	var config awsAccountModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// We don't allow prefix searches since it would be impossible to uniquely
	// identify an account with a name being the prefix of another account.
	var account aws.CloudAccount
	switch {
	case !config.AccountID.IsNull():
		account, err = aws.Wrap(polarisClient).AccountByNativeID(ctx, config.AccountID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read AWS account", err.Error())
			return
		}
	case !config.Name.IsNull():
		account, err = aws.Wrap(polarisClient).AccountByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read AWS account", err.Error())
			return
		}
	default:
		cloudAccountID, err := uuid.Parse(config.CloudAccountID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid cloud account ID", err.Error())
			return
		}
		account, err = aws.Wrap(polarisClient).AccountByID(ctx, cloudAccountID)
		if err != nil {
			res.Diagnostics.AddError("Failed to read AWS account", err.Error())
			return
		}
	}

	featureSet, diags := fromAWSAccountFeatures(account.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := awsAccountModel{
		ID:             types.StringValue(account.ID.String()),
		AccountID:      types.StringValue(account.NativeID),
		CloudAccountID: types.StringValue(account.ID.String()),
		Name:           types.StringValue(account.Name),
		Feature:        featureSet,
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

// fromAWSAccountFeatures converts a slice of aws.Feature to a Terraform
// Framework set of feature objects.
func fromAWSAccountFeatures(features []aws.Feature) (types.Set, diag.Diagnostics) {
	featureValues := make([]attr.Value, 0, len(features))
	for _, f := range features {
		groupValues := make([]attr.Value, 0, len(f.PermissionGroups))
		for _, g := range f.PermissionGroups {
			groupValues = append(groupValues, types.StringValue(string(g)))
		}

		groupSet, diags := types.SetValue(types.StringType, groupValues)
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: awsAccountFeatureAttrTypes()}), diags
		}

		featureValue, diags := types.ObjectValue(awsAccountFeatureAttrTypes(), map[string]attr.Value{
			keyName:             types.StringValue(f.Name),
			keyPermissionGroups: groupSet,
		})
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: awsAccountFeatureAttrTypes()}), diags
		}

		featureValues = append(featureValues, featureValue)
	}

	return types.SetValue(types.ObjectType{AttrTypes: awsAccountFeatureAttrTypes()}, featureValues)
}
