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
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
)

const dataSourceAWSCNPPermissionsDescription = `
The ´rubrik_aws_cnp_permissions´ data source returns the IAM policy
documents that each role must carry for a given feature set, used when
onboarding an AWS account via the AWS IAM roles workflow with the
´rubrik_aws_cnp_account´ and ´rubrik_aws_cnp_account_attachments´
resources.

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
  * ´CLOUD_CLUSTER_ES´ - Represents the basic set of permissions required to
    onboard the feature.

-> **Note:** When permission groups are specified, the ´BASIC´ permission group
   is always required except for the ´SERVERS_AND_APPS´ feature.
`

var _ datasource.DataSource = &awsPermissionsDataSource{}

type awsPermissionsDataSource struct {
	client *client
	prefix string
}

type awsPermissionsModel struct {
	ID                      types.String `tfsdk:"id"`
	Cloud                   types.String `tfsdk:"cloud"`
	EC2RecoveryRolePath     types.String `tfsdk:"ec2_recovery_role_path"`
	Feature                 types.Set    `tfsdk:"feature"`
	RoleKey                 types.String `tfsdk:"role_key"`
	CustomerManagedPolicies types.List   `tfsdk:"customer_managed_policies"`
	ManagedPolicies         types.List   `tfsdk:"managed_policies"`
}

func newAwsPermissionsDataSource() datasource.DataSource {
	return &awsPermissionsDataSource{prefix: keyRubrik}
}

func newPolarisAwsPermissionsDataSource() datasource.DataSource {
	return &awsPermissionsDataSource{prefix: keyPolaris}
}

func (d *awsPermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "awsPermissionsDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAWSCNPPermissions
}

func (d *awsPermissionsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "awsPermissionsDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAWSCNPPermissionsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 hash of the customer managed policies and the managed policies.",
			},
			keyCloud: schema.StringAttribute{
				Optional: true,
				Description: "AWS cloud type. Possible values are `STANDARD`, `CHINA` and `GOV`. Default value is " +
					"`STANDARD`.",
				Validators: []validator.String{
					stringvalidator.OneOf("STANDARD", "CHINA", "GOV"),
				},
			},
			keyCustomerManagedPolicies: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Customer managed policies.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFeature: schema.StringAttribute{
							Computed:    true,
							Description: "RSC feature name.",
						},
						keyName: schema.StringAttribute{
							Computed:    true,
							Description: "Policy name.",
						},
						keyPolicy: schema.StringAttribute{
							Computed:    true,
							Description: "AWS policy.",
						},
					},
				},
			},
			keyEC2RecoveryRolePath: schema.StringAttribute{
				Optional:    true,
				Description: "AWS EC2 recovery role path.",
			},
			keyManagedPolicies: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Managed policies.",
			},
			keyRoleKey: schema.StringAttribute{
				Required:    true,
				Description: "RSC artifact key for the AWS role.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
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
		res.Schema.DeprecationMessage = "use the `rubrik_aws_cnp_permissions` data source instead."
	}
}

func (d *awsPermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "awsPermissionsDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *awsPermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "awsPermissionsDataSource.Read")

	var config awsPermissionsModel
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

	var ec2RecoveryRolePath string
	if !config.EC2RecoveryRolePath.IsNull() {
		ec2RecoveryRolePath = config.EC2RecoveryRolePath.ValueString()
	}

	features, diags := awsToFeatures(ctx, config.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	customerPolicies, managedPolicies, err := aws.Wrap(polarisClient).Permissions(ctx, cloud, features, ec2RecoveryRolePath)
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS permissions", err.Error())
		return
	}

	roleKey := config.RoleKey.ValueString()

	slices.SortFunc(customerPolicies, func(i, j aws.CustomerManagedPolicy) int {
		if r := cmp.Compare(i.Artifact, j.Artifact); r != 0 {
			return r
		}
		if r := cmp.Compare(i.Feature.Name, j.Feature.Name); r != 0 {
			return r
		}
		return cmp.Compare(i.Name, j.Name)
	})
	slices.SortFunc(managedPolicies, func(i, j aws.ManagedPolicy) int {
		if r := cmp.Compare(i.Artifact, j.Artifact); r != 0 {
			return r
		}
		return cmp.Compare(i.Name, j.Name)
	})

	hash := sha256.New()

	var customerPolicyValues []attr.Value
	for _, policy := range customerPolicies {
		if roleKey != policy.Artifact {
			continue
		}
		obj, diags := types.ObjectValue(awsCustomerPolicyAttrTypes(), map[string]attr.Value{
			keyFeature: types.StringValue(policy.Feature.Name),
			keyName:    types.StringValue(policy.Name),
			keyPolicy:  types.StringValue(policy.Policy),
		})
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		customerPolicyValues = append(customerPolicyValues, obj)

		hash.Write([]byte(policy.Artifact))
		hash.Write([]byte(policy.Feature.Name))
		hash.Write([]byte(policy.Name))
		hash.Write([]byte(policy.Policy))
	}
	customerPolicyList, diags := types.ListValue(types.ObjectType{AttrTypes: awsCustomerPolicyAttrTypes()}, customerPolicyValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	var managedPolicyValues []attr.Value
	for _, policy := range managedPolicies {
		if roleKey != policy.Artifact {
			continue
		}
		managedPolicyValues = append(managedPolicyValues, types.StringValue(policy.Name))

		hash.Write([]byte(policy.Artifact))
		hash.Write([]byte(policy.Name))
	}
	managedPolicyList, diags := types.ListValue(types.StringType, managedPolicyValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := awsPermissionsModel{
		ID:                      types.StringValue(fmt.Sprintf("%x", hash.Sum(nil))),
		Cloud:                   config.Cloud,
		EC2RecoveryRolePath:     config.EC2RecoveryRolePath,
		Feature:                 config.Feature,
		RoleKey:                 config.RoleKey,
		CustomerManagedPolicies: customerPolicyList,
		ManagedPolicies:         managedPolicyList,
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

// awsCustomerPolicyAttrTypes returns the attribute types for the
// customer_managed_policies nested list.
func awsCustomerPolicyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyFeature: types.StringType,
		keyName:    types.StringType,
		keyPolicy:  types.StringType,
	}
}
