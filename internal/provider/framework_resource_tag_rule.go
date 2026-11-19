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
	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/sla"
)

const resourceTagRuleDescription = `
The ´rubrik_tag_rule´ resource manages RSC tag rules.

A tag is a key-value pair used to group cloud resources for a specific purpose.
This rule-based approach allows resource protection across multiple projects and
regions. A tag can be used to assign an SLA Domain to all resources belonging to
a specific application or department. When cloud resources are tagged
appropriately, they derive protection automatically when they are instantiated.

-> **Note:** Tag key and tag value are case sensitive.
`

var (
	_ resource.Resource                   = &tagRuleResource{}
	_ resource.ResourceWithIdentity       = &tagRuleResource{}
	_ resource.ResourceWithImportState    = &tagRuleResource{}
	_ resource.ResourceWithMoveState      = &tagRuleResource{}
	_ resource.ResourceWithValidateConfig = &tagRuleResource{}
)

type tagRuleResource struct {
	client *client
	prefix string
}

// tagRuleModel is the resource state. The deprecated tag_key, tag_value and
// tag_all_values attributes and the tag block are mutually exclusive, and the
// state holds the style used by the configuration.
type tagRuleModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	ObjectType      types.String `tfsdk:"object_type"`
	TagKey          types.String `tfsdk:"tag_key"`
	TagValue        types.String `tfsdk:"tag_value"`
	TagAllValues    types.Bool   `tfsdk:"tag_all_values"`
	Tag             types.List   `tfsdk:"tag"`
	CloudAccountIDs types.Set    `tfsdk:"cloud_account_ids"`
}

// tagRuleTagModel is one element of the tag block.
type tagRuleTagModel struct {
	Key      types.String `tfsdk:"key"`
	Values   types.List   `tfsdk:"values"`
	MatchAll types.Bool   `tfsdk:"match_all"`
}

type tagRuleIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func tagRuleTagAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyKey:         types.StringType,
		keyValues:      types.ListType{ElemType: types.StringType},
		keyTagMatchAll: types.BoolType,
	}
}

func newTagRuleResource() resource.Resource {
	return &tagRuleResource{prefix: keyRubrik}
}

func newPolarisTagRuleResource() resource.Resource {
	return &tagRuleResource{prefix: keyPolaris}
}

func (r *tagRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "tagRuleResource.Metadata")

	res.TypeName = r.prefix + "_" + keyTagRule
}

