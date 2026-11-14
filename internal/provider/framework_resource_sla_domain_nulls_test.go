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
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

func TestKeepNullsFromPrior(t *testing.T) {
	str := tftypes.String
	num := tftypes.Number
	boolean := tftypes.Bool
	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"s": str, "n": num, "b": boolean}}
	list := tftypes.List{ElementType: obj}

	object := func(s, n, b tftypes.Value) tftypes.Value {
		return tftypes.NewValue(obj, map[string]tftypes.Value{"s": s, "n": n, "b": b})
	}
	null := func(typ tftypes.Type) tftypes.Value { return tftypes.NewValue(typ, nil) }
	zero := object(tftypes.NewValue(str, ""), tftypes.NewValue(num, 0), tftypes.NewValue(boolean, false))

	tests := []struct {
		name          string
		remote, prior tftypes.Value
		want          tftypes.Value
	}{
		{
			name:   "zero values stay null when the prior is null",
			remote: zero,
			prior:  object(null(str), null(num), null(boolean)),
			want:   object(null(str), null(num), null(boolean)),
		},
		{
			name:   "zero values are kept when the prior is set to them",
			remote: zero,
			prior:  zero,
			want:   zero,
		},
		{
			name:   "non zero values are kept when the prior is null",
			remote: object(tftypes.NewValue(str, "x"), tftypes.NewValue(num, 3), tftypes.NewValue(boolean, true)),
			prior:  object(null(str), null(num), null(boolean)),
			want:   object(tftypes.NewValue(str, "x"), tftypes.NewValue(num, 3), tftypes.NewValue(boolean, true)),
		},
		{
			name:   "each attribute is decided on its own",
			remote: zero,
			prior:  object(null(str), tftypes.NewValue(num, 0), null(boolean)),
			want:   object(null(str), tftypes.NewValue(num, 0), null(boolean)),
		},
		{
			name:   "an unknown prior keeps the remote value",
			remote: zero,
			prior:  object(tftypes.NewValue(str, tftypes.UnknownValue), null(num), null(boolean)),
			want:   object(tftypes.NewValue(str, ""), null(num), null(boolean)),
		},
		{
			name:   "no prior keeps every remote value",
			remote: zero,
			prior:  null(obj),
			want:   zero,
		},
		{
			name:   "blocks are matched by position",
			remote: tftypes.NewValue(list, []tftypes.Value{zero, zero}),
			prior: tftypes.NewValue(list, []tftypes.Value{
				object(null(str), null(num), null(boolean)),
				zero,
			}),
			want: tftypes.NewValue(list, []tftypes.Value{
				object(null(str), null(num), null(boolean)),
				zero,
			}),
		},
		{
			name:   "a block which isn't in the prior is kept",
			remote: tftypes.NewValue(list, []tftypes.Value{zero, zero}),
			prior:  tftypes.NewValue(list, []tftypes.Value{object(null(str), null(num), null(boolean))}),
			want: tftypes.NewValue(list, []tftypes.Value{
				object(null(str), null(num), null(boolean)),
				zero,
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keepNullsFromPrior(tt.remote, tt.prior)
			if !got.Equal(tt.want) {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// TestSLADomainOptionalPrimitivesReadBackNull takes every optional attribute of
// the SLA domain which isn't computed and is a string, number or bool, and
// reads the zero value of its type back over a null plan. The result has to be
// null, or the apply fails with an inconsistent result.
func TestSLADomainOptionalPrimitivesReadBackNull(t *testing.T) {
	ctx := t.Context()
	s := slaDomainSchema(ctx)
	obj := s.Type().TerraformType(ctx)

	type target struct {
		path []string
		typ  tftypes.Type
	}
	var targets []target
	var walk func(path []string, attrs map[string]schema.Attribute, blocks map[string]schema.Block)
	walk = func(path []string, attrs map[string]schema.Attribute, blocks map[string]schema.Block) {
		for name, attr := range attrs {
			if !attr.IsOptional() || attr.IsComputed() {
				continue
			}
			switch attr.(type) {
			case schema.StringAttribute:
				targets = append(targets, target{append(slices.Clone(path), name), tftypes.String})
			case schema.Int64Attribute:
				targets = append(targets, target{append(slices.Clone(path), name), tftypes.Number})
			case schema.BoolAttribute:
				targets = append(targets, target{append(slices.Clone(path), name), tftypes.Bool})
			}
		}
		for name, block := range blocks {
			if nested, ok := block.(schema.ListNestedBlock); ok {
				walk(append(slices.Clone(path), name), nested.NestedObject.Attributes, nested.NestedObject.Blocks)
			}
		}
	}
	walk(nil, s.Attributes, s.Blocks)
	if len(targets) == 0 {
		t.Fatal("no optional primitives found")
	}

	zeros := map[string]tftypes.Value{
		tftypes.String.String(): tftypes.NewValue(tftypes.String, ""),
		tftypes.Number.String(): tftypes.NewValue(tftypes.Number, 0),
		tftypes.Bool.String():   tftypes.NewValue(tftypes.Bool, false),
	}

	// set returns the value with the attribute at the path replaced, descending
	// into the first element of every block on the way.
	var set func(v tftypes.Value, path []string, replacement tftypes.Value) tftypes.Value
	set = func(v tftypes.Value, path []string, replacement tftypes.Value) tftypes.Value {
		// As returns the map of the value, so it's copied before it is changed.
		var shared map[string]tftypes.Value
		if err := v.As(&shared); err != nil {
			t.Fatal(err)
		}
		attrs := maps.Clone(shared)
		if len(path) == 1 {
			attrs[path[0]] = replacement
			return tftypes.NewValue(v.Type(), attrs)
		}
		var sharedElems []tftypes.Value
		if err := attrs[path[0]].As(&sharedElems); err != nil {
			t.Fatal(err)
		}
		elems := slices.Clone(sharedElems)
		elems[0] = set(elems[0], path[1:], replacement)
		attrs[path[0]] = tftypes.NewValue(attrs[path[0]].Type(), elems)
		return tftypes.NewValue(v.Type(), attrs)
	}

	// blocks gives a value where every block on the path has one element.
	var blocks func(v tftypes.Value, path []string) tftypes.Value
	blocks = func(v tftypes.Value, path []string) tftypes.Value {
		if len(path) == 1 {
			return v
		}
		var shared map[string]tftypes.Value
		if err := v.As(&shared); err != nil {
			t.Fatal(err)
		}
		attrs := maps.Clone(shared)
		listType := attrs[path[0]].Type().(tftypes.List)
		elemType := listType.ElementType.(tftypes.Object)
		elem := make(map[string]tftypes.Value, len(elemType.AttributeTypes))
		for name, typ := range elemType.AttributeTypes {
			elem[name] = zeroValue(typ)
		}
		elemValue := blocks(tftypes.NewValue(elemType, elem), path[1:])
		attrs[path[0]] = tftypes.NewValue(listType, []tftypes.Value{elemValue})
		return tftypes.NewValue(v.Type(), attrs)
	}

	empty := map[string]tftypes.Value{}
	for name, typ := range obj.(tftypes.Object).AttributeTypes {
		empty[name] = zeroValue(typ)
	}
	base := tftypes.NewValue(obj, empty)

	for _, tg := range targets {
		t.Run(strings.Join(tg.path, "."), func(t *testing.T) {
			prior := blocks(base, tg.path)
			remote := set(prior, tg.path, zeros[tg.typ.String()])

			got := keepNullsFromPrior(remote, prior)

			cur := got
			for i, name := range tg.path {
				var attrs map[string]tftypes.Value
				if err := cur.As(&attrs); err != nil {
					t.Fatal(err)
				}
				cur = attrs[name]
				if i < len(tg.path)-1 {
					var elems []tftypes.Value
					if err := cur.As(&elems); err != nil {
						t.Fatal(err)
					}
					cur = elems[0]
				}
			}
			if !cur.IsNull() {
				t.Fatalf("read back %s, want null", cur)
			}
		})
	}
	t.Logf("checked %d optional attributes", len(targets))
}

func TestNullUnknownValues(t *testing.T) {
	str := tftypes.String
	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"known":   str,
		"unknown": str,
		"nested":  tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"s": str}}},
	}}
	nested := tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"s": str}}}
	elem := nested.ElementType

	value := tftypes.NewValue(obj, map[string]tftypes.Value{
		"known":   tftypes.NewValue(str, "x"),
		"unknown": tftypes.NewValue(str, tftypes.UnknownValue),
		"nested": tftypes.NewValue(nested, []tftypes.Value{
			tftypes.NewValue(elem, map[string]tftypes.Value{"s": tftypes.NewValue(str, tftypes.UnknownValue)}),
			tftypes.NewValue(elem, map[string]tftypes.Value{"s": tftypes.NewValue(str, "y")}),
		}),
	})
	want := tftypes.NewValue(obj, map[string]tftypes.Value{
		"known":   tftypes.NewValue(str, "x"),
		"unknown": tftypes.NewValue(str, nil),
		"nested": tftypes.NewValue(nested, []tftypes.Value{
			tftypes.NewValue(elem, map[string]tftypes.Value{"s": tftypes.NewValue(str, nil)}),
			tftypes.NewValue(elem, map[string]tftypes.Value{"s": tftypes.NewValue(str, "y")}),
		}),
	})

	got := nullUnknownValues(value)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
	if !got.IsFullyKnown() {
		t.Fatal("the result still holds unknown values")
	}
}

