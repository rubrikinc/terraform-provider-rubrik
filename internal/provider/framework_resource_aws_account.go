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
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
)

const resourceAWSAccountDescription = `
The ´rubrik_aws_account´ resource adds an AWS account to RSC. To grant RSC
permissions to perform certain operations on the account, a Cloud Formation
stack is created from a template provided by RSC.

There are two ways to specify the AWS account to onboard:
 1. Using the ´profile´ field. The AWS profile is used to create the Cloud
    Formation stack and lookup the AWS account ID.
 2. Using the ´assume_role´ field with, or without, the ´profile´ field. If the
    ´profile´ field is omitted, the default profile is used. The profile is used
    to assume the role. The assumed role is then used to create the Cloud
    Formation stack and lookup the account ID.

Any combination of different RSC features, except as noted below, can be enabled
for an account:
  * ´cloud_discovery´ - Enable the Cloud Discovery feature for the account.
    Required when onboarding a new account with protection features. Optional
    for existing accounts.
  * ´cloud_native_archival´ - Enable the Cloud Native Archival feature for the
    account.
  * ´cloud_native_config_protection´ - Enable the Cloud Applications feature,
    also known as Cloud Native Config Protection, for the account. Protects the
    AWS configuration surrounding an application, such as VPC and networking,
    IAM, KMS and load balancers.
  * ´cloud_native_protection´ - Enable the Cloud Native Protection feature for
    the account.
  * ´cloud_native_dynamodb_protection´ - Enable the Cloud Native DynamoDB
    Protection feature for the account.
  * ´cloud_native_s3_protection´ - Enable the Cloud Native S3 Protection feature
    for the account.
  * ´cloud_native_uem_key_management´ - Enable the Cloud Native UEM Key
    Management feature for the account. Required to register the account's KMS
    as a key manager in RSC.
  * ´cyber_recovery_data_scanning´ - Enable the Cyber Recovery Data Scanning
    feature for the account. Requires the Outpost feature to be enabled, either
    in the same account or in a separate account onboarded before this resource.
  * ´data_scanning´ - Enable the Data Scanning feature for the account. Requires
    the Outpost feature to be enabled, either in the same account or in a
    separate account onboarded before this resource.
  * ´dspm´ - Enable the DSPM feature for the account. Requires the Outpost
    feature to be enabled, either in the same account or in a separate account
    onboarded before this resource.
  * ´exocompute´ - Enable the Exocompute feature for the account. Only required
    for accounts hosting the Exocompute cluster.
  * ´kubernetes_protection´ - Enable the Kubernetes Protection feature for the
    account.
  * ´outpost´ - Enable the Outpost feature for the account. Required for the
    Cyber Recovery Data Scanning, Data Scanning and DSPM features. The outpost
    account can be the same account as where the features are enabled, or it can
    be a separate account. When using a separate account, the outpost account
    must be onboarded first, using ´depends_on´ to enforce the ordering. The
    ´outpost_account_id´ and ´outpost_account_profile´ fields are legacy and not
    recommended.
  * ´rds_protection´ - Enable the RDS Protection feature for the account.
  * ´role_chaining´ - Enable the Role Chaining feature for the account. This
    feature is mutually exclusive with all other features and cannot be combined
    with any other feature on the same account.
  * ´servers_and_apps´ - Enable the Servers and Apps feature for the account.
    Required to run CCES clusters.

## Role Chaining

The Role Chaining feature enables cross-account role chaining for AWS accounts.
This feature is mutually exclusive with all other features - it cannot be enabled
alongside any other feature on the same account.

´´´terraform
resource "rubrik_aws_account" "role_chaining" {
  profile = "role-chaining"

  role_chaining {
    permission_groups = ["BASIC"]
  }
}
´´´

To onboard an account that uses cross-account role chaining, reference the RSC
cloud account ID of the role chaining account using the ´role_chaining_account_id´
field:
´´´terraform
resource "rubrik_aws_account" "account" {
  profile                  = "target"
  role_chaining_account_id = rubrik_aws_account.role_chaining.id

  cloud_native_protection {
    permission_groups = ["BASIC"]
    regions           = ["us-east-2"]
  }
}
´´´

## Outpost Account

The Cyber Recovery Data Scanning, Data Scanning and DSPM features require an
outpost account to be onboarded. The outpost account can be the same account as
where the features are enabled, or it can be a separate account.

When the outpost account is the same account:
´´´terraform
resource "rubrik_aws_account" "main" {
  profile = "main"

  data_scanning {
    permission_groups = ["BASIC"]
    regions           = ["us-east-2"]
  }

  outpost {
    permission_groups = ["BASIC"]
  }
}
´´´

When the outpost account is a separate account, the outpost account must be
onboarded first. Use ´depends_on´ to enforce the ordering:
´´´terraform
resource "rubrik_aws_account" "outpost" {
  profile = "outpost"

  outpost {
    permission_groups = ["BASIC"]
  }
}

resource "rubrik_aws_account" "main" {
  profile = "main"

  data_scanning {
    permission_groups = ["BASIC"]
    regions           = ["us-east-2"]
  }

  depends_on = [
    rubrik_aws_account.outpost,
  ]
}
´´´

-> **Note:** To onboard an account using IAM roles instead of a CloudFormation
   stack, use the ´rubrik_aws_cnp_account´ resource.

-> **Note:** When importing the ´rubrik_aws_account´ resource, the
   ´outpost_account_id´ and ´outpost_account_profile´ fields are not imported.
`

// awsAccountPermissionsUpdateRequired is the value of the permissions field
// when a feature of the account is missing permissions.
const awsAccountPermissionsUpdateRequired = "update-required"

var (
	_ resource.Resource                   = &awsAccountResource{}
	_ resource.ResourceWithIdentity       = &awsAccountResource{}
	_ resource.ResourceWithImportState    = &awsAccountResource{}
	_ resource.ResourceWithModifyPlan     = &awsAccountResource{}
	_ resource.ResourceWithMoveState      = &awsAccountResource{}
	_ resource.ResourceWithUpgradeState   = &awsAccountResource{}
	_ resource.ResourceWithValidateConfig = &awsAccountResource{}
)

type awsAccountResource struct {
	client *client
	prefix string
}

type awsAccountResourceModel struct {
	ID                            types.String                  `tfsdk:"id"`
	AssumeRole                    types.String                  `tfsdk:"assume_role"`
	CloudDiscovery                []awsAccountFeatureModel      `tfsdk:"cloud_discovery"`
	CloudNativeArchival           []awsAccountFeatureModel      `tfsdk:"cloud_native_archival"`
	CloudNativeConfigProtection   []awsAccountFeatureModel      `tfsdk:"cloud_native_config_protection"`
	CloudNativeDynamoDBProtection []awsAccountFeatureModel      `tfsdk:"cloud_native_dynamodb_protection"`
	CloudNativeProtection         []awsAccountFeatureModel      `tfsdk:"cloud_native_protection"`
	CloudNativeS3Protection       []awsAccountFeatureModel      `tfsdk:"cloud_native_s3_protection"`
	CloudNativeUEMKeyManagement   []awsAccountFeatureModel      `tfsdk:"cloud_native_uem_key_management"`
	CyberRecoveryDataScanning     []awsAccountFeatureModel      `tfsdk:"cyber_recovery_data_scanning"`
	DataScanning                  []awsAccountFeatureModel      `tfsdk:"data_scanning"`
	DeleteSnapshotsOnDestroy      types.Bool                    `tfsdk:"delete_snapshots_on_destroy"`
	DSPM                          []awsAccountFeatureModel      `tfsdk:"dspm"`
	Exocompute                    []awsAccountFeatureModel      `tfsdk:"exocompute"`
	KubernetesProtection          []awsAccountFeatureModel      `tfsdk:"kubernetes_protection"`
	Name                          types.String                  `tfsdk:"name"`
	Outpost                       []awsAccountOutpostModel      `tfsdk:"outpost"`
	Permissions                   types.String                  `tfsdk:"permissions"`
	Profile                       types.String                  `tfsdk:"profile"`
	RDSProtection                 []awsAccountFeatureModel      `tfsdk:"rds_protection"`
	RoleChaining                  []awsAccountRoleChainingModel `tfsdk:"role_chaining"`
	RoleChainingAccountID         types.String                  `tfsdk:"role_chaining_account_id"`
	ServersAndApps                []awsAccountFeatureModel      `tfsdk:"servers_and_apps"`
}

