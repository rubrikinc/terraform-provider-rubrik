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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *azureExocomputeResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "azureExocomputeResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
		1: r.upgradeStateV1(ctx),
	}
}

// upgradeStateV0 upgrades v0 state to v2.
func (r *azureExocomputeResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: azureExocomputeSchemaV0,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "azureExocomputeResource.upgradeStateV0")

			var prior azureExocomputeModelV0
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state := upgradeAzureExocomputeStateV0(prior)
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeStateV1 upgrades v1 state to v2.
func (r *azureExocomputeResource) upgradeStateV1(ctx context.Context) resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: azureExocomputeSchemaV1(ctx),
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "azureExocomputeResource.upgradeStateV1")

			var prior azureExocomputeModelV1
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state := upgradeAzureExocomputeStateV1(prior)
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeAzureExocomputeStateV0 upgrades v0 state to v2. The polaris_managed
// field is removed, since Exocompute on Azure only supports RSC managed
// configurations. The fields added in v1 are left unset, the next read fills
// in the values stored in RSC.
func upgradeAzureExocomputeStateV0(prior azureExocomputeModelV0) azureExocomputeModel {
	return upgradeAzureExocomputeStateV1(azureExocomputeModelV1{
		ID:                    prior.ID,
		CloudAccountID:        types.StringNull(),
		HostCloudAccountID:    types.StringNull(),
		PodOverlayNetworkCIDR: types.StringNull(),
		Region:                prior.Region,
		Subnet:                prior.Subnet,
		SubscriptionID:        prior.SubscriptionID,
		TriggerHealthCheck:    types.BoolNull(),
		Timeouts: timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{
			"create": types.StringType,
			"update": types.StringType,
		})},
	})
}

// upgradeAzureExocomputeStateV1 upgrades v1 state to v2. The resource stored
// unset strings as empty strings. They are stored as null, matching a
// configuration that leaves them out, so the first plan doesn't show them
// changing.
func upgradeAzureExocomputeStateV1(prior azureExocomputeModelV1) azureExocomputeModel {
	nullEmpty := func(s types.String) types.String {
		if s.ValueString() == "" {
			return types.StringNull()
		}
		return s
	}

	// An unset block is an empty list, not null.
	optionalConfig := []azureExocomputeOptionalConfigModel{}
	for _, block := range prior.OptionalConfig {
		optionalConfig = append(optionalConfig, azureExocomputeOptionalConfigModel{
			AllowlistAdditionalIPs:         block.AllowlistAdditionalIPs,
			AllowlistRubrikIPs:             block.AllowlistRubrikIPs,
			ClusterAccess:                  block.ClusterAccess,
			ClusterTier:                    block.ClusterTier,
			DiskEncryptionAtHost:           block.DiskEncryptionAtHost,
			DiskEncryptionSetID:            block.DiskEncryptionSetID,
			HealthCheckVMNamePrefix:        block.HealthCheckVMNamePrefix,
			MaxNodeCount:                   block.MaxNodeCount,
			PostgresPrivateDNSZoneID:       block.PostgresPrivateDNSZoneID,
			PostgresSubnetID:               block.PostgresSubnetID,
			PrivateExocomputeDNSZoneID:     nullEmpty(block.PrivateExocomputeDNSZoneID),
			ResourceGroupPrefix:            nullEmpty(block.ResourceGroupPrefix),
			SnapshotPrivateAccessDNSZoneID: nullEmpty(block.SnapshotPrivateAccessDNSZoneID),
			SQLPrivateDNSZoneID:            block.SQLPrivateDNSZoneID,
			UserDefinedRouting:             block.UserDefinedRouting,
		})
	}

	return azureExocomputeModel{
		ID:                    prior.ID,
		CloudAccountID:        prior.CloudAccountID,
		HostCloudAccountID:    prior.HostCloudAccountID,
		OptionalConfig:        optionalConfig,
		PodOverlayNetworkCIDR: nullEmpty(prior.PodOverlayNetworkCIDR),
		Region:                prior.Region,
		Subnet:                nullEmpty(prior.Subnet),
		SubscriptionID:        prior.SubscriptionID,
		TriggerHealthCheck:    prior.TriggerHealthCheck,
		Timeouts:              prior.Timeouts,
	}
}

// azureExocomputeSchemaV0 is the schema of the resource in the v0 state.
var azureExocomputeSchemaV0 = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:             schema.StringAttribute{Computed: true},
		keyPolarisManaged: schema.BoolAttribute{Optional: true},
		keyRegion:         schema.StringAttribute{Required: true},
		keySubnet:         schema.StringAttribute{Required: true},
		keySubscriptionID: schema.StringAttribute{Required: true},
	},
}

