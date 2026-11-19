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
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	regions "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

func (r *azureManagementGroupResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.ModifyPlan")

	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var state, plan azureManagementGroupModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() || state.Regions.Equal(plan.Regions) {
		return
	}
	planRegions, err := plan.Regions.ToTerraformValue(ctx)
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyRegions), "Invalid planned regions", err.Error())
		return
	}
	if !planRegions.IsFullyKnown() {
		return
	}
	if slices.ContainsFunc(state.Regions.Elements(), func(oldRegion attr.Value) bool {
		return !slices.ContainsFunc(plan.Regions.Elements(), oldRegion.Equal)
	}) {
		res.Diagnostics.AddAttributeError(path.Root(keyRegions), "Region removal is not supported", "Existing regions must be retained. You can only add regions to an existing management group resource.")
		return
	}
	// Unknown values can be resolved only at apply. Recheck them there.
	featureValue, featureErr := plan.Feature.ToTerraformValue(ctx)
	subscriptionValue, subscriptionErr := plan.SubscriptionIDs.ToTerraformValue(ctx)
	if featureErr != nil || subscriptionErr != nil || !featureValue.IsFullyKnown() || !subscriptionValue.IsFullyKnown() {
		return
	}
	if !state.Feature.Equal(plan.Feature) || !state.SubscriptionIDs.Equal(plan.SubscriptionIDs) {
		res.Diagnostics.AddAttributeError(path.Root(keyRegions), "Apply region additions separately", "Do not change subscriptions or features in the same apply as a region addition.")
		return
	}
	var features []azureManagementGroupFeatureModel
	res.Diagnostics.Append(plan.Feature.ElementsAs(ctx, &features, false)...)
	if !res.Diagnostics.HasError() && !slices.ContainsFunc(features, func(f azureManagementGroupFeatureModel) bool {
		return f.Name.Equal(types.StringValue(core.CloudDiscovery))
	}) {
		res.Diagnostics.AddAttributeError(path.Root(keyRegions), "Cloud Discovery is required for region additions", "Enable and connect CLOUD_DISCOVERY on the managed subscriptions before adding regions.")
	}
}

// azureManagementGroupRegionUpdate records the Cloud Discovery change and the
// regional feature rows that RSC must align. The expected regions include
// any regions already present in Discovery, including out-of-band additions.
type azureManagementGroupRegionUpdate struct {
	subscriptionID uuid.UUID
	name           string
	toAdd          []regions.Region
	expected       []regions.Region
	align          []string
}

// azureManagementGroupRegionUpdates preflights each managed subscription
// before any mutation. Raw tenant rows are required: the SDK's high-level
// subscription view filters out features that may participate in alignment.
func azureManagementGroupRegionUpdates(accounts []gqlazure.CloudAccount, owned []string, desired []regions.Region) (azureManagementGroupRegionUpdate, error) {
	var update azureManagementGroupRegionUpdate
	cd := slices.IndexFunc(accounts, func(account gqlazure.CloudAccount) bool { return account.Feature.Feature == core.CloudDiscovery })
	if cd < 0 || !slices.Contains(owned, core.CloudDiscovery) || accounts[cd].Feature.Status != core.StatusConnected {
		return update, fmt.Errorf("region additions require a connected CLOUD_DISCOVERY feature managed by this resource")
	}
	discovery := accounts[cd]
	update.subscriptionID, update.name = discovery.ID, discovery.Name
	if discovery.App != nil && (discovery.App.AuthType == gqlazure.AuthTypeOAuthRubrikApp || discovery.App.AuthType == gqlazure.AuthTypeOAuthCustomerApp) {
		return update, fmt.Errorf("cannot add regions to an OAuth subscription with this non-OAuth resource")
	}
	present := make(map[regions.Region]bool, len(discovery.Feature.Regions))
	expected := make(map[regions.Region]bool, len(desired)+len(discovery.Feature.Regions))
	for _, r := range discovery.Feature.Regions {
		present[r.Region] = true
		expected[r.Region] = true
	}
	for _, r := range desired {
		expected[r] = true
		if !present[r] {
			update.toAdd = append(update.toAdd, r)
		}
	}
	update.expected = slices.Sorted(maps.Keys(expected))
	needsAlignment := false
	for _, account := range accounts {
		// Matches AzureCustomerRegionalFeatures in cloudaccounts. These
		// sibling rows are rewritten by the Cloud Discovery update.
		switch account.Feature.Feature {
		case core.CloudDiscovery, core.CloudNativeProtection, core.CloudNativeBlobProtection,
			core.AzureSQLDBProtection, core.AzureSQLMIProtection, core.DSPMMetadata, core.DSPMData,
			core.CyberRecoveryDataClassificationMetadata, core.CyberRecoveryDataClassificationData,
			core.LaminarTargetApplication, core.LaminarTargetManagedIdentity:
		case core.CloudNativeArchival, core.CloudNativeArchivalEncryption, core.CloudNativeUEMKeyManagement,
			core.Exocompute, core.ServerAndApps, core.AzurePostgresFlexibleServerProtection, core.CloudCostReport:
			continue // RSC does not align these features to Cloud Discovery.
		default:
			if len(account.Feature.Regions) == 0 {
				continue
			}
			return update, fmt.Errorf("cannot confirm that region alignment is safe for feature %s", account.Feature.Feature)
		}
		if !slices.Contains(owned, account.Feature.Feature) && !azureManagementGroupSameRegions(account.Feature.Regions, expected) {
			return update, fmt.Errorf("region alignment would change unmanaged feature %s", account.Feature.Feature)
		}
		for _, region := range account.Feature.Regions {
			if !expected[region.Region] {
				return update, fmt.Errorf("region alignment would remove %s from %s; include that region before retrying", region.Name(), account.Feature.Feature)
			}
		}
		if slices.Contains(owned, account.Feature.Feature) {
			update.align = append(update.align, account.Feature.Feature)
			needsAlignment = needsAlignment || !azureManagementGroupSameRegions(account.Feature.Regions, expected)
		}
	}
	if len(update.toAdd) == 0 && needsAlignment {
		// A partial update can leave Discovery up to date before its
		// sibling features. Re-send the desired regions as idempotent adds.
		update.toAdd = slices.Clone(desired)
	}
	return update, nil
}

