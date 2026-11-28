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
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/hierarchy"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// TestSLADomainAssignmentResourceSchema verifies that the migrated resource
// keeps the SDKv2 type names, schema version and deprecation, and that only the
// polaris alias is deprecated.
func TestSLADomainAssignmentResourceSchema(t *testing.T) {
	tests := []struct {
		name           string
		newResource    func() fwresource.Resource
		wantTypeName   string
		wantDeprecated bool
	}{
		{name: "rubrik", newResource: newSLADomainAssignmentResource, wantTypeName: "rubrik_sla_domain_assignment"},
		{
			name: "polaris alias", newResource: newPolarisSLADomainAssignmentResource,
			wantTypeName: "polaris_sla_domain_assignment", wantDeprecated: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.newResource()

			var meta fwresource.MetadataResponse
			r.Metadata(t.Context(), fwresource.MetadataRequest{}, &meta)
			if meta.TypeName != tc.wantTypeName {
				t.Errorf("type name = %q, want %q", meta.TypeName, tc.wantTypeName)
			}

			var res fwresource.SchemaResponse
			r.Schema(t.Context(), fwresource.SchemaRequest{}, &res)
			if diags := res.Schema.ValidateImplementation(t.Context()); diags.HasError() {
				t.Fatalf("invalid schema: %v", diags)
			}
			if got := res.Schema.DeprecationMessage != ""; got != tc.wantDeprecated {
				t.Errorf("deprecated = %t, want %t", got, tc.wantDeprecated)
			}

			// The SDKv2 schema version, so existing state is accepted as is.
			if res.Schema.Version != 1 {
				t.Errorf("schema version = %d, want 1", res.Schema.Version)
			}

			// The resource has no identity, since its ID is not unique.
			if _, ok := r.(fwresource.ResourceWithIdentity); ok {
				t.Error("the resource must not implement ResourceWithIdentity")
			}

			// The object IDs and the optional attributes with defaults.
			for _, key := range []string{keyAssignmentType, keyApplyChangesToExistingSnapshots,
				keyApplyChangesToNonPolicySnapshots} {
				attr, ok := res.Schema.Attributes[key]
				if !ok || !attr.IsOptional() || !attr.IsComputed() {
					t.Errorf("attribute %q must be optional and computed, since it has a default", key)
				}
			}
			if attr, ok := res.Schema.Attributes[keyObjectIDs]; !ok || !attr.IsRequired() {
				t.Errorf("attribute %q must be required", keyObjectIDs)
			}
		})
	}
}

