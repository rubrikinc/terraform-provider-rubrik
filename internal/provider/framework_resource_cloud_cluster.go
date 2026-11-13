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
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlcloudcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cloudcluster"
	gqlcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cluster"
)

// This file holds the code shared by the rubrik_aws_cloud_cluster and the
// rubrik_azure_cloud_cluster resources.

const (
	// defaultCloudClusterCreateTimeout is the default create timeout of the AWS
	// and Azure cloud cluster resources.
	defaultCloudClusterCreateTimeout = 60 * time.Minute

	// defaultCloudClusterReadTimeout is the default read timeout of the AWS and
	// Azure cloud cluster resources.
	defaultCloudClusterReadTimeout = 20 * time.Minute

	// cloudClusterImportedKey is the private state key which marks a resource
	// as imported. RSC can't return the create-time inputs of a cluster, so
	// until the first update the provider adopts the configured values for the
	// ones missing from the state instead of replacing the cluster.
	cloudClusterImportedKey = "imported"
)

// cloudClusterVMTypes are the VM types RSC accepts for AWS and Azure cloud
// clusters.
var cloudClusterVMTypes = []gqlcloudcluster.VmConfigType{
	gqlcloudcluster.CCVmConfigStandard,
	gqlcloudcluster.CCVmConfigDense,
	gqlcloudcluster.CCVmConfigExtraDense,
}

// cloudClusterCommonConfigModel holds the cluster_config attributes shared by
// the AWS and Azure cloud cluster resources. It's embedded by value in the
// cloud specific models.
type cloudClusterCommonConfigModel struct {
	ClusterName                 types.String `tfsdk:"cluster_name"`
	AdminEmail                  types.String `tfsdk:"admin_email"`
	AdminPassword               types.String `tfsdk:"admin_password"`
	NumNodes                    types.Int64  `tfsdk:"num_nodes"`
	DNSNameServers              types.Set    `tfsdk:"dns_name_servers"`
	DNSSearchDomains            types.Set    `tfsdk:"dns_search_domains"`
	NTPServers                  types.Set    `tfsdk:"ntp_servers"`
	KeepClusterOnFailure        types.Bool   `tfsdk:"keep_cluster_on_failure"`
	ForceClusterDeleteOnDestroy types.Bool   `tfsdk:"force_cluster_delete_on_destroy"`
	Timezone                    types.String `tfsdk:"timezone"`
	Location                    types.String `tfsdk:"location"`
}

// cloudClusterSubnetAzConfigModel is an element of the subnet_az_config block.
type cloudClusterSubnetAzConfigModel struct {
	AvailabilityZone types.String `tfsdk:"availability_zone"`
	Subnet           types.String `tfsdk:"subnet"`
}

// cloudClusterSubnetAzObjectType is the element type of the subnet_az_config
// block.
var cloudClusterSubnetAzObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyAvailabilityZone: types.StringType,
	keySubnet:           types.StringType,
}}

