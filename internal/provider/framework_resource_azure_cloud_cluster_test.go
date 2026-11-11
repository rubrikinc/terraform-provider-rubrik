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
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestValidateAzureCloudClusterConfig(t *testing.T) {
	ctx := context.Background()

	azList, diags := types.ListValueFrom(ctx, cloudClusterSubnetAzObjectType, []cloudClusterSubnetAzConfigModel{
		{AvailabilityZone: types.StringValue("1"), Subnet: types.StringValue("subnet-a")},
		{AvailabilityZone: types.StringValue("2"), Subnet: types.StringValue("subnet-b")},
	})
	if diags.HasError() {
		t.Fatalf("failed to build subnet_az_config list: %v", diags)
	}
	nullList := types.ListNull(cloudClusterSubnetAzObjectType)
	unknownList := types.ListUnknown(cloudClusterSubnetAzObjectType)

	model := func(azResilient types.Bool, subnet, availabilityZone types.String, subnetAz types.List) azureCloudClusterModel {
		return azureCloudClusterModel{
			AZResilient: azResilient,
			VMConfig:    []azureVMConfigModel{{Subnet: subnet, AvailabilityZone: availabilityZone, SubnetAzConfig: subnetAz}},
		}
	}

	tests := []struct {
		name      string
		config    azureCloudClusterModel
		wantError bool
	}{
		{
			name:   "single-AZ with subnet is valid",
			config: model(types.BoolValue(false), types.StringValue("subnet-a"), types.StringNull(), nullList),
		},
		{
			name:   "single-AZ with subnet and availability zone is valid",
			config: model(types.BoolValue(false), types.StringValue("subnet-a"), types.StringValue("2"), nullList),
		},
		{
			name:   "null az_resilient is single-AZ",
			config: model(types.BoolNull(), types.StringValue("subnet-a"), types.StringNull(), nullList),
		},
		{
			name:      "null az_resilient without subnet is rejected",
			config:    model(types.BoolNull(), types.StringNull(), types.StringNull(), nullList),
			wantError: true,
		},
		{
			name:      "single-AZ with empty subnet is rejected",
			config:    model(types.BoolValue(false), types.StringValue(""), types.StringNull(), nullList),
			wantError: true,
		},
		{
			name:      "single-AZ with subnet_az_config is rejected",
			config:    model(types.BoolValue(false), types.StringValue("subnet-a"), types.StringNull(), azList),
			wantError: true,
		},
		{
			name:   "multi-AZ with az configs is valid",
			config: model(types.BoolValue(true), types.StringNull(), types.StringNull(), azList),
		},
		{
			name:      "multi-AZ without az configs is rejected",
			config:    model(types.BoolValue(true), types.StringNull(), types.StringNull(), nullList),
			wantError: true,
		},
		{
			name:      "multi-AZ with subnet is rejected",
			config:    model(types.BoolValue(true), types.StringValue("subnet-a"), types.StringNull(), azList),
			wantError: true,
		},
		{
			name:      "multi-AZ with availability zone is rejected",
			config:    model(types.BoolValue(true), types.StringNull(), types.StringValue("1"), azList),
			wantError: true,
		},
		{
			name:   "unknown az_resilient skips validation",
			config: model(types.BoolUnknown(), types.StringNull(), types.StringNull(), nullList),
		},
		{
			name:   "unknown subnet skips the subnet-required check",
			config: model(types.BoolValue(false), types.StringUnknown(), types.StringNull(), nullList),
		},
		{
			name:   "unknown subnet_az_config skips the either-or check",
			config: model(types.BoolValue(true), types.StringNull(), types.StringNull(), unknownList),
		},
		{
			name:   "unknown availability zone skips the multi-AZ check",
			config: model(types.BoolValue(true), types.StringNull(), types.StringUnknown(), azList),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateAzureCloudClusterConfig(tt.config)
			if got := diags.HasError(); got != tt.wantError {
				t.Errorf("validateAzureCloudClusterConfig() hasError = %v, want %v (diags: %v)", got, tt.wantError, diags)
			}
		})
	}
}