// importedSLADomainPrior returns the state of an import, where only the id is set.
func importedSLADomainPrior(t *testing.T, id string) tftypes.Value {
	t.Helper()

	typ := slaDomainSchema(t.Context()).Type().TerraformType(t.Context()).(tftypes.Object)
	values := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, attrType := range typ.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	values[keyID] = tftypes.NewValue(tftypes.String, id)
	return tftypes.NewValue(typ, values)
}

func TestIsImportedPrior(t *testing.T) {
	id := uuid.NewString()
	imported := importedSLADomainPrior(t, id)
	if !isImportedPrior(imported) {
		t.Error("the state of an import isn't recognised")
	}
	if isImportedPrior(tftypes.NewValue(imported.Type(), nil)) {
		t.Error("a null value is an import")
	}

	// A plan has a name, and so has the state of a created resource, which also
	// has an id. A refresh reads with that state, and has to keep the attributes
	// which are set to their zero value.
	plan := slaDomainPlan(t, map[string]tftypes.Value{keyName: tftypes.NewValue(tftypes.String, "gold")})
	if isImportedPrior(plan.Raw) {
		t.Error("a plan with a name is an import")
	}
	state := slaDomainPlan(t, map[string]tftypes.Value{
		keyName: tftypes.NewValue(tftypes.String, "gold"),
		keyID:   tftypes.NewValue(tftypes.String, id),
	})
	if isImportedPrior(state.Raw) {
		t.Error("the state of a created resource, with a name and an id, is an import")
	}
}

