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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
)

// TestAzureExocomputeClusterAttachmentReadFailedLookup reads state without an
// identity, as written by the SDKv2 resource or moved from the polaris_
// resource, while the cluster connection can't be looked up, which is the case
// until the manifest has been applied to the cluster. Read must keep the state
// and set the identity from the ID. The fake RSC fails every request.
func TestAzureExocomputeClusterAttachmentReadFailedLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	r := &azureExocomputeClusterAttachmentResource{
		client: &client{polarisClient: &polaris.Client{GQL: graphql.NewTestClient(srv)}},
		prefix: keyRubrik,
	}

	var schemaRes resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schemaRes)
	if schemaRes.Diagnostics.HasError() {
		t.Fatal(schemaRes.Diagnostics)
	}
	var identitySchemaRes resource.IdentitySchemaResponse
	r.IdentitySchema(t.Context(), resource.IdentitySchemaRequest{}, &identitySchemaRes)
	if identitySchemaRes.Diagnostics.HasError() {
		t.Fatal(identitySchemaRes.Diagnostics)
	}
	stateType := schemaRes.Schema.Type().TerraformType(t.Context())
	identityType := identitySchemaRes.IdentitySchema.Type().TerraformType(t.Context())

	const id = "8d8f7d2a-0f6e-4b0c-8e2f-9b9e4c6f1a2b"
	values := map[string]tftypes.Value{
		keyID:           tftypes.NewValue(tftypes.String, id),
		keyClusterName:  tftypes.NewValue(tftypes.String, "cluster"),
		keyExocomputeID: tftypes.NewValue(tftypes.String, "3e1b5c9a-7d2f-4a6e-9c8b-1f0e2d3c4b5a"),
		keyManifest:     tftypes.NewValue(tftypes.String, "manifest"),
		keyTokenRefresh: tftypes.NewValue(tftypes.Number, nil),
	}

	tests := []struct {
		name  string
		moved bool
	}{{
		name: "UpgradedState",
	}, {
		name:  "MovedState",
		moved: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := tfsdk.State{Schema: schemaRes.Schema, Raw: tftypes.NewValue(stateType, values)}
			if tc.moved {
				mover := r.MoveState(t.Context())[0]
				sourceType := mover.SourceSchema.Type().TerraformType(t.Context())
				req := resource.MoveStateRequest{
					SourceProviderAddress: "registry.terraform.io/rubrikinc/polaris",
					SourceTypeName:        keyPolaris + "_" + keyAzureExocomputeClusterAttachment,
					SourceSchemaVersion:   0,
					SourceState:           &tfsdk.State{Schema: mover.SourceSchema, Raw: tftypes.NewValue(sourceType, values)},
				}
				res := resource.MoveStateResponse{
					TargetState: tfsdk.State{Schema: schemaRes.Schema, Raw: tftypes.NewValue(stateType, nil)},
				}
				mover.StateMover(t.Context(), req, &res)
				if res.Diagnostics.HasError() {
					t.Fatal(res.Diagnostics)
				}
				state = res.TargetState
			}

			nullIdentity := &tfsdk.ResourceIdentity{
				Schema: identitySchemaRes.IdentitySchema,
				Raw:    tftypes.NewValue(identityType, nil),
			}
			res := resource.ReadResponse{State: state, Identity: nullIdentity}
			r.Read(t.Context(), resource.ReadRequest{State: state, Identity: nullIdentity}, &res)
			if res.Diagnostics.HasError() {
				t.Fatal(res.Diagnostics)
			}

			if !res.State.Raw.Equal(tftypes.NewValue(stateType, values)) {
				t.Errorf("state changed: %v", res.State.Raw)
			}
			if res.Identity.Raw.IsNull() {
				t.Fatal("identity not set")
			}
			var identity azureExocomputeClusterAttachmentIdentityModel
			res.Diagnostics.Append(res.Identity.Get(t.Context(), &identity)...)
			if res.Diagnostics.HasError() {
				t.Fatal(res.Diagnostics)
			}
			if identity.ID.ValueString() != id {
				t.Errorf("identity id: got %q, want %q", identity.ID.ValueString(), id)
			}
		})
	}
}