// accountFeatures returns the feature blocks of the model which have an
// element, in the order RSC requires them to be added: outpost,
// cloud_discovery, the regular features and role_chaining. They are removed in
// the reverse order.
func (m *awsAccountResourceModel) accountFeatures(ctx context.Context) ([]awsAccountFeature, diag.Diagnostics) {
	var diags diag.Diagnostics

	var features []awsAccountFeature
	for _, elem := range m.Outpost {
		features = append(features, elem.accountFeature(ctx, &diags))
	}
	for _, elem := range m.CloudDiscovery {
		features = append(features, elem.accountFeature(ctx, keyCloudDiscovery, &diags))
	}
	for _, elem := range m.CloudNativeArchival {
		features = append(features, elem.accountFeature(ctx, keyCloudNativeArchival, &diags))
	}
	for _, elem := range m.CloudNativeConfigProtection {
		features = append(features, elem.accountFeature(ctx, keyCloudNativeConfigProtection, &diags))
	}
	for _, elem := range m.CloudNativeDynamoDBProtection {
		features = append(features, elem.accountFeature(ctx, keyCloudNativeDynamoDBProtection, &diags))
	}
	for _, elem := range m.CloudNativeProtection {
		features = append(features, elem.accountFeature(ctx, keyCloudNativeProtection, &diags))
	}
	for _, elem := range m.CloudNativeS3Protection {
		features = append(features, elem.accountFeature(ctx, keyCloudNativeS3Protection, &diags))
	}
	for _, elem := range m.CloudNativeUEMKeyManagement {
		features = append(features, elem.accountFeature(ctx, keyCloudNativeUEMKeyManagement, &diags))
	}
	for _, elem := range m.CyberRecoveryDataScanning {
		features = append(features, elem.accountFeature(ctx, keyCyberRecoveryDataScanning, &diags))
	}
	for _, elem := range m.DataScanning {
		features = append(features, elem.accountFeature(ctx, keyDataScanning, &diags))
	}
	for _, elem := range m.DSPM {
		features = append(features, elem.accountFeature(ctx, keyDSPM, &diags))
	}
	for _, elem := range m.Exocompute {
		features = append(features, elem.accountFeature(ctx, keyExocompute, &diags))
	}
	for _, elem := range m.KubernetesProtection {
		features = append(features, elem.accountFeature(ctx, keyKubernetesProtection, &diags))
	}
	for _, elem := range m.RDSProtection {
		features = append(features, elem.accountFeature(ctx, keyRDSProtection, &diags))
	}
	for _, elem := range m.ServersAndApps {
		features = append(features, elem.accountFeature(ctx, keyServersAndApps, &diags))
	}
	for _, elem := range m.RoleChaining {
		features = append(features, elem.accountFeature(ctx, &diags))
	}

	return features, diags
}

// awsAccountFeatureModel is the model of an element of a regular feature
// block.
type awsAccountFeatureModel struct {
	PermissionGroups types.Set    `tfsdk:"permission_groups"`
	Regions          types.Set    `tfsdk:"regions"`
	StackARN         types.String `tfsdk:"stack_arn"`
	Status           types.String `tfsdk:"status"`
}

// newAWSAccountFeatureModel returns the regular feature block with the
// specified name for the account. The block is empty unless the account has
// all RSC features of the block. The regions are null when RSC returns none.
func newAWSAccountFeatureModel(account aws.CloudAccount, key string) []awsAccountFeatureModel {
	feature := awsAccountRSCFeature(&account, awsAccountBlockFeatures[key])
	if feature == nil {
		return nil
	}

	groups := make([]attr.Value, 0, len(feature.PermissionGroups))
	for _, group := range feature.PermissionGroups {
		groups = append(groups, types.StringValue(string(group)))
	}

	regions := types.SetNull(types.StringType)
	if len(feature.Regions) > 0 {
		values := make([]attr.Value, 0, len(feature.Regions))
		for _, region := range feature.Regions {
			values = append(values, types.StringValue(region))
		}
		regions = types.SetValueMust(types.StringType, values)
	}

	return []awsAccountFeatureModel{{
		PermissionGroups: types.SetValueMust(types.StringType, groups),
		Regions:          regions,
		StackARN:         types.StringValue(feature.StackArn),
		Status:           types.StringValue(core.FormatStatus(feature.Status)),
	}}
}

// accountFeature returns the regular feature block with the specified name in
// RSC terms. An unknown region name is an error.
func (m awsAccountFeatureModel) accountFeature(ctx context.Context, key string, diags *diag.Diagnostics) awsAccountFeature {
	feature := awsAccountFeature{key: key}

	var groups []core.PermissionGroup
	diags.Append(m.PermissionGroups.ElementsAs(ctx, &groups, false)...)
	for _, f := range awsAccountBlockFeatures[key] {
		feature.features = append(feature.features, f.WithPermissionGroups(groups...))
	}

	var names []string
	diags.Append(m.Regions.ElementsAs(ctx, &names, false)...)
	for _, name := range names {
		region := gqlaws.RegionFromName(name)
		if region == gqlaws.RegionUnknown {
			diags.AddAttributeError(path.Root(key), "Invalid region", fmt.Sprintf("unknown region %q", name))
			continue
		}
		feature.regions = append(feature.regions, region)
	}

	return feature
}

// withComputed returns the element with the unknown status and stack ARN set
// from the account. They are set to null when the account is nil or doesn't
// have all RSC features of the regular feature block with the specified name.
func (m awsAccountFeatureModel) withComputed(account *aws.CloudAccount, key string) awsAccountFeatureModel {
	feature := awsAccountRSCFeature(account, awsAccountBlockFeatures[key])

	if m.Status.IsUnknown() {
		m.Status = types.StringNull()
		if feature != nil {
			m.Status = types.StringValue(core.FormatStatus(feature.Status))
		}
	}

	if m.StackARN.IsUnknown() {
		m.StackARN = types.StringNull()
		if feature != nil {
			m.StackARN = types.StringValue(feature.StackArn)
		}
	}

	return m
}

// awsAccountOutpostModel is the model of an element of the outpost block.
type awsAccountOutpostModel struct {
	OutpostAccountID      types.String `tfsdk:"outpost_account_id"`
	OutpostAccountProfile types.String `tfsdk:"outpost_account_profile"`
	PermissionGroups      types.Set    `tfsdk:"permission_groups"`
	StackARN              types.String `tfsdk:"stack_arn"`
	Status                types.String `tfsdk:"status"`
}

