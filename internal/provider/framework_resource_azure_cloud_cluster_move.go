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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	gqlcloudcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cloudcluster"
)

// azureCloudClusterMoveSourceModel is the state of the polaris_azure_cloud_cluster
// resource. It differs from the model of the rubrik_azure_cloud_cluster resource
// in the timeouts block, which also holds the dropped default timeout.
type azureCloudClusterMoveSourceModel struct {
	ID             types.String              `tfsdk:"id"`
	CloudAccountID types.String              `tfsdk:"cloud_account_id"`
	AZResilient    types.Bool                `tfsdk:"az_resilient"`
	ClusterConfig  []azureClusterConfigModel `tfsdk:"cluster_config"`
	VMConfig       []azureVMConfigModel      `tfsdk:"vm_config"`
	Timeouts       types.Object              `tfsdk:"timeouts"`
}

func (r *azureCloudClusterResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azureCloudClusterResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
	}
}

// moveStateV0 moves v0 state from the polaris_azure_cloud_cluster resource in
// the rubrikinc/polaris provider to the rubrik_azure_cloud_cluster resource.
// The SourceSchema is frozen at the SDKv2 schema, which has always been version
// 0. State created by older releases can lack attributes added later, e.g.
// timezone, location, az_resilient and subnet_az_config, these decode as null.
func (r *azureCloudClusterResource) moveStateV0() resource.StateMover {
	stringRequired := schema.StringAttribute{Required: true}

	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID:             schema.StringAttribute{Computed: true},
				keyCloudAccountID: stringRequired,
				keyAzResilient:    schema.BoolAttribute{Optional: true, Computed: true},
			},
			Blocks: map[string]schema.Block{
				keyClusterConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{Attributes: cloudClusterMoveSourceCommonConfigAttributes()},
				},
				keyVMConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyCDMVersion:                      stringRequired,
							keyCDMProduct:                      schema.StringAttribute{Computed: true},
							keyInstanceType:                    stringRequired,
							keyResourceGroupName:               stringRequired,
							keyStorageAccountName:              stringRequired,
							keyContainerName:                   stringRequired,
							keyEnableImmutability:              schema.BoolAttribute{Required: true},
							keyUserAssignedManagedIdentityName: stringRequired,
							keyRegion:                          stringRequired,
							keyNetworkResourceGroup:            stringRequired,
							keyVnetResourceGroup:               stringRequired,
							keySubnet:                          schema.StringAttribute{Optional: true},
							keyVnet:                            stringRequired,
							keyNetworkSecurityGroup:            stringRequired,
							keyNetworkSecurityResourceGroup:    stringRequired,
							keyVMType:                          schema.StringAttribute{Optional: true},
							keyAvailabilityZone:                schema.StringAttribute{Optional: true},
						},
						Blocks: map[string]schema.Block{
							keySubnetAzConfigs: cloudClusterMoveSourceSubnetAzBlock(),
						},
					},
				},
				keyTimeouts: cloudClusterMoveSourceTimeoutsBlock(),
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureCloudClusterResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzureCloudCluster {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var source azureCloudClusterMoveSourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &source)...)
			if res.Diagnostics.HasError() {
				return
			}

			target := azureCloudClusterModel{
				ID:             source.ID,
				CloudAccountID: source.CloudAccountID,
				AZResilient:    source.AZResilient,
				ClusterConfig:  source.ClusterConfig,
				VMConfig:       source.VMConfig,
				Timeouts:       cloudClusterMoveTimeouts(source.Timeouts),
			}
			azureCloudClusterNormalizeMovedState(&target)

			res.Diagnostics.Append(res.TargetState.Set(ctx, &target)...)
		},
	}
}

// azureCloudClusterNormalizeMovedState converts the SDKv2 representation of
// unset values to the Framework representation: empty strings become null,
// unset booleans become false, an unset vm_type becomes the default, and the
// admin credentials, which old releases stored before they became write-only,
// are dropped.
func azureCloudClusterNormalizeMovedState(state *azureCloudClusterModel) {
	for i := range state.ClusterConfig {
		cc := &state.ClusterConfig[i]
		cc.AdminEmail = types.StringNull()
		cc.AdminPassword = types.StringNull()
		if cc.ForceClusterDeleteOnDestroy.IsNull() {
			cc.ForceClusterDeleteOnDestroy = types.BoolValue(false)
		}
	}

	for i := range state.VMConfig {
		vm := &state.VMConfig[i]
		if !vm.Subnet.IsNull() && vm.Subnet.ValueString() == "" {
			vm.Subnet = types.StringNull()
		}
		if !vm.AvailabilityZone.IsNull() && vm.AvailabilityZone.ValueString() == "" {
			vm.AvailabilityZone = types.StringNull()
		}
		if vm.VMType.IsNull() || vm.VMType.ValueString() == "" {
			vm.VMType = types.StringValue(string(gqlcloudcluster.CCVmConfigDense))
		}
	}
}
