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
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The tests in this file compare the Framework schema of the SLA domain resource
// with the SDKv2 schema it replaces, for every attribute and block. They exist
// because the schema is long and was generated, and a wrong attribute name,
// requiredness or default compiles but only fails when the resource is used.
// The tests are removed together with the SDKv2 resource.

// schemaEntry is what the two schemas are compared on.
type schemaEntry struct {
	kind     string // "string", "int64", "bool", "set", "list", "block"
	required bool
	optional bool
	computed bool
	hasDef   bool
	def      string
	desc     string
}

func (e schemaEntry) String() string {
	return fmt.Sprintf("%s required=%t optional=%t computed=%t default=%q", e.kind, e.required, e.optional, e.computed, e.def)
}

// flattenSDKv2 returns every attribute and block of the SDKv2 schema by path.
func flattenSDKv2(prefix string, m map[string]*sdkschema.Schema, out map[string]schemaEntry) {
	for name, s := range m {
		path := prefix + "." + name
		e := schemaEntry{required: s.Required, optional: s.Optional, computed: s.Computed || s.Default != nil, desc: s.Description}
		if s.Default != nil {
			e.hasDef = true
			e.def = fmt.Sprint(s.Default)
		}
		switch r := s.Elem.(type) {
		case *sdkschema.Resource:
			e.kind = "block"
			// A block is never required or computed by itself in the Framework.
			e.required, e.computed = false, false
			out[path] = e
			flattenSDKv2(path, r.Schema, out)
			continue
		}
		switch s.Type {
		case sdkschema.TypeString:
			e.kind = "string"
		case sdkschema.TypeInt:
			e.kind = "int64"
		case sdkschema.TypeBool:
			e.kind = "bool"
		case sdkschema.TypeSet:
			e.kind = "set"
		case sdkschema.TypeList:
			e.kind = "list"
		}
		out[path] = e
	}
}

// flattenFramework returns every attribute and block of the Framework schema by
// path, in the same form as flattenSDKv2.
func flattenFramework(t *testing.T, prefix string, attrs map[string]schema.Attribute, blocks map[string]schema.Block, out map[string]schemaEntry) {
	t.Helper()

	for name, a := range attrs {
		path := prefix + "." + name
		e := schemaEntry{required: a.IsRequired(), optional: a.IsOptional(), computed: a.IsComputed(), desc: a.GetDescription()}
		switch v := a.(type) {
		case schema.StringAttribute:
			e.kind = "string"
			e.hasDef, e.def = frameworkDefault(t, v.Default)
		case schema.Int64Attribute:
			e.kind = "int64"
			e.hasDef, e.def = frameworkDefault(t, v.Default)
		case schema.BoolAttribute:
			e.kind = "bool"
			e.hasDef, e.def = frameworkDefault(t, v.Default)
		case schema.SetAttribute:
			e.kind = "set"
		case schema.ListAttribute:
			e.kind = "list"
		default:
			t.Fatalf("%s: unexpected attribute type %T", path, a)
		}
		out[path] = e
	}

	for name, b := range blocks {
		path := prefix + "." + name
		if _, ok := b.(schema.SingleNestedBlock); ok && path == "."+keyTimeouts {
			// The timeouts are a resource setting in SDKv2, so they are not in
			// the SDKv2 schema, see TestSLADomainTimeouts.
			continue
		}
		lb, ok := b.(schema.ListNestedBlock)
		if !ok {
			t.Fatalf("%s: unexpected block type %T", path, b)
		}
		out[path] = schemaEntry{kind: "block", optional: true, desc: lb.Description}
		flattenFramework(t, path, lb.NestedObject.Attributes, lb.NestedObject.Blocks, out)
	}
}

func frameworkDefault(t *testing.T, d any) (bool, string) {
	t.Helper()

	switch v := d.(type) {
	case nil:
		return false, ""
	case defaults.String:
		var res defaults.StringResponse
		v.DefaultString(t.Context(), defaults.StringRequest{}, &res)
		return true, res.PlanValue.ValueString()
	case defaults.Int64:
		var res defaults.Int64Response
		v.DefaultInt64(t.Context(), defaults.Int64Request{}, &res)
		return true, fmt.Sprint(res.PlanValue.ValueInt64())
	case defaults.Bool:
		var res defaults.BoolResponse
		v.DefaultBool(t.Context(), defaults.BoolRequest{}, &res)
		return true, fmt.Sprint(res.PlanValue.ValueBool())
	}
	t.Fatalf("unexpected default type %T", d)
	return false, ""
}

