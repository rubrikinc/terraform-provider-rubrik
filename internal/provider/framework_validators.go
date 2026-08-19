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
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// isUUID returns a validator that checks if a string value is a valid UUID.
func isUUID() validator.String {
	return isUUIDValidator{}
}

type isUUIDValidator struct{}

func (v isUUIDValidator) Description(_ context.Context) string {
	return "value must be a valid UUID"
}

func (v isUUIDValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isUUIDValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if _, err := uuid.Parse(req.ConfigValue.ValueString()); err != nil {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid UUID",
			fmt.Sprintf("%q is not a valid UUID: %s", req.ConfigValue.ValueString(), err))
	}
}

// isNotWhiteSpace returns a validator that checks if a string value is not
// empty or contains only whitespace.
func isNotWhiteSpace() validator.String {
	return isNotWhiteSpaceValidator{}
}

type isNotWhiteSpaceValidator struct{}

func (v isNotWhiteSpaceValidator) Description(_ context.Context) string {
	return "value must not be empty or contain only whitespace"
}

func (v isNotWhiteSpaceValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isNotWhiteSpaceValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if strings.TrimSpace(req.ConfigValue.ValueString()) == "" {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid Value",
			"value must not be empty or contain only whitespace")
	}
}

// isFilterTypeFor returns a validator that checks a data security policy filter
// type belongs to one of the given resource types. RSC requires every condition
// in a group to cover the same resource type, so this keeps a misplaced
// condition from reaching the API as an opaque format error. A filter type that
// does not belong to any known resource type passes, leaving RSC to reject it.
func isFilterTypeFor(resourceTypes ...filterResourceType) validator.String {
	return isFilterTypeForValidator{resourceTypes: resourceTypes}
}

type isFilterTypeForValidator struct {
	resourceTypes []filterResourceType
}

func (v isFilterTypeForValidator) Description(_ context.Context) string {
	names := make([]string, 0, len(v.resourceTypes))
	for _, resourceType := range v.resourceTypes {
		names = append(names, string(resourceType))
	}

	return fmt.Sprintf("value must be a %s condition type", strings.Join(names, " or "))
}

func (v isFilterTypeForValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isFilterTypeForValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// An unrecognized filter type is left to RSC to reject, rather than being
	// rejected here against a prefix list that can fall behind the filter types
	// RSC registers.
	filterType := req.ConfigValue.ValueString()
	resourceType, ok := resourceTypeOf(filterType)
	if !ok {
		return
	}

	for _, allowed := range v.resourceTypes {
		if resourceType == allowed {
			return
		}
	}

	blockName := keyObjectFilter
	if resourceType == filterResourceTypeIdentity {
		blockName = keyIdentityFilter
	}
	res.Diagnostics.AddAttributeError(req.Path, "Filter type in the wrong block", fmt.Sprintf(
		"%q is an %s condition and cannot be used here. Move it to a %s block.",
		filterType, resourceType, blockName))
}

// setMustContain returns a validator that checks a set of strings contains the
// given value. A null or unknown set passes (nothing to validate yet).
func setMustContain(value string) validator.Set {
	return setMustContainValidator{value: value}
}

type setMustContainValidator struct {
	value string
}

func (v setMustContainValidator) Description(_ context.Context) string {
	return fmt.Sprintf("set must contain %q", v.value)
}

func (v setMustContainValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v setMustContainValidator) ValidateSet(ctx context.Context, req validator.SetRequest, res *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var values []string
	res.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &values, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	for _, value := range values {
		if value == v.value {
			return
		}
	}

	res.Diagnostics.AddAttributeError(req.Path, "Missing required value",
		fmt.Sprintf("%q must be included in the set", v.value))
}