// cloudClusterCommonConfigAttributes returns the cluster_config attributes
// shared by the AWS and Azure cloud cluster resources. The caller adds the
// cloud specific attributes to the returned map.
func cloudClusterCommonConfigAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		keyClusterName: schema.StringAttribute{
			Required:    true,
			Description: "Unique name to assign to the cloud cluster.",
			Validators:  []validator.String{isNotWhiteSpace()},
		},
		keyAdminEmail: schema.StringAttribute{
			Required:    true,
			WriteOnly:   true,
			Description: "Email address for the cluster admin user. Changing this value will have no effect on the cluster.",
			Validators:  []validator.String{isEmailAddress()},
		},
		keyAdminPassword: schema.StringAttribute{
			Required:    true,
			Sensitive:   true,
			WriteOnly:   true,
			Description: "Password for the cluster admin user. Changing this value will have no effect on the cluster.",
			Validators:  []validator.String{isNotWhiteSpace()},
		},
		keyNumNodes: schema.Int64Attribute{
			Required:      true,
			Description:   "Number of nodes in the cluster. Must be 1 or at least 3, two nodes are not supported. Changing this forces a new resource to be created.",
			PlanModifiers: []planmodifier.Int64{cloudClusterRequiresReplaceInt64()},
			Validators:    []validator.Int64{isCloudClusterNodeCount()},
		},
		keyDNSNameServers: schema.SetAttribute{
			ElementType: types.StringType,
			Required:    true,
			Description: "DNS name servers for the cluster.",
			Validators:  []validator.Set{setvalidator.SizeAtLeast(1)},
		},
		keyDNSSearchDomains: schema.SetAttribute{
			ElementType:   types.StringType,
			Optional:      true,
			Computed:      true,
			Description:   "DNS search domains for the cluster.",
			PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			Validators:    []validator.Set{setvalidator.SizeAtLeast(1)},
		},
		keyNTPServers: schema.SetAttribute{
			ElementType: types.StringType,
			Required:    true,
			Description: "NTP servers for the cluster.",
			Validators:  []validator.Set{setvalidator.SizeAtLeast(1)},
		},
		keyKeepClusterOnFailure: schema.BoolAttribute{
			Required:      true,
			Description:   "Whether to keep the cluster on failure (can be useful for troubleshooting). Changing this forces a new resource to be created.",
			PlanModifiers: []planmodifier.Bool{cloudClusterRequiresReplaceBool()},
		},
		keyForceClusterDeleteOnDestroy: schema.BoolAttribute{
			Optional: true,
			Computed: true,
			Default:  booldefault.StaticBool(false),
			Description: "Whether to force delete the cluster on destroy. Only applies to disconnected clusters with blocking " +
				"conditions. The value must be applied before the resource is destroyed.",
		},
		keyTimezone: schema.StringAttribute{
			Optional:      true,
			Computed:      true,
			Description:   "Timezone for the cluster using IANA standard format e.g. America/Los_Angeles, Europe/Paris, etc.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			Validators:    []validator.String{isNotWhiteSpace()},
		},
		keyLocation: schema.StringAttribute{
			Optional:      true,
			Computed:      true,
			Description:   "Location for the cluster. This is free text, RSC will map it to the closest possible location e.g. Palo Alto, CA.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			Validators:    []validator.String{isNotWhiteSpace()},
		},
	}
}

// cloudClusterSubnetAzConfigBlock returns the subnet_az_config block of the
// vm_config block. The subnetDescription describes the subnet attribute.
func cloudClusterSubnetAzConfigBlock(subnetDescription string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Subnet and availability zone pairs for Multi-AZ deployments. Required when `az_resilient` is true. " +
			"Each block specifies a subnet and its availability zone. Changing this forces a new resource to be created.",
		PlanModifiers: []planmodifier.List{cloudClusterRequiresReplaceList()},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				keyAvailabilityZone: schema.StringAttribute{
					Required:    true,
					Description: "Availability zone name.",
					Validators:  []validator.String{isNotWhiteSpace()},
				},
				keySubnet: schema.StringAttribute{
					Required:    true,
					Description: subnetDescription,
					Validators:  []validator.String{isNotWhiteSpace()},
				},
			},
		},
	}
}

// cloudClusterIdentitySchema returns the identity schema of the cloud cluster
// resources.
func cloudClusterIdentitySchema() identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Cloud cluster ID (UUID).",
			},
		},
	}
}

// cloudClusterImportState imports a cloud cluster by its ID, given either as
// the import string or as the identity. RSC can't return the create-time inputs
// of a cluster, so the resource is marked as imported in the private state. The
// requires-replace plan modifiers then adopt the configured values for the
// attributes missing from the state, instead of replacing the cluster. The
// provider trusts the configuration to match the cluster.
func cloudClusterImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
	if res.Diagnostics.HasError() {
		return
	}

	var id types.String
	res.Diagnostics.Append(res.State.GetAttribute(ctx, path.Root(keyID), &id)...)
	if res.Diagnostics.HasError() {
		return
	}
	if _, err := uuid.Parse(id.ValueString()); err != nil {
		res.Diagnostics.AddError("Invalid cloud cluster ID", err.Error())
		return
	}

	res.Diagnostics.Append(res.Private.SetKey(ctx, cloudClusterImportedKey, []byte("true"))...)
}

// cloudClusterPrivateReader is implemented by the private state data passed to
// plan modifiers and CRUD methods.
type cloudClusterPrivateReader interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

// cloudClusterIsImported returns true if the resource is marked as imported in
// its private state.
func cloudClusterIsImported(ctx context.Context, private cloudClusterPrivateReader) bool {
	value, _ := private.GetKey(ctx, cloudClusterImportedKey)
	return len(value) > 0
}

