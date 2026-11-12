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

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