// TestSLADomainSchemaMatchesSDKv2 verifies that the Framework schema has the
// same attributes and blocks as the SDKv2 schema, and that each has the same
// type, requiredness, default and description.
func TestSLADomainSchemaMatchesSDKv2(t *testing.T) {
	sdk := map[string]schemaEntry{}
	flattenSDKv2("", resourceSLADomain().Schema, sdk)

	s := slaDomainSchema(t.Context())
	fw := map[string]schemaEntry{}
	flattenFramework(t, "", s.Attributes, s.Blocks, fw)

	// The Framework schema holds the ID and the timeouts, which the SDKv2
	// schema has too (the ID) or has as a resource setting (the timeouts).
	var problems []string
	paths := make([]string, 0, len(sdk))
	for p := range sdk {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		want := sdk[p]
		got, ok := fw[p]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: missing from the Framework schema (SDKv2: %s)", p, want))
			continue
		}
		if got.kind != want.kind {
			problems = append(problems, fmt.Sprintf("%s: kind %s, SDKv2 %s", p, got.kind, want.kind))
		}
		if got.required != want.required || got.optional != want.optional {
			problems = append(problems, fmt.Sprintf("%s: required=%t optional=%t, SDKv2 required=%t optional=%t",
				p, got.required, got.optional, want.required, want.optional))
		}
		if got.computed != want.computed {
			problems = append(problems, fmt.Sprintf("%s: computed=%t, SDKv2 computed=%t", p, got.computed, want.computed))
		}
		if got.hasDef != want.hasDef || got.def != want.def {
			problems = append(problems, fmt.Sprintf("%s: default %q (%t), SDKv2 %q (%t)", p, got.def, got.hasDef, want.def, want.hasDef))
		}
		if got.desc != want.desc {
			problems = append(problems, fmt.Sprintf("%s: description differs:\n\t%q\n\tSDKv2 %q", p, got.desc, want.desc))
		}
	}
	for p := range fw {
		if _, ok := sdk[p]; !ok {
			problems = append(problems, fmt.Sprintf("%s: in the Framework schema but not in the SDKv2 schema (%s)", p, fw[p]))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("%d differences between the schemas:\n%s", len(problems), strings.Join(problems, "\n"))
	}
	t.Logf("compared %d attributes and blocks", len(sdk))
}

// validatorProbes are the values the validators of the two schemas are both
// given, to compare whether they accept or reject the same values. They cover
// the validators of the schema: unit names, UUIDs, numbers around the limits
// used, start times and a KMS alias.
var validatorProbes = struct {
	strings []string
	ints    []int64
}{
	strings: []string{
		"", " ", "DAYS", "WEEKS", "MONTHS", "QUARTERS", "YEARS", "HOURS", "MINUTES", "MINUTE", "SECONDS", "days",
		"FIRST_DAY", "LAST_DAY", "FIFTEENTH", "LOGARCHMETH1", "LOGARCHMETH2", "LOGARCHMETH3", "AWS_GLACIER",
		"AZURE_ARCHIVE", "AWS_GLACIER_DEEP_ARCHIVE", "OTHER", "MONDAY", "JANUARY", "COMPLIANCE", "PROTECTION",
		"NO_LOCK", "VSPHERE_OBJECT_TYPE", "AWS_RDS_OBJECT_TYPE", "NOT_AN_OBJECT_TYPE", "us-east-1", "eastus",
		"mars-north-1", "550e8400-e29b-41d4-a716-446655440000", "not-a-uuid", "alias/my-key", "alias/", "my-key",
		"Mon, 15:30", "16:45", "Monday, 15:30", "25:00", "15:60", "xx:30", "Fun, 10:00", "10:00:00",
	},
	ints: []int64{-1, 0, 1, 2, 6, 7, 8, 34, 35, 36, 51, 52, 53, 3650, 3651},
}