// TestAzureCloudClusterMoveState builds states shaped like the ones the SDKv2
// resource stored, runs them through the state mover and checks that the
// result decodes into the model.
func TestAzureCloudClusterMoveState(t *testing.T) {
	ctx := context.Background()

	r := &azureCloudClusterResource{prefix: keyRubrik}
	mover := r.moveStateV0()

	var schemaRes fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatalf("failed to build schema: %v", schemaRes.Diagnostics)
	}

	str := types.StringValue
	set := func(elements ...string) types.Set {
		v, diags := types.SetValueFrom(ctx, types.StringType, elements)
		if diags.HasError() {
			t.Fatal(diags)
		}
		return v
	}
	timeoutAttrs := map[string]attr.Type{"create": types.StringType, "read": types.StringType, "default": types.StringType}

	// A current SDKv2 state: empty strings for unset optional strings, an empty
	// subnet_az_config, a null az_resilient, a timeouts object with create,
	// read and default, and a non-null admin_email.
	current := azureCloudClusterMoveSourceModel{
		ID:             str("00000000-0000-0000-0000-000000000001"),
		CloudAccountID: str("00000000-0000-0000-0000-000000000002"),
		AZResilient:    types.BoolNull(),
		ClusterConfig: []azureClusterConfigModel{{
			cloudClusterCommonConfigModel: cloudClusterCommonConfigModel{
				ClusterName:                 str("cluster"),
				AdminEmail:                  str("admin@example.com"),
				AdminPassword:               types.StringNull(),
				NumNodes:                    types.Int64Value(3),
				DNSNameServers:              set("192.0.2.1"),
				DNSSearchDomains:            types.SetValueMust(types.StringType, nil),
				NTPServers:                  set("pool.ntp.org"),
				KeepClusterOnFailure:        types.BoolValue(false),
				ForceClusterDeleteOnDestroy: types.BoolNull(),
				Timezone:                    str("America/Los_Angeles"),
				Location:                    str("Palo Alto, CA"),
			},
		}},
		VMConfig: []azureVMConfigModel{{
			CDMVersion:                   str("9.4.0-p2-00000"),
			CDMProduct:                   str("product-00000"),
			InstanceType:                 str("STANDARD_D8S_V5"),
			ResourceGroupName:            str("rg"),
			StorageAccountName:           str("storageaccount"),
			ContainerName:                str("container"),
			EnableImmutability:           types.BoolValue(false),
			UserAssignedManagedIdentity:  str("identity"),
			Region:                       str("westus"),
			NetworkResourceGroup:         str("network-rg"),
			VnetResourceGroup:            str("vnet-rg"),
			Subnet:                       str(""),
			Vnet:                         str("vnet"),
			NetworkSecurityGroup:         str("nsg"),
			NetworkSecurityResourceGroup: str("nsg-rg"),
			VMType:                       str(""),
			AvailabilityZone:             str(""),
			SubnetAzConfig: types.ListValueMust(cloudClusterSubnetAzObjectType, []attr.Value{
				types.ObjectValueMust(cloudClusterSubnetAzObjectType.AttrTypes, map[string]attr.Value{
					keyAvailabilityZone: str("1"), keySubnet: str("subnet-a"),
				}),
			}),
		}},
		Timeouts: types.ObjectValueMust(timeoutAttrs, map[string]attr.Value{
			"create": str("90m"), "read": types.StringNull(), "default": str("20m"),
		}),
	}

	// A pre-v1.5.0 state without timezone, location and az_resilient.
	minimal := current
	minimal.ClusterConfig = []azureClusterConfigModel{current.ClusterConfig[0]}
	minimal.ClusterConfig[0].Timezone = types.StringNull()
	minimal.ClusterConfig[0].Location = types.StringNull()
	minimal.ClusterConfig[0].AdminEmail = types.StringNull()
	minimal.VMConfig = []azureVMConfigModel{current.VMConfig[0]}
	minimal.VMConfig[0].Subnet = str("subnet-a")
	minimal.VMConfig[0].AvailabilityZone = str("2")
	minimal.VMConfig[0].SubnetAzConfig = types.ListNull(cloudClusterSubnetAzObjectType)
	minimal.VMConfig[0].VMType = types.StringNull()
	minimal.Timeouts = types.ObjectNull(timeoutAttrs)

	tests := []struct {
		name   string
		source azureCloudClusterMoveSourceModel
		check  func(t *testing.T, got azureCloudClusterModel)
	}{
		{
			name:   "current SDKv2 state",
			source: current,
			check: func(t *testing.T, got azureCloudClusterModel) {
				if !got.AZResilient.IsNull() {
					t.Errorf("az_resilient = %v, want null", got.AZResilient)
				}
				cc := got.ClusterConfig[0]
				if !cc.AdminEmail.IsNull() {
					t.Errorf("admin_email = %v, want null", cc.AdminEmail)
				}
				if cc.ForceClusterDeleteOnDestroy.IsNull() || cc.ForceClusterDeleteOnDestroy.ValueBool() {
					t.Errorf("force_cluster_delete_on_destroy = %v, want false", cc.ForceClusterDeleteOnDestroy)
				}
				vm := got.VMConfig[0]
				if !vm.Subnet.IsNull() || !vm.AvailabilityZone.IsNull() {
					t.Errorf("subnet = %v and availability_zone = %v, want null", vm.Subnet, vm.AvailabilityZone)
				}
				if vm.VMType.ValueString() != "DENSE" {
					t.Errorf("vm_type = %v, want DENSE", vm.VMType)
				}
				if vm.CDMProduct.ValueString() != "product-00000" {
					t.Errorf("cdm_product = %v, want it kept", vm.CDMProduct)
				}
				if len(vm.SubnetAzConfig.Elements()) != 1 {
					t.Errorf("subnet_az_config has %d elements, want 1", len(vm.SubnetAzConfig.Elements()))
				}
				if v, ok := got.Timeouts.Attributes()["create"]; !ok || v.(types.String).ValueString() != "90m" {
					t.Errorf("timeouts.create = %v, want 90m", v)
				}
				if _, ok := got.Timeouts.Attributes()["default"]; ok {
					t.Errorf("timeouts.default must be dropped")
				}
			},
		},
		{
			name:   "pre v1.5.0 state",
			source: minimal,
			check: func(t *testing.T, got azureCloudClusterModel) {
				cc := got.ClusterConfig[0]
				if !cc.Timezone.IsNull() || !cc.Location.IsNull() {
					t.Errorf("timezone and location must stay null")
				}
				vm := got.VMConfig[0]
				if vm.Subnet.ValueString() != "subnet-a" || vm.AvailabilityZone.ValueString() != "2" {
					t.Errorf("subnet = %v and availability_zone = %v, want them kept", vm.Subnet, vm.AvailabilityZone)
				}
				if !got.Timeouts.IsNull() {
					t.Errorf("timeouts = %v, want null", got.Timeouts)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceState := tfsdk.State{
				Schema: *mover.SourceSchema,
				Raw:    tftypes.NewValue(mover.SourceSchema.Type().TerraformType(ctx), nil),
			}
			if diags := sourceState.Set(ctx, &tt.source); diags.HasError() {
				t.Fatalf("failed to build source state: %v", diags)
			}

			req := fwresource.MoveStateRequest{
				SourceProviderAddress: "registry.terraform.io/rubrikinc/polaris",
				SourceTypeName:        "polaris_azure_cloud_cluster",
				SourceSchemaVersion:   0,
				SourceState:           &sourceState,
			}
			res := fwresource.MoveStateResponse{
				TargetState: tfsdk.State{
					Schema: schemaRes.Schema,
					Raw:    tftypes.NewValue(schemaRes.Schema.Type().TerraformType(ctx), nil),
				},
			}
			mover.StateMover(ctx, req, &res)
			if res.Diagnostics.HasError() {
				t.Fatalf("state mover failed: %v", res.Diagnostics)
			}

			var got azureCloudClusterModel
			if diags := res.TargetState.Get(ctx, &got); diags.HasError() {
				t.Fatalf("failed to decode the target state: %v", diags)
			}
			if got.ID != tt.source.ID {
				t.Errorf("id = %v, want %v", got.ID, tt.source.ID)
			}
			tt.check(t, got)
		})
	}

	t.Run("other source type is ignored", func(t *testing.T) {
		res := fwresource.MoveStateResponse{}
		mover.StateMover(ctx, fwresource.MoveStateRequest{
			SourceProviderAddress: "registry.terraform.io/rubrikinc/polaris",
			SourceTypeName:        "polaris_aws_cloud_cluster",
		}, &res)
		if res.Diagnostics.HasError() {
			t.Errorf("unexpected diagnostics: %v", res.Diagnostics)
		}
	})
}