func (r *tagRuleResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "tagRuleResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceTagRuleDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Tag rule ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Tag rule name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyObjectType: schema.StringAttribute{
				Required: true,
				Description: "Object type to which the tag rule will be applied. Possible values are " +
					"`AWS_EBS_VOLUME`, `AWS_EC2_INSTANCE`, `AWS_RDS_INSTANCE`, `AWS_S3_BUCKET`, " +
					"`AWS_DYNAMODB_TABLE`, `AZURE_MANAGED_DISK`, `AZURE_SQL_DATABASE_DB`, `AZURE_SQL_DATABASE_SERVER`, " +
					"`AZURE_SQL_MANAGED_INSTANCE_SERVER`, `AZURE_STORAGE_ACCOUNT` and `AZURE_VIRTUAL_MACHINE`. " +
					"Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(gqlsla.AllCloudNativeTagObjectTypesAsStrings()...),
				},
			},
			keyTagKey: schema.StringAttribute{
				Optional: true,
				Description: "Tag key to match. Changing this forces a new resource to be created. " +
					"**Deprecated:** Use `tag` instead.",
				DeprecationMessage: "Use tag instead.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot(keyTag)),
					isNotWhiteSpace(),
				},
			},
			keyTagValue: schema.StringAttribute{
				Optional: true,
				Description: "Tag value to match. If the tag value is empty, it matches empty values. " +
					"Changing this forces a new resource to be created. **Deprecated:** Use `tag` instead.",
				DeprecationMessage: "Use tag instead.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot(keyTag), path.MatchRoot(keyTagAllValues)),
				},
			},
			keyTagAllValues: schema.BoolAttribute{
				Optional: true,
				Description: "If true, all tag values are matched. Changing this forces a new resource to be created. " +
					"**Deprecated:** Use `tag` instead.",
				DeprecationMessage: "Use tag instead.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
				Validators: []validator.Bool{
					boolvalidator.ConflictsWith(path.MatchRoot(keyTag), path.MatchRoot(keyTagValue)),
				},
			},
			keyCloudAccountIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "The RSC cloud account IDs (UUID) to which the tag rule should be applied. If empty, " +
					"the tag rule will be applied to all RSC cloud accounts.",
			},
		},
		Blocks: map[string]schema.Block{
			keyTag: schema.ListNestedBlock{
				Description: "Tag conditions to match. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKey: schema.StringAttribute{
							Required:    true,
							Description: "Tag key to match.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyValues: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Tag values to match. If empty and `match_all` is false, matches empty values.",
						},
						keyTagMatchAll: schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(false),
							Description: "If true, all tag values for this key are matched. Default is false.",
						},
					},
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_tag_rule` instead."
	}
}

func (r *tagRuleResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "tagRuleResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Tag rule ID (UUID).",
			},
		},
	}
}

func (r *tagRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "tagRuleResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *tagRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "tagRuleResource.ValidateConfig")

	var config tagRuleModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(validateTagRuleConfig(ctx, config)...)
}

// validateTagRuleConfig holds the plan-time, client-free validation rules for
// the resource so they can be unit-tested in isolation. The rules are the ones
// the SDKv2 resource enforced in CustomizeDiff.
func validateTagRuleConfig(ctx context.Context, config tagRuleModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// A value which is not known yet can't be checked, so the rules are only
	// applied once the configuration is fully known.
	if config.Tag.IsUnknown() || config.TagKey.IsUnknown() || config.TagValue.IsUnknown() ||
		config.TagAllValues.IsUnknown() {
		return diags
	}

	// Validate that at least one of tag or tag_key is specified.
	if len(config.Tag.Elements()) == 0 {
		if config.TagKey.IsNull() {
			diags.AddError("Missing tag condition", "one of `tag` or `tag_key` must be specified")
			return diags
		}

		// When using the deprecated tag_key field, require either tag_value or
		// tag_all_values to preserve the original ExactlyOneOf validation.
		if config.TagValue.IsNull() && config.TagAllValues.IsNull() {
			diags.AddError("Missing tag value", fmt.Sprintf("one of `%s` or `%s` must be specified when using `%s`",
				keyTagValue, keyTagAllValues, keyTagKey))
			return diags
		}
	}

	// Validate that values is empty when match_all is true.
	var tags []tagRuleTagModel
	diags.Append(config.Tag.ElementsAs(ctx, &tags, false)...)
	if diags.HasError() {
		return diags
	}
	for _, tag := range tags {
		if tag.MatchAll.IsUnknown() || tag.Values.IsUnknown() {
			continue
		}
		if tag.MatchAll.ValueBool() && len(tag.Values.Elements()) > 0 {
			diags.AddAttributeError(path.Root(keyTag), "Invalid tag condition",
				fmt.Sprintf("`%s` must be empty when `%s` is true", keyValues, keyTagMatchAll))
		}
	}

	return diags
}

func (r *tagRuleResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "tagRuleResource.Create")

	var plan tagRuleModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloudAccounts, diags := toTagRuleCloudAccounts(ctx, polarisClient, plan.CloudAccountIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	params := gqlsla.CreateTagRuleParams{
		Name:             plan.Name.ValueString(),
		ObjectType:       gqlsla.CloudNativeTagObjectType(plan.ObjectType.ValueString()),
		CloudAccounts:    cloudAccounts,
		AllCloudAccounts: cloudAccounts == nil,
	}

	// Check if the new style tag block is used.
	if len(plan.Tag.Elements()) > 0 {
		var tags []tagRuleTagModel
		res.Diagnostics.Append(plan.Tag.ElementsAs(ctx, &tags, false)...)
		if res.Diagnostics.HasError() {
			return
		}

		tagPairs := make([]gqlsla.TagPair, 0, len(tags))
		for _, tag := range tags {
			values := make([]string, 0, len(tag.Values.Elements()))
			if !tag.Values.IsNull() {
				res.Diagnostics.Append(tag.Values.ElementsAs(ctx, &values, false)...)
				if res.Diagnostics.HasError() {
					return
				}
			}
			tagPairs = append(tagPairs, gqlsla.TagPair{
				Key:               tag.Key.ValueString(),
				MatchAllTagValues: tag.MatchAll.ValueBool(),
				Values:            values,
			})
		}
		params.TagConditions = &gqlsla.TagConditions{TagPairs: tagPairs}
	} else {
		// Use the deprecated tag fields for backward compatibility.
		//lint:ignore SA1019 using deprecated field for backward compatibility
		params.Tag = gqlsla.Tag{
			Key:       plan.TagKey.ValueString(),
			Value:     plan.TagValue.ValueString(),
			AllValues: plan.TagAllValues.ValueBool(),
		}
	}

	tagRuleID, err := sla.Wrap(polarisClient).CreateTagRule(ctx, params)
	if err != nil {
		res.Diagnostics.AddError("Failed to create tag rule", err.Error())
		return
	}

	// Set the ID in the state and the identity before reading the tag rule back,
	// so a failed read does not orphan the resource. The plan holds no unknown
	// values besides the ID, since match_all has a default.
	plan.ID = types.StringValue(tagRuleID.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, tagRuleIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	// Read back the created tag rule to populate computed fields. A failed read
	// back is a warning: the resource was created and the state above is kept.
	state, err := r.readTagRule(ctx, polarisClient, plan)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read back tag rule after create", err.Error())
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *tagRuleResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "tagRuleResource.Read")

	var state tagRuleModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	newState, err := r.readTagRule(ctx, polarisClient, state)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read tag rule", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &newState)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.Identity.Set(ctx, tagRuleIdentityModel{ID: newState.ID})...)
}

func (r *tagRuleResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "tagRuleResource.Update")

	var plan tagRuleModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state tagRuleModel
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
		res.Diagnostics.AddError("Invalid tag rule ID", err.Error())
		return
	}

	cloudAccounts, diags := toTagRuleCloudAccounts(ctx, polarisClient, plan.CloudAccountIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	err = sla.Wrap(polarisClient).UpdateTagRule(ctx, id, gqlsla.UpdateTagRuleParams{
		Name:             plan.Name.ValueString(),
		CloudAccounts:    cloudAccounts,
		AllCloudAccounts: cloudAccounts == nil,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to update tag rule", err.Error())
		return
	}

	// Read back the updated tag rule to populate computed fields. A failed read
	// back is a warning, the resource was updated.
	plan.ID = state.ID
	newState, err := r.readTagRule(ctx, polarisClient, plan)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read back tag rule after update", err.Error())
		newState = plan
	}
	res.Diagnostics.Append(res.State.Set(ctx, &newState)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.Identity.Set(ctx, tagRuleIdentityModel{ID: newState.ID})...)
}

func (r *tagRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "tagRuleResource.Delete")

	var state tagRuleModel
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
		res.Diagnostics.AddError("Invalid tag rule ID", err.Error())
		return
	}

	err = sla.Wrap(polarisClient).DeleteTagRule(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to delete tag rule", err.Error())
		return
	}
}

func (r *tagRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "tagRuleResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

// readTagRule reads the tag rule from RSC and returns the new state, based on
// the given state. The tag style of the state decides how the tag condition is
// returned: if the state has a name, which rules out an import, and a tag key,
// the deprecated tag_key, tag_value and tag_all_values attributes are populated
// and the tag block is left alone, to avoid plan diffs for configurations which
// have not migrated to the tag block. Otherwise, including on import, the tag
// block is populated.
func (r *tagRuleResource) readTagRule(ctx context.Context, polarisClient *polaris.Client, state tagRuleModel) (tagRuleModel, error) {
	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		return state, fmt.Errorf("invalid tag rule ID: %w", err)
	}

	tagRule, err := sla.Wrap(polarisClient).TagRuleByID(ctx, id)
	if err != nil {
		return state, err
	}

	useLegacyFields := !state.Name.IsNull() && !state.TagKey.IsNull()
	if useLegacyFields {
		//lint:ignore SA1019 using deprecated field for backward compatibility
		if tagRule.Tag.Key != "" {
			//lint:ignore SA1019 using deprecated field for backward compatibility
			state.TagKey = types.StringValue(tagRule.Tag.Key)
			//lint:ignore SA1019 using deprecated field for backward compatibility
			state.TagValue = types.StringValue(tagRule.Tag.Value)
			//lint:ignore SA1019 using deprecated field for backward compatibility
			state.TagAllValues = types.BoolValue(tagRule.Tag.AllValues)
		}
	} else {
		// Sort by key and by values within each tag for deterministic state
		// output regardless of the order RSC returns them in.
		pairs := slices.Clone(tagRule.TagConditions.TagPairs)
		elems := make([]attr.Value, 0, len(pairs))
		slices.SortFunc(pairs, func(a, b gqlsla.TagPair) int {
			return cmp.Compare(a.Key, b.Key)
		})

		// The values of a tag in the state are needed to tell a null values
		// list from an empty one, see tagRuleTagValues.
		priorValues := map[string]types.List{}
		if !state.Tag.IsNull() && !state.Tag.IsUnknown() {
			var priorTags []tagRuleTagModel
			if diags := state.Tag.ElementsAs(ctx, &priorTags, false); !diags.HasError() {
				for _, priorTag := range priorTags {
					priorValues[priorTag.Key.ValueString()] = priorTag.Values
				}
			}
		}

		for _, pair := range pairs {
			valueList, diags := tagRuleTagValues(pair.Values, priorValues[pair.Key])
			if diags.HasError() {
				return state, diagsToError(diags)
			}

			obj, diags := types.ObjectValue(tagRuleTagAttrTypes(), map[string]attr.Value{
				keyKey:         types.StringValue(pair.Key),
				keyValues:      valueList,
				keyTagMatchAll: types.BoolValue(pair.MatchAllTagValues),
			})
			if diags.HasError() {
				return state, diagsToError(diags)
			}
			elems = append(elems, obj)
		}
		tagList, diags := types.ListValue(types.ObjectType{AttrTypes: tagRuleTagAttrTypes()}, elems)
		if diags.HasError() {
			return state, diagsToError(diags)
		}
		state.Tag = tagList
	}

	state.Name = types.StringValue(tagRule.Name)

	tagObjectType, err := gqlsla.FromManagedObjectType(tagRule.ObjectType)
	if err != nil {
		return state, fmt.Errorf("invalid tag rule object type: %w", err)
	}
	state.ObjectType = types.StringValue(string(tagObjectType))

	if !tagRule.AllACloudAccounts {
		cloudAccountIDs := make([]attr.Value, 0, len(tagRule.CloudAccounts))
		for _, nativeCloudAccount := range tagRule.CloudAccounts {
			cloudAccountID, err := lookupCloudAccountID(ctx, polarisClient, nativeCloudAccount.ID)
			if err != nil {
				return state, fmt.Errorf("failed to look up the cloud account ID: %w", err)
			}
			cloudAccountIDs = append(cloudAccountIDs, types.StringValue(cloudAccountID.String()))
		}
		cloudAccountSet, diags := types.SetValue(types.StringType, cloudAccountIDs)
		if diags.HasError() {
			return state, diagsToError(diags)
		}
		state.CloudAccountIDs = cloudAccountSet
	}

	return state, nil
}

// tagRuleTagValues returns the values of a tag as a list, sorted so the state
// is the same regardless of the order RSC returns them in. The values attribute
// is optional, and SDKv2 treated a null list and an empty list as equal, which
// the Framework does not. When RSC returns no values, the result is therefore
// an empty list only if the prior list was an empty list, and null otherwise,
// so the state matches whichever of the two the configuration used.
func tagRuleTagValues(values []string, prior types.List) (types.List, diag.Diagnostics) {
	if len(values) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.ListNull(types.StringType), nil
	}

	values = slices.Clone(values)
	slices.Sort(values)

	elems := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elems = append(elems, types.StringValue(value))
	}
	return types.ListValue(types.StringType, elems)
}

// diagsToError converts the errors in the diagnostics to a single error.
func diagsToError(diags diag.Diagnostics) error {
	var errs []error
	for _, d := range diags.Errors() {
		errs = append(errs, fmt.Errorf("%s: %s", d.Summary(), d.Detail()))
	}
	return errors.Join(errs...)
}

// toTagRuleCloudAccounts converts the RSC cloud account IDs to the native cloud
// account IDs the tag rule API uses, grouped by cloud vendor. It returns nil
// when no cloud accounts are specified, which means all cloud accounts.
func toTagRuleCloudAccounts(ctx context.Context, polarisClient *polaris.Client, cloudAccountIDs types.Set) (*gqlsla.TagRuleCloudAccounts, diag.Diagnostics) {
	var diags diag.Diagnostics

	var ids []string
	diags.Append(cloudAccountIDs.ElementsAs(ctx, &ids, false)...)
	if diags.HasError() || len(ids) == 0 {
		return nil, diags
	}

	cloudAccounts := &gqlsla.TagRuleCloudAccounts{}
	for _, id := range ids {
		cloudAccountID, err := uuid.Parse(id)
		if err != nil {
			diags.AddError("Invalid cloud account ID", err.Error())
			return nil, diags
		}

		cloudVendor, nativeCloudAccountID, err := lookupNativeCloudAccountID(ctx, polarisClient, cloudAccountID)
		if err != nil {
			diags.AddError("Failed to look up the native cloud account ID", err.Error())
			return nil, diags
		}

		switch cloudVendor {
		case core.CloudVendorAWS:
			cloudAccounts.AWSAccountIDs = append(cloudAccounts.AWSAccountIDs, nativeCloudAccountID)
		case core.CloudVendorAzure:
			cloudAccounts.AzureSubscriptionIDs = append(cloudAccounts.AzureSubscriptionIDs, nativeCloudAccountID)
		case core.CloudVendorGCP:
			cloudAccounts.GCPProjectIDs = append(cloudAccounts.GCPProjectIDs, nativeCloudAccountID)
		default:
			diags.AddError("Unknown cloud vendor", fmt.Sprintf("unknown cloud vendor: %s", cloudVendor))
			return nil, diags
		}
	}

	return cloudAccounts, diags
}

// lookupCloudAccountID looks up the cloud account ID for the specified native
// cloud account ID. Note, AWS uses the same ID for both cloud accounts and
// native cloud accounts.
func lookupCloudAccountID(ctx context.Context, client *polaris.Client, nativeCloudAccountID uuid.UUID) (uuid.UUID, error) {
	_, err := aws.Wrap(client).AccountByID(ctx, nativeCloudAccountID)
	if err == nil {
		return nativeCloudAccountID, nil
	}
	if !errors.Is(err, graphql.ErrNotFound) {
		return uuid.Nil, err
	}

	subscription, err := azure.Wrap(client).SubscriptionByNativeCloudAccountID(ctx, nativeCloudAccountID)
	if err == nil {
		return subscription.ID, nil
	}
	if !errors.Is(err, graphql.ErrNotFound) {
		return uuid.Nil, err
	}

	project, err := gcp.Wrap(client).ProjectByNativeCloudAccountID(ctx, nativeCloudAccountID)
	if err == nil {
		return project.ID, nil
	}
	if !errors.Is(err, graphql.ErrNotFound) {
		return uuid.Nil, err
	}

	return uuid.Nil, fmt.Errorf("cloud account id for native cloud account %q %w", nativeCloudAccountID, graphql.ErrNotFound)
}

// lookupNativeCloudAccountID looks up the native cloud account ID for the
// specified cloud account ID. Note, AWS uses the same ID for both cloud
// accounts and native cloud accounts.
func lookupNativeCloudAccountID(ctx context.Context, client *polaris.Client, cloudAccountID uuid.UUID) (core.CloudVendor, uuid.UUID, error) {
	_, err := aws.Wrap(client).AccountByID(ctx, cloudAccountID)
	if err == nil {
		return core.CloudVendorAWS, cloudAccountID, nil
	}
	if !errors.Is(err, graphql.ErrNotFound) {
		return core.CloudVendorUnknown, uuid.Nil, err
	}

	nativeSubscription, err := azure.Wrap(client).NativeSubscriptionByCloudAccountID(ctx, cloudAccountID)
	if err == nil {
		return core.CloudVendorAzure, nativeSubscription.ID, nil
	}
	if !errors.Is(err, graphql.ErrNotFound) {
		return core.CloudVendorUnknown, uuid.Nil, err
	}

	nativeProject, err := gcp.Wrap(client).NativeProjectByCloudAccountID(ctx, cloudAccountID)
	if err == nil {
		return core.CloudVendorGCP, nativeProject.ID, nil
	}
	if !errors.Is(err, graphql.ErrNotFound) {
		return core.CloudVendorUnknown, uuid.Nil, err
	}

	return core.CloudVendorUnknown, uuid.Nil, fmt.Errorf("native cloud account id for cloud account %q %w", cloudAccountID, graphql.ErrNotFound)
}
