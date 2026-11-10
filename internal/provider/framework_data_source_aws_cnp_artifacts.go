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
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const dataSourceAWSCNPArtifactsDescription = `
The ´rubrik_aws_cnp_artifacts´ data source returns the instance profiles and
roles required by RSC for a given feature set, used when onboarding an AWS
account via the AWS IAM roles workflow with the ´rubrik_aws_cnp_account´
and ´rubrik_aws_cnp_account_attachments´ resources.

-> **Note:** The ´feature´ block is shown as Optional in the schema below for
   technical reasons, but at least one ´feature´ block must be specified. The
   block-style syntax is preserved to remain compatible with existing Terraform
   configurations.

{{template "awsPermissionGroups"}}
`

var _ datasource.DataSource = &awsArtifactsDataSource{}

type awsArtifactsDataSource struct {
	client *client
	prefix string
}

type awsArtifactsModel struct {
	ID                  types.String `tfsdk:"id"`
	Cloud               types.String `tfsdk:"cloud"`
	Feature             types.Set    `tfsdk:"feature"`
	InstanceProfileKeys types.Set    `tfsdk:"instance_profile_keys"`
	NativeID            types.String `tfsdk:"native_id"`
	RoleKeys            types.Set    `tfsdk:"role_keys"`
}

func newAwsArtifactsDataSource() datasource.DataSource {
	return &awsArtifactsDataSource{prefix: keyRubrik}
}

func newPolarisAwsArtifactsDataSource() datasource.DataSource {
	return &awsArtifactsDataSource{prefix: keyPolaris}
}

func (d *awsArtifactsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "awsArtifactsDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAWSCNPArtifacts
}

func (d *awsArtifactsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "awsArtifactsDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAWSCNPArtifactsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 hash of the instance profile keys and the role keys.",
			},
			keyCloud: schema.StringAttribute{
				Optional: true,
				Description: "AWS cloud type. Possible values are `STANDARD`, `CHINA` and `GOV`. Default value is " +
					"`STANDARD`.",
				Validators: []validator.String{
					stringvalidator.OneOf("STANDARD", "CHINA", "GOV"),
				},
			},
			keyInstanceProfileKeys: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Instance profile keys for the RSC features.",
			},
			keyNativeID: schema.StringAttribute{
				Optional: true,
				Description: "AWS account ID of an account onboarded to RSC. When specified, the artifacts are those " +
					"required by the account, e.g. one role per feature for an account using multiple roles. When " +
					"not specified, the artifacts are those of an account using a single role. RSC returns an error " +
					"if the account has not been onboarded.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyRoleKeys: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Role keys for the RSC features.",
			},
		},
		Blocks: map[string]schema.Block{
			// feature is modeled as a SetNestedBlock rather than a SetNestedAttribute
			// to preserve the SDKv2 block syntax that existing configurations rely on.
			// The Plugin Framework does not expose a Required flag on blocks, so the
			// at-least-one constraint is enforced by setvalidator.SizeAtLeast(1) below.
			keyFeature: schema.SetNestedBlock{
				Description: "RSC feature with permission groups. At least one `feature` block must be specified.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Required:    true,
							Description: fmt.Sprintf("RSC feature name. %s.", possibleValues(core.AWSIAMRolesFeatureNames())),
							Validators: []validator.String{
								stringvalidator.OneOf(core.AWSIAMRolesFeatureNames()...),
							},
						},
						keyPermissionGroups: schema.SetAttribute{
							ElementType: types.StringType,
							Required:    true,
							Description: fmt.Sprintf("Permission groups for the RSC feature. %s.",
								possibleValues(core.AWSIAMRolesPermissionGroupNames())),
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(stringvalidator.OneOf(
									slices.Concat(core.AWSIAMRolesPermissionGroupNames(), core.PermissionGroupNames(awsCNPExtraPermissionGroups))...,
								)),
							},
						},
					},
					Validators: []validator.Object{
						featurePermissionGroupsValidator(core.AWSIAMRolesFeatures(), awsCNPExtraPermissionGroups...),
					},
				},
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_aws_cnp_artifacts` data source instead."
	}
}

func (d *awsArtifactsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "awsArtifactsDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *awsArtifactsDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "awsArtifactsDataSource.Read")

	var config awsArtifactsModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloud := "STANDARD"
	if !config.Cloud.IsNull() {
		cloud = config.Cloud.ValueString()
	}

	features, diags := awsToFeatures(ctx, config.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	profiles, roles, err := aws.Wrap(polarisClient).ArtifactsWithParams(ctx, aws.ArtifactsParams{
		Cloud:    cloud,
		Features: features,
		NativeID: config.NativeID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS artifacts", err.Error())
		return
	}

	// To remain backwards compatible with the SDKv2 implementation, coerce nil
	// slices to empty slices so missing artifact categories surface as empty
	// sets rather than null sets. SetValueFrom would otherwise emit a null set
	// for a nil slice, which is a distinct value from an empty set in the
	// Plugin Framework.
	if profiles == nil {
		profiles = []string{}
	}
	if roles == nil {
		roles = []string{}
	}

	profileSet, diags := types.SetValueFrom(ctx, types.StringType, profiles)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	roleSet, diags := types.SetValueFrom(ctx, types.StringType, roles)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	hash := sha256.New()
	for _, profile := range profiles {
		hash.Write([]byte(profile))
	}
	for _, role := range roles {
		hash.Write([]byte(role))
	}

	state := awsArtifactsModel{
		ID:                  types.StringValue(fmt.Sprintf("%x", hash.Sum(nil))),
		Cloud:               config.Cloud,
		Feature:             config.Feature,
		InstanceProfileKeys: profileSet,
		NativeID:            config.NativeID,
		RoleKeys:            roleSet,
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