// cloudClusterIsZero returns true if the value is null or the zero value of its
// type: "", false, 0, or a set or list without elements.
func cloudClusterIsZero(value attr.Value) bool {
	if value.IsNull() {
		return true
	}
	if value.IsUnknown() {
		return false
	}

	switch v := value.(type) {
	case types.String:
		return v.ValueString() == ""
	case types.Bool:
		return !v.ValueBool()
	case types.Int64:
		return v.ValueInt64() == 0
	case types.Set:
		return len(v.Elements()) == 0
	case types.List:
		return len(v.Elements()) == 0
	}
	return false
}

// cloudClusterRequiresReplace decides if a changed create-time attribute
// requires the cluster to be replaced. It's only called when the planned value
// differs from the state value. The cluster is not replaced when:
//   - the resource is imported and the state value is null or zero. RSC can't
//     return the value, so the configured value is adopted.
//   - both the state value and the planned value are null or zero. The SDKv2
//     resources stored "", false and empty sets where the Framework plans null.
//
// In all other cases, including an unknown planned value, the cluster is
// replaced.
func cloudClusterRequiresReplace(imported bool, stateValue, planValue attr.Value) bool {
	if imported && cloudClusterIsZero(stateValue) {
		return false
	}
	if cloudClusterIsZero(stateValue) && cloudClusterIsZero(planValue) {
		return false
	}
	return true
}

const cloudClusterRequiresReplaceDescription = "Requires replacement when changed, unless the value is adopted from the " +
	"configuration after an import or only differs from the state by being empty."

func cloudClusterRequiresReplaceString() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.StringRequest, res *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			res.RequiresReplace = cloudClusterRequiresReplace(cloudClusterIsImported(ctx, req.Private), req.StateValue, req.PlanValue)
		}, cloudClusterRequiresReplaceDescription, cloudClusterRequiresReplaceDescription)
}

func cloudClusterRequiresReplaceBool() planmodifier.Bool {
	return boolplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.BoolRequest, res *boolplanmodifier.RequiresReplaceIfFuncResponse) {
			res.RequiresReplace = cloudClusterRequiresReplace(cloudClusterIsImported(ctx, req.Private), req.StateValue, req.PlanValue)
		}, cloudClusterRequiresReplaceDescription, cloudClusterRequiresReplaceDescription)
}

func cloudClusterRequiresReplaceInt64() planmodifier.Int64 {
	return int64planmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.Int64Request, res *int64planmodifier.RequiresReplaceIfFuncResponse) {
			res.RequiresReplace = cloudClusterRequiresReplace(cloudClusterIsImported(ctx, req.Private), req.StateValue, req.PlanValue)
		}, cloudClusterRequiresReplaceDescription, cloudClusterRequiresReplaceDescription)
}

func cloudClusterRequiresReplaceSet() planmodifier.Set {
	return setplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.SetRequest, res *setplanmodifier.RequiresReplaceIfFuncResponse) {
			res.RequiresReplace = cloudClusterRequiresReplace(cloudClusterIsImported(ctx, req.Private), req.StateValue, req.PlanValue)
		}, cloudClusterRequiresReplaceDescription, cloudClusterRequiresReplaceDescription)
}

func cloudClusterRequiresReplaceList() planmodifier.List {
	return listplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.ListRequest, res *listplanmodifier.RequiresReplaceIfFuncResponse) {
			res.RequiresReplace = cloudClusterRequiresReplace(cloudClusterIsImported(ctx, req.Private), req.StateValue, req.PlanValue)
		}, cloudClusterRequiresReplaceDescription, cloudClusterRequiresReplaceDescription)
}

