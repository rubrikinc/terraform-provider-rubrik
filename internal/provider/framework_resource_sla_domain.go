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
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/sla"
)

// slaDomainDeletePollInterval is how often an SLA domain is checked for assigned
// objects before it is deleted.
const slaDomainDeletePollInterval = 5 * time.Second

var (
	_ resource.Resource                     = &slaDomainResource{}
	_ resource.ResourceWithConfigure        = &slaDomainResource{}
	_ resource.ResourceWithConfigValidators = &slaDomainResource{}
	_ resource.ResourceWithIdentity         = &slaDomainResource{}
	_ resource.ResourceWithImportState      = &slaDomainResource{}
)

type slaDomainResource struct {
	client *client
	prefix string
}

type slaDomainIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newSLADomainResource() resource.Resource {
	return &slaDomainResource{prefix: keyRubrik}
}

func newPolarisSLADomainResource() resource.Resource {
	return &slaDomainResource{prefix: keyPolaris}
}

func (r *slaDomainResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "slaDomainResource.Metadata")

	res.TypeName = r.prefix + "_" + keySLADomain
}

func (r *slaDomainResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "slaDomainResource.Schema")

	res.Schema = slaDomainSchema(ctx)
	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_sla_domain` resource instead."
	}
}

func (r *slaDomainResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "slaDomainResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "SLA Domain ID (UUID).",
			},
		},
	}
}

func (r *slaDomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "slaDomainResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *slaDomainResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "slaDomainResource.ConfigValidators")

	return []resource.ConfigValidator{slaDomainAtLeastOneBlock()}
}

// slaDomainFeaturesFromRSC reads the feature flags which change how an SLA domain
// is created and updated. A lookup which fails is an error, since a flag which
// is wrongly read as off would route a backup location to the wrong place.
func slaDomainFeaturesFromRSC(ctx context.Context, polarisClient *polaris.Client) (slaDomainFeatures, error) {
	multipleBackupLocations, err := core.Wrap(polarisClient.GQL).FeatureFlag(ctx, "CNP_AWS_S3_MULTIPLE_BACKUP_LOCATIONS_ENABLED")
	if err != nil {
		return slaDomainFeatures{}, err
	}
	azureSQLRevamp, err := core.Wrap(polarisClient.GQL).FeatureFlag(ctx, "CNP_AZURE_SQL_SLA_REVAMP")
	if err != nil {
		return slaDomainFeatures{}, err
	}

	return slaDomainFeatures{
		MultipleBackupLocations: multipleBackupLocations.Enabled,
		AzureSQLRevamp:          azureSQLRevamp.Enabled,
	}, nil
}

// slaDomainParamsFromPlan converts the plan to the parameters for creating or
// updating an SLA domain.
func (r *slaDomainResource) slaDomainParamsFromPlan(ctx context.Context, polarisClient *polaris.Client, plan slaDomainModel) (gqlsla.CreateDomainParams, []string, error) {
	var objectTypeNames []string
	if diags := plan.ObjectTypes.ElementsAs(ctx, &objectTypeNames, false); diags.HasError() {
		return gqlsla.CreateDomainParams{}, nil, fmt.Errorf("failed to read %s: %s", keyObjectTypes, diags)
	}
	objectTypes := make([]gqlsla.ObjectType, 0, len(objectTypeNames))
	for _, name := range objectTypeNames {
		objectTypes = append(objectTypes, gqlsla.ObjectType(name))
	}

	features, err := slaDomainFeaturesFromRSC(ctx, polarisClient)
	if err != nil {
		return gqlsla.CreateDomainParams{}, nil, err
	}

	params, err := slaDomainParams(plan, objectTypes, features)
	return params, objectTypeNames, err
}

func (r *slaDomainResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "slaDomainResource.Create")

	var plan slaDomainModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	params, _, err := r.slaDomainParamsFromPlan(ctx, polarisClient, plan)
	if err != nil {
		res.Diagnostics.AddError("Invalid SLA domain", err.Error())
		return
	}

	id, err := sla.Wrap(polarisClient).CreateDomain(ctx, params)
	if err != nil {
		res.Diagnostics.AddError("Failed to create SLA domain", err.Error())
		return
	}

	// Save the ID to state and identity before the read-back so Terraform can
	// track the SLA domain even if the read fails. The values which are unknown
	// are set to null until they are read.
	plan.ID = types.StringValue(id.String())
	res.State.Raw = nullUnknownValues(req.Plan.Raw)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), plan.ID)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, slaDomainIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	domain, err := sla.Wrap(polarisClient).DomainByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read SLA domain after create",
			fmt.Sprintf("The SLA domain was created successfully but the computed fields could not be "+
				"populated: %s", err.Error()))
		return
	}

	state, diags := slaDomainStateFromRemote(ctx, slaDomainSchema(ctx), domain, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *slaDomainResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "slaDomainResource.Read")

	var state slaDomainModel
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
		res.Diagnostics.AddError("Invalid SLA domain ID", err.Error())
		return
	}

	domain, err := sla.Wrap(polarisClient).DomainByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read SLA domain", err.Error())
		return
	}

	newState, diags := slaDomainStateFromRemote(ctx, slaDomainSchema(ctx), domain, tfsdk.Plan(req.State))
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &newState)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, slaDomainIdentityModel{ID: newState.ID})...)
}

func (r *slaDomainResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "slaDomainResource.Update")

	var plan slaDomainModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid SLA domain ID", err.Error())
		return
	}

	params, _, err := r.slaDomainParamsFromPlan(ctx, polarisClient, plan)
	if err != nil {
		res.Diagnostics.AddError("Invalid SLA domain", err.Error())
		return
	}

	applyToExisting := plan.ApplyChangesToExistingSnapshots.ValueBool()
	updateParams := slaDomainUpdateParams(id, params, applyToExisting,
		applyToExisting && plan.ApplyChangesToNonPolicySnapshots.ValueBool())
	if err := sla.Wrap(polarisClient).UpdateDomain(ctx, updateParams); err != nil {
		res.Diagnostics.AddError("Failed to update SLA domain", err.Error())
		return
	}

	// The SDKv2 resource didn't read the SLA domain back after an update. It is
	// read here, since the backup type and the other computed values can change.
	domain, err := sla.Wrap(polarisClient).DomainByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read SLA domain after update",
			fmt.Sprintf("The SLA domain was updated successfully but the computed fields could not be "+
				"populated: %s", err.Error()))
		res.State.Raw = nullUnknownValues(req.Plan.Raw)
		return
	}

	state, diags := slaDomainStateFromRemote(ctx, slaDomainSchema(ctx), domain, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *slaDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "slaDomainResource.Delete")

	var state slaDomainModel
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
		res.Diagnostics.AddError("Invalid SLA domain ID", err.Error())
		return
	}

	timeout, diags := state.Timeouts.Delete(ctx, defaultSLADomainDeleteTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for the SLA domain service to report zero assigned objects before
	// deleting. This handles eventual consistency between the hierarchy service,
	// which processes unassignments, and the SLA domain service, which enforces
	// the "no assigned objects" precondition on delete. The wait is bounded by
	// the delete timeout.
	for {
		count, err := sla.Wrap(polarisClient).DomainObjectCount(ctx, id)
		if errors.Is(err, graphql.ErrNotFound) {
			return
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to count the objects assigned to the SLA domain", err.Error())
			return
		}
		if count == 0 {
			break
		}

		tflog.Debug(ctx, "SLA domain still has assigned objects, waiting before delete", map[string]any{
			"sla_id":       id.String(),
			"object_count": count,
		})

		select {
		case <-ctx.Done():
			res.Diagnostics.AddError("Timed out waiting for the SLA domain to have no assigned objects",
				fmt.Sprintf("%d objects are still assigned to the SLA domain: %s", count, ctx.Err()))
			return
		case <-time.After(slaDomainDeletePollInterval):
		}
	}

	err = sla.Wrap(polarisClient).DeleteDomain(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to delete SLA domain", err.Error())
	}
}

// ImportState imports an SLA domain by ID (UUID) or name. An import ID which
// is a UUID is looked up by ID, any other import ID is looked up by name.
func (r *slaDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "slaDomainResource.ImportState")

	var identity slaDomainIdentityModel
	if req.ID == "" {
		// Import by identity block.
		res.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if res.Diagnostics.HasError() {
			return
		}
	} else {
		polarisClient, err := r.client.polaris()
		if err != nil {
			res.Diagnostics.AddError("RSC client error", err.Error())
			return
		}

		var domain gqlsla.Domain
		if id, err := uuid.Parse(req.ID); err == nil {
			// Verify the SLA domain exists.
			if domain, err = sla.Wrap(polarisClient).DomainByID(ctx, id); err != nil {
				res.Diagnostics.AddError("Failed to find the SLA domain by ID", fmt.Sprintf("%q: %s", req.ID, err))
				return
			}
		} else if domain, err = sla.Wrap(polarisClient).DomainByName(ctx, req.ID); err != nil {
			res.Diagnostics.AddError("Failed to find the SLA domain by name", fmt.Sprintf("%q: %s", req.ID, err))
			return
		}
		identity = slaDomainIdentityModel{ID: types.StringValue(domain.ID.String())}
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), identity.ID)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}
