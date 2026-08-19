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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	gqldspm "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/dspm"
)

// The filter types used by the test cases below.
const (
	testObjectFilterType   = "SECURITY_SNAPPABLE_NAME"
	testDocumentFilterType = "SECURITY_DOCUMENT_DATA_CATEGORY"
	testIdentityFilterType = "SECURITY_IDENTITY_NAME"

	// The condition a threshold filter holds in practice.
	testHitCountFilterType = "SECURITY_DOCUMENT_HIT_COUNT"
)

// testCondition builds a leaf node holding a single condition of the given
// filter type.
func testCondition(filterType string) gqldspm.Node {
	return gqldspm.Node{Config: &gqldspm.Config{
		Type:         gqldspm.FilterType(filterType),
		Values:       []string{"x"},
		Relationship: gqldspm.RelIs,
	}}
}

// testGroup builds a node holding a group with the given operator and children.
func testGroup(op gqldspm.LogicalOp, children ...gqldspm.Node) gqldspm.Node {
	return gqldspm.Node{GroupConfig: &gqldspm.GroupConfig{Op: op, Filters: children}}
}

// testRoot builds a top-level group with the given operator and children.
func testRoot(op gqldspm.LogicalOp, children ...gqldspm.Node) *gqldspm.GroupConfig {
	return &gqldspm.GroupConfig{Op: op, Filters: children}
}

// testConditionGroup builds a condition group block holding one condition of
// the given filter type.
func testConditionGroup(op, filterType string) []conditionGroupModel {
	values, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"x"})

	return []conditionGroupModel{{
		Operator: types.StringValue(op),
		Condition: []conditionModel{{
			FilterType:   types.StringValue(filterType),
			Relationship: types.StringValue("IS"),
			Values:       values,
		}},
	}}
}

func TestResourceTypeOf(t *testing.T) {
	tests := []struct {
		filterType   string
		resourceType filterResourceType
		supported    bool
	}{
		{filterType: "SECURITY_DOCUMENT_SENSITIVITY", resourceType: filterResourceTypeObject, supported: true},
		{filterType: "SECURITY_SNAPPABLE_BACKUP", resourceType: filterResourceTypeObject, supported: true},
		{filterType: "SECURITY_IDENTITY_NAME", resourceType: filterResourceTypeIdentity, supported: true},

		// RSC registers the GPO family as identity conditions, but the SDK does
		// not name them yet. The prefix classifier must still place them.
		{filterType: "SECURITY_GPO_LDAP_SIGNING", resourceType: filterResourceTypeIdentity, supported: true},

		// Data security policies cannot use IDP conditions.
		{filterType: "SECURITY_IDP_TYPE", supported: false},

		{filterType: "", supported: false},
		{filterType: "SECURITY_UNKNOWN_THING", supported: false},

		// The prefixes are matched in full: a filter type missing the SECURITY_
		// prefix is not an RSC filter type.
		{filterType: "DOCUMENT_SENSITIVITY", supported: false},
	}

	for _, tc := range tests {
		t.Run(tc.filterType, func(t *testing.T) {
			resourceType, ok := resourceTypeOf(tc.filterType)
			if ok != tc.supported {
				t.Fatalf("supported = %v, want %v", ok, tc.supported)
			}
			if ok && resourceType != tc.resourceType {
				t.Errorf("resource type = %q, want %q", resourceType, tc.resourceType)
			}
		})
	}
}

func TestToPolicyFilter(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		object     []conditionGroupModel
		identity   []conditionGroupModel
		groupCount int
		groupOps   []gqldspm.LogicalOp
	}{{
		name:       "ObjectOnly",
		object:     testConditionGroup("AND", testObjectFilterType),
		groupCount: 1,
		groupOps:   []gqldspm.LogicalOp{gqldspm.LogicalAnd},
	}, {
		name:       "IdentityOnly",
		identity:   testConditionGroup("OR", testIdentityFilterType),
		groupCount: 1,
		groupOps:   []gqldspm.LogicalOp{gqldspm.LogicalOr},
	}, {
		// The object group is emitted first regardless of the order the blocks
		// appear in, so the order RSC reads back is stable.
		name:       "BothGroupsObjectFirst",
		object:     testConditionGroup("OR", testObjectFilterType),
		identity:   testConditionGroup("AND", testIdentityFilterType),
		groupCount: 2,
		groupOps:   []gqldspm.LogicalOp{gqldspm.LogicalOr, gqldspm.LogicalAnd},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, diags := toPolicyFilter(ctx, tc.object, tc.identity)
			if diags.HasError() {
				t.Fatalf("unexpected error: %s", diags.Errors())
			}

			if root.Op != gqldspm.LogicalAnd {
				t.Errorf("root operator = %q, want AND", root.Op)
			}
			if len(root.Filters) != tc.groupCount {
				t.Fatalf("root holds %d children, want %d", len(root.Filters), tc.groupCount)
			}

			for i, node := range root.Filters {
				if node.GroupConfig == nil {
					t.Fatalf("child %d is a bare condition, want a group", i)
				}
				if node.Config != nil {
					t.Errorf("child %d holds both a condition and a group", i)
				}
				if node.GroupConfig.Op != tc.groupOps[i] {
					t.Errorf("child %d operator = %q, want %q", i, node.GroupConfig.Op, tc.groupOps[i])
				}
			}
		})
	}
}

