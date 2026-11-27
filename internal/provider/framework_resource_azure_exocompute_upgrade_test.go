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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestAzureExocomputeUpgradeState upgrades state through the provider server.
// Attributes left out of the JSON state are null.
func TestAzureExocomputeUpgradeState(t *testing.T) {
	server := providerserver.NewProtocol6(&FrameworkProvider{Version: "test"})()

	var schemaRes resource.SchemaResponse
	newAzureExocomputeResource().Schema(t.Context(), resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}
	stateType := schemaRes.Schema.Type().TerraformType(t.Context())

	tests := []struct {
		name    string
		version int64
		prior   string
		want    string
	}{{
		// The polaris_managed field is removed, and the fields added in v1 are
		// unset.
		name:    "V0",
		version: 0,
		prior: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"polaris_managed": true,
			"region": "eastus2",
			"subnet": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet",
			"subscription_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a"
		}`,
		want: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"optional_config": [],
			"region": "eastus2",
			"subnet": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet",
			"subscription_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a"
		}`,
	}, {
		// The resource stored unset strings as empty strings, also in the
		// optional_config block.
		name:    "V1",
		version: 1,
		prior: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"cloud_account_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"optional_config": [{
				"allowlist_rubrik_ips": false,
				"cluster_access": "AKS_CLUSTER_ACCESS_TYPE_PRIVATE",
				"cluster_tier": "AKS_CLUSTER_TIER_FREE",
				"disk_encryption_at_host": false,
				"max_node_count": "AKS_NODE_COUNT_BUCKET_MEDIUM",
				"private_exocompute_dns_zone_id": "",
				"resource_group_prefix": "",
				"snapshot_private_access_dns_zone_id": "",
				"user_defined_routing": false
			}],
			"pod_overlay_network_cidr": "",
			"region": "eastus2",
			"subnet": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet"
		}`,
		want: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"cloud_account_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"optional_config": [{
				"allowlist_rubrik_ips": false,
				"cluster_access": "AKS_CLUSTER_ACCESS_TYPE_PRIVATE",
				"cluster_tier": "AKS_CLUSTER_TIER_FREE",
				"disk_encryption_at_host": false,
				"max_node_count": "AKS_NODE_COUNT_BUCKET_MEDIUM",
				"user_defined_routing": false
			}],
			"region": "eastus2",
			"subnet": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet"
		}`,
	}, {
		name:    "V1WithValues",
		version: 1,
		prior: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"cloud_account_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"optional_config": [{
				"allowlist_rubrik_ips": false,
				"cluster_access": "AKS_CLUSTER_ACCESS_TYPE_PRIVATE",
				"cluster_tier": "AKS_CLUSTER_TIER_FREE",
				"disk_encryption_at_host": false,
				"max_node_count": "AKS_NODE_COUNT_BUCKET_MEDIUM",
				"private_exocompute_dns_zone_id": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/privateDnsZones/exocompute",
				"resource_group_prefix": "prefix",
				"snapshot_private_access_dns_zone_id": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/privateDnsZones/snapshot",
				"user_defined_routing": false
			}],
			"pod_overlay_network_cidr": "10.244.0.0/16",
			"region": "eastus2",
			"subnet": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet"
		}`,
		want: `{
			"id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"cloud_account_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"optional_config": [{
				"allowlist_rubrik_ips": false,
				"cluster_access": "AKS_CLUSTER_ACCESS_TYPE_PRIVATE",
				"cluster_tier": "AKS_CLUSTER_TIER_FREE",
				"disk_encryption_at_host": false,
				"max_node_count": "AKS_NODE_COUNT_BUCKET_MEDIUM",
				"private_exocompute_dns_zone_id": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/privateDnsZones/exocompute",
				"resource_group_prefix": "prefix",
				"snapshot_private_access_dns_zone_id": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/privateDnsZones/snapshot",
				"user_defined_routing": false
			}],
			"pod_overlay_network_cidr": "10.244.0.0/16",
			"region": "eastus2",
			"subnet": "/subscriptions/3e1b5c9a/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet"
		}`,
	}, {
		// A configuration using a shared exocompute host has no subnet, and
		// no optional_config block.
		name:    "V1SharedHost",
		version: 1,
		prior: `{
			"id": "app-8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"cloud_account_id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"host_cloud_account_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"optional_config": [],
			"pod_overlay_network_cidr": "",
			"subnet": ""
		}`,
		want: `{
			"id": "app-8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"cloud_account_id": "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b",
			"host_cloud_account_id": "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a",
			"optional_config": []
		}`,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := server.UpgradeResourceState(t.Context(), &tfprotov6.UpgradeResourceStateRequest{
				TypeName: "rubrik_azure_exocompute",
				Version:  tc.version,
				RawState: &tfprotov6.RawState{JSON: []byte(tc.prior)},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range res.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					t.Fatal(d.Summary + ": " + d.Detail)
				}
			}

			got, err := res.UpgradedState.Unmarshal(stateType)
			if err != nil {
				t.Fatal(err)
			}
			wantState := tfprotov6.RawState{JSON: []byte(tc.want)}
			want, err := wantState.Unmarshal(stateType)
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := got.Diff(want)
			if err != nil {
				t.Fatal(err)
			}
			for _, diff := range diffs {
				t.Errorf("%s: got %v, want %v", diff.Path, diff.Value1, diff.Value2)
			}
		})
	}
}
