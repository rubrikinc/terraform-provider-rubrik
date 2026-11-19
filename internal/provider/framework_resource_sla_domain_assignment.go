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
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/hierarchy"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/sla"
)

// Note: The description uses acute accent marks (´) instead of backticks (`)
// because backticks would terminate the raw string literal. The description()
// function converts them to backticks.
const resourceSLADomainAssignmentDescription = `
The ´rubrik_sla_domain_assignment´ resource is used to assign SLA domains to
objects.

To protect an AWS cloud application, assign an SLA Domain with the
´AWS_CONFIG_OBJECT_TYPE´ object type, using the ´id´ of the
´rubrik_aws_cloud_application´ resource as the object ID.

When an object is removed from the ´rubrik_sla_domain_assignment´ resource, it
will inherit the SLA Domain of its parent object. If there is no parent object
or the parent object doesn't have an SLA Domain, the object will be unprotected.
Existing snapshots of the object will be retained according to the SLA Domain
inherited from the parent object. If the parent object doesn't have an SLA
Domain, the existing snapshots will be retained forever.

The ´assignment_type´ attribute controls how the SLA domain is assigned:

  * ´protectWithSlaId´ - Protect objects with the specified SLA domain. Requires ´sla_domain_id´ to be set.

  * ´doNotProtect´ - Do not protect objects. Requires that ´sla_domain_id´ isn't set.

~> **Note:** When importing, ´apply_changes_to_existing_snapshots´,
´apply_changes_to_non_policy_snapshots´, and ´workload´ cannot be retrieved
from the API. These attributes will use their default values after import.
For ´workload´, the default is ´ALL_SUB_HIERARCHY_TYPE´.
`

// doNotProtectID is the sentinel resource ID used when the assignment type is
// DoNotProtect.
const doNotProtectID = "doNotProtect"

var (
	_ resource.Resource                   = &slaDomainAssignmentResource{}
	_ resource.ResourceWithImportState    = &slaDomainAssignmentResource{}
	_ resource.ResourceWithMoveState      = &slaDomainAssignmentResource{}
	_ resource.ResourceWithUpgradeState   = &slaDomainAssignmentResource{}
	_ resource.ResourceWithValidateConfig = &slaDomainAssignmentResource{}
)

type slaDomainAssignmentResource struct {
	client *client
	prefix string
}

// slaDomainAssignmentModel is the resource state. The resource has no resource
// identity, since its ID isn't unique: any number of assignments can share an
// SLA domain, and every doNotProtect assignment has the same ID.
type slaDomainAssignmentModel struct {
	ID                               types.String `tfsdk:"id"`
	AssignmentType                   types.String `tfsdk:"assignment_type"`
	ObjectIDs                        types.Set    `tfsdk:"object_ids"`
	SLADomainID                      types.String `tfsdk:"sla_domain_id"`
	ApplyChangesToExistingSnapshots  types.Bool   `tfsdk:"apply_changes_to_existing_snapshots"`
	ApplyChangesToNonPolicySnapshots types.Bool   `tfsdk:"apply_changes_to_non_policy_snapshots"`
	ExistingSnapshotRetention        types.String `tfsdk:"existing_snapshot_retention"`
	Workload                         types.String `tfsdk:"workload"`
}

func newSLADomainAssignmentResource() resource.Resource {
	return &slaDomainAssignmentResource{prefix: keyRubrik}
}

func newPolarisSLADomainAssignmentResource() resource.Resource {
	return &slaDomainAssignmentResource{prefix: keyPolaris}
}

func (r *slaDomainAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.Metadata")

	res.TypeName = r.prefix + "_" + keySLADomainAssignment
}

