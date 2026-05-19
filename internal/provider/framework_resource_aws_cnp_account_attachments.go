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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const resourceAWSCNPAccountAttachmentsDescription = `
The ´rubrik_aws_cnp_account_attachments´ resource attaches AWS instance
profiles and IAM roles to an RSC cloud account, finalizing the onboarding
that begins with ´rubrik_aws_cnp_account´. RSC uses the attached roles to
perform cloud-native operations against the AWS account.

The set of artifact keys (role keys and instance profile keys) required for a
given combination of features can be looked up with the
´rubrik_aws_cnp_artifacts´ data source. The IAM policy documents that each
role must carry can be looked up with the ´rubrik_aws_cnp_permissions´ data
source.

-> **Note:** The ´features´ field takes only the feature names and not the
   permission groups associated with the features. The feature set should
   match the features enabled on the parent ´rubrik_aws_cnp_account´.

-> **Note:** The ´role´ block is shown as Optional in the schema below for
   technical reasons, but at least one ´role´ block must be specified. The
   block-style syntax is preserved to remain compatible with existing
   Terraform configurations.

-> **Note:** Set ´role_chaining_account_id´ to the RSC cloud account ID of the
   role-chaining account when onboarding a role-chained account. The roles
   attached here are then used as the chained roles, while the role-chaining
   account provides the trust anchor.

-> **Note:** The ´permissions´ field on each ´role´ block is a sentinel:
   changing its value signals to RSC that the IAM policy attached to the role
   has been updated. Pair it with the ´id´ field of the
   ´rubrik_aws_cnp_permissions´ data source so the sentinel changes whenever
   the required policy changes. The value is not returned by RSC and is
   carried forward from prior state on read.

-> **Note:** Destroying this resource does not call RSC. The attached
   artifacts are removed when the parent ´rubrik_aws_cnp_account´ is
   destroyed.
`

var (
	_ resource.Resource                = &awsCnpAccountAttachmentsResource{}
	_ resource.ResourceWithConfigure   = &awsCnpAccountAttachmentsResource{}
	_ resource.ResourceWithIdentity    = &awsCnpAccountAttachmentsResource{}
	_ resource.ResourceWithImportState = &awsCnpAccountAttachmentsResource{}
	_ resource.ResourceWithModifyPlan  = &awsCnpAccountAttachmentsResource{}
	_ resource.ResourceWithMoveState   = &awsCnpAccountAttachmentsResource{}
)

type awsCnpAccountAttachmentsResource struct {
	client *client
	prefix string
}

type awsCnpAccountAttachmentsModel struct {
	ID                    types.String `tfsdk:"id"`
	AccountID             types.String `tfsdk:"account_id"`
	Features              types.Set    `tfsdk:"features"`
	InstanceProfile       types.Set    `tfsdk:"instance_profile"`
	Role                  types.Set    `tfsdk:"role"`
	RoleChainingAccountID types.String `tfsdk:"role_chaining_account_id"`
}

type awsCnpAccountAttachmentsIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAwsCnpAccountAttachmentsResource() resource.Resource {
	return &awsCnpAccountAttachmentsResource{prefix: keyRubrik}
}

func newPolarisAwsCnpAccountAttachmentsResource() resource.Resource {
	return &awsCnpAccountAttachmentsResource{prefix: keyPolaris}
}

func (r *awsCnpAccountAttachmentsResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSCNPAccountAttachments
}

func (r *awsCnpAccountAttachmentsResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAWSCNPAccountAttachmentsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyAccountID: schema.StringAttribute{
				Required:    true,
				Description: "RSC cloud account ID (UUID). Changing this forces a new resource to be created.",
				Validators: []validator.String{
					isUUID(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			keyFeatures: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "RSC features. Possible values are `CLOUD_DISCOVERY`, `CLOUD_NATIVE_ARCHIVAL`, " +
					"`CLOUD_NATIVE_DYNAMODB_PROTECTION`, `CLOUD_NATIVE_PROTECTION`, `CLOUD_NATIVE_S3_PROTECTION`, " +
					"`EXOCOMPUTE`, `KUBERNETES_PROTECTION`, `RDS_PROTECTION`, `ROLE_CHAINING` and " +
					"`SERVERS_AND_APPS`.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(
						"CLOUD_DISCOVERY", "CLOUD_NATIVE_ARCHIVAL", "CLOUD_NATIVE_PROTECTION",
						"CLOUD_NATIVE_S3_PROTECTION", "CLOUD_NATIVE_DYNAMODB_PROTECTION", "EXOCOMPUTE",
						"RDS_PROTECTION", "KUBERNETES_PROTECTION", "SERVERS_AND_APPS", "ROLE_CHAINING",
					)),
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
		},
		Blocks: map[string]schema.Block{
			// instance_profile is modeled as a SetNestedBlock to preserve the
			// SDKv2 block syntax used by existing configurations. The Plugin
			// Framework does not expose an Optional flag on blocks; an absent
			// block simply produces an empty set, matching the legacy Optional
			// semantics.
			keyInstanceProfile: schema.SetNestedBlock{
				Description: "Instance profiles to attach to the cloud account.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKey: schema.StringAttribute{
							Required:    true,
							Description: "RSC artifact key for the AWS instance profile.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyName: schema.StringAttribute{
							Required:    true,
							Description: "AWS instance profile name.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
			// role is modeled as a SetNestedBlock for the same reason as
			// instance_profile. The at-least-one constraint replaces the
			// SDKv2 Required flag, which the Framework does not support on
			// blocks.
			keyRole: schema.SetNestedBlock{
				Description: "Roles to attach to the cloud account. At least one `role` block must be specified.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKey: schema.StringAttribute{
							Required:    true,
							Description: "RSC artifact key for the AWS role.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyARN: schema.StringAttribute{
							Required:    true,
							Description: "AWS role ARN.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyPermissions: schema.StringAttribute{
							Optional: true,
							Description: "Permissions updated signal. When this field changes, the provider will " +
								"notify RSC that the permissions for the feature has been updated. Use this field " +
								"with the `id` field of the `rubrik_aws_cnp_permissions` data source.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_aws_cnp_account_attachments` instead."
	}
}

func (r *awsCnpAccountAttachmentsResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC cloud account ID (UUID).",
			},
		},
	}
}

func (r *awsCnpAccountAttachmentsResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsCnpAccountAttachmentsResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.Create")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan awsCnpAccountAttachmentsModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	accountID, err := uuid.Parse(plan.AccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyAccountID), "Invalid UUID", err.Error())
		return
	}

	features, diags := awsAttachmentsToFeatures(ctx, plan.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	profiles, diags := awsAttachmentsToInstanceProfiles(ctx, plan.InstanceProfile)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	roles, diags := awsAttachmentsToRoles(ctx, plan.Role)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	ensureRoleChainingArtifact(roles, features)

	var roleChainingAccountID uuid.UUID
	if !plan.RoleChainingAccountID.IsNull() {
		roleChainingAccountID, err = uuid.Parse(plan.RoleChainingAccountID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyRoleChainingAccountID), "Invalid UUID", err.Error())
			return
		}
	}

	id, err := aws.Wrap(polarisClient).AddAccountArtifacts(ctx, aws.AddAccountArtifactsParams{
		CloudAccountID:        accountID,
		Features:              features,
		InstanceProfiles:      profiles,
		Roles:                 roles,
		RoleChainingAccountID: roleChainingAccountID,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to add AWS account artifacts", err.Error())
		return
	}

	plan.ID = types.StringValue(id.String())
	// Save partial state so the resource can be managed even if the refresh
	// below fails.
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	gone, diags := r.refresh(ctx, &plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	if gone {
		// Should not happen on Create, but stay defensive.
		res.State.RemoveResource(ctx)
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsCnpAccountAttachmentsIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsCnpAccountAttachmentsResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.Read")

	var state awsCnpAccountAttachmentsModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	gone, diags := r.refresh(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	if gone {
		res.State.RemoveResource(ctx)
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsCnpAccountAttachmentsIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsCnpAccountAttachmentsResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.Update")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan awsCnpAccountAttachmentsModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud account ID", err.Error())
		return
	}

	features, diags := awsAttachmentsToFeatures(ctx, plan.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	profiles, diags := awsAttachmentsToInstanceProfiles(ctx, plan.InstanceProfile)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	roles, diags := awsAttachmentsToRoles(ctx, plan.Role)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	ensureRoleChainingArtifact(roles, features)

	var roleChainingAccountID uuid.UUID
	if !plan.RoleChainingAccountID.IsNull() {
		roleChainingAccountID, err = uuid.Parse(plan.RoleChainingAccountID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyRoleChainingAccountID), "Invalid UUID", err.Error())
			return
		}
	}

	if _, err := aws.Wrap(polarisClient).AddAccountArtifacts(ctx, aws.AddAccountArtifactsParams{
		CloudAccountID:        id,
		Features:              features,
		InstanceProfiles:      profiles,
		Roles:                 roles,
		RoleChainingAccountID: roleChainingAccountID,
	}); err != nil {
		res.Diagnostics.AddError("Failed to update AWS account artifacts", err.Error())
		return
	}

	// Notify RSC that permissions for all features have been updated. There's
	// no way to map a role to a specific feature, so we signal an update for
	// every feature unconditionally rather than diffing permission hashes.
	if err := aws.Wrap(polarisClient).PermissionsUpdated(ctx, id, nil); err != nil {
		res.Diagnostics.AddError("Failed to notify RSC about updated permissions", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsCnpAccountAttachmentsIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsCnpAccountAttachmentsResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.Delete")

	// No backend call: artifacts are removed implicitly when the parent
	// rubrik_aws_cnp_account is destroyed. The framework removes the
	// resource from state automatically once Delete returns.
}

func (r *awsCnpAccountAttachmentsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.ModifyPlan")

	// Skip on destroy.
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan awsCnpAccountAttachmentsModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	if plan.Features.IsUnknown() {
		return
	}

	features, diags := awsAttachmentsToFeatures(ctx, plan.Features)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if err := core.ValidateRoleChaining(features); err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyFeatures), "Invalid feature combination", err.Error())
	}
}

func (r *awsCnpAccountAttachmentsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsCnpAccountAttachmentsResource.ImportState")

	var identity awsCnpAccountAttachmentsIdentityModel
	if req.ID != "" {
		id, err := uuid.Parse(req.ID)
		if err != nil {
			res.Diagnostics.AddError("Invalid import ID", err.Error())
			return
		}
		identity.ID = types.StringValue(id.String())
	} else {
		res.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if res.Diagnostics.HasError() {
			return
		}
		if _, err := uuid.Parse(identity.ID.ValueString()); err != nil {
			res.Diagnostics.AddError("Invalid identity id", err.Error())
			return
		}
	}

	// Seed both id and account_id from the import value: the id is the cloud
	// account UUID, and account_id is a required attribute that the post-import
	// refresh would otherwise see as null.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), identity.ID)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyAccountID), identity.ID)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

// refresh reads the current account artifacts from RSC and updates the
// fields on m in place. The bool result is true when the underlying account
// no longer exists in RSC and the resource should be removed from state.
func (r *awsCnpAccountAttachmentsResource) refresh(ctx context.Context, m *awsCnpAccountAttachmentsModel) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	polarisClient, err := r.client.polaris()
	if err != nil {
		diags.AddError("RSC client error", err.Error())
		return false, diags
	}

	id, err := uuid.Parse(m.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid cloud account ID", err.Error())
		return false, diags
	}

	account, err := aws.Wrap(polarisClient).AccountByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		return true, diags
	}
	if err != nil {
		diags.AddError("Failed to read AWS account", err.Error())
		return false, diags
	}

	instanceProfiles, roles, err := aws.Wrap(polarisClient).AccountArtifacts(ctx, id)
	if err != nil {
		diags.AddError("Failed to read AWS account artifacts", err.Error())
		return false, diags
	}

	// Workaround: the ROLE_CHAINING artifact is registered as a duplicate of
	// CROSSACCOUNT by Create/Update. Strip it from the read response so it
	// doesn't appear in state and cause a perpetual diff.
	delete(roles, "ROLE_CHAINING")

	featureValues := make([]attr.Value, 0, len(account.Features))
	for _, feature := range account.Features {
		featureValues = append(featureValues, types.StringValue(feature.Feature.Name))
	}
	featureSet, d := types.SetValue(types.StringType, featureValues)
	diags.Append(d...)
	if diags.HasError() {
		return false, diags
	}

	profileSet, d := awsAttachmentsFromInstanceProfiles(instanceProfiles)
	diags.Append(d...)
	if diags.HasError() {
		return false, diags
	}

	// Carry over the per-role permissions value from the existing state since
	// RSC does not return it.
	priorPermissions, d := awsAttachmentsRolePermissions(ctx, m.Role)
	diags.Append(d...)
	if diags.HasError() {
		return false, diags
	}

	roleSet, d := awsAttachmentsFromRoles(roles, priorPermissions)
	diags.Append(d...)
	if diags.HasError() {
		return false, diags
	}

	m.AccountID = types.StringValue(account.ID.String())
	m.Features = featureSet
	m.InstanceProfile = profileSet
	m.Role = roleSet
	return false, diags
}