// sdkv2Accepts reports whether the SDKv2 validator of the attribute accepts the
// value, and false, false when the attribute has no validator.
func sdkv2Accepts(s *sdkschema.Schema, elem bool, value any) (accepted, hasValidator bool) {
	fn := s.ValidateFunc
	if elem {
		e, ok := s.Elem.(*sdkschema.Schema)
		if !ok {
			return false, false
		}
		fn = e.ValidateFunc
	}
	if fn == nil {
		return false, false
	}
	_, errs := fn(value, "attr")
	return len(errs) == 0, true
}

// sdkEntry is an SDKv2 attribute and the path to it.
type sdkEntry struct {
	path string
	s    *sdkschema.Schema
}

func collectSDKv2(prefix string, m map[string]*sdkschema.Schema, out *[]sdkEntry) {
	for name, s := range m {
		*out = append(*out, sdkEntry{path: prefix + "." + name, s: s})
		if r, ok := s.Elem.(*sdkschema.Resource); ok {
			collectSDKv2(prefix+"."+name, r.Schema, out)
		}
	}
}

// frameworkNode finds the attribute or block at the path in the Framework
// schema.
func frameworkNode(s schema.Schema, path string) (any, bool) {
	parts := strings.Split(strings.TrimPrefix(path, "."), ".")
	attrs, blocks := s.Attributes, s.Blocks
	for i, part := range parts {
		last := i == len(parts)-1
		if a, ok := attrs[part]; ok {
			return a, last
		}
		b, ok := blocks[part]
		if !ok {
			return nil, false
		}
		if last {
			return b, true
		}
		nb, ok := b.(schema.ListNestedBlock)
		if !ok {
			return nil, false
		}
		attrs, blocks = nb.NestedObject.Attributes, nb.NestedObject.Blocks
	}
	return nil, false
}

