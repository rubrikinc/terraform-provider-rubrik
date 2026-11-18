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

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestSLADomainModelMatchesSchema decodes an SLA domain config into the model.
// The framework reports a struct field without an attribute, and an attribute
// without a struct field, so a wrong or missing tfsdk tag fails the test.
func TestSLADomainModelMatchesSchema(t *testing.T) {
	t.Run("nothing configured", func(t *testing.T) {
		var model slaDomainModel
		if diags := slaDomainConfig(t, nil).Get(t.Context(), &model); diags.HasError() {
			t.Fatalf("decode failed: %v", diags)
		}
	})

	t.Run("every block configured", func(t *testing.T) {
		s := slaDomainSchema(t.Context())
		set := make(map[string]tftypes.Value)
		for name, block := range s.Blocks {
			if name == keyTimeouts {
				continue
			}
			set[name] = oneBlock(t, name)
			_ = block
		}

		var model slaDomainModel
		if diags := slaDomainConfig(t, set).Get(t.Context(), &model); diags.HasError() {
			t.Fatalf("decode failed: %v", diags)
		}
		if len(model.Archival) != 1 || len(model.WeeklySchedule) != 1 || len(model.SapHanaConfig) != 1 ||
			len(model.AzureSQLDatabaseConfig) != 1 || len(model.NCDConfig) != 1 {
			t.Fatalf("blocks not decoded: %+v", model)
		}
	})
}