// azureCloudClusterTestConfig returns the configuration of a cluster for the
// acceptance tests. The prefix is the provider and resource prefix, and
// clusterSettings is added to the cluster_config block.
func azureCloudClusterTestConfig(prefix, clusterSettings string) string {
	return fmt.Sprintf(`
		variable "credentials" {
			type = string
		}
		variable "cluster_name" {
			type = string
		}
		variable "admin_email" {
			type = string
		}
		variable "admin_password" {
			type      = string
			sensitive = true
		}
		variable "cloud_account_id" {
			type = string
		}
		variable "region" {
			type = string
		}
		variable "cdm_version" {
			type = string
		}
		variable "instance_type" {
			type = string
		}
		variable "resource_group_name" {
			type = string
		}
		variable "storage_account_name" {
			type = string
		}
		variable "container_name" {
			type = string
		}
		variable "user_assigned_managed_identity_name" {
			type = string
		}
		variable "network_resource_group" {
			type = string
		}
		variable "vnet_resource_group" {
			type = string
		}
		variable "vnet" {
			type = string
		}
		variable "subnet" {
			type = string
		}
		variable "network_security_group" {
			type = string
		}
		variable "network_security_resource_group" {
			type = string
		}
		variable "dns_name_servers" {
			type = list(string)
		}
		variable "ntp_servers" {
			type = list(string)
		}
		provider "%[1]s" {
			credentials = var.credentials
		}
		resource "%[1]s_azure_cloud_cluster" "cluster" {
			cloud_account_id = var.cloud_account_id

			cluster_config {
				cluster_name            = var.cluster_name
				admin_email             = var.admin_email
				admin_password          = var.admin_password
				num_nodes               = 3
				dns_name_servers        = var.dns_name_servers
				ntp_servers             = var.ntp_servers
				keep_cluster_on_failure = false
				%[2]s
			}

			vm_config {
				cdm_version                         = var.cdm_version
				instance_type                       = var.instance_type
				region                              = var.region
				resource_group_name                 = var.resource_group_name
				storage_account_name                = var.storage_account_name
				container_name                      = var.container_name
				enable_immutability                 = false
				user_assigned_managed_identity_name = var.user_assigned_managed_identity_name
				network_resource_group              = var.network_resource_group
				vnet_resource_group                 = var.vnet_resource_group
				vnet                                = var.vnet
				subnet                              = var.subnet
				network_security_group              = var.network_security_group
				network_security_resource_group     = var.network_security_resource_group
			}
		}
	`, prefix, clusterSettings)
}