// cloudClusterReadNetwork reads the DNS servers, DNS search domains, NTP servers
// and cluster settings of the cluster into cc. The DNS and NTP servers are read
// from the cluster itself, so the reads fail for a disconnected cluster. Such
// failures are returned as warnings and the values already in cc are kept. A
// failure to read the cluster settings is an error. The search domains are
// always written as a set, also when empty.
func cloudClusterReadNetwork(ctx context.Context, gqlClusterAPI gqlcluster.API, id uuid.UUID, cc *cloudClusterCommonConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	settings, err := gqlClusterAPI.ClusterSettings(ctx, id)
	if err != nil {
		diags.AddError("Failed to read cluster settings", err.Error())
		return diags
	}
	cc.ClusterName = types.StringValue(settings.Name)
	cc.Timezone = types.StringValue(settings.Timezone)
	cc.Location = types.StringValue(settings.RawAddress)

	dnsServers, err := gqlClusterAPI.DNSServers(ctx, id)
	if err != nil {
		diags.AddWarning("Failed to read DNS servers", err.Error())
	} else {
		servers, searchDomains := dnsServers.Servers, dnsServers.Domains
		if servers == nil {
			servers = []string{}
		}
		if searchDomains == nil {
			searchDomains = []string{}
		}
		serverSet, d := types.SetValueFrom(ctx, types.StringType, servers)
		diags.Append(d...)
		searchSet, d := types.SetValueFrom(ctx, types.StringType, searchDomains)
		diags.Append(d...)
		if !diags.HasError() {
			cc.DNSNameServers = serverSet
			cc.DNSSearchDomains = searchSet
		}
	}

	ntpServers, err := gqlClusterAPI.NTPServers(ctx, id)
	if err != nil {
		diags.AddWarning("Failed to read NTP servers", err.Error())
	} else {
		servers := make([]string, 0, len(ntpServers))
		for _, s := range ntpServers {
			servers = append(servers, s.Server)
		}
		ntpSet, d := types.SetValueFrom(ctx, types.StringType, servers)
		diags.Append(d...)
		if !diags.HasError() {
			cc.NTPServers = ntpSet
		}
	}

	return diags
}

// cloudClusterUpdateNetwork applies the changes between the state and the plan
// to the cluster: the DNS servers and search domains (together), the NTP
// servers, and the cluster name, timezone and location (together). The updates
// are applied in that order, and the first error stops the update. The returned
// boolean is true when the cluster settings were updated. Computed values which
// are unknown in the plan are not changes.
func cloudClusterUpdateNetwork(ctx context.Context, gqlClusterAPI gqlcluster.API, id uuid.UUID, plan, state *cloudClusterCommonConfigModel) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	// An unknown plan value is a computed value which is not configured, the
	// value in the state is kept.
	searchDomains := plan.DNSSearchDomains
	if searchDomains.IsUnknown() {
		searchDomains = state.DNSSearchDomains
	}
	searchDomainsChanged := !searchDomains.Equal(state.DNSSearchDomains) &&
		!(cloudClusterIsZero(searchDomains) && cloudClusterIsZero(state.DNSSearchDomains))

	if !plan.DNSNameServers.Equal(state.DNSNameServers) || searchDomainsChanged {
		servers := decodeStringSetOrNil(ctx, plan.DNSNameServers, &diags)
		domains := decodeStringSetOrNil(ctx, searchDomains, &diags)
		if diags.HasError() {
			return false, diags
		}
		if servers == nil {
			servers = []string{}
		}
		if domains == nil {
			domains = []string{}
		}
		if err := gqlClusterAPI.UpdateDNSServersAndSearchDomains(ctx, gqlcluster.UpdateDNSServersAndSearchDomainsInput{
			ClusterID:     id,
			DNSServers:    servers,
			SearchDomains: domains,
		}); err != nil {
			diags.AddError("Failed to update DNS servers and search domains", err.Error())
			return false, diags
		}
	}

	if !plan.NTPServers.Equal(state.NTPServers) {
		servers := decodeStringSetOrNil(ctx, plan.NTPServers, &diags)
		if diags.HasError() {
			return false, diags
		}
		input := gqlcluster.UpdateClusterNTPServersInput{ClusterID: id}
		for _, server := range servers {
			input.Servers = append(input.Servers, struct {
				Server       string                      `json:"server"`
				SymmetricKey *gqlcluster.NTPSymmetricKey `json:"symmetricKey,omitempty"`
			}{Server: server})
		}
		if err := gqlClusterAPI.UpdateNTPServers(ctx, input); err != nil {
			diags.AddError("Failed to update NTP servers", err.Error())
			return false, diags
		}
	}

	// The settings API writes the name, the timezone and the location together,
	// so values which are not configured are passed back unchanged.
	timezone, location := plan.Timezone, plan.Location
	if timezone.IsUnknown() {
		timezone = state.Timezone
	}
	if location.IsUnknown() {
		location = state.Location
	}
	if plan.ClusterName.Equal(state.ClusterName) && timezone.Equal(state.Timezone) && location.Equal(state.Location) {
		return false, diags
	}
	diags.Append(updateClusterSettings(ctx, gqlClusterAPI, id, plan.ClusterName.ValueString(),
		timezone.ValueString(), location.ValueString())...)
	if diags.HasError() {
		return false, diags
	}

	return true, diags
}

