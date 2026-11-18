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

func TestAzureManagementGroupDiff(t *testing.T) {
	sub1, sub2, sub3 := uuid.New(), uuid.New(), uuid.New()
	protection := azureManagementGroupFeature{name: "CLOUD_NATIVE_PROTECTION", permissionGroups: []string{"BASIC"}}
	exocompute := azureManagementGroupFeature{name: "EXOCOMPUTE", permissionGroups: []string{"BASIC"}}
	eastus2 := []regions.Region{regions.RegionFromName("eastus2")}

	tests := []struct {
		name       string
		oldInput   azureManagementGroupInput
		newInput   azureManagementGroupInput
		want       azureManagementGroupChanges
		wantErrMsg string
	}{{
		name:     "AddAndRemoveSubscriptions",
		oldInput: azureManagementGroupInput{regions: eastus2, subscriptions: []uuid.UUID{sub1, sub2}, features: []azureManagementGroupFeature{protection}},
		newInput: azureManagementGroupInput{regions: eastus2, subscriptions: []uuid.UUID{sub2, sub3}, features: []azureManagementGroupFeature{protection}},
		want: azureManagementGroupChanges{
			addSubscriptions:    []uuid.UUID{sub3},
			removeSubscriptions: []uuid.UUID{sub1},
		},
	}, {
		name:     "AddAndRemoveFeatures",
		oldInput: azureManagementGroupInput{regions: eastus2, subscriptions: []uuid.UUID{sub1}, features: []azureManagementGroupFeature{protection}},
		newInput: azureManagementGroupInput{regions: eastus2, subscriptions: []uuid.UUID{sub1}, features: []azureManagementGroupFeature{exocompute}},
		want: azureManagementGroupChanges{
			addFeatures:    []azureManagementGroupFeature{exocompute},
			removeFeatures: []string{"CLOUD_NATIVE_PROTECTION"},
		},
	}, {
		name:     "ChangePermissionGroups",
		oldInput: azureManagementGroupInput{regions: eastus2, subscriptions: []uuid.UUID{sub1}, features: []azureManagementGroupFeature{protection}},
		newInput: azureManagementGroupInput{regions: eastus2, subscriptions: []uuid.UUID{sub1}, features: []azureManagementGroupFeature{
			{name: "CLOUD_NATIVE_PROTECTION", permissionGroups: []string{"BASIC", "EXPORT_AND_RESTORE"}},
		}},
		wantErrMsg: "permission groups",
	}, {
		name:       "ChangeRegions",
		oldInput:   azureManagementGroupInput{regions: eastus2, subscriptions: []uuid.UUID{sub1}, features: []azureManagementGroupFeature{protection}},
		newInput:   azureManagementGroupInput{regions: []regions.Region{regions.RegionFromName("westus2")}, subscriptions: []uuid.UUID{sub1}, features: []azureManagementGroupFeature{protection}},
		wantErrMsg: "regions",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := azureManagementGroupDiff(tc.oldInput, tc.newInput)
			if tc.wantErrMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Fatalf("got error %v, want it to contain %q", err, tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.addSubscriptions) != len(tc.want.addSubscriptions) || len(got.removeSubscriptions) != len(tc.want.removeSubscriptions) ||
				len(got.addFeatures) != len(tc.want.addFeatures) || len(got.removeFeatures) != len(tc.want.removeFeatures) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got.addSubscriptions {
				if got.addSubscriptions[i] != tc.want.addSubscriptions[i] {
					t.Errorf("addSubscriptions: got %v, want %v", got.addSubscriptions, tc.want.addSubscriptions)
				}
			}
			for i := range got.removeSubscriptions {
				if got.removeSubscriptions[i] != tc.want.removeSubscriptions[i] {
					t.Errorf("removeSubscriptions: got %v, want %v", got.removeSubscriptions, tc.want.removeSubscriptions)
				}
			}
			for i := range got.addFeatures {
				if got.addFeatures[i].name != tc.want.addFeatures[i].name {
					t.Errorf("addFeatures: got %v, want %v", got.addFeatures, tc.want.addFeatures)
				}
			}
			for i := range got.removeFeatures {
				if got.removeFeatures[i] != tc.want.removeFeatures[i] {
					t.Errorf("removeFeatures: got %v, want %v", got.removeFeatures, tc.want.removeFeatures)
				}
			}
		})
	}
}

// TestAzureManagementGroupSetModel verifies that the resource group RSC returns
// for a feature is kept out of the state when it was not configured, and that
// a model without prior state, as after an import, gets everything RSC returns.
func TestAzureManagementGroupSetModel(t *testing.T) {
	nativeID := uuid.New()
	data := azureManagementGroupData{
		appID: uuid.New(),
		group: azure.ManagementGroup{NativeID: "mg", Name: "MG", CustomerID: uuid.New()},
		accounts: []azure.CloudAccount{{
			NativeID: nativeID,
			Features: []azure.Feature{{
				Feature:       core.Feature{Name: "CLOUD_NATIVE_PROTECTION", PermissionGroups: []core.PermissionGroup{"BASIC"}},
				ResourceGroup: azure.FeatureResourceGroup{Name: "rg", Region: "eastus2"},
				Regions:       []string{"eastus2"},
			}},
		}},
	}

	// Imported, there is no prior state.
	imported := azureManagementGroupModel{
		Feature:                  types.SetNull(types.ObjectType{AttrTypes: azureManagementGroupFeatureAttrTypes}),
		DeleteSnapshotsOnDestroy: types.BoolNull(),
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

	// Applied without a resource group, the resource group is not stored.
	applied := imported
	applied.Feature = types.SetValueMust(imported.Feature.ElementType(t.Context()), []attr.Value{
		types.ObjectValueMust(azureManagementGroupFeatureAttrTypes, map[string]attr.Value{
			keyName:                types.StringValue("CLOUD_NATIVE_PROTECTION"),
			keyPermissionGroups:    importedFeatures[0].PermissionGroups,
			keyResourceGroupName:   types.StringNull(),
			keyResourceGroupRegion: types.StringNull(),
			keyResourceGroupTags:   types.MapNull(types.StringType),
		}),
	})
	if diags := data.setModel(t.Context(), &applied, "example.onmicrosoft.com", "mg"); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	var appliedFeatures []azureManagementGroupFeatureModel
	applied.Feature.ElementsAs(t.Context(), &appliedFeatures, false)
	if len(appliedFeatures) != 1 || !appliedFeatures[0].ResourceGroupName.IsNull() {
		t.Errorf("applied features: %+v", appliedFeatures)
	}
}

func TestAccAzureManagementGroupResource(t *testing.T) {
	skipUnlessFeatureEnabled(t, core.FeatureFlagAzureCloudAccountsManagementGroup)

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