// azureCloudClusterTestVariables returns the variables of the acceptance test
// configuration, from the fixture.
func azureCloudClusterTestVariables(t *testing.T, fixture testAzureCloudCluster) config.Variables {
	t.Helper()

	strings := func(values []string) config.Variable {
		vars := make([]config.Variable, len(values))
		for i, v := range values {
			vars[i] = config.StringVariable(v)
		}
		return config.ListVariable(vars...)
	}

	return config.Variables{
		"credentials":                         config.StringVariable(testCredentials(t)),
		"cluster_name":                        config.StringVariable(fmt.Sprintf("tf-test-%s", uuid.NewString()[:8])),
		"admin_email":                         config.StringVariable(fixture.AdminEmail),
		"admin_password":                      config.StringVariable("Tf-" + uuid.NewString()),
		"cloud_account_id":                    config.StringVariable(fixture.CloudAccountID),
		"region":                              config.StringVariable(fixture.Region),
		"cdm_version":                         config.StringVariable(fixture.CDMVersion),
		"instance_type":                       config.StringVariable(fixture.InstanceType),
		"resource_group_name":                 config.StringVariable(fixture.ResourceGroupName),
		"storage_account_name":                config.StringVariable(fixture.StorageAccountName),
		"container_name":                      config.StringVariable(fixture.ContainerName),
		"user_assigned_managed_identity_name": config.StringVariable(fixture.UserAssignedManagedIdentity),
		"network_resource_group":              config.StringVariable(fixture.NetworkResourceGroup),
		"vnet_resource_group":                 config.StringVariable(fixture.VnetResourceGroup),
		"vnet":                                config.StringVariable(fixture.Vnet),
		"subnet":                              config.StringVariable(fixture.Subnet),
		"network_security_group":              config.StringVariable(fixture.NetworkSecurityGroup),
		"network_security_resource_group":     config.StringVariable(fixture.NetworkSecurityResourceGroup),
		"dns_name_servers":                    strings(fixture.DNSNameServers),
		"ntp_servers":                         strings(fixture.NTPServers),
	}
}

