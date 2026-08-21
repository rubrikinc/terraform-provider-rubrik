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
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/dspm"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqldspm "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/dspm"
)

const resourceDataSecurityPolicyDescription = `
The ´rubrik_data_security_policy´ resource is used to create and manage data
security policies in RSC.

A policy matches on up to two groups of conditions: object conditions, in the
´object_filter´ block, and identity conditions, in the ´identity_filter´ block.
At least one of the two blocks is required. Conditions within a block are joined
by the block's ´op´ field, and the two blocks are always joined by AND. This is
the only filter shape RSC accepts, and it mirrors the RSC data security policy
editor.

Which block a condition belongs to follows from its ´filter_type´:
´SECURITY_DOCUMENT_*´ and ´SECURITY_SNAPPABLE_*´ conditions are object
conditions, ´SECURITY_IDENTITY_*´ and ´SECURITY_GPO_*´ conditions are identity
conditions.

The optional ´threshold_filter´ block decides how many matches raise a
violation. It holds a single condition, matching the RSC data security policy
editor.
`

var (
	_ resource.Resource                = &dataSecurityPolicyResource{}
	_ resource.ResourceWithIdentity    = &dataSecurityPolicyResource{}
	_ resource.ResourceWithImportState = &dataSecurityPolicyResource{}
)

type dataSecurityPolicyResource struct {
	client *client
}

type dataSecurityPolicyResourceModel struct {
	ID              types.String          `tfsdk:"id"`
	Name            types.String          `tfsdk:"name"`
	Description     types.String          `tfsdk:"description"`
	Category        types.String          `tfsdk:"category"`
	Severity        types.String          `tfsdk:"severity"`
	Enabled         types.Bool            `tfsdk:"enabled"`
	Predefined      types.Bool            `tfsdk:"predefined"`
	ObjectFilter    []conditionGroupModel `tfsdk:"object_filter"`
	IdentityFilter  []conditionGroupModel `tfsdk:"identity_filter"`
	ThresholdFilter []conditionModel      `tfsdk:"threshold_filter"`
}

type dataSecurityPolicyIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newDataSecurityPolicyResource() resource.Resource {
	return &dataSecurityPolicyResource{}
}

func (r *dataSecurityPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyDataSecurityPolicy
}

// dataSecurityPolicyRelationships holds the comparison operators RSC accepts
// for a filter condition. Which of them a given filter type actually allows is
// left to RSC, which reports the allowed set in its error.
var dataSecurityPolicyRelationships = []string{
	string(gqldspm.RelAfter),
	string(gqldspm.RelBefore),
	string(gqldspm.RelBetween),
	string(gqldspm.RelContains),
	string(gqldspm.RelEquals),
	string(gqldspm.RelExists),
	string(gqldspm.RelGreaterThan),
	string(gqldspm.RelIs),
	string(gqldspm.RelIsEmpty),
	string(gqldspm.RelIsNot),
	string(gqldspm.RelIsNotEmpty),
	string(gqldspm.RelLessThan),
	string(gqldspm.RelNoneOf),
	string(gqldspm.RelNotContains),
	string(gqldspm.RelNotEquals),
	string(gqldspm.RelOtherThan),
}

// filterTypeDescription returns the filter_type description for a condition
// group block accepting conditions of the given resource types.
func filterTypeDescription(resourceTypes ...filterResourceType) string {
	var prefixes []string
	for _, resourceType := range resourceTypes {
		for _, prefix := range filterTypePrefixes[resourceType] {
			prefixes = append(prefixes, prefix+"*")
		}
	}

	return fmt.Sprintf("Filter type. Valid filter types are %s.", strings.Join(prefixes, ", "))
}

// conditionAttributes returns the attributes of a single filter condition.
// resourceTypes restricts which conditions are accepted.
func conditionAttributes(resourceTypes ...filterResourceType) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		keyFilterType: schema.StringAttribute{
			Required:    true,
			Description: filterTypeDescription(resourceTypes...),
			Validators: []validator.String{
				isFilterTypeFor(resourceTypes...),
			},
		},
		keyRelationship: schema.StringAttribute{
			Required:    true,
			Description: "Comparison operator, e.g. IS, IS_NOT or CONTAINS. Which operators a filter type accepts depends on the filter type.",
			Validators: []validator.String{
				stringvalidator.OneOf(dataSecurityPolicyRelationships...),
			},
		},
		keyValues: schema.ListAttribute{
			ElementType: types.StringType,
			Optional:    true,
			Description: "Filter values. Left out for a relationship taking no values, such as EXISTS, IS_EMPTY and " +
				"IS_NOT_EMPTY. When specified, at least one value is required.",
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
		},
	}
}

