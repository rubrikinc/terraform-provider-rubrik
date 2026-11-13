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
	"strings"
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

func TestValidateAwsCloudClusterConfig(t *testing.T) {
	ctx := context.Background()

	azList, diags := types.ListValueFrom(ctx, cloudClusterSubnetAzObjectType, []cloudClusterSubnetAzConfigModel{
		{AvailabilityZone: types.StringValue("us-west-2a"), Subnet: types.StringValue("subnet-a")},
		{AvailabilityZone: types.StringValue("us-west-2b"), Subnet: types.StringValue("subnet-b")},
	})
	if diags.HasError() {
		t.Fatalf("failed to build subnet_az_config list: %v", diags)
	}
	nullList := types.ListNull(cloudClusterSubnetAzObjectType)
	unknownList := types.ListUnknown(cloudClusterSubnetAzObjectType)

	model := func(azResilient, placementGroups types.Bool, subnet types.String, subnetAz types.List) awsCloudClusterModel {
		return awsCloudClusterModel{
			AZResilient:        azResilient,
			UsePlacementGroups: placementGroups,
			VMConfig:           []awsVMConfigModel{{SubnetID: subnet, SubnetAzConfig: subnetAz}},
		}
	}

	tests := []struct {
		name      string
		config    awsCloudClusterModel
		wantError bool
	}{
		{
			name:   "single-AZ with subnet is valid",
			config: model(types.BoolValue(false), types.BoolValue(true), types.StringValue("subnet-a"), nullList),
		},
		{
			name:   "null az_resilient is single-AZ",
			config: model(types.BoolNull(), types.BoolNull(), types.StringValue("subnet-a"), nullList),
		},
		{
			name:      "null az_resilient without subnet is rejected",
			config:    model(types.BoolNull(), types.BoolNull(), types.StringNull(), nullList),
			wantError: true,
		},
		{
			name:      "single-AZ with empty subnet is rejected",
			config:    model(types.BoolValue(false), types.BoolNull(), types.StringValue(""), nullList),
			wantError: true,
		},
		{
			name:      "single-AZ with subnet_az_config is rejected",
			config:    model(types.BoolValue(false), types.BoolNull(), types.StringValue("subnet-a"), azList),
			wantError: true,
		},
		{
			name:   "multi-AZ with az configs is valid",
			config: model(types.BoolValue(true), types.BoolValue(false), types.StringNull(), azList),
		},
		{
			name:      "multi-AZ without az configs is rejected",
			config:    model(types.BoolValue(true), types.BoolNull(), types.StringNull(), nullList),
			wantError: true,
		},
		{
			name:      "multi-AZ with subnet is rejected",
			config:    model(types.BoolValue(true), types.BoolNull(), types.StringValue("subnet-a"), azList),
			wantError: true,
		},
		{
			name:      "multi-AZ with placement groups is rejected",
			config:    model(types.BoolValue(true), types.BoolValue(true), types.StringNull(), azList),
			wantError: true,
		},
		{
			name:   "unknown az_resilient skips validation",
			config: model(types.BoolUnknown(), types.BoolNull(), types.StringNull(), nullList),
		},
		{
			name:   "unknown subnet skips the subnet-required check",
			config: model(types.BoolValue(false), types.BoolNull(), types.StringUnknown(), nullList),
		},
		{
			name:   "unknown subnet_az_config skips the either-or check",
			config: model(types.BoolValue(true), types.BoolNull(), types.StringNull(), unknownList),
		},
		{
			name:   "unknown use_placement_groups skips the multi-AZ check",
			config: model(types.BoolValue(true), types.BoolUnknown(), types.StringNull(), azList),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateAwsCloudClusterConfig(tt.config)
			if got := diags.HasError(); got != tt.wantError {
				t.Errorf("validateAwsCloudClusterConfig() hasError = %v, want %v (diags: %v)", got, tt.wantError, diags)
			}
		})
	}
}