func (r *slaDomainAssignmentResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.Schema")

	res.Schema = schema.Schema{
		// Version 1 is the SDKv2 schema version, so existing state is accepted
		// as is. Version 0 state is upgraded by UpgradeState.
		Version:     1,
		Description: description(resourceSLADomainAssignmentDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SLA domain ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyAssignmentType: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(string(gqlsla.ProtectWithSLA)),
				Description: "SLA domain assignment type. Valid values are `protectWithSlaId` and " +
					"`doNotProtect`. Defaults to `protectWithSlaId`.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(gqlsla.ProtectWithSLA), string(gqlsla.DoNotProtect)),
				},
			},
			keyObjectIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Object IDs (UUID).",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(isUUID()),
				},
			},
			keySLADomainID: schema.StringAttribute{
				Optional:    true,
				Description: "SLA domain ID (UUID). Required when `assignment_type` is `protectWithSlaId`.",
				Validators: []validator.String{
					isUUID(),
					stringvalidator.ConflictsWith(path.MatchRoot(keyExistingSnapshotRetention)),
				},
			},
			keyApplyChangesToExistingSnapshots: schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
				Description: "Apply SLA changes to existing snapshots. Only valid when `assignment_type` is " +
					"`protectWithSlaId`. Defaults to `true`.",
			},
			keyApplyChangesToNonPolicySnapshots: schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Apply SLA changes to non-policy snapshots. Only valid when `assignment_type` is " +
					"`protectWithSlaId`. Defaults to `false`.",
			},
			keyExistingSnapshotRetention: schema.StringAttribute{
				Optional: true,
				Description: "Existing snapshot retention policy. Only valid when `assignment_type` is " +
					"`doNotProtect`. Valid values are `RETAIN_SNAPSHOTS`, `KEEP_FOREVER`, and `EXPIRE_IMMEDIATELY`.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(gqlsla.RetainSnapshots), string(gqlsla.KeepForever),
						string(gqlsla.ExpireImmediately)),
					stringvalidator.ConflictsWith(path.MatchRoot(keySLADomainID)),
				},
			},
			keyWorkload: schema.StringAttribute{
				Optional: true,
				Description: "Workload hierarchy type for SLA Domain assignments. If not specified, " +
					"`ALL_SUB_HIERARCHY_TYPE` is used. Valid values: `ALL_SUB_HIERARCHY_TYPE`, " +
					"`AZURE_NATIVE_VIRTUAL_MACHINE`, `AZURE_NATIVE_MANAGED_DISK`, `AZURE_SQL_DATABASE_DB`, " +
					"`AZURE_SQL_MANAGED_INSTANCE_DB`, `AZURE_STORAGE_ACCOUNT`.",
				Validators: []validator.String{
					stringvalidator.OneOf(hierarchy.AllWorkloadsAsStrings()...),
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_sla_domain_assignment` instead."
	}
}

func (r *slaDomainAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *slaDomainAssignmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.ValidateConfig")

	var config slaDomainAssignmentModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(validateSLADomainAssignmentConfig(config)...)
}

// validateSLADomainAssignmentConfig holds the plan-time, client-free validation
// rules for the resource so they can be unit-tested in isolation. They are the
// rules the SDKv2 resource checked in Create and Update, which means they were
// only reported at apply time.
func validateSLADomainAssignmentConfig(config slaDomainAssignmentModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// The rules depend on the assignment type, which is not known until the
	// configuration is. The assignment type defaults to protectWithSlaId, so a
	// null value is the default.
	if config.AssignmentType.IsUnknown() {
		return diags
	}
	assignType := gqlsla.AssignmentType(config.AssignmentType.ValueString())
	if config.AssignmentType.IsNull() {
		assignType = gqlsla.ProtectWithSLA
	}

	switch assignType {
	case gqlsla.ProtectWithSLA:
		if config.SLADomainID.IsNull() {
			diags.AddAttributeError(path.Root(keySLADomainID), "Missing SLA domain ID",
				fmt.Sprintf("sla_domain_id is required when assignment_type is %q", gqlsla.ProtectWithSLA))
		}
		if !config.ExistingSnapshotRetention.IsNull() && !config.ExistingSnapshotRetention.IsUnknown() {
			diags.AddAttributeError(path.Root(keyExistingSnapshotRetention), "Invalid existing snapshot retention",
				fmt.Sprintf("existing_snapshot_retention is only valid when assignment_type is %q", gqlsla.DoNotProtect))
		}
		if !config.ApplyChangesToNonPolicySnapshots.IsUnknown() && !config.ApplyChangesToExistingSnapshots.IsUnknown() {
			// An unset value takes its default: true for existing snapshots and
			// false for non-policy snapshots.
			applyToExisting := config.ApplyChangesToExistingSnapshots.IsNull() || config.ApplyChangesToExistingSnapshots.ValueBool()
			applyToNonPolicy := !config.ApplyChangesToNonPolicySnapshots.IsNull() && config.ApplyChangesToNonPolicySnapshots.ValueBool()
			if applyToNonPolicy && !applyToExisting {
				diags.AddAttributeError(path.Root(keyApplyChangesToNonPolicySnapshots),
					"Invalid snapshot settings",
					"apply_changes_to_non_policy_snapshots requires apply_changes_to_existing_snapshots to be true")
			}
		}
	case gqlsla.DoNotProtect:
		if !config.SLADomainID.IsNull() && !config.SLADomainID.IsUnknown() {
			diags.AddAttributeError(path.Root(keySLADomainID), "Invalid SLA domain ID",
				fmt.Sprintf("sla_domain_id must be empty when assignment_type is %q", gqlsla.DoNotProtect))
		}
	}

	return diags
}

func (r *slaDomainAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.Create")

	var plan slaDomainAssignmentModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	objectIDs, diags := parseObjectIDs(ctx, plan.ObjectIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	workload, err := parseWorkload(plan.Workload)
	if err != nil {
		res.Diagnostics.AddError("Invalid workload", err.Error())
		return
	}

	params, expectedDomainID, err := assignParamsFromModel(plan, objectIDs, workload)
	if err != nil {
		res.Diagnostics.AddError("Invalid SLA domain assignment", err.Error())
		return
	}

	if err := sla.Wrap(polarisClient).AssignDomain(ctx, params); err != nil {
		res.Diagnostics.AddError("Failed to assign SLA domain", err.Error())
		return
	}

	// Set the resource ID based on the assignment type, before waiting for the
	// assignment, so the assignment is tracked even if the wait fails.
	if params.DomainAssignType == gqlsla.DoNotProtect {
		plan.ID = types.StringValue(doNotProtectID)
	} else {
		plan.ID = types.StringValue(params.DomainID.String())
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Wait for the assignment to be applied.
	if err := waitForAssignment(ctx, polarisClient, expectedDomainID, objectIDs, workload); err != nil {
		res.Diagnostics.AddError("Failed to wait for the SLA domain assignment", err.Error())
	}
}

func (r *slaDomainAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.Read")

	var state slaDomainAssignmentModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	assignType := gqlsla.AssignmentType(state.AssignmentType.ValueString())
	domainIDStr := state.SLADomainID.ValueString()

	workload, err := parseWorkload(state.Workload)
	if err != nil {
		res.Diagnostics.AddError("Invalid workload", err.Error())
		return
	}

	// For protectWithSlaId, verify the SLA domain still exists.
	if assignType == gqlsla.ProtectWithSLA && domainIDStr != "" {
		domainID, err := uuid.Parse(domainIDStr)
		if err != nil {
			res.Diagnostics.AddError("Invalid SLA domain ID", err.Error())
			return
		}
		if _, err := sla.Wrap(polarisClient).DomainByID(ctx, domainID); err != nil {
			if errors.Is(err, graphql.ErrNotFound) {
				res.State.RemoveResource(ctx)
				return
			}
			res.Diagnostics.AddError("Failed to read SLA domain", err.Error())
			return
		}
	}

	// Query each object to check if it still has the expected SLA directly
	// assigned.
	var objectIDStrs []string
	res.Diagnostics.Append(state.ObjectIDs.ElementsAs(ctx, &objectIDStrs, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	remaining := make([]attr.Value, 0, len(objectIDStrs))
	for _, idStr := range objectIDStrs {
		objectID, err := uuid.Parse(idStr)
		if err != nil {
			res.Diagnostics.AddError("Invalid object ID", err.Error())
			return
		}

		obj, err := workaroundBrokenHierarchy(ctx, polarisClient, objectID, workload)
		if err != nil {
			if errors.Is(err, graphql.ErrNotFound) {
				// Object no longer exists, skip it.
				continue
			}
			res.Diagnostics.AddError("Failed to read object", err.Error())
			return
		}

		// Check if the object has the expected SLA directly assigned.
		if obj.SLAAssignment != gqlsla.Direct {
			continue
		}

		switch assignType {
		case gqlsla.DoNotProtect:
			// For doNotProtect, check effectiveSlaDomain because with
			// RETAIN_SNAPSHOTS, configuredSlaDomain keeps the previous SLA
			// (converted to retention type) while effectiveSlaDomain becomes
			// DO_NOT_PROTECT.
			if obj.EffectiveSLADomain.ID == gqlsla.DoNotProtectSLAID {
				remaining = append(remaining, types.StringValue(idStr))
			}
		case gqlsla.ProtectWithSLA:
			if obj.ConfiguredSLADomain.ID == domainIDStr {
				remaining = append(remaining, types.StringValue(idStr))
			}
		}
	}

	if len(remaining) == 0 {
		res.State.RemoveResource(ctx)
		return
	}

	objectIDs, diags := types.SetValue(types.StringType, remaining)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.ObjectIDs = objectIDs

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *slaDomainAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.Update")

	var plan slaDomainAssignmentModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state slaDomainAssignmentModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	addObjectIDs, removeObjectIDs, totalObjectIDs, diags := diffObjectIDs(ctx, state.ObjectIDs, plan.ObjectIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	workload, err := parseWorkload(plan.Workload)
	if err != nil {
		res.Diagnostics.AddError("Invalid workload", err.Error())
		return
	}

	assignType := gqlsla.AssignmentType(plan.AssignmentType.ValueString())

	// When the assignment type, domain ID, snapshot retention, apply settings
	// or workload change, all objects have to be assigned again.
	if !plan.AssignmentType.Equal(state.AssignmentType) ||
		!plan.SLADomainID.Equal(state.SLADomainID) ||
		!plan.ExistingSnapshotRetention.Equal(state.ExistingSnapshotRetention) ||
		!plan.ApplyChangesToExistingSnapshots.Equal(state.ApplyChangesToExistingSnapshots) ||
		!plan.ApplyChangesToNonPolicySnapshots.Equal(state.ApplyChangesToNonPolicySnapshots) ||
		!plan.Workload.Equal(state.Workload) {
		addObjectIDs = totalObjectIDs
	}

	if len(addObjectIDs) > 0 {
		params, expectedDomainID, err := assignParamsFromModel(plan, addObjectIDs, workload)
		if err != nil {
			res.Diagnostics.AddError("Invalid SLA domain assignment", err.Error())
			return
		}

		if err := sla.Wrap(polarisClient).AssignDomain(ctx, params); err != nil {
			res.Diagnostics.AddError("Failed to assign SLA domain", err.Error())
			return
		}

		// Wait for the assignment to be applied.
		if err := waitForAssignment(ctx, polarisClient, expectedDomainID, addObjectIDs, workload); err != nil {
			res.Diagnostics.AddError("Failed to wait for the SLA domain assignment", err.Error())
			return
		}
	}

	if len(removeObjectIDs) > 0 {
		if err := sla.Wrap(polarisClient).AssignDomain(ctx, gqlsla.AssignDomainParams{
			DomainAssignType:          gqlsla.NoAssignment,
			ObjectIDs:                 removeObjectIDs,
			ApplyToExistingSnapshots:  ptr(true),
			ApplyToNonPolicySnapshots: ptr(false),
			ApplicableWorkloadType:    workload,
		}); err != nil {
			res.Diagnostics.AddError("Failed to remove SLA domain assignment", err.Error())
			return
		}

		// Wait for the unassignment to be applied.
		if err := waitForAssignment(ctx, polarisClient, gqlsla.UnprotectedSLAID, removeObjectIDs, workload); err != nil {
			res.Diagnostics.AddError("Failed to wait for the SLA domain unassignment", err.Error())
			return
		}
	}

	// Update the resource ID based on the assignment type.
	if assignType == gqlsla.DoNotProtect {
		plan.ID = types.StringValue(doNotProtectID)
	} else {
		plan.ID = plan.SLADomainID
	}

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *slaDomainAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.Delete")

	var state slaDomainAssignmentModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// Get the current SLA domain ID to track what is being removed.
	currentSLADomainID := state.ID.ValueString()
	if currentSLADomainID == doNotProtectID {
		currentSLADomainID = gqlsla.DoNotProtectSLAID
	}

	workload, err := parseWorkload(state.Workload)
	if err != nil {
		res.Diagnostics.AddError("Invalid workload", err.Error())
		return
	}

	objectIDs, diags := parseObjectIDs(ctx, state.ObjectIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Filter objects to only unassign those that are still assigned to this
	// resource's SLA. This prevents issues where concurrent creates might have
	// already reassigned the object to a different SLA.
	var objectsToUnassign []uuid.UUID
	for _, objectID := range objectIDs {
		obj, err := workaroundBrokenHierarchy(ctx, polarisClient, objectID, workload)
		if err != nil {
			if errors.Is(err, graphql.ErrNotFound) {
				// Object no longer exists, skip.
				continue
			}
			res.Diagnostics.AddError("Failed to read object", err.Error())
			return
		}

		// Only unassign if the object still has our SLA directly assigned.
		if obj.SLAAssignment != gqlsla.Direct {
			continue
		}

		switch currentSLADomainID {
		case gqlsla.DoNotProtectSLAID:
			// For doNotProtect, check both effective and configured SLA. If
			// configuredSlaDomain is a regular SLA (not DO_NOT_PROTECT or
			// UNPROTECTED), a concurrent create has already assigned the
			// object to a new SLA, so the unassignment is skipped.
			if obj.EffectiveSLADomain.ID == gqlsla.DoNotProtectSLAID {
				configuredID := obj.ConfiguredSLADomain.ID
				if configuredID == gqlsla.DoNotProtectSLAID || configuredID == gqlsla.UnprotectedSLAID {
					objectsToUnassign = append(objectsToUnassign, objectID)
				}
			}
		default:
			// For regular SLAs, check configured SLA.
			if obj.ConfiguredSLADomain.ID == currentSLADomainID {
				objectsToUnassign = append(objectsToUnassign, objectID)
			}
		}
	}

	if len(objectsToUnassign) > 0 {
		if err := sla.Wrap(polarisClient).AssignDomain(ctx, gqlsla.AssignDomainParams{
			DomainAssignType:          gqlsla.NoAssignment,
			ObjectIDs:                 objectsToUnassign,
			ApplyToExistingSnapshots:  ptr(true),
			ApplyToNonPolicySnapshots: ptr(false),
			ApplicableWorkloadType:    workload,
		}); err != nil {
			res.Diagnostics.AddError("Failed to remove SLA domain assignment", err.Error())
			return
		}

		// Wait for the assignment to be removed.
		if err := waitForAssignmentRemoval(ctx, polarisClient, currentSLADomainID, objectsToUnassign, workload); err != nil {
			res.Diagnostics.AddError("Failed to wait for the SLA domain assignment removal", err.Error())
		}
	}
}

// ImportState imports an assignment. The resource only manages the assignments
// it owns, while an import takes ownership of all the assignments of an SLA
// domain.
//
// The import ID formats are:
//   - <sla_domain_id> - for protectWithSlaId assignments, imports all objects
//     directly assigned to the SLA domain
//   - doNotProtect:<object_id1>,<object_id2>,... - for doNotProtect
//     assignments, imports the specified objects after verifying they have
//     DO_NOT_PROTECT assigned
//
// The resource has no resource identity, so there is no import by identity.
func (r *slaDomainAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "slaDomainAssignmentResource.ImportState")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	if strings.HasPrefix(req.ID, doNotProtectID+":") {
		r.importDoNotProtect(ctx, polarisClient, req.ID, res)
		return
	}
	r.importProtectWithSLA(ctx, polarisClient, req.ID, res)
}

// importProtectWithSLA imports a protectWithSlaId assignment by querying all
// objects directly assigned to the specified SLA domain.
func (r *slaDomainAssignmentResource) importProtectWithSLA(ctx context.Context, polarisClient *polaris.Client, importID string, res *resource.ImportStateResponse) {
	domainID, err := uuid.Parse(importID)
	if err != nil {
		res.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid SLA domain ID %q: %s", importID, err))
		return
	}

	// Return a human-readable error message if the SLA domain doesn't exist.
	if _, err := sla.Wrap(polarisClient).DomainByID(ctx, domainID); err != nil {
		res.Diagnostics.AddError("Failed to read SLA domain", err.Error())
		return
	}

	objects, err := sla.Wrap(polarisClient).DomainObjects(ctx, domainID, "")
	if err != nil {
		res.Diagnostics.AddError("Failed to read the SLA domain objects", err.Error())
		return
	}

	objectIDs := make([]attr.Value, 0, len(objects))
	for _, object := range objects {
		objectIDs = append(objectIDs, types.StringValue(object.ID.String()))
	}
	objectIDSet, diags := types.SetValue(types.StringType, objectIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// The apply settings and the workload can't be read from RSC, so they get
	// their defaults, and the resource ID is the SLA domain ID.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), domainID.String())...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keySLADomainID), domainID.String())...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyAssignmentType), string(gqlsla.ProtectWithSLA))...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyObjectIDs), objectIDSet)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyApplyChangesToExistingSnapshots), true)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyApplyChangesToNonPolicySnapshots), false)...)
}

// importDoNotProtect imports a doNotProtect assignment by verifying that the
// specified objects have DO_NOT_PROTECT as their effective SLA.
func (r *slaDomainAssignmentResource) importDoNotProtect(ctx context.Context, polarisClient *polaris.Client, importID string, res *resource.ImportStateResponse) {
	// Parse the object IDs from the import ID format:
	// doNotProtect:<id1>,<id2>,...
	objectIDsStr := strings.TrimPrefix(importID, doNotProtectID+":")
	if objectIDsStr == "" {
		res.Diagnostics.AddError("Invalid import ID",
			"import format for doNotProtect requires object IDs: 'doNotProtect:<object_id1>,<object_id2>,...'")
		return
	}

	var objectIDs []uuid.UUID
	for _, idStr := range strings.Split(objectIDsStr, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		objectID, err := uuid.Parse(idStr)
		if err != nil {
			res.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid object ID %q: %s", idStr, err))
			return
		}
		objectIDs = append(objectIDs, objectID)
	}
	if len(objectIDs) == 0 {
		res.Diagnostics.AddError("Invalid import ID",
			"import format for doNotProtect requires at least one object ID: 'doNotProtect:<object_id1>,<object_id2>,...'")
		return
	}

	// Verify each object has DO_NOT_PROTECT as its effective SLA. The workload
	// type isn't known on import, so AllSubHierarchyType is used.
	verified := make([]attr.Value, 0, len(objectIDs))
	for _, objectID := range objectIDs {
		obj, err := workaroundBrokenHierarchy(ctx, polarisClient, objectID, hierarchy.WorkloadAllSubHierarchyType)
		if err != nil {
			res.Diagnostics.AddError("Failed to read object", fmt.Sprintf("failed to get object %s: %s", objectID, err))
			return
		}

		if obj.EffectiveSLADomain.ID != gqlsla.DoNotProtectSLAID {
			res.Diagnostics.AddError("Invalid import ID", fmt.Sprintf(
				"object %s does not have DO_NOT_PROTECT assigned (effective SLA: %s)", objectID, obj.EffectiveSLADomain.Name))
			return
		}
		if obj.SLAAssignment != gqlsla.Direct {
			res.Diagnostics.AddError("Invalid import ID", fmt.Sprintf(
				"object %s does not have DO_NOT_PROTECT directly assigned (assignment type: %s)", objectID, obj.SLAAssignment))
			return
		}

		verified = append(verified, types.StringValue(objectID.String()))
	}

	objectIDSet, diags := types.SetValue(types.StringType, verified)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), doNotProtectID)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyAssignmentType), string(gqlsla.DoNotProtect))...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyObjectIDs), objectIDSet)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyApplyChangesToExistingSnapshots), true)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyApplyChangesToNonPolicySnapshots), false)...)
}

// parseObjectIDs converts the object ID set to UUIDs.
func parseObjectIDs(ctx context.Context, objectIDs types.Set) ([]uuid.UUID, diag.Diagnostics) {
	var diags diag.Diagnostics

	var ids []string
	diags.Append(objectIDs.ElementsAs(ctx, &ids, false)...)
	if diags.HasError() {
		return nil, diags
	}

	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		objectID, err := uuid.Parse(id)
		if err != nil {
			diags.AddError("Invalid object ID", err.Error())
			return nil, diags
		}
		parsed = append(parsed, objectID)
	}

	return parsed, diags
}

// parseWorkload returns the workload, defaulting to AllSubHierarchyType when
// the workload is not set.
func parseWorkload(workload types.String) (hierarchy.Workload, error) {
	if workload.IsNull() || workload.IsUnknown() || workload.ValueString() == "" {
		return hierarchy.WorkloadAllSubHierarchyType, nil
	}
	return hierarchy.ToWorkload(workload.ValueString())
}

// assignParamsFromModel returns the parameters used to assign the objects, and
// the SLA domain ID to wait for. For protectWithSlaId the SLA domain ID is
// required, and for doNotProtect it must be empty and the existing snapshot
// retention is allowed. The rules are also checked at plan time by
// validateSLADomainAssignmentConfig, but are kept here as well since the model
// can come from state.
func assignParamsFromModel(model slaDomainAssignmentModel, objectIDs []uuid.UUID, workload hierarchy.Workload) (gqlsla.AssignDomainParams, string, error) {
	assignType := gqlsla.AssignmentType(model.AssignmentType.ValueString())
	params := gqlsla.AssignDomainParams{
		DomainAssignType:       assignType,
		ObjectIDs:              objectIDs,
		ApplicableWorkloadType: workload,
	}

	switch assignType {
	case gqlsla.ProtectWithSLA:
		domainID := model.SLADomainID.ValueString()
		if domainID == "" {
			return params, "", fmt.Errorf("sla_domain_id is required when assignment_type is %q", gqlsla.ProtectWithSLA)
		}
		if model.ExistingSnapshotRetention.ValueString() != "" {
			return params, "", fmt.Errorf("existing_snapshot_retention is only valid when assignment_type is %q",
				gqlsla.DoNotProtect)
		}
		applyToExisting := model.ApplyChangesToExistingSnapshots.ValueBool()
		applyToNonPolicy := model.ApplyChangesToNonPolicySnapshots.ValueBool()
		if applyToNonPolicy && !applyToExisting {
			return params, "", fmt.Errorf("apply_changes_to_non_policy_snapshots requires " +
				"apply_changes_to_existing_snapshots to be true")
		}
		id, err := uuid.Parse(domainID)
		if err != nil {
			return params, "", err
		}
		params.DomainID = &id
		params.ApplyToExistingSnapshots = ptr(applyToExisting)
		params.ApplyToNonPolicySnapshots = ptr(applyToNonPolicy)
		return params, domainID, nil
	case gqlsla.DoNotProtect:
		if model.SLADomainID.ValueString() != "" {
			return params, "", fmt.Errorf("sla_domain_id must be empty when assignment_type is %q", gqlsla.DoNotProtect)
		}
		if retention := model.ExistingSnapshotRetention.ValueString(); retention != "" {
			params.ExistingSnapshotRetention = gqlsla.ExistingSnapshotRetention(retention)
		}
		return params, gqlsla.DoNotProtectSLAID, nil
	}

	return params, "", fmt.Errorf("unknown assignment type %q", assignType)
}

// diffObjectIDs returns the object IDs to add and remove, and the total which
// should be assigned to the SLA domain after the update.
func diffObjectIDs(ctx context.Context, oldIDs, newIDs types.Set) (add, remove, total []uuid.UUID, diags diag.Diagnostics) {
	oldObjIDs, d := parseObjectIDs(ctx, oldIDs)
	diags.Append(d...)
	newObjIDs, d := parseObjectIDs(ctx, newIDs)
	diags.Append(d...)
	if diags.HasError() {
		return nil, nil, nil, diags
	}

	for _, id := range newObjIDs {
		total = append(total, id)
		if !slices.Contains(oldObjIDs, id) {
			add = append(add, id)
		}
	}
	for _, id := range oldObjIDs {
		if !slices.Contains(newObjIDs, id) {
			remove = append(remove, id)
		}
	}

	return add, remove, total, diags
}
