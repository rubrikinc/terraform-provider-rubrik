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
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// polarisSLADomainState returns the state the polaris provider writes for an SLA
// domain, built from the source schema of the mover. A daily schedule and a
// description are set, the other blocks are empty lists as in SDKv2 state, and
// the timeouts are null.
func polarisSLADomainState(t *testing.T, mover resource.StateMover, id string) *tfsdk.State {
	t.Helper()

	typ := mover.SourceSchema.Type().TerraformType(t.Context()).(tftypes.Object)
	values := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, attrType := range typ.AttributeTypes {
		switch attrType.(type) {
		case tftypes.List:
			values[name] = tftypes.NewValue(attrType, []tftypes.Value{})
		default:
			values[name] = tftypes.NewValue(attrType, nil)
		}
	}

	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }
	values[keyID] = str(id)
	values[keyName] = str("gold")
	values[keyDescription] = str("the gold SLA")
	values[keyBackupType] = str("")
	values[keyObjectTypes] = tftypes.NewValue(typ.AttributeTypes[keyObjectTypes],
		[]tftypes.Value{str("VSPHERE_OBJECT_TYPE")})

	dailyType := typ.AttributeTypes[keyDailySchedule].(tftypes.List)
	values[keyDailySchedule] = tftypes.NewValue(dailyType, []tftypes.Value{
		tftypes.NewValue(dailyType.ElementType, map[string]tftypes.Value{
			keyFrequency:     tftypes.NewValue(tftypes.Number, 1),
			keyRetention:     tftypes.NewValue(tftypes.Number, 7),
			keyRetentionUnit: str("DAYS"),
		}),
	})

	return &tfsdk.State{Schema: *mover.SourceSchema, Raw: tftypes.NewValue(typ, values)}
}

func TestSLADomainMoveStateV0(t *testing.T) {
	r := newSLADomainResource().(*slaDomainResource)
	movers := r.MoveState(t.Context())
	if len(movers) != 1 {
		t.Fatalf("expected one mover for the single source schema version, got %d", len(movers))
	}
	mover := movers[0]

	target := slaDomainSchema(t.Context())
	move := func(t *testing.T, req resource.MoveStateRequest) resource.MoveStateResponse {
		t.Helper()
		res := resource.MoveStateResponse{TargetState: tfsdk.State{
			Schema: target,
			Raw:    tftypes.NewValue(target.Type().TerraformType(t.Context()), nil),
		}}
		mover.StateMover(t.Context(), req, &res)
		return res
	}

	id := uuid.NewString()
	t.Run("moves the state of a polaris SLA domain", func(t *testing.T) {
		for _, address := range []string{"registry.terraform.io/rubrikinc/polaris", "registry.terraform.io/rubrikinc/rubrik"} {
			res := move(t, resource.MoveStateRequest{
				SourceProviderAddress: address,
				SourceTypeName:        keyPolarisSLADomain,
				SourceSchemaVersion:   0,
				SourceState:           polarisSLADomainState(t, mover, id),
			})
			if res.Diagnostics.HasError() {
				t.Fatalf("%s: unexpected error: %v", address, res.Diagnostics)
			}

			var model slaDomainModel
			if diags := res.TargetState.Get(t.Context(), &model); diags.HasError() {
				t.Fatalf("%s: target state doesn't decode: %v", address, diags)
			}
			if model.ID.ValueString() != id || model.Name.ValueString() != "gold" ||
				model.Description.ValueString() != "the gold SLA" {
				t.Errorf("%s: id, name, description = %v, %v, %v", address, model.ID, model.Name, model.Description)
			}
			if len(model.DailySchedule) != 1 || model.DailySchedule[0].Frequency.ValueInt64() != 1 ||
				model.DailySchedule[0].Retention.ValueInt64() != 7 || model.DailySchedule[0].RetentionUnit.ValueString() != "DAYS" {
				t.Errorf("%s: daily schedule = %+v", address, model.DailySchedule)
			}
			if len(model.HourlySchedule) != 0 || len(model.Archival) != 0 {
				t.Errorf("%s: blocks which weren't in the source are set: %+v", address, model)
			}
		}
	})

	skipped := []struct {
		name string
		req  resource.MoveStateRequest
	}{
		{"another provider", resource.MoveStateRequest{SourceProviderAddress: "registry.terraform.io/hashicorp/aws", SourceTypeName: keyPolarisSLADomain}},
		{"another resource type", resource.MoveStateRequest{SourceProviderAddress: "registry.terraform.io/rubrikinc/polaris", SourceTypeName: keyPolarisSLADomainAssignment}},
		{"another schema version", resource.MoveStateRequest{SourceProviderAddress: "registry.terraform.io/rubrikinc/polaris", SourceTypeName: keyPolarisSLADomain, SourceSchemaVersion: 1}},
	}
	for _, tt := range skipped {
		t.Run("skips "+tt.name, func(t *testing.T) {
			tt.req.SourceState = polarisSLADomainState(t, mover, id)
			res := move(t, tt.req)
			if res.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", res.Diagnostics)
			}
			if !res.TargetState.Raw.IsNull() {
				t.Fatalf("the state was moved: %s", res.TargetState.Raw)
			}
		})
	}
}

