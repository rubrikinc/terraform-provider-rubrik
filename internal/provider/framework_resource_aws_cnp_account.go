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
	"slices"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const resourceAWSCNPAccountDescription = `
The ´rubrik_aws_cnp_account´ resource onboards an AWS account to RSC using the
AWS IAM roles workflow. To grant RSC permissions to perform certain operations
on the account, IAM roles need to be created and communicated to RSC using the
´rubrik_aws_cnp_attachments´ resource.
The roles and permissions needed by RSC can be looked up using the
´rubrik_aws_cnp_artifact´ and ´rubrik_aws_cnp_permissions´ data sources.

The ´CLOUD_DISCOVERY´ feature enables RSC to discover resources in the AWS
account without enabling protection. It is currently optional but will become
required when onboarding protection features. Once onboarded, it cannot be
removed unless all protection features are removed first.

-> **Note:** The ´feature´ block is shown as Optional in the schema below for
   technical reasons, but at least one ´feature´ block must be specified. The
   block-style syntax is preserved to remain compatible with existing Terraform
   configurations.

-> **Note:** To onboard an account using a CloudFormation stack instead of IAM
   roles, use the ´rubrik_aws_account´ resource.

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

var (
	_ resource.Resource                = &awsCnpAccountResource{}
	_ resource.ResourceWithConfigure   = &awsCnpAccountResource{}
	_ resource.ResourceWithIdentity    = &awsCnpAccountResource{}
	_ resource.ResourceWithImportState = &awsCnpAccountResource{}
	_ resource.ResourceWithModifyPlan  = &awsCnpAccountResource{}
	_ resource.ResourceWithMoveState   = &awsCnpAccountResource{}
)

type awsCnpAccountResource struct {
	client *client
	prefix string
}

type awsCnpAccountModel struct {
	ID                       types.String `tfsdk:"id"`
	Cloud                    types.String `tfsdk:"cloud"`
	DeleteSnapshotsOnDestroy types.Bool   `tfsdk:"delete_snapshots_on_destroy"`
	ExternalID               types.String `tfsdk:"external_id"`
	Feature                  types.Set    `tfsdk:"feature"`
	Name                     types.String `tfsdk:"name"`
	NativeID                 types.String `tfsdk:"native_id"`
	RoleChainingAccountID    types.String `tfsdk:"role_chaining_account_id"`
	Regions                  types.Set    `tfsdk:"regions"`
	TrustPolicies            types.Set    `tfsdk:"trust_policies"`
}

type awsCnpAccountIdentityModel struct {
	ID         types.String `tfsdk:"id"`
	ExternalID types.String `tfsdk:"external_id"`
}

func newAwsCnpAccountResource() resource.Resource {
	return &awsCnpAccountResource{prefix: keyRubrik}
}

func newPolarisAwsCnpAccountResource() resource.Resource {
	return &awsCnpAccountResource{prefix: keyPolaris}
}

func (r *awsCnpAccountResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSCNPAccount
}

func (r *awsCnpAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAWSCNPAccountDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCloud: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("STANDARD"),
				Description: "AWS cloud type. Possible values are `STANDARD`, `CHINA` and `GOV`. Default value is " +
					"`STANDARD`. Changing this forces a new resource to be created.",
				Validators: []validator.String{
					stringvalidator.OneOf("STANDARD", "CHINA", "GOV"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Should snapshots be deleted when the resource is destroyed. Default value is `false`.",
			},
			keyExternalID: schema.StringAttribute{
				Optional: true,
				Description: "External ID used in the AWS IAM trust policy. When omitted, RSC generates a " +
					"random external ID at onboarding. Once set the value cannot be changed; changing this " +
					"field forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Account name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyNativeID: schema.StringAttribute{
				Required:    true,
				Description: "AWS account ID. Changing this forces a new resource to be created.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			keyRoleChainingAccountID: schema.StringAttribute{
				Optional: true,
				Description: "RSC cloud account ID of the role chaining account. When specified, the account will " +
					"use cross-account role chaining. Changing this forces a new resource to be created.",
				Validators: []validator.String{
					isUUID(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			keyRegions: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "AWS regions.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(isNotWhiteSpace()),
				},
			},
			keyTrustPolicies: schema.SetNestedAttribute{
				Computed: true,
				Description: "AWS IAM trust policies required by RSC. The `policy` field should be used with the " +
					"`assume_role_policy` of the `aws_iam_role` resource.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyRoleKey: schema.StringAttribute{
							Computed: true,
							Description: "RSC artifact key for the AWS role. Possible values are `CROSSACCOUNT`, " +
								"`EXOCOMPUTE_EKS_MASTERNODE`, `EXOCOMPUTE_EKS_WORKERNODE` and `EXOCOMPUTE_EKS_LAMBDA`.",
						},
						keyPolicy: schema.StringAttribute{
							Computed:    true,
							Description: "AWS IAM trust policy.",
						},
					},
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
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

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_aws_cnp_account` instead."
	}
}

