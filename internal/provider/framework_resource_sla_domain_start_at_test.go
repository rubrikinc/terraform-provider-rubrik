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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateStartAtValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		withDay bool
		wantErr string // a substring of the error, "" for none
	}{
		// Without a day of the week, for the snapshot window.
		{name: "time", value: "16:45"},
		{name: "midnight", value: "00:00"},
		{name: "last minute of the day", value: "23:59"},
		{name: "hour 24", value: "24:00", wantErr: "invalid hour"},
		{name: "negative hour", value: "-1:00", wantErr: "invalid hour"},
		{name: "minute 60", value: "10:60", wantErr: "invalid minute"},
		{name: "negative minute", value: "10:-1", wantErr: "invalid minute"},
		{name: "hour which isn't a number", value: "ab:00", wantErr: "invalid hour"},
		{name: "minute which isn't a number", value: "10:cd", wantErr: "invalid minute"},
		{name: "no minute", value: "10", wantErr: "invalid time format"},
		{name: "three time parts", value: "10:00:00", wantErr: "invalid time format"},
		{name: "empty", value: "", wantErr: "invalid time format"},
		{name: "day not allowed", value: "Mon, 16:45", wantErr: "day of week not allowed"},
		{name: "three comma parts", value: "Mon, Tue, 16:45", wantErr: "invalid format"},

		// With a day of the week, for the first full snapshot.
		{name: "day and time", value: "Mon, 15:30", withDay: true},
		{name: "every day", value: "Sun, 00:00", withDay: true},
		{name: "day required", value: "15:30", withDay: true, wantErr: "day of week required"},
		{name: "unknown day", value: "Mox, 15:30", withDay: true, wantErr: "invalid day of week"},
		{name: "day in full", value: "Monday, 15:30", withDay: true, wantErr: "invalid day of week"},
		{name: "lower case day", value: "mon, 15:30", withDay: true, wantErr: "invalid day of week"},
		{name: "day with hour 24", value: "Mon, 24:00", withDay: true, wantErr: "invalid hour"},
		{name: "day with minute 60", value: "Mon, 15:60", withDay: true, wantErr: "invalid minute"},
		{name: "day with three comma parts", value: "Mon, Tue, 15:30", withDay: true, wantErr: "invalid format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStartAtValue("start_at", tt.value, tt.withDay)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected an error containing %q, got none", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestIsStartAtValidator(t *testing.T) {
	validate := func(v validator.String, value types.String) validator.StringResponse {
		var res validator.StringResponse
		v.ValidateString(t.Context(), validator.StringRequest{Path: path.Root("start_at"), ConfigValue: value}, &res)
		return res
	}

	t.Run("a null value isn't validated", func(t *testing.T) {
		if res := validate(isStartAt(true), types.StringNull()); res.Diagnostics.HasError() {
			t.Fatalf("unexpected error: %v", res.Diagnostics)
		}
	})
	t.Run("an unknown value isn't validated", func(t *testing.T) {
		if res := validate(isStartAt(true), types.StringUnknown()); res.Diagnostics.HasError() {
			t.Fatalf("unexpected error: %v", res.Diagnostics)
		}
	})
	t.Run("a valid value passes", func(t *testing.T) {
		if res := validate(isStartAt(false), types.StringValue("16:45")); res.Diagnostics.HasError() {
			t.Fatalf("unexpected error: %v", res.Diagnostics)
		}
	})
	t.Run("an invalid value is an error on the attribute", func(t *testing.T) {
		res := validate(isStartAt(false), types.StringValue("Mon, 16:45"))
		if !res.Diagnostics.HasError() {
			t.Fatal("expected an error")
		}
		if got := res.Diagnostics.Errors()[0].Summary(); got != "Invalid Start Time" {
			t.Fatalf("summary = %q", got)
		}
	})
	t.Run("the mode is the one the validator was made with", func(t *testing.T) {
		if res := validate(isStartAt(true), types.StringValue("16:45")); !res.Diagnostics.HasError() {
			t.Fatal("a value without a day passed the validator which requires one")
		}
		if res := validate(isStartAt(false), types.StringValue("Mon, 16:45")); !res.Diagnostics.HasError() {
			t.Fatal("a value with a day passed the validator which doesn't allow one")
		}
	})
}
