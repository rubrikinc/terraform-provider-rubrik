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
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cloudapp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlcloudapp "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cloudapp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
)

const resourceAWSCloudNativeApplicationDescription = `
The ´rubrik_aws_cloud_native_application´ resource manages an AWS cloud native application, shown as a Cloud
Application in the RSC UI. An application groups the resources of one AWS account and region, so that their
configuration can be protected by an SLA Domain with the ´AWS_CONFIG_OBJECT_TYPE´ object type, and recovered to the
target account.

The AWS account must be onboarded with the ´CLOUD_NATIVE_CONFIG_PROTECTION´ feature in the application region, for
example using the ´rubrik_aws_cnp_account´ and ´rubrik_aws_cnp_account_attachments´ resources, and the feature must be
connected before the application can be created. RSC must also have finished discovering the account resources for
´resource_arns´ to be accepted. The target account must be a
different AWS account, onboarded with the ´CLOUD_NATIVE_CONFIG_PROTECTION´ feature including the ´RECOVERY´
permission groups, in the same region. RSC doesn't check the target account.

A resource joins the application when it carries any one of the ´matching_tags´, or when it is listed in
´resource_arns´. RSC also includes the resources related to a tagged or listed resource, and those must not be
listed. A resource listed in ´excluded_resource_arns´ is left out, unless it's tightly coupled to an included one.

Assign the SLA Domain with the ´rubrik_sla_domain_assignment´ resource, using the application ID as the object ID.

-> **Note:** Changing ´target_cloud_account_id´ doesn't make recovery to the new account possible straight away. It
   becomes possible once the snapshots of the workloads have been replicated to it, after a full snapshot cycle.

~> **Warning:** RSC refuses to delete an application while it has config snapshots. Set
   ´delete_snapshots_on_destroy´ to ´true´ and apply before destroying the application, or wait for the snapshots to
   expire. The ´CLOUD_NATIVE_CONFIG_PROTECTION´ feature can't be removed from an AWS account which is the source or
   the target of an application.
`

// awsCloudNativeApplicationNameRegexp matches the application names accepted
// by RSC.
var awsCloudNativeApplicationNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// defaultAWSCloudNativeApplicationDeleteTimeout is the default time to wait
// for the config snapshots of an application to be deleted.
const defaultAWSCloudNativeApplicationDeleteTimeout = 20 * time.Minute

// awsCloudNativeApplicationServices maps the service names of the services
// attribute to the RSC service flags.
var awsCloudNativeApplicationServices = map[string]func(*gqlcloudapp.AWSServices) *bool{
	"EC2":             func(s *gqlcloudapp.AWSServices) *bool { return &s.EC2 },
	"ECR":             func(s *gqlcloudapp.AWSServices) *bool { return &s.ECR },
	"ECS":             func(s *gqlcloudapp.AWSServices) *bool { return &s.ECS },
	"EKS":             func(s *gqlcloudapp.AWSServices) *bool { return &s.EKS },
	"ELASTICACHE":     func(s *gqlcloudapp.AWSServices) *bool { return &s.ElastiCache },
	"ELB":             func(s *gqlcloudapp.AWSServices) *bool { return &s.ELB },
	"LAMBDA":          func(s *gqlcloudapp.AWSServices) *bool { return &s.Lambda },
	"RDS":             func(s *gqlcloudapp.AWSServices) *bool { return &s.RDS },
	"S3":              func(s *gqlcloudapp.AWSServices) *bool { return &s.S3 },
	"SECRETS_MANAGER": func(s *gqlcloudapp.AWSServices) *bool { return &s.Secrets },
}

var (
	_ resource.Resource                   = &awsCloudNativeApplicationResource{}
	_ resource.ResourceWithConfigure      = &awsCloudNativeApplicationResource{}
	_ resource.ResourceWithIdentity       = &awsCloudNativeApplicationResource{}
	_ resource.ResourceWithImportState    = &awsCloudNativeApplicationResource{}
	_ resource.ResourceWithModifyPlan     = &awsCloudNativeApplicationResource{}
	_ resource.ResourceWithValidateConfig = &awsCloudNativeApplicationResource{}
)

type awsCloudNativeApplicationResource struct {
	client *client
}