// cloudClusterKeepPlanned sets the values in cc to the planned values, for the
// attributes which are known in the plan. Terraform requires the state saved
// by an apply to match the known planned values, but RSC can normalize what it
// returns, e.g. the location. The values read back are only used for the
// computed attributes which are unknown in the plan. Drift is picked up by the
// next Read.
func cloudClusterKeepPlanned(planned cloudClusterCommonConfigModel, cc *cloudClusterCommonConfigModel) {
	if !planned.ClusterName.IsUnknown() {
		cc.ClusterName = planned.ClusterName
	}
	if !planned.DNSNameServers.IsUnknown() {
		cc.DNSNameServers = planned.DNSNameServers
	}
	if !planned.DNSSearchDomains.IsUnknown() {
		cc.DNSSearchDomains = planned.DNSSearchDomains
	}
	if !planned.NTPServers.IsUnknown() {
		cc.NTPServers = planned.NTPServers
	}
	if !planned.Timezone.IsUnknown() {
		cc.Timezone = planned.Timezone
	}
	if !planned.Location.IsUnknown() {
		cc.Location = planned.Location
	}
}

// cloudClusterFinishCreate completes the creation of a cluster. It reads the
// cluster back and applies the configured timezone and location, since the
// create API can't set them. The cc model must hold the planned values. A
// failure is returned as a warning, since the cluster exists and the state
// must be saved. The next Read shows the drift, and the next apply corrects it.
// Any computed value still unknown, e.g. due to a failed read-back, is set to
// null.
func cloudClusterFinishCreate(ctx context.Context, gqlClusterAPI gqlcluster.API, id uuid.UUID, cc *cloudClusterCommonConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	planned := *cc
	diags.Append(cloudClusterDowngrade(ctx, cloudClusterReadNetwork(ctx, gqlClusterAPI, id, cc))...)
	cloudClusterKeepPlanned(planned, cc)

	// The settings API writes the name, the timezone and the location together,
	// so a value which is not configured is passed back unchanged from the
	// read-back. If the read-back failed there is no value to pass back.
	if !planned.Timezone.IsUnknown() || !planned.Location.IsUnknown() {
		timezone, location := cc.Timezone, cc.Location
		if timezone.IsUnknown() || location.IsUnknown() {
			diags.AddWarning("Failed to apply timezone and location",
				"The cluster settings could not be read back, so the timezone and location were not applied.")
		} else {
			diags.Append(cloudClusterDowngrade(ctx, updateClusterSettings(ctx, gqlClusterAPI, id,
				cc.ClusterName.ValueString(), timezone.ValueString(), location.ValueString()))...)
		}
	}

	if cc.Timezone.IsUnknown() {
		cc.Timezone = types.StringNull()
	}
	if cc.Location.IsUnknown() {
		cc.Location = types.StringNull()
	}
	if cc.DNSSearchDomains.IsUnknown() {
		cc.DNSSearchDomains = types.SetNull(types.StringType)
	}

	return diags
}

// cloudClusterDowngrade returns the diagnostics with every error turned into a
// warning, for failures which must not fail the operation.
func cloudClusterDowngrade(ctx context.Context, diags diag.Diagnostics) diag.Diagnostics {
	var out diag.Diagnostics
	for _, d := range diags {
		if d.Severity() == diag.SeverityError {
			tflog.Warn(ctx, "cloud cluster operation failed after the cluster was created", map[string]any{
				"summary": d.Summary(), "detail": d.Detail(),
			})
			out.AddWarning(d.Summary(), d.Detail())
			continue
		}
		out = append(out, d)
	}
	return out
}

