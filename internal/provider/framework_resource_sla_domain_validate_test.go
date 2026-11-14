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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// zeroValue returns the value the framework delivers for an attribute which
// isn't configured. Terraform sends a block which isn't configured as an empty
// list, and the framework turns it into null before the provider sees it.
func zeroValue(typ tftypes.Type) tftypes.Value {
	return tftypes.NewValue(typ, nil)
}

// slaDomainConfig returns an SLA domain config where the given attributes have
// the given values, and the others aren't configured.
func slaDomainConfig(t *testing.T, set map[string]tftypes.Value) tfsdk.Config {
	t.Helper()

	s := slaDomainSchema(t.Context())
	typ := s.Type().TerraformType(t.Context()).(tftypes.Object)
	values := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, attrType := range typ.AttributeTypes {
		values[name] = zeroValue(attrType)
	}
	for name, value := range set {
		values[name] = value
	}
	return tfsdk.Config{Schema: s, Raw: tftypes.NewValue(typ, values)}
}

// oneBlock returns the named block with a single element, which has no
// attributes set.
func oneBlock(t *testing.T, name string) tftypes.Value {
	t.Helper()

	s := slaDomainSchema(t.Context())
	typ := s.Type().TerraformType(t.Context()).(tftypes.Object).AttributeTypes[name].(tftypes.List)
	elem := typ.ElementType.(tftypes.Object)
	values := make(map[string]tftypes.Value, len(elem.AttributeTypes))
	for attrName, attrType := range elem.AttributeTypes {
		values[attrName] = zeroValue(attrType)
	}
	return tftypes.NewValue(typ, []tftypes.Value{tftypes.NewValue(elem, values)})
}

func TestSLADomainAtLeastOneBlock(t *testing.T) {
	s := slaDomainSchema(t.Context())
	typ := s.Type().TerraformType(t.Context()).(tftypes.Object)

	validate := func(cfg tfsdk.Config) resource.ValidateConfigResponse {
		var resp resource.ValidateConfigResponse
		slaDomainAtLeastOneBlock().ValidateResource(t.Context(), resource.ValidateConfigRequest{Config: cfg}, &resp)
		return resp
	}

	t.Run("no block is an error, reported once", func(t *testing.T) {
		resp := validate(slaDomainConfig(t, nil))
		if n := resp.Diagnostics.ErrorsCount(); n != 1 {
			t.Fatalf("expected 1 error, got %d: %v", n, resp.Diagnostics)
		}
	})

	// The blocks are listed here, and not taken from the rule, so that a block
	// which is dropped from the rule fails the test.
	for _, name := range []string{
		keyDailySchedule, keyHourlySchedule, keyMinuteSchedule, keyMonthlySchedule, keyQuarterlySchedule,
		keyWeeklySchedule, keyYearlySchedule, keyAWSRDSConfig, keyAzureSQLDatabaseConfig,
		keyAzureSQLManagedInstanceConfig,
	} {
		t.Run(name+" alone is enough", func(t *testing.T) {
			resp := validate(slaDomainConfig(t, map[string]tftypes.Value{name: oneBlock(t, name)}))
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics)
			}
		})

		t.Run(name+" unknown is not decided", func(t *testing.T) {
			unknown := tftypes.NewValue(typ.AttributeTypes[name], tftypes.UnknownValue)
			resp := validate(slaDomainConfig(t, map[string]tftypes.Value{name: unknown}))
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics)
			}
		})
	}

	t.Run("an empty list is not null, so it counts as configured", func(t *testing.T) {
		// The framework never delivers an empty list for a block, so this is
		// the stock validator's behaviour and not one the resource relies on.
		empty := tftypes.NewValue(typ.AttributeTypes[keyDailySchedule], []tftypes.Value{})
		resp := validate(slaDomainConfig(t, map[string]tftypes.Value{keyDailySchedule: empty}))
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error: %v", resp.Diagnostics)
		}
	})

	t.Run("blocks which aren't snapshot blocks don't count", func(t *testing.T) {
		resp := validate(slaDomainConfig(t, map[string]tftypes.Value{keyVMwareVMConfig: oneBlock(t, keyVMwareVMConfig)}))
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error")
		}
	})
}