// awsCloudNativeApplicationModel is the resource state. Field order follows
// the schema.
type awsCloudNativeApplicationModel struct {
	ID                       types.String   `tfsdk:"id"`
	Name                     types.String   `tfsdk:"name"`
	CloudAccountID           types.String   `tfsdk:"cloud_account_id"`
	Region                   types.String   `tfsdk:"region"`
	TargetCloudAccountID     types.String   `tfsdk:"target_cloud_account_id"`
	MatchingTags             types.Map      `tfsdk:"matching_tags"`
	ResourceARNs             types.Set      `tfsdk:"resource_arns"`
	ExcludedResourceARNs     types.Set      `tfsdk:"excluded_resource_arns"`
	Services                 types.Set      `tfsdk:"services"`
	DeleteSnapshotsOnDestroy types.Bool     `tfsdk:"delete_snapshots_on_destroy"`
	RefreshStatus            types.String   `tfsdk:"refresh_status"`
	ProtectionStatus         types.String   `tfsdk:"protection_status"`
	Timeouts                 timeouts.Value `tfsdk:"timeouts"`
}

// awsCloudNativeApplicationIdentityModel is the resource identity, shared with
// the list resource.
type awsCloudNativeApplicationIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAWSCloudNativeApplicationResource() resource.Resource {
	return &awsCloudNativeApplicationResource{}
}

func (r *awsCloudNativeApplicationResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyAWSCloudNativeApplication
}

func (r *awsCloudNativeApplicationResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.Schema")

	services := slices.Sorted(func(yield func(string) bool) {
		for name := range awsCloudNativeApplicationServices {
			if !yield(name) {
				return
			}
		}
	})
	serviceValues := make([]attr.Value, 0, len(services))
	for _, name := range services {
		serviceValues = append(serviceValues, types.StringValue(name))
	}

	res.Schema = schema.Schema{
		Description: description(resourceAWSCloudNativeApplicationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Application ID (UUID). Also the object ID when assigning an SLA Domain to the application.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required: true,
				Description: "Application name. Must be unique and consist of letters, digits, `-` and `_`, at most " +
					"1024 characters.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 1024),
					stringvalidator.RegexMatches(awsCloudNativeApplicationNameRegexp,
						"must consist of letters, digits, `-` and `_`"),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Required: true,
				Description: "RSC cloud account ID (UUID) of the AWS account the application lives in. The account " +
					"must be onboarded with the `CLOUD_NATIVE_CONFIG_PROTECTION` feature. Changing this forces a new " +
					"resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyRegion: schema.StringAttribute{
				Required:    true,
				Description: "AWS region of the application. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(aws.AllRegionNames()...),
				},
			},
			keyTargetCloudAccountID: schema.StringAttribute{
				Required: true,
				Description: "RSC cloud account ID (UUID) of the AWS account the application is recovered to. Must " +
					"differ from `cloud_account_id`.",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyMatchingTags: schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "AWS tags, as key value pairs. A resource carrying any one of the tags joins the " +
					"application. At most 3 tags. Once set, the tags can be changed but not all removed. At least one " +
					"of `matching_tags` and `resource_arns` must be specified.",
				Validators: []validator.Map{
					mapvalidator.SizeBetween(1, 3),
					mapvalidator.KeysAre(isNotWhiteSpace()),
				},
			},
			keyResourceARNs: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "ARNs of resources explicitly included in the application. Each resource must have been " +
					"discovered by RSC in the account and region, and its service must be enabled. Tagged resources, " +
					"and resources related to the listed ones, are included by RSC and must not be listed.",
				Validators: []validator.Set{
					setvalidator.SizeAtMost(10000),
					setvalidator.ValueStringsAre(isNotWhiteSpace()),
				},
			},
			keyExcludedResourceARNs: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "ARNs of resources excluded from the application, even when tagged or related to an " +
					"included resource. A resource tightly coupled to an included one can't be excluded.",
				Validators: []validator.Set{
					setvalidator.SizeAtMost(10000),
					setvalidator.ValueStringsAre(isNotWhiteSpace()),
				},
			},
			keyServices: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "AWS services whose resources are protected. Possible values are `" +
					strings.Join(services, "`, `") + "`. Default is all of them. VPCs, IAM roles and Route 53 hosted " +
					"zones are always included.",
				Default: setdefault.StaticValue(types.SetValueMust(types.StringType, serviceValues)),
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(services...)),
				},
			},
			keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Delete the config snapshots of the application when the resource is destroyed. RSC " +
					"refuses to delete an application with config snapshots. When `true`, the application is set to Do " +
					"Not Protect with its snapshots expiring immediately, and destroy waits for the snapshots to be " +
					"gone. The snapshots of the workloads in the application are not affected. Default is `false`.",
			},
			// RSC changes this on its own, so it isn't kept from the state on
			// update: Update reads the current value back.
			keyRefreshStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Status of the last refresh of the application resources.",
			},
			// RSC changes this on its own, so it isn't kept from the state on
			// update: Update reads the current value back.
			keyProtectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Protection status of the application.",
			},
			keyTimeouts: timeouts.Attributes(ctx, timeouts.Opts{
				Delete: true,
				DeleteDescription: fmt.Sprintf("How long to wait for the config snapshots to be deleted when "+
					"`delete_snapshots_on_destroy` is `true`. Default is `%s`.",
					defaultAWSCloudNativeApplicationDeleteTimeout),
			}),
		},
	}
}

func (r *awsCloudNativeApplicationResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Application ID (UUID).",
			},
		},
	}
}

func (r *awsCloudNativeApplicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsCloudNativeApplicationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.ValidateConfig")

	var config awsCloudNativeApplicationModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(validateAWSCloudNativeApplication(config)...)
}

// validateAWSCloudNativeApplication checks the plan-time rules of the
// resource: at least one matching tag or resource ARN, and a target account
// which differs from the source account. The RSC UI enforces the second rule,
// RSC itself doesn't. Unknown values are skipped.
func validateAWSCloudNativeApplication(config awsCloudNativeApplicationModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if !config.MatchingTags.IsUnknown() && !config.ResourceARNs.IsUnknown() &&
		len(config.MatchingTags.Elements()) == 0 && len(config.ResourceARNs.Elements()) == 0 {
		diags.AddAttributeError(path.Root(keyMatchingTags), "Missing resource selection",
			fmt.Sprintf("At least one of %q and %q must be specified.", keyMatchingTags, keyResourceARNs))
	}

	if !config.CloudAccountID.IsUnknown() && !config.TargetCloudAccountID.IsUnknown() &&
		!config.CloudAccountID.IsNull() && config.CloudAccountID.Equal(config.TargetCloudAccountID) {
		diags.AddAttributeError(path.Root(keyTargetCloudAccountID), "Invalid target cloud account",
			fmt.Sprintf("%q must differ from %q.", keyTargetCloudAccountID, keyCloudAccountID))
	}

	return diags
}

func (r *awsCloudNativeApplicationResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.ModifyPlan")

	// Nothing to check on create and destroy.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state, plan awsCloudNativeApplicationModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// RSC ignores an empty list of matching tags on update, so removing all the
	// tags would never take effect.
	if len(state.MatchingTags.Elements()) > 0 && !plan.MatchingTags.IsUnknown() &&
		len(plan.MatchingTags.Elements()) == 0 {
		res.Diagnostics.AddAttributeError(path.Root(keyMatchingTags), "Matching tags can't be removed",
			"RSC can't remove all the matching tags of an application. Keep at least one tag, or replace the "+
				"application.")
	}
}

func (r *awsCloudNativeApplicationResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.Create")

	var plan awsCloudNativeApplicationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloudAccountID, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud account ID", err.Error())
		return
	}
	targetCloudAccountID, err := uuid.Parse(plan.TargetCloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid target cloud account ID", err.Error())
		return
	}
	region := aws.RegionFromAny(plan.Region.ValueString())
	if region == aws.RegionUnknown {
		res.Diagnostics.AddError("Invalid region", fmt.Sprintf("Unknown AWS region: %q", plan.Region.ValueString()))
		return
	}

	params := gqlcloudapp.CreateAWSApplicationParams{
		CloudAccountID:       cloudAccountID,
		TargetCloudAccountID: targetCloudAccountID,
		Name:                 plan.Name.ValueString(),
		Region:               region.ToRegionEnum(),
	}
	var services gqlcloudapp.AWSServices
	res.Diagnostics.Append(plan.toAWSCloudNativeApplicationParams(ctx, &params.MatchingTags, &params.ResourceARNs,
		&params.ExcludedResourceARNs, &services)...)
	if res.Diagnostics.HasError() {
		return
	}
	params.ServiceConfig = gqlcloudapp.NewServiceConfig(services)

	api := cloudapp.Wrap(polarisClient)
	id, err := api.CreateAWSApplication(ctx, params)
	if err != nil {
		res.Diagnostics.AddError("Failed to create cloud native application", err.Error())
		return
	}

	// Persist the ID and identity as soon as the application exists, before
	// anything else in Create can fail.
	plan.ID = types.StringValue(id.String())
	plan.RefreshStatus = types.StringNull()
	plan.ProtectionStatus = types.StringNull()
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, awsCloudNativeApplicationIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	app, err := api.ApplicationByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read cloud native application after create",
			fmt.Sprintf("The cloud native application was created but could not be read back: %s", err.Error()))
		return
	}
	plan.RefreshStatus = types.StringValue(string(app.RefreshStatus))
	plan.ProtectionStatus = types.StringValue(string(app.ProtectionStatus))
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *awsCloudNativeApplicationResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.Read")

	var state awsCloudNativeApplicationModel
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
		res.Diagnostics.AddError("Invalid cloud native application ID", err.Error())
		return
	}

	app, err := cloudapp.Wrap(polarisClient).ApplicationByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read cloud native application", err.Error())
		return
	}

	res.Diagnostics.Append(state.fromAWSCloudNativeApplication(app)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, awsCloudNativeApplicationIdentityModel{ID: state.ID})...)
}

func (r *awsCloudNativeApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.Update")

	var plan, state awsCloudNativeApplicationModel
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

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud native application ID", err.Error())
		return
	}
	targetCloudAccountID, err := uuid.Parse(plan.TargetCloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid target cloud account ID", err.Error())
		return
	}

	api := cloudapp.Wrap(polarisClient)
	if !plan.Name.Equal(state.Name) {
		status, err := api.ValidateApplicationName(ctx, plan.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to validate application name", err.Error())
			return
		}
		if status != gqlcloudapp.NameValid {
			res.Diagnostics.AddAttributeError(path.Root(keyName), "Invalid application name",
				fmt.Sprintf("RSC rejected the application name %q: %s.", plan.Name.ValueString(), status))
			return
		}
	}

	params := gqlcloudapp.UpdateAWSApplicationParams{
		Name:                 plan.Name.ValueString(),
		TargetCloudAccountID: targetCloudAccountID,
	}
	var services gqlcloudapp.AWSServices
	res.Diagnostics.Append(plan.toAWSCloudNativeApplicationParams(ctx, &params.MatchingTags, &params.ResourceARNs,
		&params.ExcludedResourceARNs, &services)...)
	if res.Diagnostics.HasError() {
		return
	}
	params.ServiceConfig = gqlcloudapp.NewServiceConfig(services)
	if err := api.UpdateAWSApplication(ctx, id, params); err != nil {
		res.Diagnostics.AddError("Failed to update cloud native application", err.Error())
		return
	}

	plan.ID = state.ID
	plan.RefreshStatus = types.StringNull()
	plan.ProtectionStatus = types.StringNull()
	app, err := api.ApplicationByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read cloud native application after update",
			fmt.Sprintf("The cloud native application was updated but could not be read back: %s", err.Error()))
	} else {
		plan.RefreshStatus = types.StringValue(string(app.RefreshStatus))
		plan.ProtectionStatus = types.StringValue(string(app.ProtectionStatus))
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, awsCloudNativeApplicationIdentityModel{ID: plan.ID})...)
}

func (r *awsCloudNativeApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.Delete")

	var state awsCloudNativeApplicationModel
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
		res.Diagnostics.AddError("Invalid cloud native application ID", err.Error())
		return
	}

	timeout, diags := state.Timeouts.Delete(ctx, defaultAWSCloudNativeApplicationDeleteTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	deleteSnapshots := state.DeleteSnapshotsOnDestroy.ValueBool()
	err = cloudapp.Wrap(polarisClient).RemoveApplication(ctx, id, deleteSnapshots)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		detail := err.Error()
		if !deleteSnapshots && strings.Contains(detail, "config snapshot exists") {
			detail += fmt.Sprintf("\n\nSet %q to true and apply before destroying the application, or wait for "+
				"the snapshots to expire.", keyDeleteSnapshotsOnDestroy)
		}
		res.Diagnostics.AddError("Failed to delete cloud native application", detail)
		return
	}
}

