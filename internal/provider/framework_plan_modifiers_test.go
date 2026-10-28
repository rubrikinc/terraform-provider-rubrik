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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
)

func TestRequiresReplaceUnlessImported(t *testing.T) {
	elementType := types.ObjectType{AttrTypes: map[string]attr.Type{"version": types.StringType}}
	elementList := func(versions ...string) types.List {
		var elements []attr.Value
		for _, version := range versions {
			elements = append(elements, types.ObjectValueMust(elementType.AttrTypes, map[string]attr.Value{
				"version": types.StringValue(version),
			}))
		}
		return types.ListValueMust(elementType, elements)
	}

	// The modifier reads encryption_type from the state, so the state needs a
	// schema holding that attribute.
	stateSchema := schema.Schema{Attributes: map[string]schema.Attribute{
		keyEncryptionType: schema.StringAttribute{Computed: true},
	}}
	stateType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{keyEncryptionType: tftypes.String}}
	state := func(encryptionType *string) tfsdk.State {
		var value any
		if encryptionType != nil {
			value = *encryptionType
		}
		return tfsdk.State{
			Schema: stateSchema,
			Raw:    tftypes.NewValue(stateType, map[string]tftypes.Value{keyEncryptionType: tftypes.NewValue(tftypes.String, value)}),
		}
	}

	tests := []struct {
		name           string
		state          types.List
		plan           types.List
		encryptionType *string
		requireReplace bool
	}{{
		name:           "ImportedAddBlock",
		state:          types.ListNull(elementType),
		plan:           elementList("1"),
		encryptionType: toPointer("SSE_CPK"),
	}, {
		name:           "ImportedAddBlockToEmptyList",
		state:          elementList(),
		plan:           elementList("1"),
		encryptionType: toPointer("SSE_CPK"),
	}, {
		name:           "AddBlockToOtherKeyType",
		state:          types.ListNull(elementType),
		plan:           elementList("1"),
		encryptionType: toPointer("UEKM_AWS_KMS_BASED"),
		requireReplace: true,
	}, {
		name:           "AddBlockToRCVManagedKey",
		state:          types.ListNull(elementType),
		plan:           elementList("1"),
		encryptionType: toPointer("UNKNOWN_ENCRYPTION_TYPE"),
		requireReplace: true,
	}, {
		name:           "AddBlockNullEncryptionType",
		state:          types.ListNull(elementType),
		plan:           elementList("1"),
		requireReplace: true,
	}, {
		name:           "AddBlockUnknownPlan",
		state:          types.ListNull(elementType),
		plan:           types.ListUnknown(elementType),
		encryptionType: toPointer("SSE_CPK"),
		requireReplace: true,
	}, {
		name:           "RemoveBlock",
		state:          elementList("1"),
		plan:           types.ListNull(elementType),
		encryptionType: toPointer("SSE_CPK"),
		requireReplace: true,
	}, {
		name:           "ChangeVersion",
		state:          elementList("1"),
		plan:           elementList("2"),
		encryptionType: toPointer("SSE_CPK"),
		requireReplace: true,
	}, {
		name:           "Unchanged",
		state:          elementList("1"),
		plan:           elementList("1"),
		encryptionType: toPointer("SSE_CPK"),
	}, {
		name:           "NullToNull",
		state:          types.ListNull(elementType),
		plan:           types.ListNull(elementType),
		encryptionType: toPointer("SSE_CPK"),
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The modifier only checks that the plan isn't null, and reads the
			// block from the plan value of the request, so the plan itself can be
			// any non-null value of the right type. The create and destroy cases
			// aren't tested, the framework doesn't call the modifier for them.
			planRaw := tftypes.NewValue(stateType, map[string]tftypes.Value{
				keyEncryptionType: tftypes.NewValue(tftypes.String, nil),
			})

			req := planmodifier.ListRequest{
				State:      state(tc.encryptionType),
				Plan:       tfsdk.Plan{Schema: stateSchema, Raw: planRaw},
				StateValue: tc.state,
				PlanValue:  tc.plan,
			}
			var res planmodifier.ListResponse
			requiresReplaceUnlessImported(gqlarchival.TargetEncryptionSSECustomerKey).PlanModifyList(context.Background(), req, &res)
			if res.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", res.Diagnostics)
			}
			if res.RequiresReplace != tc.requireReplace {
				t.Errorf("expected RequiresReplace to be %v, got %v", tc.requireReplace, res.RequiresReplace)
			}
		})
	}
}

func TestRequiresReplaceOnAddOrRemove(t *testing.T) {
	elementType := types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType}}
	list := func(names ...string) types.List {
		var elements []attr.Value
		for _, name := range names {
			elements = append(elements, types.ObjectValueMust(elementType.AttrTypes, map[string]attr.Value{
				"name": types.StringValue(name),
			}))
		}
		return types.ListValueMust(elementType, elements)
	}
	null := types.ListNull(elementType)
	unknown := types.ListUnknown(elementType)

	tests := []struct {
		name           string
		state          types.List
		plan           types.List
		create         bool
		destroy        bool
		requireReplace bool
	}{{
		name:           "NullToElement",
		state:          null,
		plan:           list("a"),
		requireReplace: true,
	}, {
		name:           "ElementToNull",
		state:          list("a"),
		plan:           null,
		requireReplace: true,
	}, {
		name:           "EmptyToElement",
		state:          list(),
		plan:           list("a"),
		requireReplace: true,
	}, {
		name:           "ElementToEmpty",
		state:          list("a"),
		plan:           list(),
		requireReplace: true,
	}, {
		name:  "ElementChange",
		state: list("a"),
		plan:  list("b"),
	}, {
		name:  "ElementUnchanged",
		state: list("a"),
		plan:  list("a"),
	}, {
		name:  "NullToNull",
		state: null,
		plan:  null,
	}, {
		name:  "NullToEmpty",
		state: null,
		plan:  list(),
	}, {
		name:  "UnknownPlan",
		state: list("a"),
		plan:  unknown,
	}, {
		name:   "Create",
		state:  null,
		plan:   list("a"),
		create: true,
	}, {
		name:    "Destroy",
		state:   list("a"),
		plan:    null,
		destroy: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Only the null state of the raw values is inspected by the
			// modifier, the typed values carry the list contents.
			raw := func(null bool) tftypes.Value {
				if null {
					return tftypes.NewValue(tftypes.Object{}, nil)
				}
				return tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})
			}

			req := planmodifier.ListRequest{
				State:      tfsdk.State{Raw: raw(tc.create)},
				Plan:       tfsdk.Plan{Raw: raw(tc.destroy)},
				StateValue: tc.state,
				PlanValue:  tc.plan,
			}
			var res planmodifier.ListResponse

			requiresReplaceOnAddOrRemove().PlanModifyList(context.Background(), req, &res)

			if res.RequiresReplace != tc.requireReplace {
				t.Errorf("expected RequiresReplace to be %v, got %v", tc.requireReplace, res.RequiresReplace)
			}
		})
	}
}

func toPointer[T any](t T) *T {
	return &t
}
