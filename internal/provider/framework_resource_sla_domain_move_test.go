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
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