// cloudClusterDelete removes the cluster from RSC. A cluster which no longer
// exists is not an error. The blocking conditions of a failed removal are
// included in the error detail.
func cloudClusterDelete(ctx context.Context, polarisClient *polaris.Client, id uuid.UUID, force bool) diag.Diagnostics {
	var diags diag.Diagnostics

	info, err := cluster.Wrap(polarisClient).RemoveCluster(ctx, id, force, 0)
	if err != nil {
		if errors.Is(err, graphql.ErrNotFound) {
			return diags
		}
		diags.AddError("Failed to remove cloud cluster", fmt.Sprintf("%s\n\nBlocking conditions: %t. Force removal eligible: %t.",
			err, info.BlockingConditions, info.ForceRemovalEligible))
	}

	return diags
}

// cloudClusterCheckBlocks verifies that the single-instance cluster_config and
// vm_config blocks are present before the resources index them. The schema
// validators guarantee exactly one of each in the configuration, but a model
// decoded from state carries no such guarantee, so without this a state file
// modified outside of Terraform would panic the provider instead of reporting
// an error.
func cloudClusterCheckBlocks(clusterConfigs, vmConfigs int) diag.Diagnostics {
	var diags diag.Diagnostics

	if clusterConfigs == 0 {
		diags.AddAttributeError(path.Root(keyClusterConfig), "Missing cluster_config block",
			"Exactly one `cluster_config` block is required. If the configuration has one, the state may have been modified outside of Terraform.")
	}
	if vmConfigs == 0 {
		diags.AddAttributeError(path.Root(keyVMConfig), "Missing vm_config block",
			"Exactly one `vm_config` block is required. If the configuration has one, the state may have been modified outside of Terraform.")
	}

	return diags
}

// cloudClusterMoveSourceCommonConfigAttributes returns the cluster_config
// attributes shared by the AWS and Azure polaris cloud cluster resources, for
// use in the source schemas of the state movers. The caller adds the cloud
// specific attributes. Source schemas hold no descriptions or validators.
func cloudClusterMoveSourceCommonConfigAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		keyClusterName:                 schema.StringAttribute{Required: true},
		keyAdminEmail:                  schema.StringAttribute{Optional: true},
		keyAdminPassword:               schema.StringAttribute{Optional: true},
		keyNumNodes:                    schema.Int64Attribute{Required: true},
		keyDNSNameServers:              schema.SetAttribute{ElementType: types.StringType, Required: true},
		keyDNSSearchDomains:            schema.SetAttribute{ElementType: types.StringType, Optional: true},
		keyNTPServers:                  schema.SetAttribute{ElementType: types.StringType, Required: true},
		keyKeepClusterOnFailure:        schema.BoolAttribute{Required: true},
		keyForceClusterDeleteOnDestroy: schema.BoolAttribute{Optional: true},
		keyTimezone:                    schema.StringAttribute{Optional: true, Computed: true},
		keyLocation:                    schema.StringAttribute{Optional: true, Computed: true},
	}
}

// cloudClusterMoveSourceSubnetAzBlock returns the subnet_az_config block of the
// polaris cloud cluster resources, for use in the source schemas of the state
// movers.
func cloudClusterMoveSourceSubnetAzBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				keyAvailabilityZone: schema.StringAttribute{Required: true},
				keySubnet:           schema.StringAttribute{Required: true},
			},
		},
	}
}

// cloudClusterMoveSourceTimeoutsBlock returns the timeouts block of the polaris
// cloud cluster resources, for use in the source schemas of the state movers.
// The SDKv2 resources also accepted a default timeout.
func cloudClusterMoveSourceTimeoutsBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		Attributes: map[string]schema.Attribute{
			"create":  schema.StringAttribute{Optional: true},
			"read":    schema.StringAttribute{Optional: true},
			"default": schema.StringAttribute{Optional: true},
		},
	}
}

// cloudClusterMoveTimeouts converts the timeouts block of a polaris cloud
// cluster to the timeouts block of the Framework resources. The default
// timeout is dropped.
func cloudClusterMoveTimeouts(source types.Object) timeouts.Value {
	attrTypes := map[string]attr.Type{"create": types.StringType, "read": types.StringType}
	if source.IsNull() || source.IsUnknown() {
		return timeouts.Value{Object: types.ObjectNull(attrTypes)}
	}

	attrs := map[string]attr.Value{"create": types.StringNull(), "read": types.StringNull()}
	for name := range attrs {
		if v, ok := source.Attributes()[name]; ok {
			attrs[name] = v
		}
	}

	return timeouts.Value{Object: types.ObjectValueMust(attrTypes, attrs)}
}