// TestSLADomainImportedStateKeepsOptionalAttributesNull reads an SLA domain right
// after an import, for each optional attribute of the schema which isn't
// computed. The block holding the attribute isn't in the prior value then, and
// the zero value RSC returns has to be null, or the first plan after the import
// shows a diff for an attribute the configuration doesn't set.
func TestSLADomainImportedStateKeepsOptionalAttributesNull(t *testing.T) {
	ctx := t.Context()
	s := slaDomainSchema(ctx)

	paths := optionalOnlyAttributePaths(s)
	if len(paths) != 40 {
		t.Fatalf("found %d optional attributes, want 40", len(paths))
	}

	// remote is a value where every block has one element, and every optional
	// attribute which isn't computed has the zero value of its type.
	var fill func(typ tftypes.Type, path []string) tftypes.Value
	fill = func(typ tftypes.Type, path []string) tftypes.Value {
		switch typ := typ.(type) {
		case tftypes.Object:
			values := make(map[string]tftypes.Value, len(typ.AttributeTypes))
			for name, attrType := range typ.AttributeTypes {
				values[name] = fill(attrType, append(slices.Clone(path), name))
			}
			return tftypes.NewValue(typ, values)
		case tftypes.List:
			if _, ok := typ.ElementType.(tftypes.Object); ok {
				return tftypes.NewValue(typ, []tftypes.Value{fill(typ.ElementType, path)})
			}
			return tftypes.NewValue(typ, nil)
		}
		switch {
		case typ.Is(tftypes.String):
			return tftypes.NewValue(typ, "")
		case typ.Is(tftypes.Number):
			return tftypes.NewValue(typ, 0)
		case typ.Is(tftypes.Bool):
			return tftypes.NewValue(typ, false)
		}
		return tftypes.NewValue(typ, nil)
	}
	remote := fill(s.Type().TerraformType(ctx), nil)

	got, err := nullOptionalZeroValues(s, remote)
	if err != nil {
		t.Fatal(err)
	}

	// Walk the result: an attribute is null exactly when it is optional only.
	checked := 0
	err = tftypes.Walk(got, func(path *tftypes.AttributePath, v tftypes.Value) (bool, error) {
		if len(path.Steps()) == 0 || !isZeroOrNull(v) {
			return true, nil
		}
		key := attributePathKey(path)
		switch {
		case paths[key] && !v.IsNull():
			t.Errorf("%s is %s after an import, want null", key, v)
		case !paths[key] && v.IsNull() && isPrimitive(v):
			t.Errorf("%s is null after an import, but isn't an optional attribute", key)
		}
		if paths[key] {
			checked++
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 31 {
		t.Errorf("checked %d optional primitives, want at least 31", checked)
	}
}

func isZeroOrNull(v tftypes.Value) bool { return v.IsNull() || isZeroPrimitive(v) }

func isPrimitive(v tftypes.Value) bool {
	return v.Type().Is(tftypes.String) || v.Type().Is(tftypes.Number) || v.Type().Is(tftypes.Bool)
}

func TestSLADomainStateFromRemoteAfterImport(t *testing.T) {
	ctx := t.Context()
	s := slaDomainSchema(ctx)
	id := uuid.New()

	domain := gqlsla.Domain{ID: id, Name: "gold", ObjectTypes: []gqlsla.ObjectType{gqlsla.ObjectVSphereVM}}
	domain.SnapshotSchedule.Weekly = &gqlsla.WeeklySnapshotSchedule{
		BasicSchedule: gqlsla.BasicSnapshotSchedule{Frequency: 1, Retention: 1, RetentionUnit: gqlsla.Weeks},
	}

	model, diags := slaDomainStateFromRemote(ctx, s, domain, tfsdk.Plan{Schema: s, Raw: importedSLADomainPrior(t, id.String())})
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !model.Description.IsNull() {
		t.Errorf("description = %v, want null", model.Description)
	}
	if len(model.WeeklySchedule) != 1 || !model.WeeklySchedule[0].DayOfWeek.IsNull() {
		t.Errorf("weekly schedule = %+v, want a null day of week", model.WeeklySchedule)
	}
	if model.WeeklySchedule[0].Frequency.ValueInt64() != 1 || model.Name.ValueString() != "gold" {
		t.Errorf("values RSC returned weren't kept: %+v", model)
	}

	// A day of the week which RSC returns is kept.
	domain.SnapshotSchedule.Weekly.DayOfWeek = gqlsla.Monday
	model, diags = slaDomainStateFromRemote(ctx, s, domain, tfsdk.Plan{Schema: s, Raw: importedSLADomainPrior(t, id.String())})
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if model.WeeklySchedule[0].DayOfWeek.ValueString() != string(gqlsla.Monday) {
		t.Errorf("day of week = %v, want MONDAY", model.WeeklySchedule[0].DayOfWeek)
	}
}
