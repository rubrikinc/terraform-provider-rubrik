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
	"fmt"
	"regexp"
	"strconv"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const dataSourceGCPProjectDescription = `
The ´rubrik_gcp_project´ data source is used to access information about a GCP
project added to RSC. A GCP project is looked up using either the GCP project
ID, the GCP project number, the RSC cloud account ID or the name.

-> **Note:** The project name is the name of the GCP project as it appears in
   RSC.
`

var (
	_ datasource.DataSource              = &gcpProjectDataSource{}
	_ datasource.DataSourceWithConfigure = &gcpProjectDataSource{}
)

type gcpProjectDataSource struct {
	client *client
	prefix string
}

type gcpProjectModel struct {
	ID               types.String `tfsdk:"id"`
	Feature          types.Set    `tfsdk:"feature"`
	Name             types.String `tfsdk:"name"`
	OrganizationName types.String `tfsdk:"organization_name"`
	ProjectID        types.String `tfsdk:"project_id"`
	ProjectNumber    types.String `tfsdk:"project_number"`
}

func newGcpProjectDataSource() datasource.DataSource {
	return &gcpProjectDataSource{prefix: keyRubrik}
}

func newPolarisGcpProjectDataSource() datasource.DataSource {
	return &gcpProjectDataSource{prefix: keyPolaris}
}

func (d *gcpProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "gcpProjectDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyGCPProject
}

func (d *gcpProjectDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "gcpProjectDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceGCPProjectDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot(keyID),
						path.MatchRoot(keyName),
						path.MatchRoot(keyProjectID),
						path.MatchRoot(keyProjectNumber),
					),
				},
			},
			keyFeature: schema.SetNestedAttribute{
				Computed:    true,
				Description: "RSC feature with permission groups and status.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Computed: true,
							Description: fmt.Sprintf("RSC feature name. %s.",
								possibleValues(core.GCPProjectFeatureNames())),
						},
						keyPermissionGroups: schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: fmt.Sprintf("Permission groups for the RSC feature. %s.",
								possibleValues(core.GCPProjectPermissionGroupNames())),
						},
						keyStatus: schema.StringAttribute{
							Computed:    true,
							Description: "Status of the feature.",
						},
					},
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "GCP project name.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			keyOrganizationName: schema.StringAttribute{
				Computed:    true,
				Description: "GCP organization name.",
			},
			keyProjectID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "GCP project ID.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			keyProjectNumber: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "GCP project number.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[+-]?[0-9]+$`), "must be an integer"),
				},
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_gcp_project` data source instead."
	}
}

func (d *gcpProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpProjectDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *gcpProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "gcpProjectDataSource.Read")

	var config gcpProjectModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// We don't allow prefix searches since it would be impossible to uniquely
	// identify a project with a name being the prefix of another project.
	var project gcp.CloudAccount
	switch {
	case !config.ProjectID.IsNull():
		project, err = gcp.Wrap(polarisClient).ProjectByNativeID(ctx, config.ProjectID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP project", err.Error())
			return
		}
	case !config.ProjectNumber.IsNull():
		n, err := strconv.ParseInt(config.ProjectNumber.ValueString(), 10, 64)
		if err != nil {
			res.Diagnostics.AddError("Invalid project number", fmt.Sprintf("failed to parse project number: %s", err))
			return
		}
		project, err = gcp.Wrap(polarisClient).ProjectByProjectNumber(ctx, n)
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP project", err.Error())
			return
		}
	case !config.Name.IsNull():
		project, err = gcp.Wrap(polarisClient).ProjectByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP project", err.Error())
			return
		}
	default:
		id, err := uuid.Parse(config.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid ID", fmt.Sprintf("failed to parse id: %s", err))
			return
		}
		project, err = gcp.Wrap(polarisClient).ProjectByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP project", err.Error())
			return
		}
	}

	featureAttrTypes := map[string]attr.Type{
		keyName:             types.StringType,
		keyPermissionGroups: types.SetType{ElemType: types.StringType},
		keyStatus:           types.StringType,
	}
	featureValues := make([]attr.Value, 0, len(project.Features))
	for _, feature := range project.Features {
		groupValues := make([]attr.Value, 0, len(feature.PermissionGroups))
		for _, group := range feature.PermissionGroups {
			groupValues = append(groupValues, types.StringValue(string(group)))
		}
		groupSet, diags := types.SetValue(types.StringType, groupValues)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		featureValue, diags := types.ObjectValue(featureAttrTypes, map[string]attr.Value{
			keyName:             types.StringValue(feature.Name),
			keyPermissionGroups: groupSet,
			keyStatus:           types.StringValue(string(feature.Status)),
		})
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		featureValues = append(featureValues, featureValue)
	}
	featureSet, diags := types.SetValue(types.ObjectType{AttrTypes: featureAttrTypes}, featureValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := gcpProjectModel{
		ID:               types.StringValue(project.ID.String()),
		Feature:          featureSet,
		Name:             types.StringValue(project.Name),
		OrganizationName: types.StringValue(project.OrganizationName),
		ProjectID:        types.StringValue(project.NativeID),
		ProjectNumber:    types.StringValue(strconv.FormatInt(project.ProjectNumber, 10)),
	}

	// Terraform rejects a data source changing a configured value, so the
	// configured lookup value is kept as is.
	if !config.ID.IsNull() {
		state.ID = config.ID
	}
	if !config.Name.IsNull() {
		state.Name = config.Name
	}
	if !config.ProjectID.IsNull() {
		state.ProjectID = config.ProjectID
	}
	if !config.ProjectNumber.IsNull() {
		state.ProjectNumber = config.ProjectNumber
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