func azureManagementGroupSameRegions(actual []regions.CloudAccountRegionEnum, expected map[regions.Region]bool) bool {
	if len(actual) != len(expected) {
		return false
	}
	for _, region := range actual {
		if !expected[region.Region] {
			return false
		}
	}
	return true
}

// azureManagementGroupReadRegionRows groups all feature rows by native
// subscription ID for the given Azure tenant and management group.
func azureManagementGroupReadRegionRows(ctx context.Context, api gqlazure.API, tenantDomain, groupID string) (map[uuid.UUID][]gqlazure.CloudAccount, error) {
	tenants, err := api.CloudAccountTenants(ctx, core.FeatureAll, true)
	if err != nil {
		return nil, err
	}
	rows := make(map[uuid.UUID][]gqlazure.CloudAccount)
	for _, tenant := range tenants {
		if !strings.EqualFold(tenantDomain, tenant.DomainName) {
			continue
		}
		for _, account := range tenant.Accounts {
			if account.ManagementGroup == nil || !strings.EqualFold(groupID, account.ManagementGroup.NativeID) {
				continue
			}
			rows[account.NativeID] = append(rows[account.NativeID], account)
		}
	}
	return rows, nil
}

// azureManagementGroupAddRegions adds regions through the same Cloud Discovery
// mutation as the UI and checks the aligned feature rows before returning.
func azureManagementGroupAddRegions(ctx context.Context, api gqlazure.API, tenantDomain, managementGroupID string, owned map[string][]string,
	subscriptionIDs []uuid.UUID, desired []regions.Region) error {
	rows, err := azureManagementGroupReadRegionRows(ctx, api, tenantDomain, managementGroupID)
	if err != nil {
		return fmt.Errorf("read current feature regions: %w", err)
	}
	updates := make(map[uuid.UUID]azureManagementGroupRegionUpdate, len(subscriptionIDs))
	for _, id := range subscriptionIDs {
		update, err := azureManagementGroupRegionUpdates(rows[id], owned[id.String()], desired)
		if err != nil {
			return fmt.Errorf("subscription %s: %w", id, err)
		}
		if len(update.toAdd) != 0 {
			updates[id] = update
		}
	}
	if len(updates) == 0 {
		return nil
	}
	for _, id := range subscriptionIDs {
		update, ok := updates[id]
		if !ok {
			continue
		}
		if err := api.UpdateCloudAccount(ctx, update.subscriptionID, core.FeatureCloudDiscovery, update.name, update.toAdd, nil); err != nil {
			return fmt.Errorf("add Cloud Discovery regions for subscription %s: %w", id, err)
		}
	}
	rows, err = azureManagementGroupReadRegionRows(ctx, api, tenantDomain, managementGroupID)
	if err != nil {
		return fmt.Errorf("verify added feature regions: %w", err)
	}
	for _, id := range subscriptionIDs {
		update, ok := updates[id]
		if !ok {
			continue
		}
		expected := make(map[regions.Region]bool, len(update.expected))
		for _, region := range update.expected {
			expected[region] = true
		}
		for _, feature := range update.align {
			if !slices.ContainsFunc(rows[id], func(row gqlazure.CloudAccount) bool {
				return row.Feature.Feature == feature && row.Feature.Status == core.StatusConnected && azureManagementGroupSameRegions(row.Feature.Regions, expected)
			}) {
				return fmt.Errorf("subscription %s feature %s did not read back as connected with the expected regions", id, feature)
			}
		}
	}
	return nil
}