// TestCloudClusterRequiresReplace tests the decision which protects running
// clusters from being replaced.
func TestCloudClusterRequiresReplace(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }
	set := func(elements ...string) attr.Value {
		v, _ := types.SetValueFrom(context.Background(), types.StringType, elements)
		return v
	}

	tests := []struct {
		name     string
		imported bool
		state    attr.Value
		plan     attr.Value
		want     bool
	}{
		{name: "real change", state: str("a"), plan: str("b"), want: true},
		{name: "empty string state, null plan", state: str(""), plan: types.StringNull(), want: false},
		{name: "null state, empty string plan", state: types.StringNull(), plan: str(""), want: false},
		{name: "null state, configured plan", state: types.StringNull(), plan: str("a"), want: true},
		{name: "configured state, null plan", state: str("a"), plan: types.StringNull(), want: true},
		{name: "unknown plan", state: str("a"), plan: types.StringUnknown(), want: true},
		{name: "null bool state, false plan", state: types.BoolNull(), plan: types.BoolValue(false), want: false},
		{name: "false state, true plan", state: types.BoolValue(false), plan: types.BoolValue(true), want: true},
		{name: "true state, false plan", state: types.BoolValue(true), plan: types.BoolValue(false), want: true},
		{name: "null number state, configured plan", state: types.Int64Null(), plan: types.Int64Value(3), want: true},
		{name: "number change", state: types.Int64Value(3), plan: types.Int64Value(4), want: true},
		{name: "empty set state, null plan", state: set(), plan: types.SetNull(types.StringType), want: false},
		{name: "set change", state: set("a"), plan: set("a", "b"), want: true},
		{name: "empty list state, null plan", state: types.ListValueMust(types.StringType, nil), plan: types.ListNull(types.StringType), want: false},
		{name: "imported adopts a string", imported: true, state: types.StringNull(), plan: str("a"), want: false},
		{name: "imported adopts a bool", imported: true, state: types.BoolNull(), plan: types.BoolValue(true), want: false},
		{name: "imported adopts a number", imported: true, state: types.Int64Null(), plan: types.Int64Value(3), want: false},
		{name: "imported adopts a set", imported: true, state: types.SetNull(types.StringType), plan: set("a"), want: false},
		{name: "imported adopts an unknown", imported: true, state: types.StringNull(), plan: types.StringUnknown(), want: false},
		{name: "imported with a state value still replaces", imported: true, state: str("a"), plan: str("b"), want: true},
		{name: "not imported null state still replaces", state: types.StringNull(), plan: str("a"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cloudClusterRequiresReplace(tt.imported, tt.state, tt.plan); got != tt.want {
				t.Errorf("cloudClusterRequiresReplace() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAwsCloudClusterMoveState builds states shaped like the ones the SDKv2
// resource stored, runs them through the state mover and checks that the
// result decodes into the model, which also verifies that the embedded common
// cluster_config model works with the Framework reflection.
func TestAwsCloudClusterMoveState(t *testing.T) {
	ctx := context.Background()

	r := &awsCloudClusterResource{prefix: keyRubrik}
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
	current := awsCloudClusterMoveSourceModel{
		ID:                 str("00000000-0000-0000-0000-000000000001"),
		CloudAccountID:     str("00000000-0000-0000-0000-000000000002"),
		Region:             str("us-west-2"),
		UsePlacementGroups: types.BoolNull(),
		AZResilient:        types.BoolNull(),
		ClusterConfig: []awsClusterConfigModel{{
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
			BucketName:            str("bucket"),
			EnableImmutability:    types.BoolValue(true),
			DynamicScalingEnabled: types.BoolNull(),
		}},
		VMConfig: []awsVMConfigModel{{
			CDMVersion:          str("9.4.0-p2-00000"),
			CDMProduct:          str("9.4.0-p2-00000"),
			InstanceType:        str("M6I_2XLARGE"),
			InstanceProfileName: str("profile"),
			VPCID:               str("vpc-00000000"),
			SubnetID:            str(""),
			SecurityGroupIDs:    set("sg-00000000"),
			VMType:              str(""),
			SubnetAzConfig: types.ListValueMust(cloudClusterSubnetAzObjectType, []attr.Value{
				types.ObjectValueMust(cloudClusterSubnetAzObjectType.AttrTypes, map[string]attr.Value{
					keyAvailabilityZone: str("us-west-2a"), keySubnet: str("subnet-a"),
				}),
			}),
		}},
		Timeouts: types.ObjectValueMust(timeoutAttrs, map[string]attr.Value{
			"create": str("90m"), "read": types.StringNull(), "default": str("20m"),
		}),
	}

	// A pre-v1.5.0 state without timezone, location and az_resilient.
	minimal := current
	minimal.AZResilient = types.BoolNull()
	minimal.ClusterConfig = []awsClusterConfigModel{current.ClusterConfig[0]}
	minimal.ClusterConfig[0].Timezone = types.StringNull()
	minimal.ClusterConfig[0].Location = types.StringNull()
	minimal.ClusterConfig[0].AdminEmail = types.StringNull()
	minimal.VMConfig = []awsVMConfigModel{current.VMConfig[0]}
	minimal.VMConfig[0].SubnetID = str("subnet-a")
	minimal.VMConfig[0].SubnetAzConfig = types.ListNull(cloudClusterSubnetAzObjectType)
	minimal.VMConfig[0].VMType = types.StringNull()
	minimal.Timeouts = types.ObjectNull(timeoutAttrs)

	tests := []struct {
		name   string
		source awsCloudClusterMoveSourceModel
		check  func(t *testing.T, got awsCloudClusterModel)
	}{
		{
			name:   "current SDKv2 state",
			source: current,
			check: func(t *testing.T, got awsCloudClusterModel) {
				if !got.AZResilient.IsNull() {
					t.Errorf("az_resilient = %v, want null", got.AZResilient)
				}
				if got.UsePlacementGroups.ValueBool() || got.UsePlacementGroups.IsNull() {
					t.Errorf("use_placement_groups = %v, want false", got.UsePlacementGroups)
				}
				cc := got.ClusterConfig[0]
				if !cc.AdminEmail.IsNull() {
					t.Errorf("admin_email = %v, want null", cc.AdminEmail)
				}
				if cc.ForceClusterDeleteOnDestroy.IsNull() || cc.DynamicScalingEnabled.IsNull() {
					t.Errorf("unset booleans must become false")
				}
				vm := got.VMConfig[0]
				if !vm.SubnetID.IsNull() {
					t.Errorf("subnet_id = %v, want null", vm.SubnetID)
				}
				if vm.VMType.ValueString() != "DENSE" {
					t.Errorf("vm_type = %v, want DENSE", vm.VMType)
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
			check: func(t *testing.T, got awsCloudClusterModel) {
				cc := got.ClusterConfig[0]
				if !cc.Timezone.IsNull() || !cc.Location.IsNull() {
					t.Errorf("timezone and location must stay null")
				}
				if got.VMConfig[0].SubnetID.ValueString() != "subnet-a" {
					t.Errorf("subnet_id = %v, want subnet-a", got.VMConfig[0].SubnetID)
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
				SourceTypeName:        "polaris_aws_cloud_cluster",
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

			var got awsCloudClusterModel
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
			SourceTypeName:        "polaris_azure_cloud_cluster",
		}, &res)
		if res.Diagnostics.HasError() {
			t.Errorf("unexpected diagnostics: %v", res.Diagnostics)
		}
	})
}

// awsCloudClusterTestConfig returns the configuration of a cluster for the
// acceptance tests. The prefix is the provider and resource prefix, and
// clusterSettings is added to the cluster_config block.
func awsCloudClusterTestConfig(prefix, clusterSettings string) string {
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
		variable "instance_profile_name" {
			type = string
		}
		variable "vpc_id" {
			type = string
		}
		variable "subnet_id" {
			type = string
		}
		variable "security_group_ids" {
			type = list(string)
		}
		variable "bucket_name" {
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
		resource "%[1]s_aws_cloud_cluster" "cluster" {
			cloud_account_id = var.cloud_account_id
			region           = var.region

			cluster_config {
				cluster_name            = var.cluster_name
				admin_email             = var.admin_email
				admin_password          = var.admin_password
				num_nodes               = 1
				dns_name_servers        = var.dns_name_servers
				dns_search_domains      = ["example.com"]
				ntp_servers             = var.ntp_servers
				bucket_name             = var.bucket_name
				enable_immutability     = false
				keep_cluster_on_failure = false
				%[2]s
			}

			vm_config {
				cdm_version           = var.cdm_version
				instance_type         = var.instance_type
				instance_profile_name = var.instance_profile_name
				vpc_id                = var.vpc_id
				subnet_id             = var.subnet_id
				security_group_ids    = var.security_group_ids
			}
		}
	`, prefix, clusterSettings)
}

// awsCloudClusterTestVariables returns the variables of the acceptance test
// configuration, from the fixture.
func awsCloudClusterTestVariables(t *testing.T, fixture testAWSCloudCluster) config.Variables {
	t.Helper()

	stringList := func(values []string) config.Variable {
		vars := make([]config.Variable, len(values))
		for i, v := range values {
			vars[i] = config.StringVariable(v)
		}
		return config.ListVariable(vars...)
	}

	return config.Variables{
		"credentials":           config.StringVariable(testCredentials(t)),
		"cluster_name":          config.StringVariable(fmt.Sprintf("tf-test-%s", uuid.NewString()[:8])),
		"admin_email":           config.StringVariable(fixture.AdminEmail),
		"admin_password":        config.StringVariable("Tf_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:18]),
		"cloud_account_id":      config.StringVariable(fixture.CloudAccountID),
		"region":                config.StringVariable(fixture.Region),
		"cdm_version":           config.StringVariable(fixture.CDMVersion),
		"instance_type":         config.StringVariable(fixture.InstanceType),
		"instance_profile_name": config.StringVariable(fixture.InstanceProfileName),
		"vpc_id":                config.StringVariable(fixture.VPCID),
		"subnet_id":             config.StringVariable(fixture.SubnetID),
		"security_group_ids":    stringList(fixture.SecurityGroupIDs),
		"bucket_name":           config.StringVariable(fixture.BucketName),
		"dns_name_servers":      stringList(fixture.DNSNameServers),
		"ntp_servers":           stringList(fixture.NTPServers),
	}
}

func TestAccAwsCloudClusterResource(t *testing.T) {
	fixture := testAWSCloudClusterConfig(t)
	vars := awsCloudClusterTestVariables(t, fixture)

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

	// An import of a cluster matching the configuration never plans a
	// replacement. The create-time values are read from the configuration RSC
	// recorded at create, and the few it does not record, e.g. cdm_version, are
	// adopted from the configuration, so the import plans an in-place update
	// rather than an empty plan.
	importPlanChecks := resource.ImportPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("rubrik_aws_cloud_cluster.cluster", plancheck.ResourceActionUpdate),
		},
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             awsCloudClusterCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Create the cluster and verify that the timezone and location are
			// applied after the create.
			Config: awsCloudClusterTestConfig("rubrik", `
				timezone = "America/Los_Angeles"
				location = "Palo Alto, CA"
			`),
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyUsePlacementGroups), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyTimezone), knownvalue.StringExact("America/Los_Angeles")),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyLocation), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyAdminEmail), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyAdminPassword), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster",
					tfjsonpath.New(keyVMConfig).AtSliceIndex(0).AtMapKey(keyCDMProduct), knownvalue.NotNull()),
				statecheck.ExpectIdentity("rubrik_aws_cloud_cluster.cluster", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyID)),
			},
		}, {
			// Update the DNS servers, the NTP servers, the timezone and the
			// location in place.
			Config: awsCloudClusterTestConfig("rubrik", `
				timezone = "Europe/Paris"
				location = "Paris, France"
			`),
			ConfigVariables: updatedVars,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_aws_cloud_cluster.cluster", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyTimezone), knownvalue.StringExact("Europe/Paris")),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyLocation), knownvalue.StringExact("Paris, France")),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyDNSNameServers),
					knownvalue.SetSizeExact(len(fixture.DNSNameServers)+1)),
				statecheck.ExpectIdentityValueMatchesState("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_aws_cloud_cluster.cluster",
			ConfigVariables:   vars,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateVerifyIgnore: []string{
				// Write-only attributes are never stored in the state.
				keyClusterConfig + ".0." + keyAdminEmail,
				keyClusterConfig + ".0." + keyAdminPassword,
				// A provider side setting, not stored in RSC.
				keyClusterConfig + ".0." + keyForceClusterDeleteOnDestroy,
				// Not recorded by RSC, adopted from the configuration after an
				// import: the cluster may have been upgraded since it was created.
				keyVMConfig + ".0." + keyCDMVersion,
				// Not recorded by RSC for the STANDARD VM type, adopted from the
				// configuration after an import.
				keyVMConfig + ".0." + keyVMType,
				// Not part of the cluster.
				keyTimeouts,
			},
		}, {
			// import {} block with id attribute.
			ResourceName:     "rubrik_aws_cloud_cluster.cluster",
			ConfigVariables:  vars,
			ImportStateKind:  resource.ImportBlockWithID,
			ImportState:      true,
			ImportPlanChecks: importPlanChecks,
		}, {
			// import {} block with identity attribute.
			ResourceName:     "rubrik_aws_cloud_cluster.cluster",
			ConfigVariables:  vars,
			ImportStateKind:  resource.ImportBlockWithResourceIdentity,
			ImportState:      true,
			ImportPlanChecks: importPlanChecks,
		}},
	})
}

// TestAccAwsCloudClusterResource_FrameworkMigration verifies that the migrated
// aws_cloud_cluster resource is backwards compatible with the SDKv2 provider.
// Step 1 creates the cluster with the SDKv2 provider; Step 2 swaps to the
// framework provider with the same config and asserts an empty plan.
//
// The test needs the released provider in step 1. A dev_overrides entry for
// rubrikinc/rubrik in the Terraform CLI configuration replaces it with a local
// build, which makes the test compare two builds of this provider and not the
// SDKv2 provider with this one. Run the test with a CLI configuration without
// overrides:
//
//	printf 'provider_installation {\n  direct {}\n}\n' > /tmp/no-override.tfrc
//	TF_CLI_CONFIG_FILE=/tmp/no-override.tfrc go test -run '^TestAccAwsCloudClusterResource_FrameworkMigration$' ./internal/provider
func TestAccAwsCloudClusterResource_FrameworkMigration(t *testing.T) {
	fixture := testAWSCloudClusterConfig(t)
	vars := awsCloudClusterTestVariables(t, fixture)
	conf := awsCloudClusterTestConfig("rubrik", "")

	clusterConfig := func(name string) tfjsonpath.Path {
		return tfjsonpath.New(keyClusterConfig).AtSliceIndex(0).AtMapKey(name)
	}
	vmConfig := func(name string) tfjsonpath.Path {
		return tfjsonpath.New(keyVMConfig).AtSliceIndex(0).AtMapKey(name)
	}

	resource.Test(t, resource.TestCase{
		CheckDestroy: awsCloudClusterCheckDestroy(t),
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
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyCloudAccountID), knownvalue.StringExact(fixture.CloudAccountID)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyRegion), knownvalue.StringExact(fixture.Region)),
				// The SDKv2 provider stores empty values where the Framework
				// plans null. Pinning them documents what the second step must
				// accept without a plan.
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", tfjsonpath.New(keyUsePlacementGroups), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyNumNodes), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyBucketName), knownvalue.StringExact(fixture.BucketName)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyDNSNameServers), knownvalue.SetSizeExact(len(fixture.DNSNameServers))),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyDNSSearchDomains), knownvalue.SetSizeExact(1)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyNTPServers), knownvalue.SetSizeExact(len(fixture.NTPServers))),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyForceClusterDeleteOnDestroy), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", clusterConfig(keyAdminPassword), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keyCDMVersion), knownvalue.StringExact(fixture.CDMVersion)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keyCDMProduct), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keyInstanceType), knownvalue.StringExact(fixture.InstanceType)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keyInstanceProfileName), knownvalue.StringExact(fixture.InstanceProfileName)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keyVPCID), knownvalue.StringExact(fixture.VPCID)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keySubnetID), knownvalue.StringExact(fixture.SubnetID)),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keySecurityGroupIDs), knownvalue.SetSizeExact(len(fixture.SecurityGroupIDs))),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keyVMType), knownvalue.StringExact("DENSE")),
				statecheck.ExpectKnownValue("rubrik_aws_cloud_cluster.cluster", vmConfig(keySubnetAzConfigs), knownvalue.ListSizeExact(0)),
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

// TestAccAwsCloudClusterResource_MoveState verifies that state from a
// polaris_aws_cloud_cluster resource created by the rubrikinc/polaris provider
// can be moved to a rubrik_aws_cloud_cluster resource using the moved {} block.
//
// The test needs the released rubrikinc/polaris provider in step 1. A
// dev_overrides entry for rubrikinc/polaris in the Terraform CLI configuration
// replaces it with a local build, see TestAccAwsCloudClusterResource_FrameworkMigration
// for how to run the test without overrides.
func TestAccAwsCloudClusterResource_MoveState(t *testing.T) {
	fixture := testAWSCloudClusterConfig(t)
	vars := awsCloudClusterTestVariables(t, fixture)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		CheckDestroy: awsCloudClusterCheckDestroy(t),
		Steps: []resource.TestStep{{
			ExternalProviders: map[string]resource.ExternalProvider{
				"polaris": {
					Source:            "rubrikinc/polaris",
					VersionConstraint: "1.12.0",
				},
			},
			Config:          awsCloudClusterTestConfig("polaris", ""),
			ConfigVariables: vars,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("polaris_aws_cloud_cluster.cluster", tfjsonpath.New(keyID), NonNullUUID()),
			},
		}, {
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			Config: `
				moved {
					from = polaris_aws_cloud_cluster.cluster
					to   = rubrik_aws_cloud_cluster.cluster
				}
			` + awsCloudClusterTestConfig("rubrik", ""),
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