// TestFromPolicyFilter covers the filter shapes RSC accepts and rejects.
func TestFromPolicyFilter(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		root        *gqldspm.GroupConfig
		wantErr     bool
		wantObject  bool
		wantIdenity bool
	}{
		// Accepted shapes.
		{
			name:       "ObjectGroupWithAnd",
			root:       testRoot(gqldspm.LogicalAnd, testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType))),
			wantObject: true,
		}, {
			name:        "IdentityGroupWithOr",
			root:        testRoot(gqldspm.LogicalAnd, testGroup(gqldspm.LogicalOr, testCondition(testIdentityFilterType))),
			wantIdenity: true,
		}, {
			name: "ObjectAndIdentityGroups",
			root: testRoot(gqldspm.LogicalAnd,
				testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType)),
				testGroup(gqldspm.LogicalOr, testCondition(testIdentityFilterType))),
			wantObject:  true,
			wantIdenity: true,
		}, {
			// Snappable and document conditions are both object conditions, so
			// they may share a group.
			name: "ObjectGroupMixingSnappableAndDocument",
			root: testRoot(gqldspm.LogicalAnd, testGroup(gqldspm.LogicalAnd,
				testCondition(testObjectFilterType),
				testCondition(testDocumentFilterType))),
			wantObject: true,
		}, {
			// The identity group may come first on the wire.
			name: "IdentityGroupFirst",
			root: testRoot(gqldspm.LogicalAnd,
				testGroup(gqldspm.LogicalOr, testCondition(testIdentityFilterType)),
				testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType))),
			wantObject:  true,
			wantIdenity: true,
		},

		// Rejected shapes.
		{
			name:    "NoFilter",
			root:    nil,
			wantErr: true,
		}, {
			name:    "TopLevelOr",
			root:    testRoot(gqldspm.LogicalOr, testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType))),
			wantErr: true,
		}, {
			name:    "EmptyRootGroup",
			root:    testRoot(gqldspm.LogicalAnd),
			wantErr: true,
		}, {
			// The flat shape the provider used to send.
			name:    "BareConditionChild",
			root:    testRoot(gqldspm.LogicalAnd, testCondition(testObjectFilterType)),
			wantErr: true,
		}, {
			name: "ThreeGroups",
			root: testRoot(gqldspm.LogicalAnd,
				testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType)),
				testGroup(gqldspm.LogicalOr, testCondition(testIdentityFilterType)),
				testGroup(gqldspm.LogicalAnd, testCondition(testDocumentFilterType))),
			wantErr: true,
		}, {
			name: "GroupInGroup",
			root: testRoot(gqldspm.LogicalAnd, testGroup(gqldspm.LogicalAnd,
				testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType)))),
			wantErr: true,
		}, {
			name:    "EmptyChildGroup",
			root:    testRoot(gqldspm.LogicalAnd, testGroup(gqldspm.LogicalAnd)),
			wantErr: true,
		}, {
			name: "TwoObjectGroups",
			root: testRoot(gqldspm.LogicalAnd,
				testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType)),
				testGroup(gqldspm.LogicalAnd, testCondition(testDocumentFilterType))),
			wantErr: true,
		}, {
			name: "GroupMixingObjectAndIdentity",
			root: testRoot(gqldspm.LogicalAnd, testGroup(gqldspm.LogicalAnd,
				testCondition(testObjectFilterType),
				testCondition(testIdentityFilterType))),
			wantErr: true,
		}, {
			name:    "UnsupportedConditionType",
			root:    testRoot(gqldspm.LogicalAnd, testGroup(gqldspm.LogicalAnd, testCondition("SECURITY_IDP_TYPE"))),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			object, identity, diags := fromPolicyFilter(ctx, tc.root)
			if tc.wantErr {
				if !diags.HasError() {
					t.Fatal("expected an error, got none")
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected error: %s", diags.Errors())
			}

			if got := object != nil; got != tc.wantObject {
				t.Errorf("object filter present = %v, want %v", got, tc.wantObject)
			}
			if got := identity != nil; got != tc.wantIdenity {
				t.Errorf("identity filter present = %v, want %v", got, tc.wantIdenity)
			}
		})
	}
}

func TestFromFilterGroupConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("Absent", func(t *testing.T) {
		group, diags := fromFilterGroupConfig(ctx, nil)
		if diags.HasError() {
			t.Fatalf("unexpected error: %s", diags.Errors())
		}
		if group != nil {
			t.Errorf("group = %v, want nil", group)
		}
	})

	t.Run("FlatGroup", func(t *testing.T) {
		group, diags := fromFilterGroupConfig(ctx, testRoot(gqldspm.LogicalOr,
			testCondition(testObjectFilterType)))
		if diags.HasError() {
			t.Fatalf("unexpected error: %s", diags.Errors())
		}
		if len(group) != 1 {
			t.Fatalf("group holds %d blocks, want 1", len(group))
		}
		if op := group[0].Operator.ValueString(); op != "OR" {
			t.Errorf("operator = %q, want OR", op)
		}
		if len(group[0].Condition) != 1 {
			t.Fatalf("group holds %d conditions, want 1", len(group[0].Condition))
		}
		if got, want := group[0].Condition[0].FilterType.ValueString(), testObjectFilterType; got != want {
			t.Errorf("filter type = %q, want %q", got, want)
		}
	})

	t.Run("NestedGroup", func(t *testing.T) {
		_, diags := fromFilterGroupConfig(ctx, testRoot(gqldspm.LogicalAnd,
			testGroup(gqldspm.LogicalAnd, testCondition(testObjectFilterType))))
		if !diags.HasError() {
			t.Error("expected an error for a nested group, got none")
		}
	})
}

// testThresholdCondition builds the threshold_filter block holding a single hit
// count condition.
func testThresholdCondition() []conditionModel {
	values, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"100"})

	return []conditionModel{{
		FilterType:   types.StringValue(testHitCountFilterType),
		Relationship: types.StringValue("GREATER_THAN"),
		Values:       values,
	}}
}

func TestToThresholdGroupConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("Absent", func(t *testing.T) {
		gc, diags := toThresholdGroupConfig(ctx, nil)
		if diags.HasError() {
			t.Fatalf("unexpected error: %s", diags.Errors())
		}
		if gc != nil {
			t.Errorf("group = %v, want nil", gc)
		}
	})

	// The block has no operator, but RSC rejects a group without one at
	// evaluation time, so AND must be sent.
	t.Run("SingleCondition", func(t *testing.T) {
		gc, diags := toThresholdGroupConfig(ctx, testThresholdCondition())
		if diags.HasError() {
			t.Fatalf("unexpected error: %s", diags.Errors())
		}
		if gc == nil {
			t.Fatal("group = nil, want a group")
		}
		if gc.Op != gqldspm.LogicalAnd {
			t.Errorf("operator = %q, want AND", gc.Op)
		}
		if len(gc.Filters) != 1 {
			t.Fatalf("group holds %d conditions, want 1", len(gc.Filters))
		}
		if gc.Filters[0].Config == nil {
			t.Fatal("condition is a group, want a bare condition")
		}
		if got, want := string(gc.Filters[0].Config.Type), testHitCountFilterType; got != want {
			t.Errorf("filter type = %q, want %q", got, want)
		}
	})
}

func TestFromThresholdGroupConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("Absent", func(t *testing.T) {
		threshold, diags := fromThresholdGroupConfig(ctx, nil)
		if diags.HasError() {
			t.Fatalf("unexpected error: %s", diags.Errors())
		}
		if threshold != nil {
			t.Errorf("threshold filter = %v, want nil", threshold)
		}
	})

	t.Run("EmptyGroup", func(t *testing.T) {
		threshold, diags := fromThresholdGroupConfig(ctx, testRoot(gqldspm.LogicalAnd))
		if diags.HasError() {
			t.Fatalf("unexpected error: %s", diags.Errors())
		}
		if threshold != nil {
			t.Errorf("threshold filter = %v, want nil", threshold)
		}
	})

	t.Run("SingleCondition", func(t *testing.T) {
		threshold, diags := fromThresholdGroupConfig(ctx, testRoot(gqldspm.LogicalAnd,
			testCondition(testHitCountFilterType)))
		if diags.HasError() {
			t.Fatalf("unexpected error: %s", diags.Errors())
		}
		if len(threshold) != 1 {
			t.Fatalf("threshold filter holds %d conditions, want 1", len(threshold))
		}
		if got, want := threshold[0].FilterType.ValueString(), testHitCountFilterType; got != want {
			t.Errorf("filter type = %q, want %q", got, want)
		}
	})

	// RSC does not enforce a single threshold condition, so a policy created
	// outside of the policy editor can hold more than one.
	t.Run("MultipleConditions", func(t *testing.T) {
		_, diags := fromThresholdGroupConfig(ctx, testRoot(gqldspm.LogicalAnd,
			testCondition(testHitCountFilterType),
			testCondition(testHitCountFilterType)))
		if !diags.HasError() {
			t.Error("expected an error for multiple threshold conditions, got none")
		}
	})

	t.Run("NestedGroup", func(t *testing.T) {
		_, diags := fromThresholdGroupConfig(ctx, testRoot(gqldspm.LogicalAnd,
			testGroup(gqldspm.LogicalAnd, testCondition(testHitCountFilterType))))
		if !diags.HasError() {
			t.Error("expected an error for a nested group, got none")
		}
	})
}

// TestThresholdGroupConfigRoundTrip verifies that a threshold filter written to
// RSC and read back lands in the same block, so a policy does not produce a
// permanent diff.
func TestThresholdGroupConfigRoundTrip(t *testing.T) {
	ctx := context.Background()

	gc, diags := toThresholdGroupConfig(ctx, testThresholdCondition())
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}

	threshold, diags := fromThresholdGroupConfig(ctx, gc)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}

	if len(threshold) != 1 {
		t.Fatalf("threshold filter holds %d conditions, want 1", len(threshold))
	}
	if got, want := threshold[0].FilterType.ValueString(), testHitCountFilterType; got != want {
		t.Errorf("filter type = %q, want %q", got, want)
	}
	if got, want := threshold[0].Relationship.ValueString(), "GREATER_THAN"; got != want {
		t.Errorf("relationship = %q, want %q", got, want)
	}
}

// TestConditionWithoutValuesRoundTrip verifies that a condition using a
// relationship taking no values, such as EXISTS, round-trips without a
// permanent diff: the omitted values attribute is sent as an empty list and
// read back as null.
func TestConditionWithoutValuesRoundTrip(t *testing.T) {
	ctx := context.Background()

	object := []conditionGroupModel{{
		Operator: types.StringValue("AND"),
		Condition: []conditionModel{{
			FilterType:   types.StringValue(testObjectFilterType),
			Relationship: types.StringValue("EXISTS"),
			Values:       types.ListNull(types.StringType),
		}},
	}}

	root, diags := toPolicyFilter(ctx, object, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}

	// RSC expects an empty list rather than null on the wire.
	config := root.Filters[0].GroupConfig.Filters[0].Config
	if config.Values == nil {
		t.Fatal("values = nil, want an empty list")
	}
	if len(config.Values) != 0 {
		t.Errorf("values = %v, want an empty list", config.Values)
	}

	gotObject, _, diags := fromPolicyFilter(ctx, &root)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}
	if values := gotObject[0].Condition[0].Values; !values.IsNull() {
		t.Errorf("values = %v, want null", values)
	}
}

// TestPolicyFilterRoundTrip verifies that a filter written to RSC and read
// back lands in the same blocks, so a policy does not produce a permanent diff.
func TestPolicyFilterRoundTrip(t *testing.T) {
	ctx := context.Background()

	object := testConditionGroup("OR", testObjectFilterType)
	identity := testConditionGroup("AND", testIdentityFilterType)

	root, diags := toPolicyFilter(ctx, object, identity)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}

	gotObject, gotIdentity, diags := fromPolicyFilter(ctx, &root)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}

	if got, want := gotObject[0].Operator.ValueString(), "OR"; got != want {
		t.Errorf("object operator = %q, want %q", got, want)
	}
	if got, want := gotObject[0].Condition[0].FilterType.ValueString(), testObjectFilterType; got != want {
		t.Errorf("object filter type = %q, want %q", got, want)
	}
	if got, want := gotIdentity[0].Operator.ValueString(), "AND"; got != want {
		t.Errorf("identity operator = %q, want %q", got, want)
	}
	if got, want := gotIdentity[0].Condition[0].FilterType.ValueString(), testIdentityFilterType; got != want {
		t.Errorf("identity filter type = %q, want %q", got, want)
	}
}
