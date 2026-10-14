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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

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
	}{
		{name: "NullToElement", state: null, plan: list("a"), requireReplace: true},
		{name: "ElementToNull", state: list("a"), plan: null, requireReplace: true},
		{name: "EmptyToElement", state: list(), plan: list("a"), requireReplace: true},
		{name: "ElementToEmpty", state: list("a"), plan: list(), requireReplace: true},
		{name: "ElementChange", state: list("a"), plan: list("b")},
		{name: "ElementUnchanged", state: list("a"), plan: list("a")},
		{name: "NullToNull", state: null, plan: null},
		{name: "NullToEmpty", state: null, plan: list()},
		{name: "UnknownPlan", state: list("a"), plan: unknown},
		{name: "Create", state: null, plan: list("a"), create: true},
		{name: "Destroy", state: list("a"), plan: null, destroy: true},
	}

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