func (r *awsCloudNativeApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyDeleteSnapshotsOnDestroy), false)...)
}

// toAWSCloudNativeApplicationParams converts the resource selection and the
// services of the model to their SDK form. Shared by Create and Update, and
// unit tested on its own.
func (m awsCloudNativeApplicationModel) toAWSCloudNativeApplicationParams(ctx context.Context, tags *[]core.Tag, arns, excludedARNs *[]string, services *gqlcloudapp.AWSServices) diag.Diagnostics {
	var diags diag.Diagnostics

	var tagMap map[string]string
	diags.Append(m.MatchingTags.ElementsAs(ctx, &tagMap, false)...)
	for _, key := range slices.Sorted(func(yield func(string) bool) {
		for key := range tagMap {
			if !yield(key) {
				return
			}
		}
	}) {
		*tags = append(*tags, core.Tag{Key: key, Value: tagMap[key]})
	}

	diags.Append(m.ResourceARNs.ElementsAs(ctx, arns, false)...)
	diags.Append(m.ExcludedResourceARNs.ElementsAs(ctx, excludedARNs, false)...)

	var serviceNames []string
	diags.Append(m.Services.ElementsAs(ctx, &serviceNames, false)...)
	for _, name := range serviceNames {
		if flag, ok := awsCloudNativeApplicationServices[name]; ok {
			*flag(services) = true
		}
	}

	return diags
}

// fromAWSCloudNativeApplication updates the model from the application read
// from RSC. Shared by Read and the list resource, and unit tested on its own.
// Optional collections RSC returns empty are kept null when the model holds
// null, so an unset attribute doesn't show drift.
func (m *awsCloudNativeApplicationModel) fromAWSCloudNativeApplication(app gqlcloudapp.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.ID.String())
	m.Name = types.StringValue(app.Name)
	m.CloudAccountID = types.StringValue(app.CloudAccountID.String())
	m.Region = types.StringValue(app.Region.Region.AWSRegion.Region.Name())
	m.TargetCloudAccountID = types.StringValue(app.TargetCloudAccountID.String())

	if len(app.MatchingTags.Tags) > 0 || !m.MatchingTags.IsNull() {
		tags := make(map[string]attr.Value, len(app.MatchingTags.Tags))
		for _, tag := range app.MatchingTags.Tags {
			tags[tag.Key] = types.StringValue(tag.Value)
		}
		var d diag.Diagnostics
		m.MatchingTags, d = types.MapValue(types.StringType, tags)
		diags.Append(d...)
	}

	for _, field := range []struct {
		value *types.Set
		arns  []string
	}{{&m.ResourceARNs, app.ResourceARNs}, {&m.ExcludedResourceARNs, app.ExcludedResourceARNs}} {
		if len(field.arns) == 0 && field.value.IsNull() {
			continue
		}
		values := make([]attr.Value, 0, len(field.arns))
		for _, arn := range field.arns {
			values = append(values, types.StringValue(arn))
		}
		var d diag.Diagnostics
		*field.value, d = types.SetValue(types.StringType, values)
		diags.Append(d...)
	}

	services := make([]attr.Value, 0, len(awsCloudNativeApplicationServices))
	for name, flag := range awsCloudNativeApplicationServices {
		if *flag(&app.ServiceConfig.CloudConfig.AWS) {
			services = append(services, types.StringValue(name))
		}
	}
	var d diag.Diagnostics
	m.Services, d = types.SetValue(types.StringType, services)
	diags.Append(d...)

	m.RefreshStatus = types.StringValue(string(app.RefreshStatus))
	m.ProtectionStatus = types.StringValue(string(app.ProtectionStatus))

	if m.DeleteSnapshotsOnDestroy.IsNull() {
		m.DeleteSnapshotsOnDestroy = types.BoolValue(false)
	}

	return diags
}