// TestSLADomainMoveSourceSchemaMatchesSDKv2 compares the frozen source schema of
// the mover with the SDKv2 schema of the resource, which is what the polaris
// provider has. Both are walked on their own: every attribute and block has to
// be in the other with the same kind, and the same required, optional and
// computed flags. The SDKv2 resource is removed when the resource is migrated,
// and this test with it.
func TestSLADomainMoveSourceSchemaMatchesSDKv2(t *testing.T) {
	mover := newSLADomainResource().(*slaDomainResource).MoveState(t.Context())[0]

	type entry struct{ kind, flags string }
	flags := func(required, optional, computed bool) string {
		return fmt.Sprintf("required=%t optional=%t computed=%t", required, optional, computed)
	}

	sdk := map[string]entry{}
	var walkSDK func(prefix string, m map[string]*sdkschema.Schema)
	walkSDK = func(prefix string, m map[string]*sdkschema.Schema) {
		for name, s := range m {
			path := prefix + "." + name
			if r, ok := s.Elem.(*sdkschema.Resource); ok {
				sdk[path] = entry{kind: "block"}
				walkSDK(path, r.Schema)
				continue
			}
			kind := map[sdkschema.ValueType]string{
				sdkschema.TypeString: "string", sdkschema.TypeInt: "int64", sdkschema.TypeBool: "bool",
				sdkschema.TypeSet: "set", sdkschema.TypeList: "list",
			}[s.Type]
			sdk[path] = entry{kind: kind, flags: flags(s.Required, s.Optional, s.Computed)}
		}
	}
	walkSDK("", resourceSLADomain().Schema)

	frozen := map[string]entry{}
	var walkFrozen func(prefix string, attrs map[string]schema.Attribute, blocks map[string]schema.Block)
	walkFrozen = func(prefix string, attrs map[string]schema.Attribute, blocks map[string]schema.Block) {
		for name, a := range attrs {
			var kind string
			switch a.(type) {
			case schema.StringAttribute:
				kind = "string"
			case schema.Int64Attribute:
				kind = "int64"
			case schema.BoolAttribute:
				kind = "bool"
			case schema.SetAttribute:
				kind = "set"
			case schema.ListAttribute:
				kind = "list"
			default:
				t.Fatalf("%s.%s: unexpected attribute type %T", prefix, name, a)
			}
			frozen[prefix+"."+name] = entry{kind: kind, flags: flags(a.IsRequired(), a.IsOptional(), a.IsComputed())}
		}
		for name, b := range blocks {
			path := prefix + "." + name
			if _, ok := b.(schema.SingleNestedBlock); ok && path == "."+keyTimeouts {
				// The timeouts are a setting of the SDKv2 resource and not in its schema.
				continue
			}
			nested, ok := b.(schema.ListNestedBlock)
			if !ok {
				t.Fatalf("%s: unexpected block type %T", path, b)
			}
			frozen[path] = entry{kind: "block"}
			walkFrozen(path, nested.NestedObject.Attributes, nested.NestedObject.Blocks)
		}
	}
	walkFrozen("", mover.SourceSchema.Attributes, mover.SourceSchema.Blocks)

	if len(sdk) != len(frozen) {
		t.Errorf("the SDKv2 schema has %d entries, the frozen schema %d", len(sdk), len(frozen))
	}
	for path, want := range sdk {
		got, ok := frozen[path]
		switch {
		case !ok:
			t.Errorf("%s is in the SDKv2 schema and not in the frozen schema", path)
		case got != want:
			t.Errorf("%s: frozen %+v, SDKv2 %+v", path, got, want)
		}
	}
	for path := range frozen {
		if _, ok := sdk[path]; !ok {
			t.Errorf("%s is in the frozen schema and not in the SDKv2 schema", path)
		}
	}
	t.Logf("compared %d entries", len(sdk))
}