func TestAccAzureCloudClusterResource(t *testing.T) {
	fixture := testAzureCloudClusterConfig(t)
	vars := azureCloudClusterTestVariables(t, fixture)

	clusterConfig := func(name string) tfjsonpath.Path {
		return tfjsonpath.New(keyClusterConfig).AtSliceIndex(0).AtMapKey(name)
	}

	// The DNS servers are updated in the second step.
	updatedVars := config.Variables{}
	for k, v := range vars {
		updatedVars[k] = v
	}
	dnsServers := []config.Variable{config.StringVariable("192.0.2.53")}
	for _, server := range fixture.DNSNameServers {
		dnsServers = append(dnsServers, config.StringVariable(server))
	}
	updatedVars["dns_name_servers"] = config.ListVariable(dnsServers...)

	// The RSC cluster is replaced neither by an import nor by a refresh. The
	// values RSC can't return are adopted from the configuration, so the import
	// plans an in-place update and not an empty plan.
	importPlanChecks := resource.ImportPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("rubrik_azure_cloud_cluster.cluster", plancheck.ResourceActionUpdate),
		},
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureCloudClusterCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Create the cluster and verify that the timezone and location are
			// applied after the create and that the CDM product is computed.
			Config: azureCloudClusterTestConfig("rubrik", `
				timezone = "America/Los_Angeles"
				location = "Palo Alto, CA"
			`),
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", clusterConfig(keyTimezone), knownvalue.StringExact("America/Los_Angeles")),
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", clusterConfig(keyLocation), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", clusterConfig(keyAdminEmail), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", clusterConfig(keyAdminPassword), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster",
					tfjsonpath.New(keyVMConfig).AtSliceIndex(0).AtMapKey(keyCDMProduct), knownvalue.NotNull()),
				statecheck.ExpectIdentity("rubrik_azure_cloud_cluster.cluster", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_cloud_cluster.cluster", tfjsonpath.New(keyID)),
			},
		}, {
			// Update the DNS servers, the timezone and the location in place.
			Config: azureCloudClusterTestConfig("rubrik", `
				timezone = "Europe/Paris"
				location = "Paris, France"
			`),
			ConfigVariables: updatedVars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_cloud_cluster.cluster", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", clusterConfig(keyTimezone), knownvalue.StringExact("Europe/Paris")),
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", clusterConfig(keyLocation), knownvalue.StringExact("Paris, France")),
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", clusterConfig(keyDNSNameServers),
					knownvalue.SetSizeExact(len(fixture.DNSNameServers)+1)),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_cloud_cluster.cluster", tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_azure_cloud_cluster.cluster",
			ConfigVariables:   vars,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateVerifyIgnore: []string{
				// RSC can't return any of the create-time inputs, so they are missing
				// from the state of an imported cluster and are adopted from the
				// configuration on the first apply.
				keyCloudAccountID, keyAzResilient,
				// Write-only attributes are never stored in the state.
				keyClusterConfig + ".0." + keyAdminEmail,
				keyClusterConfig + ".0." + keyAdminPassword,
				// Create-time inputs RSC does not return.
				keyClusterConfig + ".0." + keyNumNodes,
				keyClusterConfig + ".0." + keyKeepClusterOnFailure,
				// A provider side setting, not stored in RSC.
				keyClusterConfig + ".0." + keyForceClusterDeleteOnDestroy,
				// RSC does not return the VM configuration, cdm_product is only computed
				// at create.
				keyVMConfig + ".0." + keyCDMProduct,
				keyVMConfig + ".0." + keyInstanceType,
				keyVMConfig + ".0." + keyResourceGroupName,
				keyVMConfig + ".0." + keyStorageAccountName,
				keyVMConfig + ".0." + keyContainerName,
				keyVMConfig + ".0." + keyEnableImmutability,
				keyVMConfig + ".0." + keyUserAssignedManagedIdentityName,
				keyVMConfig + ".0." + keyRegion,
				keyVMConfig + ".0." + keyNetworkResourceGroup,
				keyVMConfig + ".0." + keyVnetResourceGroup,
				keyVMConfig + ".0." + keySubnet,
				keyVMConfig + ".0." + keyVnet,
				keyVMConfig + ".0." + keyNetworkSecurityGroup,
				keyVMConfig + ".0." + keyNetworkSecurityResourceGroup,
				keyVMConfig + ".0." + keyVMType,
				// The cdm_version is the installed version after an import, and the
				// configured version otherwise.
				keyVMConfig + ".0." + keyCDMVersion,
				// Not part of the cluster.
				keyTimeouts,
			},
		}, {
			// import {} block with id attribute.
			ResourceName:     "rubrik_azure_cloud_cluster.cluster",
			ConfigVariables:  vars,
			ImportStateKind:  resource.ImportBlockWithID,
			ImportState:      true,
			ImportPlanChecks: importPlanChecks,
		}, {
			// import {} block with identity attribute.
			ResourceName:     "rubrik_azure_cloud_cluster.cluster",
			ConfigVariables:  vars,
			ImportStateKind:  resource.ImportBlockWithResourceIdentity,
			ImportState:      true,
			ImportPlanChecks: importPlanChecks,
		}},
	})
}