// conditionGroupBlockSchema returns the schema for a flat group of filter
// conditions, used by the object_filter and identity_filter blocks.
// resourceTypes restricts which conditions the block accepts.
func conditionGroupBlockSchema(description string, resourceTypes ...filterResourceType) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		Validators: []validator.List{
			listvalidator.SizeAtMost(1),
		},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				keyOp: schema.StringAttribute{
					Optional:    true,
					Computed:    true,
					Default:     stringdefault.StaticString("AND"),
					Description: "Logical operator joining the conditions in this block. Valid values are AND and OR. Default value is AND.",
					Validators: []validator.String{
						stringvalidator.OneOf("AND", "OR"),
					},
				},
			},
			Blocks: map[string]schema.Block{
				keyCondition: schema.ListNestedBlock{
					Description: "Filter condition.",
					Validators: []validator.List{
						listvalidator.SizeAtLeast(1),
					},
					NestedObject: schema.NestedBlockObject{
						Attributes: conditionAttributes(resourceTypes...),
					},
				},
			},
		},
	}
}

// thresholdFilterBlockSchema returns the schema for the threshold_filter block.
// The block holds the condition directly, and at most one block can be
// declared, matching the single threshold field of the RSC data security policy
// editor. Should the editor gain support for more, the block can be extended
// accordingly.
func thresholdFilterBlockSchema(description string, resourceTypes ...filterResourceType) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		Validators: []validator.List{
			listvalidator.SizeAtMost(1),
		},
		NestedObject: schema.NestedBlockObject{
			Attributes: conditionAttributes(resourceTypes...),
		},
	}
}

func (r *dataSecurityPolicyResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceDataSecurityPolicyDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Data security policy ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Name of the data security policy.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyDescription: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Description of the data security policy.",
			},
			keyCategory: schema.StringAttribute{
				Required:    true,
				Description: "Category of the data security policy. Valid values are MISPLACED, OVEREXPOSED, REDUNDANT and UNPROTECTED.",
				Validators: []validator.String{
					stringvalidator.OneOf("MISPLACED", "OVEREXPOSED", "REDUNDANT", "UNPROTECTED"),
				},
			},
			keySeverity: schema.StringAttribute{
				Required:    true,
				Description: "Severity of the data security policy. Valid values are LOW, MEDIUM, HIGH and CRITICAL.",
				Validators: []validator.String{
					stringvalidator.OneOf("LOW", "MEDIUM", "HIGH", "CRITICAL"),
				},
			},
			keyEnabled: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the data security policy is enabled.",
			},
			keyPredefined: schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the data security policy is predefined.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			keyObjectFilter: conditionGroupBlockSchema(
				"Object conditions for the data security policy.",
				filterResourceTypeObject,
			),
			keyIdentityFilter: conditionGroupBlockSchema(
				"Identity conditions for the data security policy.",
				filterResourceTypeIdentity,
			),
			keyThresholdFilter: thresholdFilterBlockSchema(
				"Threshold condition deciding how many matches raise a violation. Typically a "+
					"SECURITY_DOCUMENT_HIT_COUNT condition.",
				filterResourceTypeObject, filterResourceTypeIdentity,
			),
		},
	}
}

func (r *dataSecurityPolicyResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Data security policy ID (UUID).",
			},
		},
	}
}

func (r *dataSecurityPolicyResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "dataSecurityPolicyResource.ConfigValidators")

	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot(keyObjectFilter),
			path.MatchRoot(keyIdentityFilter),
		),
	}
}

