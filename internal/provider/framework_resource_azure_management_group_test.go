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
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	regions "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

func TestAzureManagementGroupRequiresCloudDiscovery(t *testing.T) {
	featureTypes := map[string]attr.Type{keyName: types.StringType}
	for _, tc := range []struct {
		name    string
		feature types.String
		wantErr bool
	}{
		{name: "Missing", feature: types.StringValue(core.CloudNativeProtection), wantErr: true},
		{name: "Configured", feature: types.StringValue(core.CloudDiscovery)},
		{name: "Unknown", feature: types.StringUnknown()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			feature := types.ObjectValueMust(featureTypes, map[string]attr.Value{keyName: tc.feature})
			var res validator.SetResponse
			azureManagementGroupDiscoveryValidator{}.ValidateSet(t.Context(), validator.SetRequest{
				Path:        path.Root(keyFeature),
				ConfigValue: types.SetValueMust(types.ObjectType{AttrTypes: featureTypes}, []attr.Value{feature}),
			}, &res)
			if res.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("unexpected diagnostics: %v", res.Diagnostics)
			}
		})
	}
}

func TestAzureManagementGroupReconcile(t *testing.T) {
	first, second, external := uuid.New(), uuid.New(), uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	discovery := core.FeatureCloudDiscovery.WithPermissionGroups(core.PermissionGroupBasic)
	discoveryInput := azureManagementGroupFeature{name: discovery.Name, permissionGroups: []string{string(core.PermissionGroupBasic)}}
	protection := core.FeatureCloudNativeProtection
	archival := core.FeatureCloudNativeArchival
	costReport := azure.Feature{Feature: core.FeatureCloudCostReport, Status: core.StatusMissingPermissions}
	connected := func(f core.Feature) azure.Feature { return azure.Feature{Feature: f, Status: core.StatusConnected} }
	accounts := []azure.CloudAccount{
		{ID: a, NativeID: first, Features: []azure.Feature{connected(discovery), connected(protection), connected(core.FeatureCloudNativeBlobProtection), costReport}},
		{ID: b, NativeID: second, Features: []azure.Feature{connected(discovery), connected(protection)}},
		{ID: c, NativeID: external, Features: []azure.Feature{connected(discovery), connected(protection), connected(archival)}},
	}
	owned := map[string][]string{
		first.String():  {discovery.Name, protection.Name},
		second.String(): {discovery.Name, protection.Name, archival.Name},
	}
	present := azureManagementGroupPresentSubscriptions(accounts, owned, []attr.Value{types.StringValue(strings.ToUpper(first.String()))})
	if !slices.Equal(present, []string{strings.ToUpper(first.String())}) {
		t.Fatalf("refresh returned %v; unmanaged features must not cause drift, and partially onboarded subscriptions must not mask it", present)
	}

	// Even if the SDK exposes this row someday, it is not part of the owned
	// feature set and does not require an update or deletion.
	adds, removes, err := azureManagementGroupReconcile(accounts[:1], owned, azureManagementGroupInput{
		subscriptions: []uuid.UUID{first},
		features:      []azureManagementGroupFeature{discoveryInput, {name: protection.Name}},
	})
	if err != nil || len(adds) != 0 || len(removes) != 0 {
		t.Fatalf("unmanaged features caused an update: adds=%v removes=%v err=%v", adds, removes, err)
	}

	added := uuid.New()
	adds, removes, err = azureManagementGroupReconcile(accounts[:1], owned, azureManagementGroupInput{
		subscriptions: []uuid.UUID{first, added},
		features:      []azureManagementGroupFeature{discoveryInput, {name: protection.Name}},
	})
	if err != nil || len(removes) != 0 || len(adds) != 1 || adds[0].NativeID != added || len(adds[0].Features) != 2 ||
		!slices.ContainsFunc(adds[0].Features, func(f azure.ManagementGroupFeature) bool { return f.Feature.DeepEqual(discovery) }) {
		t.Fatalf("new subscription must onboard Cloud Discovery with BASIC: adds=%v removes=%v err=%v", adds, removes, err)
	}
	_, _, err = azureManagementGroupReconcile(accounts[:1], owned, azureManagementGroupInput{
		subscriptions: []uuid.UUID{first},
		features:      []azureManagementGroupFeature{{name: protection.Name}},
	})
	if err == nil || !strings.Contains(err.Error(), "include CLOUD_DISCOVERY") {
		t.Fatalf("expected apply to reject missing Cloud Discovery, got %v", err)
	}

	// Discovery cannot be removed from the first subscription while its
	// unmanaged Blob protection remains there.
	_, _, err = azureManagementGroupReconcile(accounts[:1], owned, azureManagementGroupInput{})
	if err == nil || !strings.Contains(err.Error(), "cannot remove Cloud Discovery") {
		t.Fatalf("expected Cloud Discovery removal to be blocked, got %v", err)
	}

	accounts[0].Features = []azure.Feature{connected(discovery), connected(protection), costReport}
	adds, removes, err = azureManagementGroupReconcile(accounts, owned, azureManagementGroupInput{
		subscriptions: []uuid.UUID{second},
		features:      []azureManagementGroupFeature{discoveryInput, {name: protection.Name}, {name: archival.Name}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(adds) != 1 || adds[0].NativeID != second || len(adds[0].Features) != 1 || !adds[0].Features[0].Feature.Equal(archival) {
		t.Errorf("expected retry of only the missing pair, got %+v", adds)
	}
	if len(removes) != 2 || !slices.Equal(removes[discovery.Name], []uuid.UUID{a}) || !slices.Equal(removes[protection.Name], []uuid.UUID{a}) {
		t.Errorf("update would affect unowned features: %v", removes)
	}
	_, removes, err = azureManagementGroupReconcile(accounts, owned, azureManagementGroupInput{})
	if err != nil || len(removes) != 2 || !slices.Equal(removes[discovery.Name], []uuid.UUID{a, b}) ||
		!slices.Equal(removes[protection.Name], []uuid.UUID{a, b}) {
		t.Errorf("destroy would affect unowned features: %v, %v", removes, err)
	}
	_, _, err = azureManagementGroupReconcile(accounts, owned, azureManagementGroupInput{
		subscriptions: []uuid.UUID{external}, features: []azureManagementGroupFeature{discoveryInput, {name: protection.Name}},
	})
	if err == nil || !strings.Contains(err.Error(), "not managed") {
		t.Fatalf("expected explicitly requested unowned existing pair to be rejected, got %v", err)
	}
}

func TestAzureManagementGroupReconcileProtectsArchivalEncryption(t *testing.T) {
	id := uuid.New()
	accounts := []azure.CloudAccount{{ID: uuid.New(), NativeID: id, Features: []azure.Feature{
		{Feature: core.FeatureCloudNativeArchival, Status: core.StatusConnected},
		{Feature: core.FeatureCloudNativeArchivalEncryption, Status: core.StatusConnected},
	}}}
	for _, tc := range []struct {
		name    string
		owned   []string
		desired azureManagementGroupInput
	}{
		{name: "unmanaged encryption", owned: []string{core.CloudNativeArchival}},
		{name: "retained encryption", owned: []string{core.CloudNativeArchival, core.CloudNativeArchivalEncryption}, desired: azureManagementGroupInput{
			subscriptions: []uuid.UUID{id}, features: []azureManagementGroupFeature{
				{name: core.CloudDiscovery, permissionGroups: []string{string(core.PermissionGroupBasic)}},
				{name: core.CloudNativeArchivalEncryption},
			},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := azureManagementGroupReconcile(accounts, map[string][]string{id.String(): tc.owned}, tc.desired)
			if err == nil || !strings.Contains(err.Error(), "Archival Encryption feature depends on it") {
				t.Fatalf("expected the Archival deletion to be blocked, got %v", err)
			}
		})
	}
}

func TestAzureManagementGroupValidateUpdate(t *testing.T) {
	old := azureManagementGroupInput{
		regions:  []regions.Region{regions.RegionFromName("eastus2")},
		features: []azureManagementGroupFeature{{name: core.FeatureCloudNativeProtection.Name, permissionGroups: []string{"BASIC"}}},
	}
	t.Run("regions", func(t *testing.T) {
		changed := old
		changed.regions = []regions.Region{regions.RegionFromName("westus2")}
		if err := azureManagementGroupValidateUpdate(old, changed); err == nil {
			t.Fatal("accepted a region change")
		}
	})
	t.Run("region additions", func(t *testing.T) {
		changed := old
		changed.regions = append(slices.Clone(old.regions), regions.RegionFromName("westus2"))
		if err := azureManagementGroupValidateUpdate(old, changed); err != nil {
			t.Fatalf("rejected a region addition: %v", err)
		}
	})
	t.Run("permission groups", func(t *testing.T) {
		changed := old
		changed.features = []azureManagementGroupFeature{{name: core.FeatureCloudNativeProtection.Name, permissionGroups: []string{"BASIC", "EXPORT_AND_RESTORE"}}}
		if err := azureManagementGroupValidateUpdate(old, changed); err == nil {
			t.Fatal("accepted a permission group change")
		}
	})
}

func TestAzureManagementGroupImportRejectsDifferentSettings(t *testing.T) {
	protection := azure.Feature{
		Feature: core.Feature{Name: core.CloudNativeProtection, PermissionGroups: []core.PermissionGroup{core.PermissionGroupBasic}},
		Status:  core.StatusConnected,
		Regions: []string{"eastus2"},
	}
	for _, tc := range []struct {
		name   string
		change func(*azure.Feature)
	}{
		{name: "Permission groups", change: func(f *azure.Feature) {
			f.PermissionGroups = []core.PermissionGroup{core.PermissionGroupBasic, "EXPORT_AND_RESTORE"}
		}},
		{name: "Regions", change: func(f *azure.Feature) { f.Regions = []string{"westus2"} }},
		{name: "Resource group", change: func(f *azure.Feature) { f.ResourceGroup.Name = "another-group" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := protection
			tc.change(&other)
			data := azureManagementGroupData{
				appID: uuid.New(),
				group: azure.ManagementGroup{NativeID: "mg", Name: "MG", CustomerID: uuid.New()},
				accounts: []azure.CloudAccount{
					{NativeID: uuid.New(), Features: []azure.Feature{protection}},
					{NativeID: uuid.New(), Features: []azure.Feature{other}},
				},
			}
			var imported azureManagementGroupModel
			diags := data.setModel(t.Context(), &imported, "example.onmicrosoft.com", "mg")
			if !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), "different settings") {
				t.Fatalf("expected mismatched feature settings to reject import, got %v", diags)
			}
		})
	}
}

func TestAccAzureManagementGroupResource(t *testing.T) {
	skipUnlessFeatureEnabled(t, core.FeatureFlagAzureCloudAccountsManagementGroup)
	skipUnlessFeatureEnabled(t, core.FeatureFlagAzureDisableDeleteCombined)

	// The management group must contain at least two subscriptions that are
	// eligible for CLOUD_NATIVE_PROTECTION, and the service principal of the
	// app must have the Contributor and User Access Administrator roles at the
	// management group scope.
	appID := os.Getenv("TEST_AZURE_MANAGEMENT_GROUP_APP_ID")
	managementGroupID := os.Getenv("TEST_AZURE_MANAGEMENT_GROUP_ID")
	subscriptionIDs := strings.Split(os.Getenv("TEST_AZURE_MANAGEMENT_GROUP_SUBSCRIPTION_IDS"), ",")
	if appID == "" || managementGroupID == "" || len(subscriptionIDs) < 2 || subscriptionIDs[0] == "" {
		t.Skip("TEST_AZURE_MANAGEMENT_GROUP_APP_ID, TEST_AZURE_MANAGEMENT_GROUP_ID and " +
			"TEST_AZURE_MANAGEMENT_GROUP_SUBSCRIPTION_IDS (at least two) must be set")
	}
	tenantDomain := testAzureTenantDomain(t)

	credentials := testAzureCredentials(t)
	config := func(subscriptions ...string) string {
		quoted := make([]string, 0, len(subscriptions))
		for _, id := range subscriptions {
			quoted = append(quoted, fmt.Sprintf("%q", id))
		}
		return fmt.Sprintf(`
			resource "rubrik_azure_service_principal" "default" {
				credentials   = %q
				tenant_domain = %q
			}

			resource "rubrik_azure_management_group" "test" {
				tenant_domain       = rubrik_azure_service_principal.default.tenant_domain
				app_id              = %q
				management_group_id = %q
				regions             = ["eastus2"]
				subscription_ids    = [%s]

					feature = [
					{
						name              = "CLOUD_DISCOVERY"
						permission_groups = ["BASIC"]
					},
					{
						name                  = "CLOUD_NATIVE_PROTECTION"
						permission_groups     = ["BASIC"]
						resource_group_name   = "terraform-mg-acceptance-test"
						resource_group_region = "eastus2"
					},
				]
			}
		`, credentials, tenantDomain, appID, managementGroupID, strings.Join(quoted, ", "))
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureManagementGroupCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that a management group can be onboarded with a single
			// subscription.
			Config: config(subscriptionIDs[0]),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_management_group.test", tfjsonpath.New(keyID),
					knownvalue.StringExact(tenantDomain+"/"+managementGroupID)),
				statecheck.ExpectKnownValue("rubrik_azure_management_group.test", tfjsonpath.New(keyCustomerManagementGroupID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_management_group.test", tfjsonpath.New(keySubscriptionIDs),
					knownvalue.SetSizeExact(1)),
				statecheck.ExpectKnownValue("rubrik_azure_management_group.test", tfjsonpath.New(keyManagedSubscriptionFeatures),
					knownvalue.MapExact(map[string]knownvalue.Check{
						strings.ToLower(subscriptionIDs[0]): knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact(core.CloudDiscovery), knownvalue.StringExact(core.CloudNativeProtection),
						}),
					})),
				statecheck.ExpectIdentity("rubrik_azure_management_group.test", map[string]knownvalue.Check{
					keyID: knownvalue.StringExact(tenantDomain + "/" + managementGroupID),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_management_group.test", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that a subscription can be added in place.
			Config: config(subscriptionIDs[0], subscriptionIDs[1]),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_management_group.test", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_management_group.test", tfjsonpath.New(keySubscriptionIDs),
					knownvalue.SetSizeExact(2)),
			},
		}, {
			// Verify that a subscription can be removed in place.
			Config: config(subscriptionIDs[1]),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_management_group.test", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_management_group.test", tfjsonpath.New(keySubscriptionIDs),
					knownvalue.SetSizeExact(1)),
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_azure_management_group.test",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_azure_management_group.test",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_azure_management_group.test",
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		}},
	})
}