func TestValidateSLADomainAssignmentConfig(t *testing.T) {
	id := types.StringValue("11111111-1111-1111-1111-111111111111")
	null := types.StringNull()

	tests := []struct {
		name    string
		config  slaDomainAssignmentModel
		wantErr string
	}{
		{
			name: "ProtectWithDefaults",
			config: slaDomainAssignmentModel{
				AssignmentType: null, SLADomainID: id, ExistingSnapshotRetention: null,
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
		},
		{
			name: "ProtectWithoutDomain",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringValue("protectWithSlaId"), SLADomainID: null,
				ExistingSnapshotRetention:       null,
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
			wantErr: "sla_domain_id is required",
		},
		{
			name: "DefaultTypeWithoutDomain",
			config: slaDomainAssignmentModel{
				AssignmentType: null, SLADomainID: null, ExistingSnapshotRetention: null,
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
			wantErr: "sla_domain_id is required",
		},
		{
			name: "ProtectWithRetention",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringValue("protectWithSlaId"), SLADomainID: id,
				ExistingSnapshotRetention:       types.StringValue("KEEP_FOREVER"),
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
			wantErr: "existing_snapshot_retention is only valid",
		},
		{
			name: "NonPolicyWithoutExisting",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringValue("protectWithSlaId"), SLADomainID: id, ExistingSnapshotRetention: null,
				ApplyChangesToExistingSnapshots:  types.BoolValue(false),
				ApplyChangesToNonPolicySnapshots: types.BoolValue(true),
			},
			wantErr: "requires apply_changes_to_existing_snapshots to be true",
		},
		{
			// An unset apply_changes_to_existing_snapshots defaults to true.
			name: "NonPolicyWithDefaultExisting",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringValue("protectWithSlaId"), SLADomainID: id, ExistingSnapshotRetention: null,
				ApplyChangesToExistingSnapshots:  types.BoolNull(),
				ApplyChangesToNonPolicySnapshots: types.BoolValue(true),
			},
		},
		{
			name: "DoNotProtect",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringValue("doNotProtect"), SLADomainID: null,
				ExistingSnapshotRetention:       types.StringValue("RETAIN_SNAPSHOTS"),
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
		},
		{
			name: "DoNotProtectWithDomain",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringValue("doNotProtect"), SLADomainID: id, ExistingSnapshotRetention: null,
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
			wantErr: "sla_domain_id must be empty",
		},
		{
			name: "UnknownTypeIsNotChecked",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringUnknown(), SLADomainID: null, ExistingSnapshotRetention: null,
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
		},
		{
			name: "UnknownDomainIsNotRequiredToBeKnown",
			config: slaDomainAssignmentModel{
				AssignmentType: types.StringValue("doNotProtect"), SLADomainID: types.StringUnknown(),
				ExistingSnapshotRetention:       null,
				ApplyChangesToExistingSnapshots: types.BoolNull(), ApplyChangesToNonPolicySnapshots: types.BoolNull(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateSLADomainAssignmentConfig(tc.config)
			if tc.wantErr == "" {
				if diags.HasError() {
					t.Fatalf("unexpected error: %v", diags)
				}
				return
			}
			if !diags.HasError() {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			found := false
			for _, d := range diags.Errors() {
				if strings.Contains(d.Summary()+" "+d.Detail(), tc.wantErr) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected an error containing %q, got %v", tc.wantErr, diags)
			}
		})
	}
}

// TestUpgradeSLADomainAssignmentStateV0 verifies that version 0 state gets the
// values the SDKv2 state upgrader gave the attributes added in version 1. It is
// also what the version 0 state mover uses.
func TestUpgradeSLADomainAssignmentStateV0(t *testing.T) {
	objectIDs := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("22222222-2222-2222-2222-222222222222")})
	prior := slaDomainAssignmentModelV0{
		ID:          types.StringValue("11111111-1111-1111-1111-111111111111"),
		ObjectIDs:   objectIDs,
		SLADomainID: types.StringValue("11111111-1111-1111-1111-111111111111"),
	}

	got := upgradeSLADomainAssignmentStateV0(prior)

	if !got.ID.Equal(prior.ID) || !got.SLADomainID.Equal(prior.SLADomainID) || !got.ObjectIDs.Equal(prior.ObjectIDs) {
		t.Errorf("the id, sla_domain_id and object_ids must be kept, got %+v", got)
	}
	if got.AssignmentType.ValueString() != string(gqlsla.ProtectWithSLA) {
		t.Errorf("assignment_type = %q, want %q", got.AssignmentType.ValueString(), gqlsla.ProtectWithSLA)
	}
	if !got.ApplyChangesToExistingSnapshots.ValueBool() {
		t.Error("apply_changes_to_existing_snapshots must be true")
	}
	if got.ApplyChangesToNonPolicySnapshots.IsNull() || got.ApplyChangesToNonPolicySnapshots.ValueBool() {
		t.Error("apply_changes_to_non_policy_snapshots must be false, and not null")
	}
	if !got.ExistingSnapshotRetention.IsNull() || !got.Workload.IsNull() {
		t.Error("existing_snapshot_retention and workload must be null")
	}
}