// awsAttachmentsInstanceProfileAttrTypes returns the attribute types for an
// element of the instance_profile set.
func awsAttachmentsInstanceProfileAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyKey:  types.StringType,
		keyName: types.StringType,
	}
}

// awsAttachmentsRoleAttrTypes returns the attribute types for an element of
// the role set.
func awsAttachmentsRoleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyKey:         types.StringType,
		keyARN:         types.StringType,
		keyPermissions: types.StringType,
	}
}

func awsAttachmentsToFeatures(ctx context.Context, set types.Set) ([]core.Feature, diag.Diagnostics) {
	var names []string
	diags := set.ElementsAs(ctx, &names, false)
	if diags.HasError() {
		return nil, diags
	}
	features := make([]core.Feature, 0, len(names))
	for _, name := range names {
		features = append(features, core.Feature{Name: name})
	}
	return features, diags
}

func awsAttachmentsToInstanceProfiles(ctx context.Context, set types.Set) (map[string]string, diag.Diagnostics) {
	var models []struct {
		Key  types.String `tfsdk:"key"`
		Name types.String `tfsdk:"name"`
	}
	diags := set.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	profiles := make(map[string]string, len(models))
	for _, m := range models {
		profiles[m.Key.ValueString()] = m.Name.ValueString()
	}
	return profiles, diags
}

func awsAttachmentsToRoles(ctx context.Context, set types.Set) (map[string]string, diag.Diagnostics) {
	var models []struct {
		Key         types.String `tfsdk:"key"`
		ARN         types.String `tfsdk:"arn"`
		Permissions types.String `tfsdk:"permissions"`
	}
	diags := set.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	roles := make(map[string]string, len(models))
	for _, m := range models {
		roles[m.Key.ValueString()] = m.ARN.ValueString()
	}
	return roles, diags
}

// awsAttachmentsRolePermissions extracts the per-role permissions field from
// the existing state set, keyed by the role's `key` attribute. Used to carry
// the value forward across reads since RSC does not return it.
func awsAttachmentsRolePermissions(ctx context.Context, set types.Set) (map[string]types.String, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}
	var models []struct {
		Key         types.String `tfsdk:"key"`
		ARN         types.String `tfsdk:"arn"`
		Permissions types.String `tfsdk:"permissions"`
	}
	diags = set.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	out := make(map[string]types.String, len(models))
	for _, m := range models {
		out[m.Key.ValueString()] = m.Permissions
	}
	return out, diags
}

func awsAttachmentsFromInstanceProfiles(profiles map[string]string) (types.Set, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: awsAttachmentsInstanceProfileAttrTypes()}
	values := make([]attr.Value, 0, len(profiles))
	for key, name := range profiles {
		obj, d := types.ObjectValue(awsAttachmentsInstanceProfileAttrTypes(), map[string]attr.Value{
			keyKey:  types.StringValue(key),
			keyName: types.StringValue(name),
		})
		if d.HasError() {
			return types.SetNull(objType), d
		}
		values = append(values, obj)
	}
	return types.SetValue(objType, values)
}

func awsAttachmentsFromRoles(roles map[string]string, priorPermissions map[string]types.String) (types.Set, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: awsAttachmentsRoleAttrTypes()}
	values := make([]attr.Value, 0, len(roles))
	for key, arn := range roles {
		permissions := types.StringNull()
		if prior, ok := priorPermissions[key]; ok {
			permissions = prior
		}
		obj, d := types.ObjectValue(awsAttachmentsRoleAttrTypes(), map[string]attr.Value{
			keyKey:         types.StringValue(key),
			keyARN:         types.StringValue(arn),
			keyPermissions: permissions,
		})
		if d.HasError() {
			return types.SetNull(objType), d
		}
		values = append(values, obj)
	}
	return types.SetValue(objType, values)
}

// ensureRoleChainingArtifact duplicates the CROSSACCOUNT role ARN as
// ROLE_CHAINING when the ROLE_CHAINING feature is present. This is a
// workaround for the RSC backend not returning the ROLE_CHAINING_ROLE_ARN
// artifact.
func ensureRoleChainingArtifact(roles map[string]string, features []core.Feature) {
	crossAccountARN, ok := roles["CROSSACCOUNT"]
	if !ok {
		return
	}
	if _, ok := roles["ROLE_CHAINING"]; ok {
		return
	}
	if _, ok := core.LookupFeature(features, core.FeatureRoleChaining); ok {
		roles["ROLE_CHAINING"] = crossAccountARN
	}
}
