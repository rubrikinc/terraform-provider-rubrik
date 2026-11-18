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
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	regions "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

func TestAzureManagementGroupReconcile(t *testing.T) {
	first, second, external := uuid.New(), uuid.New(), uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	protection := core.FeatureCloudNativeProtection
	archival := core.FeatureCloudNativeArchival
	connected := func(f core.Feature) azure.Feature { return azure.Feature{Feature: f, Status: core.StatusConnected} }
	accounts := []azure.CloudAccount{
		{ID: a, NativeID: first, Features: []azure.Feature{connected(protection), connected(archival)}},
		{ID: b, NativeID: second, Features: []azure.Feature{connected(protection)}},
		{ID: c, NativeID: external, Features: []azure.Feature{connected(protection), connected(archival)}},
	}
	owned := map[string][]string{first.String(): {protection.Name}, second.String(): {protection.Name, archival.Name}}
	present := azureManagementGroupPresentSubscriptions(accounts, owned, []attr.Value{types.StringValue(strings.ToUpper(first.String()))})
	if !slices.Equal(present, []string{strings.ToUpper(first.String())}) {
		t.Fatalf("refresh returned %v; external and partially onboarded subscriptions must not mask drift", present)
	}

	adds, removes, err := azureManagementGroupReconcile(accounts, owned, azureManagementGroupInput{
		subscriptions: []uuid.UUID{second},
		features:      []azureManagementGroupFeature{{name: protection.Name}, {name: archival.Name}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(adds) != 1 || adds[0].NativeID != second || len(adds[0].Features) != 1 || !adds[0].Features[0].Feature.Equal(archival) {
		t.Errorf("expected retry of only the missing pair, got %+v", adds)
	}
	if len(removes) != 1 || !slices.Equal(removes[protection.Name], []uuid.UUID{a}) {
		t.Errorf("update would affect unowned features: %v", removes)
	}
	_, removes, err = azureManagementGroupReconcile(accounts, owned, azureManagementGroupInput{})
	if err != nil || len(removes) != 1 || !slices.Equal(removes[protection.Name], []uuid.UUID{a, b}) {
		t.Errorf("destroy would affect unowned features: %v, %v", removes, err)
	}
	_, _, err = azureManagementGroupReconcile(accounts, owned, azureManagementGroupInput{
		subscriptions: []uuid.UUID{external}, features: []azureManagementGroupFeature{{name: protection.Name}},
	})
	if err == nil || !strings.Contains(err.Error(), "not managed") {
		t.Fatalf("expected explicitly requested unowned existing pair to be rejected, got %v", err)
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
	t.Run("permission groups", func(t *testing.T) {
		changed := old
		changed.features = []azureManagementGroupFeature{{name: core.FeatureCloudNativeProtection.Name, permissionGroups: []string{"BASIC", "EXPORT_AND_RESTORE"}}}
		if err := azureManagementGroupValidateUpdate(old, changed); err == nil {
			t.Fatal("accepted a permission group change")
		}
	})
}

// TestAzureManagementGroupSetModel verifies configuration and ownership are recovered on import.
func TestAzureManagementGroupSetModel(t *testing.T) {
	nativeID := uuid.New()
	data := azureManagementGroupData{
		appID: uuid.New(),
		group: azure.ManagementGroup{NativeID: "mg", Name: "MG", CustomerID: uuid.New()},
		accounts: []azure.CloudAccount{{
			NativeID: nativeID,
			Features: []azure.Feature{{
				Feature:       core.Feature{Name: core.FeatureCloudNativeArchival.Name, PermissionGroups: []core.PermissionGroup{"BASIC"}},
				Status:        core.StatusConnected,
				ResourceGroup: azure.FeatureResourceGroup{Name: "rg", Region: "eastus2"},
				Regions:       []string{"eastus2"},
			}},
		}},
	}

	// Imported, there is no prior state.
	imported := azureManagementGroupModel{
		ManagedSubscriptionFeatures: types.MapNull(types.SetType{ElemType: types.StringType}),
		Feature:                     types.SetNull(types.ObjectType{AttrTypes: azureManagementGroupFeatureAttrTypes}),
		DeleteSnapshotsOnDestroy:    types.BoolNull(),
	}
	if diags := data.setModel(t.Context(), &imported, "example.onmicrosoft.com", "mg"); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	var importedFeatures []azureManagementGroupFeatureModel
	imported.Feature.ElementsAs(t.Context(), &importedFeatures, false)
	if len(importedFeatures) != 1 || importedFeatures[0].ResourceGroupName.ValueString() != "rg" {
		t.Errorf("imported features: %+v", importedFeatures)
	}
	if imported.ID.ValueString() != "example.onmicrosoft.com/mg" || imported.AppID.ValueString() != data.appID.String() {
		t.Errorf("imported ID or app ID: %s, %s", imported.ID, imported.AppID)
	}

	input, diags := azureManagementGroupInputFromModel(t.Context(), imported)
	if diags.HasError() || !slices.Equal(input.managed[nativeID.String()], []string{core.FeatureCloudNativeArchival.Name}) {
		t.Fatalf("ownership did not round-trip: %+v, %v", input.managed, diags)
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

	config := func(subscriptions ...string) string {
		quoted := `"` + strings.Join(subscriptions, `", "`) + `"`
		return `
			resource "rubrik_azure_service_principal" "default" {
				credentials   = "` + testAzureCredentials(t) + `"
				tenant_domain = "` + tenantDomain + `"
			}

			resource "rubrik_azure_management_group" "test" {
				tenant_domain       = rubrik_azure_service_principal.default.tenant_domain
				app_id              = "` + appID + `"
				management_group_id = "` + managementGroupID + `"
				regions             = ["eastus2"]
				subscription_ids    = [` + quoted + `]

				feature {
					name              = "CLOUD_NATIVE_PROTECTION"
					permission_groups = ["BASIC"]
				}
			}
		`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
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