// newAWSAccountOutpostModel returns the outpost block for the account, with the
// specified outpost account ID and profile. The block is empty unless the
// account has the outpost feature.
func newAWSAccountOutpostModel(account aws.CloudAccount, outpostID, outpostProfile types.String) []awsAccountOutpostModel {
	feature := awsAccountRSCFeature(&account, awsAccountBlockFeatures[keyOutpost])
	if feature == nil {
		return nil
	}

	groups := make([]attr.Value, 0, len(feature.PermissionGroups))
	for _, group := range feature.PermissionGroups {
		groups = append(groups, types.StringValue(string(group)))
	}

	return []awsAccountOutpostModel{{
		OutpostAccountID:      outpostID,
		OutpostAccountProfile: outpostProfile,
		PermissionGroups:      types.SetValueMust(types.StringType, groups),
		StackARN:              types.StringValue(feature.StackArn),
		Status:                types.StringValue(core.FormatStatus(feature.Status)),
	}}
}

// accountFeature returns the outpost block in RSC terms.
func (m awsAccountOutpostModel) accountFeature(ctx context.Context, diags *diag.Diagnostics) awsAccountFeature {
	var groups []core.PermissionGroup
	diags.Append(m.PermissionGroups.ElementsAs(ctx, &groups, false)...)

	return awsAccountFeature{
		key:            keyOutpost,
		features:       []core.Feature{core.FeatureOutpost.WithPermissionGroups(groups...)},
		outpostID:      m.OutpostAccountID.ValueString(),
		outpostProfile: m.OutpostAccountProfile.ValueString(),
	}
}

// withComputed returns the element with the unknown status and stack ARN set
// from the account. They are set to null when the account is nil or doesn't
// have the outpost feature.
func (m awsAccountOutpostModel) withComputed(account *aws.CloudAccount) awsAccountOutpostModel {
	feature := awsAccountRSCFeature(account, awsAccountBlockFeatures[keyOutpost])

	if m.Status.IsUnknown() {
		m.Status = types.StringNull()
		if feature != nil {
			m.Status = types.StringValue(core.FormatStatus(feature.Status))
		}
	}

	if m.StackARN.IsUnknown() {
		m.StackARN = types.StringNull()
		if feature != nil {
			m.StackARN = types.StringValue(feature.StackArn)
		}
	}

	return m
}

// awsAccountRoleChainingModel is the model of an element of the role_chaining
// block.
type awsAccountRoleChainingModel struct {
	PermissionGroups types.Set    `tfsdk:"permission_groups"`
	StackARN         types.String `tfsdk:"stack_arn"`
	Status           types.String `tfsdk:"status"`
}

// newAWSAccountRoleChainingModel returns the role_chaining block for the
// account. The block is empty unless the account has the role chaining
// feature.
func newAWSAccountRoleChainingModel(account aws.CloudAccount) []awsAccountRoleChainingModel {
	feature := awsAccountRSCFeature(&account, awsAccountBlockFeatures[keyRoleChaining])
	if feature == nil {
		return nil
	}

	groups := make([]attr.Value, 0, len(feature.PermissionGroups))
	for _, group := range feature.PermissionGroups {
		groups = append(groups, types.StringValue(string(group)))
	}

	return []awsAccountRoleChainingModel{{
		PermissionGroups: types.SetValueMust(types.StringType, groups),
		StackARN:         types.StringValue(feature.StackArn),
		Status:           types.StringValue(core.FormatStatus(feature.Status)),
	}}
}

// accountFeature returns the role_chaining block in RSC terms.
func (m awsAccountRoleChainingModel) accountFeature(ctx context.Context, diags *diag.Diagnostics) awsAccountFeature {
	var groups []core.PermissionGroup
	diags.Append(m.PermissionGroups.ElementsAs(ctx, &groups, false)...)

	return awsAccountFeature{
		key:      keyRoleChaining,
		features: []core.Feature{core.FeatureRoleChaining.WithPermissionGroups(groups...)},
	}
}

// withComputed returns the element with the unknown status and stack ARN set
// from the account. They are set to null when the account is nil or doesn't
// have the role chaining feature.
func (m awsAccountRoleChainingModel) withComputed(account *aws.CloudAccount) awsAccountRoleChainingModel {
	feature := awsAccountRSCFeature(account, awsAccountBlockFeatures[keyRoleChaining])

	if m.Status.IsUnknown() {
		m.Status = types.StringNull()
		if feature != nil {
			m.Status = types.StringValue(core.FormatStatus(feature.Status))
		}
	}

	if m.StackARN.IsUnknown() {
		m.StackARN = types.StringNull()
		if feature != nil {
			m.StackARN = types.StringValue(feature.StackArn)
		}
	}

	return m
}

type awsAccountIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAwsAccountResource() resource.Resource {
	return &awsAccountResource{prefix: keyRubrik}
}

func newPolarisAwsAccountResource() resource.Resource {
	return &awsAccountResource{prefix: keyPolaris}
}

func (r *awsAccountResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsAccountResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSAccount
}

