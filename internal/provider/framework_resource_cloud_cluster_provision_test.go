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
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlcloudcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cloudcluster"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

// TestAzureCloudClusterFromProvisionConfig verifies the decisions made when an
// imported Azure cloud cluster is filled in from the recorded create
// configuration: empty optional strings become null, the VM type is only set
// when recorded and the CDM version is never set.
func TestAzureCloudClusterFromProvisionConfig(t *testing.T) {
	newState := func() azureCloudClusterModel {
		return azureCloudClusterModel{
			CloudAccountID: types.StringNull(),
			ClusterConfig:  []azureClusterConfigModel{{}},
			VMConfig: []azureVMConfigModel{{
				CDMVersion:     types.StringNull(),
				VMType:         types.StringNull(),
				SubnetAzConfig: types.ListNull(cloudClusterSubnetAzObjectType),
			}},
		}
	}
	base := gqlcloudcluster.AzureProvisionConfig{
		CloudAccountID: uuid.MustParse("0a9b8c7d-6e5f-4a3b-8c2d-1e0f9a8b7c6d"),
		ClusterConfig:  gqlcloudcluster.ProvisionClusterConfig{NumNodes: 3},
		ImageVersion:   "3.7.31669",
		Region:         gqlazure.RegionCentralUS,
		InstanceType:   gqlcloudcluster.AzureInstanceTypeStandardD8SV5,
		Subnet:         "my-subnet",
	}

	t.Run("SingleAZWithVMType", func(t *testing.T) {
		config := base
		config.VMType, config.VMTypeKnown = gqlcloudcluster.CCVmConfigExtraDense, true
		state := newState()
		azureCloudClusterFromProvisionConfig(config, &state)

		vm := state.VMConfig[0]
		if !vm.CDMVersion.IsNull() {
			t.Errorf("cdm_version: got %v, want null", vm.CDMVersion)
		}
		if vm.CDMProduct.ValueString() != "3.7.31669" {
			t.Errorf("cdm_product: got %v", vm.CDMProduct)
		}
		if vm.VMType.ValueString() != "EXTRA_DENSE" {
			t.Errorf("vm_type: got %v", vm.VMType)
		}
		if vm.Region.ValueString() != "centralus" {
			t.Errorf("region: got %v", vm.Region)
		}
		if vm.Subnet.ValueString() != "my-subnet" || !vm.AvailabilityZone.IsNull() {
			t.Errorf("subnet/availability_zone: got %v/%v, want my-subnet/null", vm.Subnet, vm.AvailabilityZone)
		}
		if !vm.SubnetAzConfig.IsNull() {
			t.Errorf("subnet_az_config: got %v, want null", vm.SubnetAzConfig)
		}
		if state.CloudAccountID.ValueString() != base.CloudAccountID.String() || state.ClusterConfig[0].NumNodes.ValueInt64() != 3 {
			t.Errorf("cloud_account_id/num_nodes: got %v/%v", state.CloudAccountID, state.ClusterConfig[0].NumNodes)
		}
	})

	t.Run("MultiAZWithoutVMType", func(t *testing.T) {
		config := base
		config.Subnet = ""
		config.IsAzResilient = true
		config.SubnetAzConfigs = []gqlcloudcluster.SubnetAzConfig{{Subnet: "subnet-1", AvailabilityZone: "1"}, {Subnet: "subnet-2", AvailabilityZone: "2"}}
		state := newState()
		azureCloudClusterFromProvisionConfig(config, &state)

		vm := state.VMConfig[0]
		if !vm.VMType.IsNull() {
			t.Errorf("vm_type: got %v, want null when not recorded", vm.VMType)
		}
		if !vm.Subnet.IsNull() {
			t.Errorf("subnet: got %v, want null", vm.Subnet)
		}
		var subnetAz []cloudClusterSubnetAzConfigModel
		if diags := vm.SubnetAzConfig.ElementsAs(t.Context(), &subnetAz, false); diags.HasError() {
			t.Fatal(diags)
		}
		if len(subnetAz) != 2 || subnetAz[1].Subnet.ValueString() != "subnet-2" || subnetAz[1].AvailabilityZone.ValueString() != "2" {
			t.Errorf("subnet_az_config: got %v", subnetAz)
		}
		if !state.AZResilient.ValueBool() {
			t.Errorf("az_resilient: got %v, want true", state.AZResilient)
		}
	})
}

