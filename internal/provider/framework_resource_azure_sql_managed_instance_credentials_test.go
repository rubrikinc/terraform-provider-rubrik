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

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestValidateSQLCredentialsRequired verifies that the sql_credentials block is
// only demanded when RSC is the one creating the backup user.
//
// The combinations which depend on the authentication mechanisms the managed
// instance supports are not covered here: they need RSC to be queried, so they
// are rejected when the resource is applied rather than by this rule.
func TestValidateSQLCredentialsRequired(t *testing.T) {
	// Placeholders. The rule under test only checks whether the block is
	// present, so the values are never read.
	credentials := []sqlCredentialsModel{{
		SQLUsername: types.StringValue("example-login"),
		SQLPassword: types.StringValue("example-password"),
	}}

	tests := []struct {
		name    string
		config  azureSQLManagedInstanceCredentialsResourceModel
		wantErr bool
	}{{
		// RSC creates the backup user, so it needs a login to connect with.
		name: "CredentialsRequiredWhenScriptNotInstalled",
		config: azureSQLManagedInstanceCredentialsResourceModel{
			SetupScriptInstalled: types.BoolValue(false),
		},
		wantErr: true,
	}, {
		// The default is false, so a null value demands credentials too.
		name:    "CredentialsRequiredWhenScriptInstalledIsNull",
		config:  azureSQLManagedInstanceCredentialsResourceModel{},
		wantErr: true,
	}, {
		name: "CredentialsPresentWhenScriptNotInstalled",
		config: azureSQLManagedInstanceCredentialsResourceModel{
			SetupScriptInstalled: types.BoolValue(false),
			SQLCredentials:       credentials,
		},
	}, {
		// The script already created the user, so whether credentials are
		// needed depends on the managed instance and cannot be decided here.
		name: "CredentialsOptionalWhenScriptInstalled",
		config: azureSQLManagedInstanceCredentialsResourceModel{
			SetupScriptInstalled: types.BoolValue(true),
		},
	}, {
		name: "CredentialsAllowedWhenScriptInstalled",
		config: azureSQLManagedInstanceCredentialsResourceModel{
			SetupScriptInstalled: types.BoolValue(true),
			SQLCredentials:       credentials,
		},
	}, {
		// An unknown value comes from another resource and is not resolved
		// until apply, so there is nothing to check yet.
		name: "UnknownScriptInstalledSkipsValidation",
		config: azureSQLManagedInstanceCredentialsResourceModel{
			SetupScriptInstalled: types.BoolUnknown(),
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateSQLCredentialsRequired(tt.config)
			if hasErr := diags.HasError(); hasErr != tt.wantErr {
				t.Errorf("HasError: got %t, want %t (diagnostics: %v)", hasErr, tt.wantErr, diags)
			}
		})
	}
}
