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
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
)

func TestSLADomainResourceTypeNames(t *testing.T) {
	tests := []struct {
		name        string
		constructor func() resource.Resource
		wantType    string
		deprecated  bool
	}{
		{"rubrik", newSLADomainResource, "rubrik_sla_domain", false},
		{"polaris alias", newPolarisSLADomainResource, "polaris_sla_domain", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.constructor()

			var meta resource.MetadataResponse
			r.Metadata(t.Context(), resource.MetadataRequest{}, &meta)
			if meta.TypeName != tt.wantType {
				t.Errorf("type name = %q, want %q", meta.TypeName, tt.wantType)
			}

			var res resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &res)
			if diags := res.Schema.ValidateImplementation(t.Context()); diags.HasError() {
				t.Fatalf("invalid schema: %v", diags)
			}
			if got := res.Schema.DeprecationMessage != ""; got != tt.deprecated {
				t.Errorf("deprecated = %t, want %t", got, tt.deprecated)
			}

			var id resource.IdentitySchemaResponse
			r.(resource.ResourceWithIdentity).IdentitySchema(t.Context(), resource.IdentitySchemaRequest{}, &id)
			attr, ok := id.IdentitySchema.Attributes[keyID].(identityschema.StringAttribute)
			if !ok || !attr.RequiredForImport {
				t.Errorf("identity attribute %q = %+v, want a string required for import", keyID, id.IdentitySchema.Attributes[keyID])
			}

			validators := r.(resource.ResourceWithConfigValidators).ConfigValidators(t.Context())
			if len(validators) != 1 {
				t.Errorf("config validators = %d, want 1", len(validators))
			}
		})
	}
}
