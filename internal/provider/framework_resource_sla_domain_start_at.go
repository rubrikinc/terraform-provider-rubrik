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
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// isStartAt returns a validator that checks that a string value is a valid
// start time, like "Mon, 15:30" or "16:45". The day of the week is optional in
// the API, but the endpoints use the value in different ways. The first full
// snapshot requires the day of the week, and the snapshot window requires it to
// be omitted.
func isStartAt(withDay bool) validator.String {
	return isStartAtValidator{withDay: withDay}
}

type isStartAtValidator struct {
	withDay bool
}

func (v isStartAtValidator) Description(_ context.Context) string {
	if v.withDay {
		return `value must be a start time with a day of the week, like "Mon, 15:30"`
	}
	return `value must be a start time without a day of the week, like "15:30"`
}

func (v isStartAtValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isStartAtValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if err := validateStartAtValue(req.Path.String(), req.ConfigValue.ValueString(), v.withDay); err != nil {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid Start Time", err.Error())
	}
}

// validateStartAtValue validates a start time. The key is the name of the
// attribute, which is used in the error messages.
func validateStartAtValue(key, value string, withDay bool) error {
	parts := strings.Split(value, ", ")
	var timeParts []string
	switch len(parts) {
	case 1:
		// Day of week not specified.
		if withDay {
			return fmt.Errorf("day of week required for %s: %s", key, value)
		}
		timeParts = strings.Split(parts[0], ":")
	case 2:
		// Day of week specified.
		if !withDay {
			return fmt.Errorf("day of week not allowed for %s: %s", key, value)
		}
		if !slices.Contains([]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}, parts[0]) {
			return fmt.Errorf("invalid day of week for %s: %s", key, value)
		}
		timeParts = strings.Split(parts[1], ":")
	default:
		return fmt.Errorf("invalid format for %s: %s", key, value)
	}
	if len(timeParts) != 2 {
		return fmt.Errorf("invalid time format for %s: %s", key, value)
	}

	if n, err := strconv.Atoi(timeParts[0]); err != nil || n < 0 || n > 23 {
		return fmt.Errorf("invalid hour for %s: %s", key, value)
	}

	if n, err := strconv.Atoi(timeParts[1]); err != nil || n < 0 || n > 59 {
		return fmt.Errorf("invalid minute for %s: %s", key, value)
	}

	return nil
}