func (r *awsAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsAccountResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAWSAccountDescription),
		Version:     3,
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyAssumeRole: schema.StringAttribute{
				Optional:    true,
				Description: "Role ARN of role to assume.",
				Validators: []validator.String{
					isARN(),
					stringvalidator.AtLeastOneOf(path.MatchRoot(keyProfile)),
				},
			},
			keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Should snapshots be deleted when the resource is destroyed. Default value is `false`.",
			},
			keyName: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Account name in RSC. If not given the name is taken from AWS Organizations or, if " +
					"the required permissions are missing, is derived from the AWS account ID and the named profile.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyPermissions: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "When set to 'update' feature permissions can be updated by applying the " +
					"configuration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("update"),
				},
			},
			keyProfile: schema.StringAttribute{
				Optional:    true,
				Description: "AWS named profile.",
				Validators: []validator.String{
					isNotWhiteSpace(),
					stringvalidator.AtLeastOneOf(path.MatchRoot(keyAssumeRole)),
				},
			},
			keyRoleChainingAccountID: schema.StringAttribute{
				Optional: true,
				Description: "RSC cloud account ID of the AWS account with the Role Chaining feature " +
					"enabled. When specified, the account will use cross-account role chaining.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			keyCloudDiscovery: awsAccountFeatureBlock(core.CloudDiscovery,
				"Enable the Cloud Discovery feature for the account."),
			keyCloudNativeArchival: awsAccountFeatureBlock(core.CloudNativeArchival,
				"Enable the Cloud Native Archival feature for the account."),
			keyCloudNativeConfigProtection: awsAccountFeatureBlock(core.CloudNativeConfigProtection,
				"Enable the Cloud Applications feature, also known as Cloud Native Config Protection, for the "+
					"account."),
			keyCloudNativeDynamoDBProtection: awsAccountFeatureBlock(core.CloudNativeDynamoDBProtection,
				"Enable the Cloud Native DynamoDB Protection feature for the account."),
			keyCloudNativeProtection: awsAccountFeatureBlock(core.CloudNativeProtection,
				"Enable the Cloud Native Protection feature for the account.",
				// The following permission groups cannot be used when onboarding
				// an AWS account. They have been accepted in the past so we still
				// silently allow them.
				core.PermissionGroupExportAndRestore,
				core.PermissionGroupFileLevelRecovery,
				core.PermissionGroupSnapshotPrivateAccess,
			),
			keyCloudNativeS3Protection: awsAccountFeatureBlock(core.CloudNativeS3Protection,
				"Enable the Cloud Native S3 Protection feature for the account."),
			keyCloudNativeUEMKeyManagement: awsAccountFeatureBlock(core.CloudNativeUEMKeyManagement,
				"Enable the Cloud Native UEM Key Management feature for the account. Required to register the "+
					"account's KMS as a key manager in RSC."),
			keyCyberRecoveryDataScanning: awsAccountFeatureBlock(core.CyberRecoveryDataClassificationData,
				"Enable the Cyber Recovery Data Scanning feature for the account. The Cyber Recovery Data "+
					"Scanning feature requires the Outpost feature to be enabled."),
			keyDataScanning: awsAccountFeatureBlock(core.LaminarInternal,
				"Enable the Data Scanning feature for the account. The Data Scanning feature requires the "+
					"Outpost feature to be enabled."),
			keyDSPM: awsAccountFeatureBlock(core.DSPMData,
				"Enable the DSPM feature for the account. The DSPM feature requires the Outpost feature to be "+
					"enabled."),
			keyExocompute: awsAccountFeatureBlock(core.Exocompute,
				"Enable the Exocompute feature for the account.",
				// The following permission groups cannot be used when onboarding
				// an AWS account. They have been accepted in the past so we still
				// silently allow them.
				core.PermissionGroupPrivateEndpoints,
			),
			keyKubernetesProtection: awsAccountFeatureBlock(core.KubernetesProtection,
				"Enable the Kubernetes Protection feature for the AWS account."),
			keyOutpost: awsAccountOutpostBlock,
			keyRDSProtection: awsAccountFeatureBlock(core.RDSProtection,
				"Enable the RDS Protection feature for the account."),
			keyRoleChaining: awsAccountRoleChainingBlock,
			keyServersAndApps: awsAccountFeatureBlock(core.ServerAndApps,
				"Enable the Servers and Apps feature for the account."),
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_aws_account` instead."
	}
}

// awsAccountFeatureBlock returns the schema of a regular feature block. The
// permission groups are those of the feature with the specified name. The
// extra permission groups are accepted but not documented.
//
// UseNonNullStateForUnknown keeps the stack ARN in the plan. A block added to
// an existing account has a null stack ARN in state, which UseStateForUnknown
// would plan as the value after apply.
func awsAccountFeatureBlock(featureName, description string, extraGroups ...core.PermissionGroup) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		Validators: []validator.List{
			listvalidator.SizeAtMost(1),
		},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				keyPermissionGroups: schema.SetAttribute{
					ElementType: types.StringType,
					Required:    true,
					Description: fmt.Sprintf("Permission groups to assign to the feature. %s.",
						possiblePermissionGroups(core.AWSCloudFormationFeatures(), featureName)),
					Validators: []validator.Set{
						setvalidator.ValueStringsAre(
							permissionGroupValidator(core.AWSCloudFormationFeatures(), featureName, extraGroups...)),
					},
				},
				keyRegions: schema.SetAttribute{
					ElementType: types.StringType,
					Required:    true,
					Description: "Regions the feature will be enabled in.",
					Validators: []validator.Set{
						setvalidator.SizeAtLeast(1),
						setvalidator.ValueStringsAre(isNotWhiteSpace()),
					},
				},
				keyStackARN: schema.StringAttribute{
					Computed:    true,
					Description: "CloudFormation stack ARN.",
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseNonNullStateForUnknown(),
					},
				},
				keyStatus: schema.StringAttribute{
					Computed:    true,
					Description: "Status of the feature.",
				},
			},
		},
	}
}

// awsAccountOutpostBlock is the schema of the outpost block.
var awsAccountOutpostBlock = schema.ListNestedBlock{
	Description: "Enable the Outpost feature for the account. To use the DSPM, Data Scanning and " +
		"Cyber Recovery Data Scanning features, one account must have the Outpost feature enabled.",
	Validators: []validator.List{
		listvalidator.SizeAtMost(1),
	},
	NestedObject: schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			keyOutpostAccountID: schema.StringAttribute{
				Optional:    true,
				Description: "AWS account ID of the outpost account. Defaults to the current account.",
				Validators: []validator.String{
					isAWSAccountID(),
				},
			},
			keyOutpostAccountProfile: schema.StringAttribute{
				Optional: true,
				Description: "AWS named profile for the outpost account. Defaults to the profile used " +
					"for the current account.",
				Validators: []validator.String{
					isNotWhiteSpace(),
					stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(keyOutpostAccountID)),
				},
			},
			keyPermissionGroups: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: fmt.Sprintf("Permission groups to assign to the Outpost feature. %s.",
					possiblePermissionGroups(core.AWSCloudFormationFeatures(), core.Outpost)),
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(
						permissionGroupValidator(core.AWSCloudFormationFeatures(), core.Outpost)),
				},
			},
			keyStackARN: schema.StringAttribute{
				Computed:    true,
				Description: "CloudFormation stack ARN.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseNonNullStateForUnknown(),
				},
			},
			keyStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Status of the Outpost feature.",
			},
		},
	},
}

// awsAccountRoleChainingBlock is the schema of the role_chaining block.
var awsAccountRoleChainingBlock = schema.ListNestedBlock{
	Description: "Enable the Role Chaining feature for the account. This feature is mutually " +
		"exclusive with all other features.",
	Validators: []validator.List{
		listvalidator.SizeAtMost(1),
	},
	NestedObject: schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			keyPermissionGroups: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: fmt.Sprintf("Permission groups to assign to the Role Chaining feature. %s.",
					possiblePermissionGroups(core.AWSCloudFormationFeatures(), core.RoleChaining)),
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(
						permissionGroupValidator(core.AWSCloudFormationFeatures(), core.RoleChaining)),
				},
			},
			keyStackARN: schema.StringAttribute{
				Computed:    true,
				Description: "CloudFormation stack ARN.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseNonNullStateForUnknown(),
				},
			},
			keyStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Status of the Role Chaining feature.",
			},
		},
	},
}

func (r *awsAccountResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsAccountResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC cloud account ID (UUID).",
			},
		},
	}
}

// ValidateConfig checks the combinations of feature blocks: at least one block
// must be specified, role_chaining can't be combined with any other block or
// with role_chaining_account_id, and an outpost block with an outpost account
// ID needs another block.
func (r *awsAccountResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "awsAccountResource.ValidateConfig")

	// A dynamic block with an unknown for_each makes the whole block unknown,
	// which can't be read into the model. Nothing is checked until it's known.
	var present []string
	for key := range req.Config.Schema.GetBlocks() {
		var block types.List
		res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(key), &block)...)
		if res.Diagnostics.HasError() || block.IsUnknown() {
			return
		}
		if len(block.Elements()) > 0 {
			present = append(present, key)
		}
	}
	slices.Sort(present)

	var config awsAccountResourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	if len(present) == 0 {
		res.Diagnostics.AddError("Missing feature block", fmt.Sprintf("At least one of %s must be specified.",
			strings.Join(slices.Sorted(maps.Keys(awsAccountBlockFeatures)), ", ")))
		return
	}

	if len(config.RoleChaining) > 0 {
		others := slices.DeleteFunc(slices.Clone(present), func(key string) bool { return key == keyRoleChaining })
		if len(others) > 0 {
			res.Diagnostics.AddAttributeError(path.Root(keyRoleChaining), "Invalid block combination",
				fmt.Sprintf("role_chaining conflicts with %s.", strings.Join(others, ", ")))
		}
		if !config.RoleChainingAccountID.IsNull() && !config.RoleChainingAccountID.IsUnknown() {
			res.Diagnostics.AddAttributeError(path.Root(keyRoleChainingAccountID), "Invalid attribute combination",
				"role_chaining_account_id conflicts with role_chaining.")
		}
	}

	// A separate outpost account returns no RSC cloud account ID for the
	// account, which then needs another feature.
	if len(config.Outpost) > 0 && len(present) == 1 && config.Outpost[0].OutpostAccountID.ValueString() != "" {
		res.Diagnostics.AddAttributeError(path.Root(keyOutpost), "Invalid block combination",
			"when outpost_account_id is specified for the outpost feature, at least one other feature must be "+
				"enabled")
	}
}

// ModifyPlan rejects an update removing the cloud_discovery block while a
// protection feature block remains, and an update adding the role_chaining
// block to an account with other feature blocks in state.
func (r *awsAccountResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	tflog.Trace(ctx, "awsAccountResource.ModifyPlan")

	// Nothing to check on create or destroy.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	// A dynamic block with an unknown for_each makes the whole block unknown,
	// which can't be read into the model. Nothing is checked until it's known.
	for key := range req.Plan.Schema.GetBlocks() {
		var block types.List
		res.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(key), &block)...)
		if res.Diagnostics.HasError() || block.IsUnknown() {
			return
		}
	}

	var state, plan awsAccountResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Cloud Discovery is the parent of the protection features and must
	// outlive them.
	hasProtection := len(plan.CloudNativeProtection) > 0 || len(plan.CloudNativeConfigProtection) > 0 ||
		len(plan.CloudNativeDynamoDBProtection) > 0 || len(plan.CloudNativeS3Protection) > 0 ||
		len(plan.KubernetesProtection) > 0 || len(plan.RDSProtection) > 0
	if len(state.CloudDiscovery) > 0 && len(plan.CloudDiscovery) == 0 && hasProtection {
		res.Diagnostics.AddAttributeError(path.Root(keyCloudDiscovery), "Invalid feature removal",
			"cloud_discovery cannot be removed while protection features are enabled")
	}

	// The Role Chaining feature is mutually exclusive with all other features.
	if len(plan.RoleChaining) > 0 && len(state.RoleChaining) == 0 {
		features, diags := state.accountFeatures(ctx)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		if len(features) > 0 {
			res.Diagnostics.AddAttributeError(path.Root(keyRoleChaining), "Invalid feature addition",
				fmt.Sprintf("role_chaining cannot be added while %s is enabled", features[0].key))
		}
	}
}

func (r *awsAccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsAccountResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsAccountResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsAccountResource.Create")

	var plan awsAccountResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	features, diags := plan.accountFeatures(ctx)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// The Cyber Recovery Data Scanning, Data Scanning and DSPM features need an
	// outpost account, in this resource or already onboarded in RSC.
	needsOutpost := len(plan.CyberRecoveryDataScanning) > 0 || len(plan.DataScanning) > 0 || len(plan.DSPM) > 0
	if needsOutpost && len(plan.Outpost) == 0 {
		outposts, err := aws.Wrap(polarisClient).AccountsByFeatureStatus(ctx, core.FeatureOutpost, "",
			[]core.Status{core.StatusConnected, core.StatusMissingPermissions})
		if err == nil && len(outposts) == 0 {
			err = errors.New("cyber_recovery_data_scanning, data_scanning and dspm require an outpost account")
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to find an outpost account", err.Error())
			return
		}
	}

	profile := plan.Profile.ValueString()
	roleARN := plan.AssumeRole.ValueString()
	var account aws.AccountFunc
	switch {
	case profile != "" && roleARN == "":
		account = aws.Profile(profile)
	case profile != "":
		account = aws.ProfileWithRole(profile, roleARN)
	default:
		account = aws.DefaultWithRole(roleARN)
	}

	var opts []aws.OptionFunc
	if name := plan.Name.ValueString(); name != "" {
		opts = append(opts, aws.Name(name))
	}
	if roleChainingID := plan.RoleChainingAccountID.ValueString(); roleChainingID != "" {
		opts = append(opts, aws.RoleChainingAccountID(roleChainingID))
	}

	// Onboard the features, those with the same regions together. The ID is
	// saved as soon as RSC returns it, so a later failure doesn't leave the
	// account untracked. Adding the outpost feature to a separate outpost
	// account returns no ID.
	var id uuid.UUID
	for _, feature := range awsAccountMergeFeatures(features) {
		featureOpts := slices.Clone(opts)
		for _, region := range feature.regions {
			featureOpts = append(featureOpts, aws.Region(region.Name()))
		}
		if feature.outpostID != "" {
			if feature.outpostProfile != "" {
				featureOpts = append(featureOpts, aws.OutpostAccountWithProfile(feature.outpostID, feature.outpostProfile))
			} else {
				featureOpts = append(featureOpts, aws.OutpostAccount(feature.outpostID))
			}
		}

		cloudAccountID, err := aws.Wrap(polarisClient).AddAccountWithCFT(ctx, account, feature.features, featureOpts...)
		if err != nil {
			res.Diagnostics.AddError("Failed to add AWS account", err.Error())
			return
		}
		if cloudAccountID != uuid.Nil && id == uuid.Nil {
			id = cloudAccountID
			plan.ID = types.StringValue(id.String())
			res.Diagnostics.Append(res.State.Set(ctx, awsAccountComputed(plan, nil))...)
			res.Diagnostics.Append(res.Identity.Set(ctx, awsAccountIdentityModel{ID: plan.ID})...)
			if res.Diagnostics.HasError() {
				return
			}
		}
	}
	if id == uuid.Nil {
		res.Diagnostics.AddError("Failed to add AWS account", "RSC returned no cloud account ID for the account")
		return
	}

	// Set the computed values from the onboarded account.
	cloudAccount, err := aws.Wrap(polarisClient).AccountByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read AWS account after create",
			fmt.Sprintf("The account was added successfully but could not be read back: %s", err))
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, awsAccountComputed(plan, &cloudAccount))...)
}

func (r *awsAccountResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsAccountResource.Read")

	var state awsAccountResourceModel
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
		res.Diagnostics.AddError("Invalid AWS account ID", err.Error())
		return
	}

	account, err := aws.Wrap(polarisClient).AccountByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS account", err.Error())
		return
	}

	// A separate outpost account is identified by the outpost account ID.
	var outpostAccount *aws.CloudAccount
	if len(state.Outpost) > 0 && state.Outpost[0].OutpostAccountID.ValueString() != "" {
		outposts, err := aws.Wrap(polarisClient).AccountsByFeatureStatus(ctx, core.FeatureOutpost, "",
			[]core.Status{core.StatusConnected, core.StatusMissingPermissions})
		if err != nil {
			res.Diagnostics.AddError("Failed to read AWS outpost account", err.Error())
			return
		}
		if len(outposts) > 0 {
			outpostAccount = &outposts[0]
		}
	}

	updateAWSAccountResourceModel(&state, account, outpostAccount)
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, awsAccountIdentityModel{ID: state.ID})...)
}

func (r *awsAccountResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsAccountResource.Update")

	var plan, state awsAccountResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	profile := plan.Profile.ValueString()
	roleARN := plan.AssumeRole.ValueString()
	var account aws.AccountFunc
	switch {
	case profile != "" && roleARN == "":
		account = aws.Profile(profile)
	case profile != "":
		account = aws.ProfileWithRole(profile, roleARN)
	default:
		account = aws.DefaultWithRole(roleARN)
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid AWS account ID", err.Error())
		return
	}

	// Check that the resource ID and the AWS profile refer to the same account.
	config, err := account(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to look up native account ID", err.Error())
		return
	}
	cloudAccount, err := aws.Wrap(polarisClient).AccountByNativeID(ctx, config.NativeID)
	if errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("AWS account not found", "account identified by profile/role could not be found")
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to look up AWS account", err.Error())
		return
	}
	if cloudAccount.ID != id {
		res.Diagnostics.AddError("AWS account mismatch", "resource id and profile/role refer to different accounts")
		return
	}

	oldFeatures, diags := state.accountFeatures(ctx)
	res.Diagnostics.Append(diags...)
	newFeatures, diags := plan.accountFeatures(ctx)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// The Cyber Recovery Data Scanning, Data Scanning and DSPM features need an
	// outpost account, in this resource or already onboarded in RSC.
	addsOutpostDependent := len(state.CyberRecoveryDataScanning) == 0 && len(plan.CyberRecoveryDataScanning) > 0 ||
		len(state.DataScanning) == 0 && len(plan.DataScanning) > 0 ||
		len(state.DSPM) == 0 && len(plan.DSPM) > 0
	if addsOutpostDependent && len(plan.Outpost) == 0 {
		outposts, err := aws.Wrap(polarisClient).AccountsByFeatureStatus(ctx, core.FeatureOutpost, "",
			[]core.Status{core.StatusConnected, core.StatusMissingPermissions})
		if err == nil && len(outposts) == 0 {
			err = errors.New("cyber_recovery_data_scanning, data_scanning and dspm require an outpost account")
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to find an outpost account", err.Error())
			return
		}
	}

	// The Role Chaining feature is mutually exclusive with all other features,
	// so it's removed before any feature is added.
	deleteSnapshots := plan.DeleteSnapshotsOnDestroy.ValueBool()
	if len(state.RoleChaining) > 0 && len(plan.RoleChaining) == 0 {
		err := aws.Wrap(polarisClient).RemoveAccountWithCFT(ctx, account, awsAccountBlockFeatures[keyRoleChaining],
			deleteSnapshots)
		if err != nil {
			res.Diagnostics.AddError("Failed to remove the role_chaining feature", err.Error())
			return
		}
	}

	// Add the blocks added to the configuration and update the changed ones, in
	// order, so a feature is added after the features it depends on.
	for _, newFeature := range newFeatures {
		var opts []aws.OptionFunc
		for _, region := range newFeature.regions {
			opts = append(opts, aws.Region(region.Name()))
		}

		i := slices.IndexFunc(oldFeatures, func(f awsAccountFeature) bool { return f.key == newFeature.key })
		if i == -1 {
			if newFeature.outpostID != "" {
				if newFeature.outpostProfile != "" {
					opts = append(opts, aws.OutpostAccountWithProfile(newFeature.outpostID, newFeature.outpostProfile))
				} else {
					opts = append(opts, aws.OutpostAccount(newFeature.outpostID))
				}
			}
			if roleChainingID := plan.RoleChainingAccountID.ValueString(); roleChainingID != "" {
				opts = append(opts, aws.RoleChainingAccountID(roleChainingID))
			}
			if _, err := aws.Wrap(polarisClient).AddAccountWithCFT(ctx, account, newFeature.features, opts...); err != nil {
				res.Diagnostics.AddError(fmt.Sprintf("Failed to add the %s feature", newFeature.key), err.Error())
				return
			}
			continue
		}

		// Only the regions of a changed block are updated.
		oldFeature := oldFeatures[i]
		if awsAccountRegionsKey(oldFeature.regions) == awsAccountRegionsKey(newFeature.regions) &&
			slices.EqualFunc(oldFeature.features, newFeature.features, core.Feature.DeepEqual) &&
			oldFeature.outpostID == newFeature.outpostID && oldFeature.outpostProfile == newFeature.outpostProfile {
			continue
		}
		for _, feature := range newFeature.features {
			if err := aws.Wrap(polarisClient).UpdateAccount(ctx, id, feature, opts...); err != nil {
				res.Diagnostics.AddError(fmt.Sprintf("Failed to update the %s feature", newFeature.key), err.Error())
				return
			}
		}
	}

	// Remove the blocks removed from the configuration after the features are
	// added, so the account always keeps a feature. RSC removes an account
	// without features, and its snapshots become relics. The blocks are
	// removed in reverse order, so a feature is removed before the features it
	// depends on.
	for _, oldFeature := range slices.Backward(oldFeatures) {
		if oldFeature.key == keyOutpost || oldFeature.key == keyRoleChaining ||
			slices.ContainsFunc(newFeatures, func(f awsAccountFeature) bool { return f.key == oldFeature.key }) {
			continue
		}
		err := aws.Wrap(polarisClient).RemoveAccountWithCFT(ctx, account, awsAccountBlockFeatures[oldFeature.key],
			deleteSnapshots)
		if err != nil {
			res.Diagnostics.AddError(fmt.Sprintf("Failed to remove the %s feature", oldFeature.key), err.Error())
			return
		}
	}

	// The outpost feature is removed last. A separate outpost account is
	// identified by the outpost account ID and profile.
	if len(state.Outpost) > 0 && len(plan.Outpost) == 0 {
		if err := awsAccountCheckOutpostMappedAccounts(ctx, polarisClient); err != nil {
			res.Diagnostics.AddError("Failed to remove the outpost feature", err.Error())
			return
		}
		outpostAccount := account
		if outpostID := state.Outpost[0].OutpostAccountID.ValueString(); outpostID != "" {
			outpostAccount = aws.ProfileWithAccountID(profile, outpostID)
			if outpostProfile := state.Outpost[0].OutpostAccountProfile.ValueString(); outpostProfile != "" {
				outpostAccount = aws.Profile(outpostProfile)
			}
		}
		err := aws.Wrap(polarisClient).RemoveAccountWithCFT(ctx, outpostAccount, []core.Feature{core.FeatureOutpost},
			deleteSnapshots)
		if err != nil {
			res.Diagnostics.AddError("Failed to remove the outpost feature", err.Error())
			return
		}
	}

	if state.Permissions.ValueString() == awsAccountPermissionsUpdateRequired &&
		plan.Permissions.ValueString() == "update" {
		var features []core.Feature
		for _, feature := range cloudAccount.Features {
			if feature.Status == core.StatusMissingPermissions {
				features = append(features, feature.Feature)
			}
		}
		if err := aws.Wrap(polarisClient).UpdatePermissions(ctx, account, features); err != nil {
			res.Diagnostics.AddError("Failed to update AWS account permissions", err.Error())
			return
		}
	}

	// The feature passed is only used to update regions, so any feature will do.
	if name := plan.Name; !name.IsUnknown() && !name.IsNull() && !name.Equal(state.Name) {
		err := aws.Wrap(polarisClient).UpdateAccount(ctx, id, core.FeatureCloudNativeProtection,
			aws.Name(name.ValueString()))
		if err != nil {
			res.Diagnostics.AddError("Failed to update AWS account name", err.Error())
			return
		}
	}

	// Set the computed values from the updated account.
	cloudAccount, err = aws.Wrap(polarisClient).AccountByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS account", err.Error())
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, awsAccountComputed(plan, &cloudAccount))...)
	res.Diagnostics.Append(res.Identity.Set(ctx, awsAccountIdentityModel{ID: plan.ID})...)
}

func (r *awsAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsAccountResource.Delete")

	var state awsAccountResourceModel
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
		res.Diagnostics.AddError("Invalid AWS account ID", err.Error())
		return
	}

	profile := state.Profile.ValueString()
	roleARN := state.AssumeRole.ValueString()
	var account aws.AccountFunc
	switch {
	case profile != "" && roleARN == "":
		account = aws.Profile(profile)
	case profile != "":
		account = aws.ProfileWithRole(profile, roleARN)
	default:
		account = aws.DefaultWithRole(roleARN)
	}

	// Check that the resource ID and the AWS profile refer to the same account.
	config, err := account(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to look up native account ID", err.Error())
		return
	}
	cloudAccount, err := aws.Wrap(polarisClient).AccountByNativeID(ctx, config.NativeID)
	if errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("AWS account not found", "account identified by profile/role could not be found")
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to look up AWS account", err.Error())
		return
	}
	if cloudAccount.ID != id {
		res.Diagnostics.AddError("AWS account mismatch", "resource id and profile/role refer to different accounts")
		return
	}

	features, diags := state.accountFeatures(ctx)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// The protection features are removed first, then the Cloud Discovery
	// feature and last the outpost feature, since other features depend on it.
	deleteSnapshots := state.DeleteSnapshotsOnDestroy.ValueBool()
	var removeFeatures []core.Feature
	for _, feature := range features {
		if feature.key != keyCloudDiscovery && feature.key != keyOutpost {
			removeFeatures = append(removeFeatures, awsAccountBlockFeatures[feature.key]...)
		}
	}
	if len(removeFeatures) > 0 {
		if err := aws.Wrap(polarisClient).RemoveAccountWithCFT(ctx, account, removeFeatures, deleteSnapshots); err != nil {
			res.Diagnostics.AddError("Failed to remove AWS account", err.Error())
			return
		}
	}

	if len(state.CloudDiscovery) > 0 {
		err := aws.Wrap(polarisClient).RemoveAccountWithCFT(ctx, account, []core.Feature{core.FeatureCloudDiscovery},
			deleteSnapshots)
		if err != nil {
			res.Diagnostics.AddError("Failed to remove AWS account", err.Error())
			return
		}
	}

	// A separate outpost account is identified by the outpost account ID and
	// profile.
	if len(state.Outpost) > 0 {
		if err := awsAccountCheckOutpostMappedAccounts(ctx, polarisClient); err != nil {
			res.Diagnostics.AddError("Failed to remove AWS account", err.Error())
			return
		}
		outpostAccount := account
		if outpostID := state.Outpost[0].OutpostAccountID.ValueString(); outpostID != "" {
			outpostAccount = aws.ProfileWithAccountID(profile, outpostID)
			if outpostProfile := state.Outpost[0].OutpostAccountProfile.ValueString(); outpostProfile != "" {
				outpostAccount = aws.Profile(outpostProfile)
			}
		}
		err := aws.Wrap(polarisClient).RemoveAccountWithCFT(ctx, outpostAccount, []core.Feature{core.FeatureOutpost},
			deleteSnapshots)
		if err != nil {
			res.Diagnostics.AddError("Failed to remove AWS account", err.Error())
			return
		}
	}
}

func (r *awsAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsAccountResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
	if res.Diagnostics.HasError() {
		return
	}

	// The default isn't applied to imported state, Read keeps the value.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyDeleteSnapshotsOnDestroy), false)...)
}

// awsAccountBlockFeatures maps the feature blocks to their RSC features. A
// block with more than one RSC feature gives all of them the permission groups
// of the block.
var awsAccountBlockFeatures = map[string][]core.Feature{
	keyCloudDiscovery:                {core.FeatureCloudDiscovery},
	keyCloudNativeArchival:           {core.FeatureCloudNativeArchival},
	keyCloudNativeConfigProtection:   {core.FeatureCloudNativeConfigProtection},
	keyCloudNativeDynamoDBProtection: {core.FeatureCloudNativeDynamoDBProtection},
	keyCloudNativeProtection:         {core.FeatureCloudNativeProtection},
	keyCloudNativeS3Protection:       {core.FeatureCloudNativeS3Protection},
	keyCloudNativeUEMKeyManagement:   {core.FeatureCloudNativeUEMKeyManagement},
	keyCyberRecoveryDataScanning: {
		core.FeatureCyberRecoveryDataClassificationData,
		core.FeatureCyberRecoveryDataClassificationMetadata,
	},
	keyDataScanning: {
		core.FeatureLaminarInternal,
		core.FeatureLaminarCrossAccount,
	},
	keyDSPM: {
		core.FeatureDSPMData,
		core.FeatureDSPMMetadata,
	},
	keyExocompute:           {core.FeatureExocompute},
	keyKubernetesProtection: {core.FeatureKubernetesProtection},
	keyOutpost:              {core.FeatureOutpost},
	keyRDSProtection:        {core.FeatureRDSProtection},
	keyRoleChaining:         {core.FeatureRoleChaining},
	keyServersAndApps:       {core.FeatureServerAndApps},
}

// awsAccountFeature is a feature block in RSC terms. The RSC features have the
// permission groups of the block.
type awsAccountFeature struct {
	key            string
	features       []core.Feature
	regions        []gqlaws.Region
	outpostID      string
	outpostProfile string
}

// awsAccountMergeFeatures merges the feature blocks with the same regions, to
// reduce the number of CloudFormation stack updates. The result is sorted on
// the number of regions, which puts the outpost feature first.
func awsAccountMergeFeatures(features []awsAccountFeature) []awsAccountFeature {
	merged := make(map[string]*awsAccountFeature)
	for _, feature := range features {
		key := awsAccountRegionsKey(feature.regions)
		m, ok := merged[key]
		if !ok {
			merged[key] = &feature
			continue
		}
		m.features = slices.Concat(m.features, feature.features)
		if feature.outpostID != "" {
			m.outpostID = feature.outpostID
			m.outpostProfile = feature.outpostProfile
		}
	}

	result := make([]awsAccountFeature, 0, len(merged))
	for _, feature := range merged {
		result = append(result, *feature)
	}
	slices.SortFunc(result, func(i, j awsAccountFeature) int {
		return cmp.Compare(len(i.regions), len(j.regions))
	})

	return result
}

// awsAccountRegionsKey returns a key for the set of regions.
func awsAccountRegionsKey(regions []gqlaws.Region) string {
	var key strings.Builder
	for _, region := range slices.Sorted(slices.Values(regions)) {
		key.WriteString(strconv.Itoa(int(region)))
		key.WriteString(",")
	}

	return key.String()
}

// awsAccountCheckOutpostMappedAccounts checks that the outpost account isn't
// used by other accounts.
func awsAccountCheckOutpostMappedAccounts(ctx context.Context, client *polaris.Client) error {
	accounts, err := aws.Wrap(client).AccountsByFeatureStatus(ctx, core.FeatureOutpost, "",
		[]core.Status{core.StatusConnected, core.StatusMissingPermissions})
	if err != nil {
		return err
	}

	for _, account := range accounts {
		for _, feature := range account.Features {
			if len(feature.MappedAccounts) > 0 {
				return errors.New("outpost feature is still enabled for other accounts")
			}
		}
	}

	return nil
}

// updateAWSAccountResourceModel updates the model with the AWS cloud account
// read from RSC. Fields RSC doesn't return, profile, assume_role and the
// outpost account profile, keep their values.
//
// A feature block is empty unless all RSC features of the block are onboarded,
// so a partly onboarded block shows up as a change and is onboarded again.
//
// When the outpost block has an outpost account ID, the outpost block is read
// from the outpost account, which is nil if RSC has none.
func updateAWSAccountResourceModel(m *awsAccountResourceModel, account aws.CloudAccount, outpostAccount *aws.CloudAccount) {
	m.Name = types.StringValue(account.Name)
	if account.RoleChainingAccountID != uuid.Nil {
		m.RoleChainingAccountID = types.StringValue(account.RoleChainingAccountID.String())
	}
	if m.DeleteSnapshotsOnDestroy.IsNull() {
		m.DeleteSnapshotsOnDestroy = types.BoolValue(false)
	}
	if slices.ContainsFunc(account.Features, func(f aws.Feature) bool { return f.Status == core.StatusMissingPermissions }) {
		m.Permissions = types.StringValue(awsAccountPermissionsUpdateRequired)
	}

	m.CloudDiscovery = newAWSAccountFeatureModel(account, keyCloudDiscovery)
	m.CloudNativeArchival = newAWSAccountFeatureModel(account, keyCloudNativeArchival)
	m.CloudNativeConfigProtection = newAWSAccountFeatureModel(account, keyCloudNativeConfigProtection)
	m.CloudNativeDynamoDBProtection = newAWSAccountFeatureModel(account, keyCloudNativeDynamoDBProtection)
	m.CloudNativeProtection = newAWSAccountFeatureModel(account, keyCloudNativeProtection)
	m.CloudNativeS3Protection = newAWSAccountFeatureModel(account, keyCloudNativeS3Protection)
	m.CloudNativeUEMKeyManagement = newAWSAccountFeatureModel(account, keyCloudNativeUEMKeyManagement)
	m.CyberRecoveryDataScanning = newAWSAccountFeatureModel(account, keyCyberRecoveryDataScanning)
	m.DataScanning = newAWSAccountFeatureModel(account, keyDataScanning)
	m.DSPM = newAWSAccountFeatureModel(account, keyDSPM)
	m.Exocompute = newAWSAccountFeatureModel(account, keyExocompute)
	m.KubernetesProtection = newAWSAccountFeatureModel(account, keyKubernetesProtection)
	m.RDSProtection = newAWSAccountFeatureModel(account, keyRDSProtection)
	m.ServersAndApps = newAWSAccountFeatureModel(account, keyServersAndApps)
	m.RoleChaining = newAWSAccountRoleChainingModel(account)

	var outpostID, outpostProfile types.String
	if len(m.Outpost) > 0 {
		outpostID = m.Outpost[0].OutpostAccountID
		outpostProfile = m.Outpost[0].OutpostAccountProfile
	}
	switch {
	case outpostID.ValueString() == "":
		m.Outpost = newAWSAccountOutpostModel(account, types.StringNull(), types.StringNull())
	case outpostAccount != nil:
		m.Outpost = newAWSAccountOutpostModel(*outpostAccount, types.StringValue(outpostAccount.NativeID), outpostProfile)
	default:
		m.Outpost = nil
	}
}

// awsAccountRSCFeature returns the feature of the account standing for a
// feature block with the specified RSC features, which is the first of them.
// Nil is returned when the account is nil or doesn't have all of them.
func awsAccountRSCFeature(account *aws.CloudAccount, features []core.Feature) *aws.Feature {
	if account == nil || len(features) == 0 {
		return nil
	}
	for _, f := range features {
		if _, ok := account.Feature(f); !ok {
			return nil
		}
	}

	feature, _ := account.Feature(features[0])
	return &feature
}

// awsAccountComputed returns the model with the computed values left unknown by
// the plan set from the account. Known values are kept, since Terraform
// requires the state after an apply to match them. When the account is nil, or
// doesn't have a feature, the unknown values are set to null, since an unknown
// value can't be saved to state.
//
// The blocks are new slices, so the model passed in isn't changed.
func awsAccountComputed(m awsAccountResourceModel, account *aws.CloudAccount) awsAccountResourceModel {
	if m.Name.IsUnknown() {
		m.Name = types.StringNull()
		if account != nil {
			m.Name = types.StringValue(account.Name)
		}
	}
	if m.Permissions.IsUnknown() {
		m.Permissions = types.StringNull()
		if account != nil && slices.ContainsFunc(account.Features, func(f aws.Feature) bool {
			return f.Status == core.StatusMissingPermissions
		}) {
			m.Permissions = types.StringValue(awsAccountPermissionsUpdateRequired)
		}
	}

	if len(m.CloudDiscovery) > 0 {
		m.CloudDiscovery = []awsAccountFeatureModel{m.CloudDiscovery[0].withComputed(account, keyCloudDiscovery)}
	}
	if len(m.CloudNativeArchival) > 0 {
		m.CloudNativeArchival = []awsAccountFeatureModel{
			m.CloudNativeArchival[0].withComputed(account, keyCloudNativeArchival),
		}
	}
	if len(m.CloudNativeConfigProtection) > 0 {
		m.CloudNativeConfigProtection = []awsAccountFeatureModel{
			m.CloudNativeConfigProtection[0].withComputed(account, keyCloudNativeConfigProtection),
		}
	}
	if len(m.CloudNativeDynamoDBProtection) > 0 {
		m.CloudNativeDynamoDBProtection = []awsAccountFeatureModel{
			m.CloudNativeDynamoDBProtection[0].withComputed(account, keyCloudNativeDynamoDBProtection),
		}
	}
	if len(m.CloudNativeProtection) > 0 {
		m.CloudNativeProtection = []awsAccountFeatureModel{
			m.CloudNativeProtection[0].withComputed(account, keyCloudNativeProtection),
		}
	}
	if len(m.CloudNativeS3Protection) > 0 {
		m.CloudNativeS3Protection = []awsAccountFeatureModel{
			m.CloudNativeS3Protection[0].withComputed(account, keyCloudNativeS3Protection),
		}
	}
	if len(m.CloudNativeUEMKeyManagement) > 0 {
		m.CloudNativeUEMKeyManagement = []awsAccountFeatureModel{
			m.CloudNativeUEMKeyManagement[0].withComputed(account, keyCloudNativeUEMKeyManagement),
		}
	}
	if len(m.CyberRecoveryDataScanning) > 0 {
		m.CyberRecoveryDataScanning = []awsAccountFeatureModel{
			m.CyberRecoveryDataScanning[0].withComputed(account, keyCyberRecoveryDataScanning),
		}
	}
	if len(m.DataScanning) > 0 {
		m.DataScanning = []awsAccountFeatureModel{m.DataScanning[0].withComputed(account, keyDataScanning)}
	}
	if len(m.DSPM) > 0 {
		m.DSPM = []awsAccountFeatureModel{m.DSPM[0].withComputed(account, keyDSPM)}
	}
	if len(m.Exocompute) > 0 {
		m.Exocompute = []awsAccountFeatureModel{m.Exocompute[0].withComputed(account, keyExocompute)}
	}
	if len(m.KubernetesProtection) > 0 {
		m.KubernetesProtection = []awsAccountFeatureModel{
			m.KubernetesProtection[0].withComputed(account, keyKubernetesProtection),
		}
	}
	if len(m.RDSProtection) > 0 {
		m.RDSProtection = []awsAccountFeatureModel{m.RDSProtection[0].withComputed(account, keyRDSProtection)}
	}
	if len(m.ServersAndApps) > 0 {
		m.ServersAndApps = []awsAccountFeatureModel{m.ServersAndApps[0].withComputed(account, keyServersAndApps)}
	}

	// The outpost feature of a separate outpost account isn't on the account,
	// its status and stack ARN are null until the next Read.
	if len(m.Outpost) > 0 {
		m.Outpost = []awsAccountOutpostModel{m.Outpost[0].withComputed(account)}
	}
	if len(m.RoleChaining) > 0 {
		m.RoleChaining = []awsAccountRoleChainingModel{m.RoleChaining[0].withComputed(account)}
	}

	return m
}