func (r *dataSecurityPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *dataSecurityPolicyResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.Create")

	var plan dataSecurityPolicyResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	filter, diags := toPolicyFilter(ctx, plan.ObjectFilter, plan.IdentityFilter)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	thresholdFilter, diags := toThresholdGroupConfig(ctx, plan.ThresholdFilter)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	input := gqldspm.CreateInput{
		Name:            plan.Name.ValueString(),
		Description:     plan.Description.ValueString(),
		Category:        gqldspm.Category(plan.Category.ValueString()),
		Severity:        gqldspm.Severity(plan.Severity.ValueString()),
		Filter:          filter,
		ThresholdFilter: thresholdFilter,
	}

	policyID, err := dspm.Wrap(polarisClient).CreatePolicy(ctx, input)
	if err != nil {
		res.Diagnostics.AddError("Failed to create data security policy", err.Error())
		return
	}

	// The policy is not read back: CreatePolicy returns the ID, and a policy
	// created by the provider is never predefined. The filter and the other
	// user-provided fields stay as the plan specified, to avoid inconsistent
	// results from normalization.
	plan.ID = types.StringValue(policyID.String())
	plan.Predefined = types.BoolValue(false)

	// The policy exists in RSC from here on, so the state is written before the
	// follow-up update below. Without it, a failing update would leave behind a
	// policy that Terraform has no record of.
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.Identity.Set(ctx, dataSecurityPolicyIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	// The create API always creates an enabled policy. If the user requested
	// disabled, issue a follow-up update. Should it fail, the enabled field
	// written above is corrected by the next read.
	if !plan.Enabled.ValueBool() {
		enabled := false
		if err := dspm.Wrap(polarisClient).UpdatePolicy(ctx, gqldspm.UpdateInput{
			ID:      policyID,
			Enabled: &enabled,
		}); err != nil {
			res.Diagnostics.AddError("Failed to disable data security policy after create", err.Error())
			return
		}
	}
}

func (r *dataSecurityPolicyResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.Read")

	var state dataSecurityPolicyResourceModel
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
		res.Diagnostics.AddError("Invalid policy ID", err.Error())
		return
	}

	policy, err := dspm.Wrap(polarisClient).PolicyByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read data security policy", err.Error())
		return
	}

	res.Diagnostics.Append(r.policyToModel(ctx, &state, policy)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.Identity.Set(ctx, dataSecurityPolicyIdentityModel{ID: state.ID})...)
}

func (r *dataSecurityPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.Update")

	var plan dataSecurityPolicyResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state dataSecurityPolicyResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	name := plan.Name.ValueString()
	desc := plan.Description.ValueString()
	cat := gqldspm.Category(plan.Category.ValueString())
	sev := gqldspm.Severity(plan.Severity.ValueString())
	enabled := plan.Enabled.ValueBool()

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid policy ID", err.Error())
		return
	}

	filter, diags := toPolicyFilter(ctx, plan.ObjectFilter, plan.IdentityFilter)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	thresholdFilter, diags := toThresholdGroupConfig(ctx, plan.ThresholdFilter)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	input := gqldspm.UpdateInput{
		ID:              id,
		Name:            &name,
		Description:     &desc,
		Category:        &cat,
		Severity:        &sev,
		Enabled:         &enabled,
		Filter:          &filter,
		ThresholdFilter: thresholdFilter,

		// A nil threshold filter is ambiguous on the wire: it means both
		// "leave the stored value alone" and "clear it". Removing the block
		// must clear it, so ask for the nil to be honored.
		ForceUpdateThresholdFilter: thresholdFilter == nil,
	}

	if err := dspm.Wrap(polarisClient).UpdatePolicy(ctx, input); err != nil {
		res.Diagnostics.AddError("Failed to update data security policy", err.Error())
		return
	}

	// The policy is not read back, as in Create. Both computed fields are
	// already carried over from the state by their UseStateForUnknown plan
	// modifiers. Reading would only risk dropping a successful update from the
	// state. The filter and the other user-provided fields stay as the plan
	// specified, to avoid inconsistent results from normalization.
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.Identity.Set(ctx, dataSecurityPolicyIdentityModel{ID: plan.ID})...)
}

func (r *dataSecurityPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.Delete")

	var state dataSecurityPolicyResourceModel
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
		res.Diagnostics.AddError("Invalid policy ID", err.Error())
		return
	}

	err = dspm.Wrap(polarisClient).DeletePolicy(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to delete data security policy", err.Error())
	}
}

func (r *dataSecurityPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyResource.ImportState")

	// An empty import ID means the import used an identity block, where the
	// policy ID is read from the identity instead.
	if req.ID != "" {
		if _, err := uuid.Parse(req.ID); err != nil {
			res.Diagnostics.AddError("Invalid import ID",
				"Expected a valid UUID as the import ID.")
			return
		}
	}

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

func (r *dataSecurityPolicyResource) policyToModel(ctx context.Context, model *dataSecurityPolicyResourceModel, policy gqldspm.Policy) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(policy.ID.String())
	model.Name = types.StringValue(policy.Name)
	model.Description = types.StringValue(policy.Description)
	model.Category = types.StringValue(string(policy.Category))
	model.Severity = types.StringValue(string(policy.Severity))
	model.Enabled = types.BoolValue(policy.Enabled)
	model.Predefined = types.BoolValue(policy.Predefined)

	object, identity, d := fromPolicyFilter(ctx, policy.Filter)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	model.ObjectFilter = object
	model.IdentityFilter = identity

	thresholdFilter, d := fromThresholdGroupConfig(ctx, policy.ThresholdFilter)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	model.ThresholdFilter = thresholdFilter

	return diags
}