// TestSLADomainValidatorsMatchSDKv2 verifies that every attribute with an SDKv2
// validator has a Framework validator which accepts and rejects the same values.
// The validators are compared on probe values, since the SDKv2 validators are
// functions which can't be compared any other way.
func TestSLADomainValidatorsMatchSDKv2(t *testing.T) {
	var entries []sdkEntry
	collectSDKv2("", resourceSLADomain().Schema, &entries)
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	fwSchema := slaDomainSchema(t.Context())
	var problems []string
	compared := 0

	for _, e := range entries {
		node, ok := frameworkNode(fwSchema, e.path)
		if !ok {
			continue // reported by the structural test
		}

		isCollection := e.s.Type == sdkschema.TypeSet || e.s.Type == sdkschema.TypeList

		// Attribute level validators.
		if !isCollection || e.s.ValidateFunc != nil {
			switch e.s.Type {
			case sdkschema.TypeString:
				for _, v := range validatorProbes.strings {
					want, has := sdkv2Accepts(e.s, false, v)
					if !has {
						break
					}
					compared++
					if got := frameworkAccepts(t, node, v); got != want {
						problems = append(problems, fmt.Sprintf("%s: value %q accepted=%t, SDKv2 accepted=%t", e.path, v, got, want))
					}
				}
			case sdkschema.TypeInt:
				for _, v := range validatorProbes.ints {
					want, has := sdkv2Accepts(e.s, false, int(v))
					if !has {
						break
					}
					compared++
					if got := frameworkAccepts(t, node, v); got != want {
						problems = append(problems, fmt.Sprintf("%s: value %d accepted=%t, SDKv2 accepted=%t", e.path, v, got, want))
					}
				}
			}
		}

		// Element validators of a set or list of strings, compared on each
		// probe as a one element collection.
		if isCollection {
			if elem, ok := e.s.Elem.(*sdkschema.Schema); ok && elem.ValidateFunc != nil {
				for _, v := range validatorProbes.strings {
					want, _ := sdkv2Accepts(e.s, true, v)
					compared++
					if got := frameworkAccepts(t, node, []string{v}); got != want {
						problems = append(problems, fmt.Sprintf("%s: element %q accepted=%t, SDKv2 accepted=%t", e.path, v, got, want))
					}
				}
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("%d differences between the validators:\n%s", len(problems), strings.Join(problems, "\n"))
	}
	t.Logf("compared %d validator results", compared)
	if compared == 0 {
		t.Fatal("no validators were compared")
	}
}

// validatorsOf returns the validators of a Framework attribute or block, in the
// form of functions which return true when the value is accepted.
func validatorsOf(a any) (stringV []validator.String, int64V []validator.Int64, setV []validator.Set, listV []validator.List) {
	switch v := a.(type) {
	case schema.StringAttribute:
		return v.Validators, nil, nil, nil
	case schema.Int64Attribute:
		return nil, v.Validators, nil, nil
	case schema.SetAttribute:
		return nil, nil, v.Validators, nil
	case schema.ListAttribute:
		return nil, nil, nil, v.Validators
	case schema.ListNestedBlock:
		return nil, nil, nil, v.Validators
	}
	return nil, nil, nil, nil
}

// frameworkAccepts runs the validators of the attribute on a value, and returns
// whether they all accept it. A value is a string, an int64 or a list of
// strings, which a set or list attribute applies to its elements.
func frameworkAccepts(t *testing.T, a any, value any) bool {
	t.Helper()

	stringV, int64V, setV, listV := validatorsOf(a)
	switch v := value.(type) {
	case string:
		for _, val := range stringV {
			var res validator.StringResponse
			val.ValidateString(t.Context(), validator.StringRequest{ConfigValue: types.StringValue(v)}, &res)
			if res.Diagnostics.HasError() {
				return false
			}
		}
	case int64:
		for _, val := range int64V {
			var res validator.Int64Response
			val.ValidateInt64(t.Context(), validator.Int64Request{ConfigValue: types.Int64Value(v)}, &res)
			if res.Diagnostics.HasError() {
				return false
			}
		}
	case []string:
		elems := make([]attr.Value, 0, len(v))
		for _, e := range v {
			elems = append(elems, types.StringValue(e))
		}
		for _, val := range setV {
			var res validator.SetResponse
			val.ValidateSet(t.Context(), validator.SetRequest{ConfigValue: types.SetValueMust(types.StringType, elems)}, &res)
			if res.Diagnostics.HasError() {
				return false
			}
		}
		for _, val := range listV {
			var res validator.ListResponse
			val.ValidateList(t.Context(), validator.ListRequest{ConfigValue: types.ListValueMust(types.StringType, elems)}, &res)
			if res.Diagnostics.HasError() {
				return false
			}
		}
	}
	return true
}

// TestSLADomainTimeouts verifies the timeouts, which the SDKv2 resource sets as a
// resource setting, and so the comparison of the schemas doesn't cover. They are
// a block, and not an attribute, so configurations which write
// `timeouts { delete = "5m" }` keep working.
func TestSLADomainTimeouts(t *testing.T) {
	sdk := resourceSLADomain()
	if sdk.Timeouts == nil || sdk.Timeouts.Delete == nil || *sdk.Timeouts.Delete != defaultSLADomainDeleteTimeout {
		t.Fatalf("the SDKv2 delete timeout is %v, the test needs updating", sdk.Timeouts)
	}
	if sdk.Timeouts.Create != nil || sdk.Timeouts.Read != nil || sdk.Timeouts.Update != nil {
		t.Fatal("the SDKv2 resource has timeouts besides delete, the test needs updating")
	}

	s := slaDomainSchema(t.Context())
	if _, ok := s.Attributes[keyTimeouts]; ok {
		t.Fatalf("%q must be a block and not an attribute, since it is a block in the SDKv2 resource", keyTimeouts)
	}
	block, ok := s.Blocks[keyTimeouts].(schema.SingleNestedBlock)
	if !ok {
		t.Fatalf("%q must be a single nested block, got %T", keyTimeouts, s.Blocks[keyTimeouts])
	}

	// Only the delete timeout is applied, as in SDKv2.
	if len(block.Attributes) != 1 {
		t.Errorf("expected only the delete timeout, got %d attributes", len(block.Attributes))
	}
	if _, ok := block.Attributes["delete"]; !ok {
		t.Error("expected a delete timeout")
	}
}