func TestAssignParamsFromModel(t *testing.T) {
	domainID := "11111111-1111-1111-1111-111111111111"
	objectIDs := []uuid.UUID{uuid.MustParse("22222222-2222-2222-2222-222222222222")}

	t.Run("Protect", func(t *testing.T) {
		model := slaDomainAssignmentModel{
			AssignmentType: types.StringValue("protectWithSlaId"), SLADomainID: types.StringValue(domainID),
			ApplyChangesToExistingSnapshots: types.BoolValue(true), ApplyChangesToNonPolicySnapshots: types.BoolValue(false),
			ExistingSnapshotRetention: types.StringNull(),
		}
		params, expected, err := assignParamsFromModel(model, objectIDs, hierarchy.WorkloadAllSubHierarchyType)
		if err != nil {
			t.Fatal(err)
		}
		if expected != domainID || params.DomainID == nil || params.DomainID.String() != domainID {
			t.Errorf("expected domain %q, got %q and %v", domainID, expected, params.DomainID)
		}
		if params.ApplyToExistingSnapshots == nil || !*params.ApplyToExistingSnapshots ||
			params.ApplyToNonPolicySnapshots == nil || *params.ApplyToNonPolicySnapshots {
			t.Errorf("unexpected apply settings: %v %v", params.ApplyToExistingSnapshots, params.ApplyToNonPolicySnapshots)
		}
	})

	t.Run("DoNotProtectWithRetention", func(t *testing.T) {
		model := slaDomainAssignmentModel{
			AssignmentType: types.StringValue("doNotProtect"), SLADomainID: types.StringNull(),
			ExistingSnapshotRetention: types.StringValue("KEEP_FOREVER"),
		}
		params, expected, err := assignParamsFromModel(model, objectIDs, hierarchy.WorkloadAllSubHierarchyType)
		if err != nil {
			t.Fatal(err)
		}
		if expected != gqlsla.DoNotProtectSLAID || params.DomainID != nil {
			t.Errorf("expected %q and no domain, got %q and %v", gqlsla.DoNotProtectSLAID, expected, params.DomainID)
		}
		if params.ExistingSnapshotRetention != gqlsla.KeepForever {
			t.Errorf("retention = %q, want %q", params.ExistingSnapshotRetention, gqlsla.KeepForever)
		}
	})

	t.Run("ProtectWithoutDomain", func(t *testing.T) {
		model := slaDomainAssignmentModel{AssignmentType: types.StringValue("protectWithSlaId")}
		if _, _, err := assignParamsFromModel(model, objectIDs, hierarchy.WorkloadAllSubHierarchyType); err == nil {
			t.Error("expected an error for a missing SLA domain ID")
		}
	})
}

func TestDiffObjectIDs(t *testing.T) {
	a, b, c := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333"
	set := func(ids ...string) types.Set {
		elems := make([]attr.Value, 0, len(ids))
		for _, id := range ids {
			elems = append(elems, types.StringValue(id))
		}
		return types.SetValueMust(types.StringType, elems)
	}
	strs := func(ids []uuid.UUID) []string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			out = append(out, id.String())
		}
		slices.Sort(out)
		return out
	}

	add, remove, total, diags := diffObjectIDs(t.Context(), set(a, b), set(b, c))
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := strs(add); !slices.Equal(got, []string{c}) {
		t.Errorf("add = %v, want [%s]", got, c)
	}
	if got := strs(remove); !slices.Equal(got, []string{a}) {
		t.Errorf("remove = %v, want [%s]", got, a)
	}
	if got := strs(total); !slices.Equal(got, []string{b, c}) {
		t.Errorf("total = %v, want [%s %s]", got, b, c)
	}
}

// TestAccSLADomainAssignmentResource_PlanValidation verifies that the plan-time
// rules are reported at plan time. The SDKv2 resource only checked some of them
// when applying. The steps only plan, so they create nothing in RSC.
func TestAccSLADomainAssignmentResource_PlanValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// protectWithSlaId, which is the default, requires an SLA domain.
			Config: `
				resource "rubrik_sla_domain_assignment" "test" {
					object_ids = ["22222222-2222-2222-2222-222222222222"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`sla_domain_id is required`),
		}, {
			// doNotProtect doesn't allow an SLA domain.
			Config: `
				resource "rubrik_sla_domain_assignment" "test" {
					assignment_type = "doNotProtect"
					sla_domain_id   = "11111111-1111-1111-1111-111111111111"
					object_ids      = ["22222222-2222-2222-2222-222222222222"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`sla_domain_id must be empty when assignment_type is`),
		}, {
			// The object IDs must be UUIDs.
			Config: `
				resource "rubrik_sla_domain_assignment" "test" {
					sla_domain_id = "11111111-1111-1111-1111-111111111111"
					object_ids    = ["not-a-uuid"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid UUID`),
		}, {
			// At least one object ID is required.
			Config: `
				resource "rubrik_sla_domain_assignment" "test" {
					sla_domain_id = "11111111-1111-1111-1111-111111111111"
					object_ids    = []
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`at least 1`),
		}, {
			// The assignment type must be one of the two values.
			Config: `
				resource "rubrik_sla_domain_assignment" "test" {
					assignment_type = "other"
					sla_domain_id   = "11111111-1111-1111-1111-111111111111"
					object_ids      = ["22222222-2222-2222-2222-222222222222"]
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		}},
	})
}
