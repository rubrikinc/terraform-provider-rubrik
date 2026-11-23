// Copyright 2025 Rubrik, Inc.
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
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
)

const dataSourceGCPArchivalLocationDescription = `
The ´rubrik_gcp_archival_location´ data source is used to access information
about a GCP archival location. An archival location is looked up using either
the ID or the name.
`

var (
	_ datasource.DataSource              = &gcpArchivalLocationDataSource{}
	_ datasource.DataSourceWithConfigure = &gcpArchivalLocationDataSource{}
)

type gcpArchivalLocationDataSource struct {
	client *client
	prefix string
}

type gcpArchivalLocationModel struct {
	ID                 types.String `tfsdk:"id"`
	BucketLabels       types.Map    `tfsdk:"bucket_labels"`
	BucketPrefix       types.String `tfsdk:"bucket_prefix"`
	CloudAccountID     types.String `tfsdk:"cloud_account_id"`
	ConnectionStatus   types.String `tfsdk:"connection_status"`
	CustomerManagedKey types.Set    `tfsdk:"customer_managed_key"`
	LocationTemplate   types.String `tfsdk:"location_template"`
	Name               types.String `tfsdk:"name"`
	Region             types.String `tfsdk:"region"`
	StorageClass       types.String `tfsdk:"storage_class"`
}

func newGcpArchivalLocationDataSource() datasource.DataSource {
	return &gcpArchivalLocationDataSource{prefix: keyRubrik}
}

func newPolarisGcpArchivalLocationDataSource() datasource.DataSource {
	return &gcpArchivalLocationDataSource{prefix: keyPolaris}
}

func (d *gcpArchivalLocationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyGCPArchivalLocation
}

func (d *gcpArchivalLocationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceGCPArchivalLocationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cloud native archival location ID (UUID).",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyID), path.MatchRoot(keyName)),
					isUUID(),
				},
			},
			keyBucketLabels: schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "GCP bucket labels.",
			},
			keyBucketPrefix: schema.StringAttribute{
				Computed:    true,
				Description: "GCP bucket prefix. Note, `rubrik-` will always be prepended to the prefix.",
			},
			keyCloudAccountID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status of the archival location.",
			},
			keyCustomerManagedKey: schema.SetNestedAttribute{
				Computed: true,
				Description: "Customer managed storage encryption. For `SPECIFIC_REGION`, a customer managed key " +
					"for the specific region will be returned. For `SOURCE_REGION`, a customer managed key for each " +
					"source region will be returned, for other regions, data will be encrypted using platform " +
					"managed keys.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Computed:    true,
							Description: "Key name.",
						},
						keyRegion: schema.StringAttribute{
							Computed:    true,
							Description: "The region in which the key will be used.",
						},
						keyRingName: schema.StringAttribute{
							Computed:    true,
							Description: "Key ring name.",
						},
					},
				},
			},
			keyLocationTemplate: schema.StringAttribute{
				Computed: true,
				Description: "RSC location template. If a region was specified, it will be `SPECIFIC_REGION`, " +
					"otherwise `SOURCE_REGION`.",
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the cloud native archival location.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyRegion: schema.StringAttribute{
				Computed: true,
				Description: "GCP region to store the snapshots in (`SPECIFIC_REGION`). If not specified, the " +
					"snapshots will be stored in the same region as the workload (`SOURCE_REGION`).",
			},
			keyStorageClass: schema.StringAttribute{
				Computed: true,
				Description: "GCP bucket storage class. Possible values are `ARCHIVE`, `COLDLINE`, " +
					"`NEARLINE`, `STANDARD` and `DURABLE_REDUCED_AVAILABILITY`.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_gcp_archival_location` data source instead."
	}
}

func (d *gcpArchivalLocationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *gcpArchivalLocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationDataSource.Read")

	var config gcpArchivalLocationModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// Read the archival location using either the ID or the name.
	var targetMapping gqlarchival.GCPTargetMapping
	if !config.ID.IsNull() {
		id, err := uuid.Parse(config.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid ID", err.Error())
			return
		}
		targetMapping, err = archival.Wrap(polarisClient).GCPTargetMappingByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP archival location", err.Error())
			return
		}
	} else {
		targetMapping, err = archival.Wrap(polarisClient).GCPTargetMappingByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP archival location", err.Error())
			return
		}
	}
	targetTemplate := targetMapping.TargetTemplate

	labelValues := make(map[string]attr.Value, len(targetTemplate.BucketLabels))
	for _, tag := range targetTemplate.BucketLabels {
		labelValues[tag.Key] = types.StringValue(tag.Value)
	}
	labelMap, diags := types.MapValue(types.StringType, labelValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	keyAttrTypes := map[string]attr.Type{
		keyName:     types.StringType,
		keyRegion:   types.StringType,
		keyRingName: types.StringType,
	}
	keyValues := make([]attr.Value, 0, len(targetTemplate.CMKInfo))
	for _, key := range targetTemplate.CMKInfo {
		keyValue, diags := types.ObjectValue(keyAttrTypes, map[string]attr.Value{
			keyName:     types.StringValue(key.Name),
			keyRegion:   types.StringValue(key.Region.Name()),
			keyRingName: types.StringValue(key.RingName),
		})
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		keyValues = append(keyValues, keyValue)
	}
	keySet, diags := types.SetValue(types.ObjectType{AttrTypes: keyAttrTypes}, keyValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := gcpArchivalLocationModel{
		ID:                 types.StringValue(targetMapping.ID.String()),
		BucketLabels:       labelMap,
		BucketPrefix:       types.StringValue(strings.TrimPrefix(targetTemplate.BucketPrefix, "rubrik-")),
		CloudAccountID:     types.StringValue(targetTemplate.CloudAccount.ID.String()),
		ConnectionStatus:   types.StringValue(targetMapping.ConnectionStatus.Status),
		CustomerManagedKey: keySet,
		LocationTemplate:   types.StringValue(targetTemplate.LocTemplate),
		Name:               types.StringValue(targetMapping.Name),
		Region:             types.StringValue(targetTemplate.Region.Name()),
		StorageClass:       types.StringValue(fromGCPStorageClass(targetTemplate.StorageClass)),
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