// TestAccAzureCloudClusterResource_FrameworkMigration verifies that the
// migrated azure_cloud_cluster resource is backwards compatible with the SDKv2
// provider. Step 1 creates the cluster with the SDKv2 provider; Step 2 swaps to
// the framework provider with the same config and asserts an empty plan.
func TestAccAzureCloudClusterResource_FrameworkMigration(t *testing.T) {
	fixture := testAzureCloudClusterConfig(t)
	vars := azureCloudClusterTestVariables(t, fixture)
	conf := azureCloudClusterTestConfig("rubrik", "")

	resource.Test(t, resource.TestCase{
		CheckDestroy: azureCloudClusterCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"rubrik": {
					Source:            "rubrikinc/rubrik",
					VersionConstraint: "1.12.0",
				},
			},
			Config:          conf,
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_cloud_cluster.cluster", tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			// The state written by the SDKv2 provider must not plan any change.
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config:                   conf,
			ConfigVariables:          vars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

// TestAccAzureCloudClusterResource_MoveState verifies that state from a
// polaris_azure_cloud_cluster resource created by the rubrikinc/polaris
// provider can be moved to a rubrik_azure_cloud_cluster resource using the
// moved {} block.
func TestAccAzureCloudClusterResource_MoveState(t *testing.T) {
	fixture := testAzureCloudClusterConfig(t)
	vars := azureCloudClusterTestVariables(t, fixture)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		CheckDestroy: azureCloudClusterCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.12.0",
				},
			},
			Config:          azureCloudClusterTestConfig("polaris", ""),
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_azure_cloud_cluster.cluster", tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				moved {
					from = polaris_azure_cloud_cluster.cluster
					to   = rubrik_azure_cloud_cluster.cluster
				}
			` + azureCloudClusterTestConfig("rubrik", ""),
			ConfigVariables: vars,
			// Verify the plan is empty and the move succeeded without drift.
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
