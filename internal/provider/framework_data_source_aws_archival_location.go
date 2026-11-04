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
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
)

const dataSourceAWSArchivalLocationDescription = `
The ´rubrik_aws_archival_location´ data source is used to access information about an
AWS archival location. An archival location is looked up using either the ID or the name.
`

var _ datasource.DataSource = &awsArchivalLocationDataSource{}
var _ datasource.DataSourceWithConfigure = &awsArchivalLocationDataSource{}

type awsArchivalLocationDataSource struct {
	client *client
	prefix string
}

type awsArchivalLocationDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	AccountID          types.String `tfsdk:"account_id"`
	ArchivalLocationID types.String `tfsdk:"archival_location_id"`
	BucketPrefix       types.String `tfsdk:"bucket_prefix"`
	BucketTags         types.Map    `tfsdk:"bucket_tags"`
	ConnectionStatus   types.String `tfsdk:"connection_status"`
	KMSMasterKey       types.String `tfsdk:"kms_master_key"`
	LocationTemplate   types.String `tfsdk:"location_template"`
	Name               types.String `tfsdk:"name"`
	Region             types.String `tfsdk:"region"`
	StorageClass       types.String `tfsdk:"storage_class"`
}

func newAwsArchivalLocationDataSource() datasource.DataSource {
	return &awsArchivalLocationDataSource{prefix: keyRubrik}
}

func newPolarisAwsArchivalLocationDataSource() datasource.DataSource {
	return &awsArchivalLocationDataSource{prefix: keyPolaris}
}

func (d *awsArchivalLocationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "awsArchivalLocationDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAWSArchivalLocation
}

func (d *awsArchivalLocationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "awsArchivalLocationDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAWSArchivalLocationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cloud native archival location ID (UUID).",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot(keyID),
						path.MatchRoot(keyArchivalLocationID),
						path.MatchRoot(keyName),
					),
					isUUID(),
				},
			},
			keyAccountID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
			},
			keyArchivalLocationID: schema.StringAttribute{
				Optional:           true,
				Computed:           true,
				Description:        "Cloud native archival location ID (UUID). **Deprecated:** use `id` instead.",
				DeprecationMessage: "use the `id` field instead.",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyBucketPrefix: schema.StringAttribute{
				Computed:    true,
				Description: "AWS bucket prefix. Note, `rubrik-` will always be prepended to the prefix.",
			},
			keyBucketTags: schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "AWS bucket tags.",
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status of the archival location.",
			},
			keyKMSMasterKey: schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "AWS KMS master key alias/ID.",
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
				Description: "AWS region to store the snapshots in. If not specified, the snapshots will be stored " +
					"in the same region as the workload.",
			},
			keyStorageClass: schema.StringAttribute{
				Computed: true,
				Description: "AWS bucket storage class. Possible values are `STANDARD`, `STANDARD_IA`, `ONEZONE_IA`, " +
					"`GLACIER_INSTANT_RETRIEVAL`, `GLACIER_DEEP_ARCHIVE` and `GLACIER_FLEXIBLE_RETRIEVAL`. Default " +
					"value is `STANDARD_IA`.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_aws_archival_location` data source instead."
	}
}

func (d *awsArchivalLocationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "awsArchivalLocationDataSource.Configure")

	if req.ProviderData == nil {
		return
	}

	d.client = req.ProviderData.(*client)
}

func (d *awsArchivalLocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "awsArchivalLocationDataSource.Read")

	var config awsArchivalLocationDataSourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	client, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	var targetMapping gqlarchival.AWSTargetMapping
	if !config.ID.IsNull() && !config.ID.IsUnknown() {
		id, err := uuid.Parse(config.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyID), "Invalid UUID", err.Error())
			return
		}
		targetMapping, err = archival.Wrap(client).AWSTargetMappingByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to Read AWS Archival Location by ID", err.Error())
			return
		}
	} else if !config.ArchivalLocationID.IsNull() && !config.ArchivalLocationID.IsUnknown() {
		id, err := uuid.Parse(config.ArchivalLocationID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyArchivalLocationID), "Invalid UUID", err.Error())
			return
		}
		targetMapping, err = archival.Wrap(client).AWSTargetMappingByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to Read AWS Archival Location by Archival Location ID", err.Error())
			return
		}
	} else {
		targetMapping, err = archival.Wrap(client).AWSTargetMappingByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to Read AWS Archival Location by Name", err.Error())
			return
		}
	}

	targetTemplate := targetMapping.TargetTemplate
	state := awsArchivalLocationDataSourceModel{
		ID:                 types.StringValue(targetMapping.ID.String()),
		ArchivalLocationID: types.StringValue(targetMapping.ID.String()),
		AccountID:          types.StringValue(targetTemplate.CloudAccount.ID.String()),
		BucketPrefix:       types.StringValue(strings.TrimPrefix(targetTemplate.BucketPrefix, "rubrik-")),
		ConnectionStatus:   types.StringValue(targetMapping.ConnectionStatus.Status),
		KMSMasterKey:       types.StringValue(targetTemplate.KMSMasterKey),
		LocationTemplate:   types.StringValue(targetTemplate.LocTemplate),
		Name:               types.StringValue(targetMapping.Name),
		Region:             types.StringValue(targetTemplate.Region.Name()),
		StorageClass:       types.StringValue(targetTemplate.StorageClass),
	}

	bucketTags := make(map[string]string, len(targetTemplate.BucketTags))
	for _, tag := range targetTemplate.BucketTags {
		bucketTags[tag.Key] = tag.Value
	}
	state.BucketTags, res.Diagnostics = types.MapValueFrom(ctx, types.StringType, bucketTags)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