func TestAzureManagementGroupRegionUpdates(t *testing.T) {
	east := regions.RegionFromName("eastus2")
	west := regions.RegionFromName("westus2")
	central := regions.RegionFromName("centralus")
	id := uuid.New()
	row := func(feature string, names ...regions.Region) gqlazure.CloudAccount {
		return gqlazure.CloudAccount{
			ID: id, Name: "subscription", Feature: gqlazure.Feature{
				Feature: feature, Status: core.StatusConnected, Regions: gqlazure.RegionsToCloudAccountRegionEnum(names),
			},
		}
	}
	owned := []string{core.CloudDiscovery, core.CloudNativeProtection, core.Exocompute}
	tests := []struct {
		name     string
		accounts []gqlazure.CloudAccount
		wantAdd  []regions.Region
		wantErr  string
	}{
		{name: "Preserves discovery extras and excludes Exocompute", accounts: []gqlazure.CloudAccount{
			row(core.CloudDiscovery, east, central), row(core.CloudNativeProtection, east, central), row(core.Exocompute, central),
		}, wantAdd: []regions.Region{west}},
		{name: "Rejects alignment that drops a sibling's region", accounts: []gqlazure.CloudAccount{
			row(core.CloudDiscovery, east), row(core.CloudNativeProtection, east, central),
		}, wantErr: "would remove centralus"},
		{name: "Rejects alignment that changes unmanaged feature", accounts: []gqlazure.CloudAccount{
			row(core.CloudDiscovery, east), row(core.CloudNativeProtection, east), row(core.DSPMMetadata, east),
		}, wantErr: "would change unmanaged feature DSPM_METADATA"},
		{name: "Requires Cloud Discovery", accounts: []gqlazure.CloudAccount{
			row(core.CloudNativeProtection, east),
		}, wantErr: "require a connected CLOUD_DISCOVERY"},
		{name: "Resumes partial alignment", accounts: []gqlazure.CloudAccount{
			row(core.CloudDiscovery, east, west), row(core.CloudNativeProtection, east),
		}, wantAdd: []regions.Region{east, west}},
		{name: "Already aligned", accounts: []gqlazure.CloudAccount{
			row(core.CloudDiscovery, east, west), row(core.CloudNativeProtection, east, west),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := azureManagementGroupRegionUpdates(tt.accounts, owned, []regions.Region{east, west})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("wanted %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil || got.subscriptionID != id || !slices.Equal(got.toAdd, tt.wantAdd) || slices.Contains(got.align, core.Exocompute) {
				t.Fatalf("unexpected update: %+v, %v", got, err)
			}
			if len(tt.accounts[0].Feature.Regions) > 1 && slices.Contains(tt.accounts[0].Feature.Regions, central.ToCloudAccountRegionEnum()) && !slices.Contains(got.expected, central) {
				t.Fatal("did not preserve the existing discovery region")
			}
		})
	}
}