func (r *awsCnpAccountResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC cloud account ID (UUID).",
			},
			keyExternalID: identityschema.StringAttribute{
				OptionalForImport: true,
				Description: "External ID set when the account was onboarded. Omit for accounts onboarded without " +
					"an external ID (RSC generates one in that case). The value is stored as provided and is not " +
					"verified against RSC, since RSC does not return external IDs.",
			},
		},
	}
}

func (r *awsCnpAccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsCnpAccountResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.Create")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan awsCnpAccountModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	features, diags := awsToFeatures(ctx, plan.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	var regions []string
	res.Diagnostics.Append(plan.Regions.ElementsAs(ctx, &regions, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	var roleChainingAccountID uuid.UUID
	if !plan.RoleChainingAccountID.IsNull() {
		roleChainingAccountID, err = uuid.Parse(plan.RoleChainingAccountID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyRoleChainingAccountID), "Invalid UUID", err.Error())
			return
		}
	}

	cloud := plan.Cloud.ValueString()
	nativeID := plan.NativeID.ValueString()
	name := plan.Name.ValueString()
	externalID := plan.ExternalID.ValueString()

	id, err := aws.Wrap(polarisClient).AddAccountWithIAM(ctx, aws.AccountWithName(cloud, nativeID, name), features, aws.Regions(regions...))
	if err != nil {
		res.Diagnostics.AddError("Failed to add AWS account", err.Error())
		return
	}

	// The AWS account has been onboarded in RSC. Save partial state with the
	// ID immediately so Terraform can manage the resource (e.g. via destroy)
	// even if subsequent calls fail. trust_policies stays Unknown until Read
	// populates it on the next refresh.
	plan.ID = types.StringValue(id.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	policies, err := aws.Wrap(polarisClient).TrustPolicies(ctx, aws.TrustPoliciesParams{
		Cloud:                 gqlaws.Cloud(cloud),
		CloudAccountID:        id,
		Features:              features,
		ExternalID:            externalID,
		RoleChainingAccountID: roleChainingAccountID,
	})
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read AWS trust policies",
			fmt.Sprintf("The AWS account was onboarded successfully but the trust policies could not be read: %s", err.Error()))
		return
	}

	policySet, diags := awsFromTrustPolicies(policies)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	plan.TrustPolicies = policySet

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsCnpAccountIdentityModel{ID: plan.ID, ExternalID: plan.ExternalID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsCnpAccountResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.Read")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state awsCnpAccountModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud account ID", err.Error())
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

	features := make([]core.Feature, 0, len(account.Features))
	for _, feature := range account.Features {
		features = append(features, feature.Feature)
	}

	externalID := state.ExternalID.ValueString()
	policies, err := aws.Wrap(polarisClient).TrustPolicies(ctx, aws.TrustPoliciesParams{
		Cloud:                 gqlaws.Cloud(account.Cloud),
		CloudAccountID:        id,
		Features:              features,
		ExternalID:            externalID,
		RoleChainingAccountID: account.RoleChainingAccountID,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS trust policies", err.Error())
		return
	}

	state.Cloud = types.StringValue(account.Cloud)
	state.Name = types.StringValue(account.Name)
	state.NativeID = types.StringValue(account.NativeID)

	if account.RoleChainingAccountID != uuid.Nil {
		state.RoleChainingAccountID = types.StringValue(account.RoleChainingAccountID.String())
	} else {
		state.RoleChainingAccountID = types.StringNull()
	}

	featureSet, diags := awsFromFeatures(ctx, account.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.Feature = featureSet

	regionSet, diags := awsFromFeatureRegions(account.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.Regions = regionSet

	policySet, diags := awsFromTrustPolicies(policies)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.TrustPolicies = policySet

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsCnpAccountIdentityModel{ID: state.ID, ExternalID: state.ExternalID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsCnpAccountResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.Update")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan awsCnpAccountModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state awsCnpAccountModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud account ID", err.Error())
		return
	}

	// Update the account name.
	name := plan.Name.ValueString()
	if !plan.Name.Equal(state.Name) {
		if err := aws.Wrap(polarisClient).UpdateAccount(ctx, id, core.FeatureAll, aws.Name(name)); err != nil {
			res.Diagnostics.AddError("Failed to update AWS account name", err.Error())
			return
		}
	}

	planFeatures, diags := awsToFeatures(ctx, plan.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateFeatures, diags := awsToFeatures(ctx, state.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	var planRegions []string
	res.Diagnostics.Append(plan.Regions.ElementsAs(ctx, &planRegions, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	cloud := plan.Cloud.ValueString()
	nativeID := plan.NativeID.ValueString()
	deleteSnapshots := plan.DeleteSnapshotsOnDestroy.ValueBool()

	// When adding new features the list should include all features. When
	// removing features only the features to be removed should be passed in.
	if !plan.Feature.Equal(state.Feature) {
		removeFeatures, updateFeatures := diffFeatures(stateFeatures, planFeatures)
		account := aws.AccountWithName(cloud, nativeID, name)
		if len(updateFeatures) > 0 {
			if _, err := aws.Wrap(polarisClient).AddAccountWithIAM(ctx, account, updateFeatures, aws.Regions(planRegions...)); err != nil {
				res.Diagnostics.AddError("Failed to add AWS account features", err.Error())
				return
			}
		}
		if len(removeFeatures) > 0 {
			if err := aws.Wrap(polarisClient).RemoveAccountWithIAM(ctx, account, removeFeatures, deleteSnapshots); err != nil {
				res.Diagnostics.AddError("Failed to remove AWS account features", err.Error())
				return
			}
		}
	}

	if !plan.Regions.Equal(state.Regions) {
		for _, feature := range planFeatures {
			if err := aws.Wrap(polarisClient).UpdateAccount(ctx, id, feature, aws.Regions(planRegions...)); err != nil {
				res.Diagnostics.AddError("Failed to update AWS account regions", err.Error())
				return
			}
		}
	}

	// Re-read the account to refresh state with backend-derived values.
	account, err := aws.Wrap(polarisClient).AccountByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS account", err.Error())
		return
	}

	currentFeatures := make([]core.Feature, 0, len(account.Features))
	for _, feature := range account.Features {
		currentFeatures = append(currentFeatures, feature.Feature)
	}

	policies, err := aws.Wrap(polarisClient).TrustPolicies(ctx, aws.TrustPoliciesParams{
		Cloud:                 gqlaws.Cloud(account.Cloud),
		CloudAccountID:        id,
		Features:              currentFeatures,
		ExternalID:            plan.ExternalID.ValueString(),
		RoleChainingAccountID: account.RoleChainingAccountID,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS trust policies", err.Error())
		return
	}

	plan.Cloud = types.StringValue(account.Cloud)
	plan.Name = types.StringValue(account.Name)
	plan.NativeID = types.StringValue(account.NativeID)
	if account.RoleChainingAccountID != uuid.Nil {
		plan.RoleChainingAccountID = types.StringValue(account.RoleChainingAccountID.String())
	}

	featureSet, diags := awsFromFeatures(ctx, account.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	plan.Feature = featureSet

	regionSet, diags := awsFromFeatureRegions(account.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	plan.Regions = regionSet

	policySet, diags := awsFromTrustPolicies(policies)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	plan.TrustPolicies = policySet

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsCnpAccountIdentityModel{ID: plan.ID, ExternalID: plan.ExternalID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsCnpAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.Delete")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state awsCnpAccountModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	features, diags := awsToFeatures(ctx, state.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	cloud := state.Cloud.ValueString()
	nativeID := state.NativeID.ValueString()
	name := state.Name.ValueString()
	deleteSnapshots := state.DeleteSnapshotsOnDestroy.ValueBool()

	if err := aws.Wrap(polarisClient).RemoveAccountWithIAM(ctx, aws.AccountWithName(cloud, nativeID, name), features, deleteSnapshots); err != nil {
		res.Diagnostics.AddError("Failed to remove AWS account", err.Error())
		return
	}
}

func (r *awsCnpAccountResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.ModifyPlan")

	// Skip on destroy.
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan awsCnpAccountModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	planFeatures, diags := awsToFeatures(ctx, plan.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Prevent ROLE_CHAINING from being combined with other features.
	if err := core.ValidateRoleChaining(planFeatures); err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyFeature), "Invalid feature combination", err.Error())
		return
	}

	// Skip on create.
	if req.State.Raw.IsNull() {
		return
	}

	var state awsCnpAccountModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	stateFeatures, diags := awsToFeatures(ctx, state.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Mark trust_policies unknown when the feature set changes so it recomputes
	// during apply.
	if !plan.Feature.Equal(state.Feature) {
		res.Diagnostics.Append(res.Plan.SetAttribute(ctx, path.Root(keyTrustPolicies),
			types.SetUnknown(types.ObjectType{AttrTypes: awsTrustPolicyAttrTypes()}))...)
	}

	// Prevent removing CLOUD_DISCOVERY while protection features are still
	// enabled.
	hasCloudDiscovery := func(features []core.Feature) bool {
		return slices.ContainsFunc(features, func(f core.Feature) bool {
			return f.Name == core.FeatureCloudDiscovery.Name
		})
	}
	if hasCloudDiscovery(stateFeatures) && !hasCloudDiscovery(planFeatures) {
		for _, feature := range core.AllProtectionFeatures(core.CloudVendorAWS) {
			if slices.ContainsFunc(planFeatures, func(f core.Feature) bool { return f.Name == feature.Name }) {
				res.Diagnostics.AddAttributeError(path.Root(keyFeature),
					"CLOUD_DISCOVERY cannot be removed while protection features are enabled",
					"Remove the protection features first before removing CLOUD_DISCOVERY.")
			}
		}
	}
}

func (r *awsCnpAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsCnpAccountResource.ImportState")

	var identity awsCnpAccountIdentityModel
	if req.ID != "" {
		// Import by string ID.
		accountID, externalID, err := splitAccountID(req.ID)
		if err != nil {
			res.Diagnostics.AddError("Invalid import ID", err.Error())
			return
		}

		externalIDValue := types.StringNull()
		if externalID != "" {
			externalIDValue = types.StringValue(externalID)
		}
		identity = awsCnpAccountIdentityModel{
			ID:         types.StringValue(accountID.String()),
			ExternalID: externalIDValue,
		}
	} else {
		// Import by identity block.
		res.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if res.Diagnostics.HasError() {
			return
		}

		if _, err := uuid.Parse(identity.ID.ValueString()); err != nil {
			res.Diagnostics.AddError("Invalid identity id", err.Error())
			return
		}
		if identity.ExternalID.ValueString() == "" {
			identity.ExternalID = types.StringNull()
		}
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), identity.ID)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyExternalID), identity.ExternalID)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyDeleteSnapshotsOnDestroy), false)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

// diffFeatures splits a desired feature list against the existing one into the
// features that need to be removed and the features whose configuration has
// changed (and therefore need to be (re)added).
func diffFeatures(oldFeatures, newFeatures []core.Feature) ([]core.Feature, []core.Feature) {
	oldSet := make(map[string]core.Feature)
	for _, feature := range oldFeatures {
		oldSet[feature.Name] = feature
	}
	newSet := make(map[string]core.Feature)
	for _, feature := range newFeatures {
		newSet[feature.Name] = feature
	}

	for name, oldFeature := range oldSet {
		if newFeature, ok := newSet[name]; ok {
			if oldFeature.DeepEqual(newFeature) {
				delete(newSet, name)
			}
			delete(oldSet, name)
		}
	}

	removeFeatures := make([]core.Feature, 0, len(oldSet))
	for _, feature := range oldSet {
		removeFeatures = append(removeFeatures, feature)
	}
	slices.SortFunc(removeFeatures, func(lhs, rhs core.Feature) int {
		return cmp.Compare(lhs.Name, rhs.Name)
	})

	updateFeatures := make([]core.Feature, 0, len(newSet))
	for _, feature := range newSet {
		updateFeatures = append(updateFeatures, feature)
	}
	slices.SortFunc(updateFeatures, func(lhs, rhs core.Feature) int {
		return cmp.Compare(lhs.Name, rhs.Name)
	})

	return removeFeatures, updateFeatures
}

// splitAccountID parses a resource id of the form <uuid>, <uuid>:<external-id>
// or <uuid>-<external-id> into its components. Used by the string-id import
// path. The dash form remains accepted for backwards compatibility.
func splitAccountID(id string) (uuid.UUID, string, error) {
	const uuidStringLen = 36

	if len(id) < uuidStringLen {
		return uuid.Nil, "", fmt.Errorf("invalid resource id: %s", id)
	}

	accountID, err := uuid.Parse(id[:uuidStringLen])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid resource id: %s", id)
	}
	if len(id) == uuidStringLen {
		return accountID, "", nil
	}

	if sep := id[uuidStringLen]; sep == '-' || sep == ':' {
		externalID := id[uuidStringLen+1:]
		if externalID == "" {
			return uuid.Nil, "", fmt.Errorf("invalid resource id: %s", id)
		}
		return accountID, externalID, nil
	}

	return uuid.Nil, "", fmt.Errorf("invalid resource id: %s", id)
}
