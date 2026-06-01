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

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
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

## Permission Groups
Following is a list of features and their applicable permission groups. These
are used when specifying the feature set.

´CLOUD_DISCOVERY´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´CLOUD_NATIVE_ARCHIVAL´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´CLOUD_NATIVE_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´DOWNLOAD_FILE´ - Represents the set of permissions required to download
    files from snapshots.
  * ´EXPORT_POWER_OFF´ - Represents the set of permissions required to export
    EC2 instances and leave them powered off.
  * ´EXPORT_POWER_ON´ - Represents the set of permissions required to export
    EC2 instances and power them on.
  * ´RESTORE´ - Represents the set of permissions required to restore from
    snapshots.

´CLOUD_NATIVE_DYNAMODB_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of elevated permissions required to perform
    recovery operations.

´CLOUD_NATIVE_S3_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´EXOCOMPUTE´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RSC_MANAGED_CLUSTER´ - Represents the set of permissions required for the
    Rubrik-managed Exocompute cluster.

´KUBERNETES_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´RDS_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of elevated permissions required to perform
    recovery operations.

´ROLE_CHAINING´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´SERVERS_AND_APPS´
  * ´CLOUD_CLUSTER_ES´ - Represents the basic set of permissions required to onboard the
    feature.

-> **Note:** When permission groups are specified, the ´BASIC´ permission group
   is always required except for the ´SERVERS_AND_APPS´ feature.
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
							Required: true,
							Description: "RSC feature name. Possible values are `CLOUD_DISCOVERY`, " +
								"`CLOUD_NATIVE_ARCHIVAL`, `CLOUD_NATIVE_DYNAMODB_PROTECTION`, " +
								"`CLOUD_NATIVE_PROTECTION`, `CLOUD_NATIVE_S3_PROTECTION`, `EXOCOMPUTE`, " +
								"`KUBERNETES_PROTECTION`, `RDS_PROTECTION`, `ROLE_CHAINING` and " +
								"`SERVERS_AND_APPS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(
									"CLOUD_DISCOVERY", "CLOUD_NATIVE_ARCHIVAL", "CLOUD_NATIVE_PROTECTION",
									"CLOUD_NATIVE_DYNAMODB_PROTECTION", "CLOUD_NATIVE_S3_PROTECTION",
									"KUBERNETES_PROTECTION", "EXOCOMPUTE", "ROLE_CHAINING",
									"RDS_PROTECTION", "SERVERS_AND_APPS",
								),
							},
						},
						keyPermissionGroups: schema.SetAttribute{
							ElementType: types.StringType,
							Required:    true,
							Description: "RSC permission groups for the feature. Possible values are " +
								"`BASIC`, `CLOUD_CLUSTER_ES`, `DOWNLOAD_FILE`, `EXPORT_POWER_ON`, " +
								"`EXPORT_POWER_OFF`, `RECOVERY`, `RESTORE` and `RSC_MANAGED_CLUSTER`. " +
								"For backwards compatibility, `[]` is interpreted as all applicable " +
								"permission groups.",
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(stringvalidator.OneOf(
									"BASIC", "RECOVERY", "RSC_MANAGED_CLUSTER", "CLOUD_CLUSTER_ES",
									"EXPORT_POWER_ON", "EXPORT_POWER_OFF", "RESTORE", "DOWNLOAD_FILE",
									// The following permission groups cannot be used when onboarding an
									// AWS account. They have been accepted in the past so we still
									// silently allow them.
									"EXPORT_AND_RESTORE", "FILE_LEVEL_RECOVERY", "SNAPSHOT_PRIVATE_ACCESS",
									"PRIVATE_ENDPOINT",
								)),
							},
						},
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

	profiles, roles, err := aws.Wrap(polarisClient).Artifacts(ctx, cloud, features)
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
		RoleKeys:            roleSet,
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
