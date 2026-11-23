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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
)

func TestGcpCloudClusterImportState(t *testing.T) {
	const id = "6f1d4c7a-2b8e-4f3a-9c5d-1e2f3a4b5c6d"
	const accountID = "0a9b8c7d-6e5f-4a3b-8c2d-1e0f9a8b7c6d"
	tests := []struct {
		name         string
		identity     bool
		multiAZ      bool
		missing      bool
		mixedNetwork bool
		wantError    string
	}{
		{name: "single AZ by ID"},
		{name: "multi AZ by identity", identity: true, multiAZ: true},
		{name: "no create record", missing: true, wantError: "Cloud cluster create configuration not found"},
		{name: "mixed networks", mixedNetwork: true, wantError: "Cannot import GCP cloud cluster"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			const zone = "us-west1-a"
			nodes := 1
			networks := []map[string]string{{"network": "my-vpc", "name": "subnet-a"}}
			var azs []map[string]string
			if tt.multiAZ {
				nodes = 3
				networks = []map[string]string{
					{"network": "my-vpc", "name": "subnet-a", "host_project": "shared-vpc"},
					{"network": "my-vpc", "name": "subnet-b", "host_project": "shared-vpc"},
					{"network": "my-vpc", "name": "subnet-c", "host_project": "shared-vpc"},
				}
				azs = []map[string]string{
					{"subnet": "subnet-a", "availability_zone": zone},
					{"subnet": "subnet-b", "availability_zone": "us-west1-b"},
					{"subnet": "subnet-c", "availability_zone": "us-west1-c"},
				}
			}
			if tt.mixedNetwork {
				nodes = 2
				networks = append(networks, map[string]string{"network": "other-vpc", "name": "subnet-other"})
			}
			metadata := map[string]any{
				"cloud_account_id": accountID, "region": "us-west1", "is_az_resilient": tt.multiAZ,
				"cluster_config": map[string]any{
					"cluster_name": "my-cluster", "num_nodes": nodes,
					"gcp_es_config": map[string]string{"bucket_name": "my-bucket"},
				},
				"vm_config": map[string]any{
					"instance_type": 1, "cdm_version": "9.4.0-p2-30507", "cdm_product": "rubrik-cdm",
					"network_config": networks, "subnet_az_configs": azs,
					"service_accounts": []map[string]string{{"email": "node@example.iam.gserviceaccount.com"}},
				},
			}
			if !tt.multiAZ {
				metadata["zone"] = zone
			}
			marshaled, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			status := "COMPLETED"
			if tt.missing {
				status = "JOB_NOTFOUND"
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.URL.Path == "/api/session" {
					// A dummy JWT accepted by the SDK's localhost test client.
					const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjQ3NzgzNzUzMDZ9.test"
					if err := json.NewEncoder(w).Encode(map[string]any{"access_token": token, "is_eula_accepted": true}); err != nil {
						t.Error(err)
					}
					return
				}
				var request struct {
					Variables map[string]string `json:"variables"`
				}
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if request.Variables["clusterUuid"] != id || request.Variables["jobType"] != "CLUSTER_CREATE" {
					t.Errorf("unexpected request variables: %v", request.Variables)
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": map[string]string{
					"clusterUuid": id, "jobType": "CLUSTER_CREATE", "status": status,
					"vendor": "GCP", "marshaledConfig": string(marshaled),
				}}}); err != nil {
					t.Error(err)
				}
			}))
			defer srv.Close()

			r := &gcpCloudClusterResource{client: &client{polarisClient: &polaris.Client{GQL: graphql.NewTestClient(srv)}}}
			var schemaResponse resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
			var identityResponse resource.IdentitySchemaResponse
			r.IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &identityResponse)

			identity := tfsdk.ResourceIdentity{
				Schema: identityResponse.IdentitySchema,
				Raw:    tftypes.NewValue(identityResponse.IdentitySchema.Type().TerraformType(ctx), nil),
			}
			req := resource.ImportStateRequest{ID: id}
			if tt.identity {
				input := identity
				if diags := input.SetAttribute(ctx, path.Root(keyID), id); diags.HasError() {
					t.Fatal(diags)
				}
				req = resource.ImportStateRequest{Identity: &input}
			}
			res := resource.ImportStateResponse{
				State: tfsdk.State{
					Schema: schemaResponse.Schema,
					Raw:    tftypes.NewValue(schemaResponse.Schema.Type().TerraformType(ctx), nil),
				},
				Identity: &identity,
			}
			r.ImportState(ctx, req, &res)
			if tt.wantError != "" {
				for _, d := range res.Diagnostics {
					if d.Summary() == tt.wantError {
						return
					}
				}
				t.Fatalf("wanted %q, got %v", tt.wantError, res.Diagnostics)
			}
			if res.Diagnostics.HasError() {
				t.Fatal(res.Diagnostics)
			}

			var state gcpCloudClusterModel
			if diags := res.State.Get(ctx, &state); diags.HasError() {
				t.Fatal(diags)
			}
			var identityID types.String
			if diags := res.Identity.GetAttribute(ctx, path.Root(keyID), &identityID); diags.HasError() {
				t.Fatal(diags)
			}
			if state.ID.ValueString() != id || state.ID != identityID || state.CloudAccountID.ValueString() != accountID {
				t.Fatalf("unexpected cluster ID, identity or cloud account: %v, %v, %v", state.ID, identityID, state.CloudAccountID)
			}
			if state.Zone.ValueString() != zone || state.AZResilient.ValueBool() != tt.multiAZ || state.Region.ValueString() != "us-west1" {
				t.Fatalf("unexpected zone, resiliency or region: %v, %v, %v", state.Zone, state.AZResilient, state.Region)
			}

			cc, vm := state.ClusterConfig[0], state.VMConfig[0]
			if cc.NumNodes.ValueInt64() != int64(nodes) || cc.BucketName.ValueString() != "my-bucket" || !cc.AdminPassword.IsNull() || !cc.AdminEmail.IsNull() {
				t.Fatalf("unexpected cluster configuration: %+v", cc)
			}
			wantServiceAccount := types.StringValue("node@example.iam.gserviceaccount.com")
			if vm.InstanceType.ValueString() != "N2_STANDARD_8" || vm.CDMVersion.ValueString() != "9.4.0-p2-30507" ||
				vm.CDMProduct.ValueString() != "rubrik-cdm" || vm.Network.ValueString() != "my-vpc" ||
				!vm.DeleteProtection.Equal(types.BoolValue(false)) ||
				len(vm.ServiceAccounts.Elements()) != 1 || !vm.ServiceAccounts.Elements()[0].Equal(wantServiceAccount) {
				t.Fatalf("unexpected VM configuration: %+v", vm)
			}
			if tt.multiAZ {
				var got []gcpSubnetAzConfigModel
				if diags := vm.SubnetAzConfig.ElementsAs(ctx, &got, false); diags.HasError() {
					t.Fatal(diags)
				}
				if !vm.Subnet.IsNull() || vm.HostProject.ValueString() != "shared-vpc" || len(got) != 3 ||
					got[1].Subnet.ValueString() != "subnet-b" || got[1].AvailabilityZone.ValueString() != "us-west1-b" {
					t.Fatalf("unexpected Multi-AZ network: subnet=%v host_project=%v pairs=%v", vm.Subnet, vm.HostProject, got)
				}
			} else if vm.Subnet.ValueString() != "subnet-a" || !vm.HostProject.IsNull() ||
				vm.SubnetAzConfig.IsNull() || len(vm.SubnetAzConfig.Elements()) != 0 {
				t.Fatalf("unexpected single-AZ network: subnet=%v host_project=%v pairs=%v", vm.Subnet, vm.HostProject, vm.SubnetAzConfig)
			}
		})
	}
}