// azureExocomputeModelV0 is the model of the v0 state.
type azureExocomputeModelV0 struct {
	ID             types.String `tfsdk:"id"`
	PolarisManaged types.Bool   `tfsdk:"polaris_managed"`
	Region         types.String `tfsdk:"region"`
	Subnet         types.String `tfsdk:"subnet"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
}

// azureExocomputeSchemaV1 returns the schema of the resource in the v1 state.
// The schema is frozen at the attribute set of the latest polaris release.
// State written by earlier polaris releases lacks the fields added since, and
// the Framework sets those to null when decoding the state.
func azureExocomputeSchemaV1(ctx context.Context) *schema.Schema {
	return &schema.Schema{
		Attributes: map[string]schema.Attribute{
			keyID:                    schema.StringAttribute{Computed: true},
			keyCloudAccountID:        schema.StringAttribute{Optional: true},
			keyHostCloudAccountID:    schema.StringAttribute{Optional: true},
			keyPodOverlayNetworkCIDR: schema.StringAttribute{Optional: true},
			keyRegion:                schema.StringAttribute{Optional: true},
			keySubnet:                schema.StringAttribute{Optional: true},
			keySubscriptionID:        schema.StringAttribute{Optional: true},
			keyTriggerHealthCheck:    schema.BoolAttribute{Optional: true},
		},
		Blocks: map[string]schema.Block{
			keyOptionalConfig: schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyAllowlistAdditionalIPs:         schema.SetAttribute{ElementType: types.StringType, Optional: true},
						keyAllowlistRubrikIPs:             schema.BoolAttribute{Optional: true},
						keyClusterAccess:                  schema.StringAttribute{Optional: true},
						keyClusterTier:                    schema.StringAttribute{Optional: true},
						keyDiskEncryptionAtHost:           schema.BoolAttribute{Optional: true},
						keyDiskEncryptionSetID:            schema.StringAttribute{Optional: true, Computed: true},
						keyHealthCheckVMNamePrefix:        schema.StringAttribute{Optional: true, Computed: true},
						keyMaxNodeCount:                   schema.StringAttribute{Optional: true},
						keyPostgresPrivateDNSZoneID:       schema.StringAttribute{Optional: true, Computed: true},
						keyPostgresSubnetID:               schema.StringAttribute{Optional: true, Computed: true},
						keyPrivateExocomputeDNSZoneID:     schema.StringAttribute{Optional: true},
						keyResourceGroupPrefix:            schema.StringAttribute{Optional: true},
						keySnapshotPrivateAccessDNSZoneID: schema.StringAttribute{Optional: true},
						keySQLPrivateDNSZoneID:            schema.StringAttribute{Optional: true, Computed: true},
						keyUserDefinedRouting:             schema.BoolAttribute{Optional: true},
					},
				},
			},
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

// azureExocomputeModelV1 is the model of the v1 state.
type azureExocomputeModelV1 struct {
	ID                    types.String                           `tfsdk:"id"`
	CloudAccountID        types.String                           `tfsdk:"cloud_account_id"`
	HostCloudAccountID    types.String                           `tfsdk:"host_cloud_account_id"`
	OptionalConfig        []azureExocomputeOptionalConfigModelV1 `tfsdk:"optional_config"`
	PodOverlayNetworkCIDR types.String                           `tfsdk:"pod_overlay_network_cidr"`
	Region                types.String                           `tfsdk:"region"`
	Subnet                types.String                           `tfsdk:"subnet"`
	SubscriptionID        types.String                           `tfsdk:"subscription_id"`
	TriggerHealthCheck    types.Bool                             `tfsdk:"trigger_health_check"`
	Timeouts              timeouts.Value                         `tfsdk:"timeouts"`
}

// azureExocomputeOptionalConfigModelV1 is the model of nested optional_config
// blocks in the v1 state.
type azureExocomputeOptionalConfigModelV1 struct {
	AllowlistAdditionalIPs         []string     `tfsdk:"allowlist_additional_ips"`
	AllowlistRubrikIPs             types.Bool   `tfsdk:"allowlist_rubrik_ips"`
	ClusterAccess                  types.String `tfsdk:"cluster_access"`
	ClusterTier                    types.String `tfsdk:"cluster_tier"`
	DiskEncryptionAtHost           types.Bool   `tfsdk:"disk_encryption_at_host"`
	DiskEncryptionSetID            types.String `tfsdk:"disk_encryption_set_id"`
	HealthCheckVMNamePrefix        types.String `tfsdk:"health_check_vm_name_prefix"`
	MaxNodeCount                   types.String `tfsdk:"max_node_count"`
	PostgresPrivateDNSZoneID       types.String `tfsdk:"postgres_private_dns_zone_id"`
	PostgresSubnetID               types.String `tfsdk:"postgres_subnet_id"`
	PrivateExocomputeDNSZoneID     types.String `tfsdk:"private_exocompute_dns_zone_id"`
	ResourceGroupPrefix            types.String `tfsdk:"resource_group_prefix"`
	SnapshotPrivateAccessDNSZoneID types.String `tfsdk:"snapshot_private_access_dns_zone_id"`
	SQLPrivateDNSZoneID            types.String `tfsdk:"sql_private_dns_zone_id"`
	UserDefinedRouting             types.Bool   `tfsdk:"user_defined_routing"`
}
