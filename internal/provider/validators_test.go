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

	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

func TestValidatePermissionGroup(t *testing.T) {
	for _, tc := range []struct {
		name      string
		value     string
		expectErr bool
	}{
		{
			name:      "FeatureGroup",
			value:     "RESTORE",
			expectErr: false,
		},
		{
			name:      "ExtraGroup",
			value:     "EXPORT_AND_RESTORE",
			expectErr: false,
		},
		{
			name:      "OtherGroup",
			value:     "RECOVERY",
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			validate := validatePermissionGroup(core.AWSCloudFormationFeatures(), core.CloudNativeProtection, core.PermissionGroupExportAndRestore)

			_, errs := validate(tc.value, keyPermissionGroups)

			if tc.expectErr && len(errs) == 0 {
				t.Errorf("expected error for %q, got none", tc.value)
			}
			if !tc.expectErr && len(errs) != 0 {
				t.Errorf("expected no error for %q, got: %v", tc.value, errs)
			}
		})
	}

	t.Run("UnknownFeature", func(t *testing.T) {
		validate := validatePermissionGroup(core.AWSCloudFormationFeatures(), core.CloudSQLProtection)

		_, errs := validate("BASIC", keyPermissionGroups)

		if len(errs) == 0 {
			t.Error("expected error for unknown feature, got none")
		}
	})
}

func TestCheckFeaturePermissionGroups(t *testing.T) {
	for _, tc := range []struct {
		name      string
		feature   string
		groups    []string
		expectErr bool
	}{
		{
			name:      "SupportedGroups",
			feature:   core.CloudSQLProtection,
			groups:    []string{"BASIC", "EXPORT_AND_RESTORE"},
			expectErr: false,
		},
		{
			name:      "UnsupportedGroup",
			feature:   core.CloudSQLProtection,
			groups:    []string{"BASIC", "ENCRYPTION"},
			expectErr: true,
		},
		{
			name:      "NoGroups",
			feature:   core.CloudSQLProtection,
			groups:    nil,
			expectErr: false,
		},
		{
			name:      "UnknownFeature",
			feature:   core.AzureSQLDBProtection,
			groups:    []string{"BASIC"},
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkFeaturePermissionGroups(core.GCPProjectFeatures(), tc.feature, tc.groups)

			if tc.expectErr && err == nil {
				t.Errorf("expected error for %s %v, got none", tc.feature, tc.groups)
			}
			if !tc.expectErr && err != nil {
				t.Errorf("expected no error for %s %v, got: %s", tc.feature, tc.groups, err)
			}
		})
	}
}
