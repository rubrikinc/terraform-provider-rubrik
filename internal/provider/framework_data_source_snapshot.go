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
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/hierarchy"
)

const dataSourceSnapshotDescription = `
The ´rubrik_snapshot´ data source looks up the closest snapshot to a given
point in time for a workload object.

Exactly one of ´before_time´ or ´after_time´ must be specified:

- ´before_time´ — returns the latest snapshot with date <= the given timestamp.
- ´after_time´ — returns the earliest snapshot with date >= the given timestamp.

Only clean snapshots are returned by default. Set ´exclude_quarantined´ or
´exclude_anomalous´ to ´false´ to include those snapshot types.
`

var _ datasource.DataSource = &snapshotDataSource{}

type snapshotDataSource struct {
	client *client
	prefix string
}

// snapshotModel is the data source state. The workload ID and the time bounds
// are lookup keys, the exclude flags are optional with a default of true, and
// the ID and date are computed.
type snapshotModel struct {
	ID                 types.String `tfsdk:"id"`
	WorkloadID         types.String `tfsdk:"workload_id"`
	BeforeTime         types.String `tfsdk:"before_time"`
	AfterTime          types.String `tfsdk:"after_time"`
	ExcludeQuarantined types.Bool   `tfsdk:"exclude_quarantined"`
	ExcludeAnomalous   types.Bool   `tfsdk:"exclude_anomalous"`
	Date               types.String `tfsdk:"date"`
}

func newSnapshotDataSource() datasource.DataSource {
	return &snapshotDataSource{prefix: keyRubrik}
}

func newPolarisSnapshotDataSource() datasource.DataSource {
	return &snapshotDataSource{prefix: keyPolaris}
}

func (d *snapshotDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "snapshotDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keySnapshot
}

func (d *snapshotDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "snapshotDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceSnapshotDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Snapshot ID (UUID).",
			},
			keyWorkloadID: schema.StringAttribute{
				Required:    true,
				Description: "Workload ID (UUID).",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyBeforeTime: schema.StringAttribute{
				Optional: true,
				Description: "Return the latest snapshot with date <= this RFC 3339 timestamp " +
					"(e.g. 2025-01-01T00:00:00Z). Mutually exclusive with ´after_time´.",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyBeforeTime), path.MatchRoot(keyAfterTime)),
					isRFC3339(),
				},
			},
			keyAfterTime: schema.StringAttribute{
				Optional: true,
				Description: "Return the earliest snapshot with date >= this RFC 3339 timestamp " +
					"(e.g. 2025-01-01T00:00:00Z). Mutually exclusive with ´before_time´.",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyBeforeTime), path.MatchRoot(keyAfterTime)),
					isRFC3339(),
				},
			},
			// A data source attribute has no schema default. The attributes are
			// computed so that Read can set the default, true, when they are not
			// configured. This keeps the value of true in the state, as the SDKv2
			// default did.
			keyExcludeQuarantined: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Exclude quarantined snapshots. Defaults to `true`.",
			},
			keyExcludeAnomalous: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Exclude anomalous snapshots. Defaults to `true`.",
			},
			keyDate: schema.StringAttribute{
				Computed:    true,
				Description: "Snapshot timestamp.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_snapshot` data source instead."
	}
}

func (d *snapshotDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "snapshotDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *snapshotDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "snapshotDataSource.Read")

	var config snapshotModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// The exclude flags default to true when they are not configured.
	if config.ExcludeQuarantined.IsNull() {
		config.ExcludeQuarantined = types.BoolValue(true)
	}
	if config.ExcludeAnomalous.IsNull() {
		config.ExcludeAnomalous = types.BoolValue(true)
	}

	params := hierarchy.ClosestSnapshotParams{
		WorkloadID:         config.WorkloadID.ValueString(),
		ExcludeQuarantined: config.ExcludeQuarantined.ValueBool(),
		ExcludeAnomalous:   config.ExcludeAnomalous.ValueBool(),
	}

	if !config.AfterTime.IsNull() {
		t, err := time.Parse(time.RFC3339, config.AfterTime.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid after_time", err.Error())
			return
		}
		params.AfterTime = &t
	} else {
		t, err := time.Parse(time.RFC3339, config.BeforeTime.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid before_time", err.Error())
			return
		}
		params.BeforeTime = &t
	}

	result, err := hierarchy.Wrap(polarisClient.GQL).ClosestSnapshot(ctx, params)
	if err != nil {
		res.Diagnostics.AddError("Failed to read snapshot", err.Error())
		return
	}
	if result.Snapshot == nil {
		res.Diagnostics.AddError("Failed to read snapshot",
			fmt.Sprintf("no snapshot found for workload %s", params.WorkloadID))
		return
	}

	config.ID = types.StringValue(result.Snapshot.ID)
	config.Date = types.StringValue(result.Snapshot.Date.Format(time.RFC3339))
	res.Diagnostics.Append(res.State.Set(ctx, &config)...)
}
