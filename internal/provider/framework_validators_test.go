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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestIsUUIDValidator(t *testing.T) {
	tests := []struct {
		name      string
		value     basetypes.StringValue
		expectErr bool
	}{
		{
			name:      "ValidUUID",
			value:     basetypes.NewStringValue("550e8400-e29b-41d4-a716-446655440000"),
			expectErr: false,
		},
		{
			name:      "InvalidUUID",
			value:     basetypes.NewStringValue("not-a-uuid"),
			expectErr: true,
		},
		{
			name:      "EmptyString",
			value:     basetypes.NewStringValue(""),
			expectErr: true,
		},
		{
			name:      "NullValue",
			value:     basetypes.NewStringNull(),
			expectErr: false,
		},
		{
			name:      "UnknownValue",
			value:     basetypes.NewStringUnknown(),
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				ConfigValue: tc.value,
			}
			var res validator.StringResponse

			isUUIDValidator{}.ValidateString(context.Background(), req, &res)

			if tc.expectErr && !res.Diagnostics.HasError() {
				t.Errorf("expected error for %q, got none", tc.value)
			}
			if !tc.expectErr && res.Diagnostics.HasError() {
				t.Errorf("expected no error for %q, got: %s", tc.value, res.Diagnostics.Errors())
			}
		})
	}
}

func TestIsNotWhiteSpaceValidator(t *testing.T) {
	tests := []struct {
		name      string
		value     basetypes.StringValue
		expectErr bool
	}{
		{
			name:      "ValidString",
			value:     basetypes.NewStringValue("valid"),
			expectErr: false,
		},
		{
			name:      "StringWithSpaces",
			value:     basetypes.NewStringValue("  has spaces  "),
			expectErr: false,
		},
		{
			name:      "EmptyString",
			value:     basetypes.NewStringValue(""),
			expectErr: true,
		},
		{
			name:      "SpacesOnly",
			value:     basetypes.NewStringValue("   "),
			expectErr: true,
		},
		{
			name:      "TabsAndNewlines",
			value:     basetypes.NewStringValue("\t\n"),
			expectErr: true,
		},
		{
			name:      "NullValue",
			value:     basetypes.NewStringNull(),
			expectErr: false,
		},
		{
			name:      "UnknownValue",
			value:     basetypes.NewStringUnknown(),
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				ConfigValue: tc.value,
			}
			var res validator.StringResponse

			isNotWhiteSpaceValidator{}.ValidateString(context.Background(), req, &res)

			if tc.expectErr && !res.Diagnostics.HasError() {
				t.Errorf("expected error for %q, got none", tc.value)
			}
			if !tc.expectErr && res.Diagnostics.HasError() {
				t.Errorf("expected no error for %q, got: %s", tc.value, res.Diagnostics.Errors())
			}
		})
	}
}

func TestIsFilterTypeForValidator(t *testing.T) {
	tests := []struct {
		name          string
		resourceTypes []filterResourceType
		value         basetypes.StringValue
		expectErr     bool
	}{
		{
			name:          "ObjectTypeInObjectBlock",
			resourceTypes: []filterResourceType{filterResourceTypeObject},
			value:         basetypes.NewStringValue("SECURITY_DOCUMENT_SENSITIVITY"),
			expectErr:     false,
		},
		{
			name:          "SnappableTypeInObjectBlock",
			resourceTypes: []filterResourceType{filterResourceTypeObject},
			value:         basetypes.NewStringValue("SECURITY_SNAPPABLE_BACKUP"),
			expectErr:     false,
		},
		{
			name:          "IdentityTypeInObjectBlock",
			resourceTypes: []filterResourceType{filterResourceTypeObject},
			value:         basetypes.NewStringValue("SECURITY_IDENTITY_NAME"),
			expectErr:     true,
		},
		{
			name:          "ObjectTypeInIdentityBlock",
			resourceTypes: []filterResourceType{filterResourceTypeIdentity},
			value:         basetypes.NewStringValue("SECURITY_DOCUMENT_SENSITIVITY"),
			expectErr:     true,
		},
		{
			name:          "GpoTypeInIdentityBlock",
			resourceTypes: []filterResourceType{filterResourceTypeIdentity},
			value:         basetypes.NewStringValue("SECURITY_GPO_LDAP_SIGNING"),
			expectErr:     false,
		},
		{
			name:          "IdentityTypeInThresholdBlock",
			resourceTypes: []filterResourceType{filterResourceTypeObject, filterResourceTypeIdentity},
			value:         basetypes.NewStringValue("SECURITY_IDENTITY_NAME"),
			expectErr:     false,
		},
		// A filter type belonging to no known resource type cannot be placed in
		// the wrong block, so it passes and is left to RSC to reject.
		{
			name:          "IdpTypeIsLeftToRSC",
			resourceTypes: []filterResourceType{filterResourceTypeObject, filterResourceTypeIdentity},
			value:         basetypes.NewStringValue("SECURITY_IDP_TYPE"),
			expectErr:     false,
		},
		{
			name:          "UnknownTypeIsLeftToRSC",
			resourceTypes: []filterResourceType{filterResourceTypeObject},
			value:         basetypes.NewStringValue("SECURITY_FUTURE_FAMILY_NAME"),
			expectErr:     false,
		},
		{
			name:          "NullValue",
			resourceTypes: []filterResourceType{filterResourceTypeObject},
			value:         basetypes.NewStringNull(),
			expectErr:     false,
		},
		{
			name:          "UnknownValue",
			resourceTypes: []filterResourceType{filterResourceTypeObject},
			value:         basetypes.NewStringUnknown(),
			expectErr:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				ConfigValue: tc.value,
			}
			var res validator.StringResponse

			isFilterTypeForValidator{resourceTypes: tc.resourceTypes}.ValidateString(context.Background(), req, &res)

			if tc.expectErr && !res.Diagnostics.HasError() {
				t.Errorf("expected error for %q, got none", tc.value)
			}
			if !tc.expectErr && res.Diagnostics.HasError() {
				t.Errorf("expected no error for %q, got: %s", tc.value, res.Diagnostics.Errors())
			}
		})
	}
}