// TestAwsCloudClusterFromProvisionConfig verifies the same decisions for an
// imported AWS cloud cluster.
func TestAwsCloudClusterFromProvisionConfig(t *testing.T) {
	newState := func() awsCloudClusterModel {
		return awsCloudClusterModel{
			CloudAccountID: types.StringNull(),
			ClusterConfig:  []awsClusterConfigModel{{}},
			VMConfig: []awsVMConfigModel{{
				CDMVersion:     types.StringNull(),
				VMType:         types.StringNull(),
				SubnetAzConfig: types.ListNull(cloudClusterSubnetAzObjectType),
			}},
		}
	}
	config := gqlcloudcluster.AwsProvisionConfig{
		CloudAccountID: uuid.MustParse("0a9b8c7d-6e5f-4a3b-8c2d-1e0f9a8b7c6d"),
		Region:         gqlaws.RegionUsEast2,
		ClusterConfig:  gqlcloudcluster.ProvisionClusterConfig{NumNodes: 1, DynamicScalingEnabled: true},
		BucketName:     "my-bucket",
		CDMVersion:     "9.4.0-p2-30507",
		CDMProduct:     "rubrik-cdm-product-code",
		InstanceType:   gqlcloudcluster.AwsInstanceTypeM6I_2XLarge,
		VPC:            "vpc-0123456789abcdef0",
		SecurityGroups: []string{"sg-0123456789abcdef0"},
	}
	state := newState()
	awsCloudClusterFromProvisionConfig(config, &state)

	vm := state.VMConfig[0]
	if !vm.CDMVersion.IsNull() {
		t.Errorf("cdm_version: got %v, want null, the cluster may have been upgraded", vm.CDMVersion)
	}
	if !vm.VMType.IsNull() {
		t.Errorf("vm_type: got %v, want null when not recorded", vm.VMType)
	}
	if !vm.SubnetID.IsNull() {
		t.Errorf("subnet_id: got %v, want null for an empty subnet", vm.SubnetID)
	}
	if state.Region.ValueString() != "us-east-2" || vm.InstanceType.ValueString() != "M6I_2XLARGE" {
		t.Errorf("region/instance_type: got %v/%v", state.Region, vm.InstanceType)
	}
	if len(vm.SecurityGroupIDs.Elements()) != 1 {
		t.Errorf("security_group_ids: got %v", vm.SecurityGroupIDs)
	}
	if !state.ClusterConfig[0].DynamicScalingEnabled.ValueBool() || state.ClusterConfig[0].BucketName.ValueString() != "my-bucket" {
		t.Errorf("dynamic_scaling_enabled/bucket_name: got %v/%v", state.ClusterConfig[0].DynamicScalingEnabled, state.ClusterConfig[0].BucketName)
	}
}

// TestCloudClusterInstanceTypes verifies that instance_type accepts every
// instance type RSC offers for a CDM version. The lists are the instance types
// from RSC's per CDM version support lists, a type RSC rejects for the CDM
// version of a cluster is reported by RSC and not by the provider. A type
// missing from the provider is rejected at plan time, even when RSC offers it.
func TestCloudClusterInstanceTypes(t *testing.T) {
	offeredAws := []gqlcloudcluster.AwsCCInstanceType{
		// CDM 9.4.
		gqlcloudcluster.AwsInstanceTypeM6I_2XLarge,
		gqlcloudcluster.AwsInstanceTypeM6I_4XLarge,
		gqlcloudcluster.AwsInstanceTypeR6I_4XLarge,
		// CDM 9.5.
		gqlcloudcluster.AwsInstanceTypeM6A_2XLarge,
		gqlcloudcluster.AwsInstanceTypeM6A_4XLarge,
		gqlcloudcluster.AwsInstanceTypeR6A_4XLarge,
		gqlcloudcluster.AwsInstanceTypeM7I_2XLarge,
		gqlcloudcluster.AwsInstanceTypeM7I_4XLarge,
		gqlcloudcluster.AwsInstanceTypeR7I_4XLarge,
		// CDM 9.6.
		gqlcloudcluster.AwsInstanceTypeM7A_2XLarge,
		gqlcloudcluster.AwsInstanceTypeM7A_4XLarge,
		gqlcloudcluster.AwsInstanceTypeR7A_4XLarge,
	}
	for _, instanceType := range offeredAws {
		if !slices.Contains(awsCloudClusterInstanceTypes, instanceType) {
			t.Errorf("AWS instance type %q is offered by RSC but not accepted by instance_type", instanceType)
		}
	}

	offeredAzure := []gqlcloudcluster.AzureCCESSupportedInstanceType{
		// CDM 9.4.
		gqlcloudcluster.AzureInstanceTypeStandardD8SV5,
		gqlcloudcluster.AzureInstanceTypeStandardD16SV5,
		gqlcloudcluster.AzureInstanceTypeStandardE16SV5,
		// CDM 9.5.
		gqlcloudcluster.AzureInstanceTypeStandardD8ASV5,
		gqlcloudcluster.AzureInstanceTypeStandardD16ASV5,
		gqlcloudcluster.AzureInstanceTypeStandardE16ASV5,
		// CDM 9.5.1.
		gqlcloudcluster.AzureInstanceTypeStandardD8SV6,
		gqlcloudcluster.AzureInstanceTypeStandardD16SV6,
		gqlcloudcluster.AzureInstanceTypeStandardE16SV6,
		gqlcloudcluster.AzureInstanceTypeStandardD8ASV6,
		gqlcloudcluster.AzureInstanceTypeStandardD16ASV6,
		gqlcloudcluster.AzureInstanceTypeStandardE16ASV6,
	}
	for _, instanceType := range offeredAzure {
		if !slices.Contains(azureCloudClusterInstanceTypes, instanceType) {
			t.Errorf("Azure instance type %q is offered by RSC but not accepted by instance_type", instanceType)
		}
	}
}
