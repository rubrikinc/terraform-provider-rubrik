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
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *gcpServiceAccountResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "gcpServiceAccountResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(),
	}
}

// gcpServiceAccountSourceSchema is the SDKv2 schema of the
// polaris_gcp_service_account resource. The attributes are the same in v0 and
// v1, only the meaning of the ID differs.
var gcpServiceAccountSourceSchema = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:              schema.StringAttribute{Computed: true},
		keyCredentials:     schema.StringAttribute{Required: true},
		keyName:            schema.StringAttribute{Optional: true, Computed: true},
		keyPermissionsHash: schema.StringAttribute{Optional: true},
	},
}

// moveStateV0 moves v0 state, written by polaris provider versions before
// v1.3.0, from the polaris_gcp_service_account resource to the
// rubrik_gcp_service_account resource. In v0 the ID is the service account
// name, in v1 it is the SHA-256 sum of the name.
//
// Unlike the v0 state upgrader, the mover doesn't read the name from RSC.
// Terraform doesn't configure the resource before moving state, so there is
// no RSC client. The name in the source state is used instead, Read refreshes
// it from RSC.
func (r *gcpServiceAccountResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: gcpServiceAccountSourceSchema,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "gcpServiceAccountResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyGCPServiceAccount {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state gcpServiceAccountModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			name := state.Name.ValueString()
			if name == "" {
				name = state.ID.ValueString()
			}
			state.ID = types.StringValue(fmt.Sprintf("%x", sha256.Sum256([]byte(name))))

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves v1 state from the polaris_gcp_service_account resource to
// the rubrik_gcp_service_account resource. The state is copied as is.
func (r *gcpServiceAccountResource) moveStateV1() resource.StateMover {
	return resource.StateMover{
		SourceSchema: gcpServiceAccountSourceSchema,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "gcpServiceAccountResource.moveStateV1")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyGCPServiceAccount {
				return
			}
			if req.SourceSchemaVersion != 1 {
				return
			}

			var state gcpServiceAccountModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
