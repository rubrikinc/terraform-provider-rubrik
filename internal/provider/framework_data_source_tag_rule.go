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
	"cmp"
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/sla"
)

const dataSourceTagRuleDescription = `
The ´rubrik_tag_rule´ data source is used to access information about RSC tag
rules. A tag rule is looked up using either the tag rule ID or the tag rule
name.
`

var _ datasource.DataSource = &tagRuleDataSource{}

type tagRuleDataSource struct {
	client *client
	prefix string
}

// tagRuleDataSourceModel is the data source state. The ID and the name are the
// lookup keys, everything else is computed.
type tagRuleDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	ObjectType      types.String `tfsdk:"object_type"`
	TagKey          types.String `tfsdk:"tag_key"`
	TagValue        types.String `tfsdk:"tag_value"`
	TagAllValues    types.Bool   `tfsdk:"tag_all_values"`
	Tag             types.List   `tfsdk:"tag"`
	CloudAccountIDs types.Set    `tfsdk:"cloud_account_ids"`
}

func newTagRuleDataSource() datasource.DataSource {
	return &tagRuleDataSource{prefix: keyRubrik}
}

func newPolarisTagRuleDataSource() datasource.DataSource {
	return &tagRuleDataSource{prefix: keyPolaris}
}

func (d *tagRuleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "tagRuleDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyTagRule
}

func (d *tagRuleDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "tagRuleDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceTagRuleDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Tag rule ID (UUID).",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyName)),
					isUUID(),
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Tag rule name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyObjectType: schema.StringAttribute{
				Computed: true,
				Description: "Object type to which the tag rule will be applied. Possible values are " +
					"`AWS_EBS_VOLUME`, `AWS_EC2_INSTANCE`, `AWS_RDS_INSTANCE`, `AWS_S3_BUCKET`, " +
					"`AZURE_MANAGED_DISK`, `AZURE_SQL_DATABASE_DB`, `AZURE_SQL_DATABASE_SERVER`, " +
					"`AZURE_SQL_MANAGED_INSTANCE_SERVER`, `AZURE_STORAGE_ACCOUNT` and `AZURE_VIRTUAL_MACHINE`.",
			},
			keyTagKey: schema.StringAttribute{
				Computed:           true,
				Description:        "Tag key to match. **Deprecated:** Use `tag` instead.",
				DeprecationMessage: "Use tag instead.",
			},
			keyTagValue: schema.StringAttribute{
				Computed:           true,
				Description:        "Tag value to match. If the tag value is empty, it matches empty values. **Deprecated:** Use `tag` instead.",
				DeprecationMessage: "Use tag instead.",
			},
			keyTagAllValues: schema.BoolAttribute{
				Computed:           true,
				Description:        "If true, all tag values are matched. **Deprecated:** Use `tag` instead.",
				DeprecationMessage: "Use tag instead.",
			},
			keyTag: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Tag conditions to match.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyKey: schema.StringAttribute{
							Computed:    true,
							Description: "Tag key to match.",
						},
						keyValues: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Tag values to match.",
						},
						keyTagMatchAll: schema.BoolAttribute{
							Computed:    true,
							Description: "If true, all tag values for this key are matched.",
						},
					},
				},
			},
			keyCloudAccountIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "The RSC cloud account IDs (UUID) to which the tag rule should be applied. If empty, " +
					"the tag rule will be applied to all RSC cloud accounts.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_tag_rule` data source instead."
	}
}

func (d *tagRuleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "tagRuleDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *tagRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "tagRuleDataSource.Read")

	var config tagRuleDataSourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var tagRule gqlsla.TagRule
	if !config.ID.IsNull() {
		id, err := uuid.Parse(config.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid tag rule ID", err.Error())
			return
		}
		tagRule, err = sla.Wrap(polarisClient).TagRuleByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read tag rule", err.Error())
			return
		}
	} else {
		tagRule, err = sla.Wrap(polarisClient).TagRuleByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read tag rule", err.Error())
			return
		}
	}

	tagObjectType, err := gqlsla.FromManagedObjectType(tagRule.ObjectType)
	if err != nil {
		res.Diagnostics.AddError("Invalid tag rule object type", err.Error())
		return
	}

	// Always populate the tag attribute to reflect the current state from RSC.
	// Sort by key and by values within each tag for deterministic state output
	// regardless of the order RSC returns them in.
	pairs := slices.Clone(tagRule.TagConditions.TagPairs)
	slices.SortFunc(pairs, func(a, b gqlsla.TagPair) int {
		return cmp.Compare(a.Key, b.Key)
	})
	tagElems := make([]attr.Value, 0, len(pairs))
	for _, pair := range pairs {
		values := slices.Clone(pair.Values)
		slices.Sort(values)

		valueElems := make([]attr.Value, 0, len(values))
		for _, value := range values {
			valueElems = append(valueElems, types.StringValue(value))
		}
		valueList, diags := types.ListValue(types.StringType, valueElems)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}

		obj, diags := types.ObjectValue(tagRuleTagAttrTypes(), map[string]attr.Value{
			keyKey:         types.StringValue(pair.Key),
			keyValues:      valueList,
			keyTagMatchAll: types.BoolValue(pair.MatchAllTagValues),
		})
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		tagElems = append(tagElems, obj)
	}
	tagList, diags := types.ListValue(types.ObjectType{AttrTypes: tagRuleTagAttrTypes()}, tagElems)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// For backward compatibility, the deprecated attributes are populated if
	// the deprecated Tag is set, which is a single tag pair with at most one
	// value. Otherwise they are null.
	state := tagRuleDataSourceModel{
		ID:              types.StringValue(tagRule.ID.String()),
		Name:            types.StringValue(tagRule.Name),
		ObjectType:      types.StringValue(string(tagObjectType)),
		TagKey:          types.StringNull(),
		TagValue:        types.StringNull(),
		TagAllValues:    types.BoolNull(),
		Tag:             tagList,
		CloudAccountIDs: types.SetNull(types.StringType),
	}
	//lint:ignore SA1019 using deprecated field for backward compatibility
	if tagRule.Tag.Key != "" {
		//lint:ignore SA1019 using deprecated field for backward compatibility
		state.TagKey = types.StringValue(tagRule.Tag.Key)
		//lint:ignore SA1019 using deprecated field for backward compatibility
		state.TagValue = types.StringValue(tagRule.Tag.Value)
		//lint:ignore SA1019 using deprecated field for backward compatibility
		state.TagAllValues = types.BoolValue(tagRule.Tag.AllValues)
	}

	// The data source holds the native cloud account IDs from RSC, as the
	// SDKv2 data source did, and no ID lookup is made.
	if !tagRule.AllACloudAccounts {
		cloudAccountIDs := make([]attr.Value, 0, len(tagRule.CloudAccounts))
		for _, cloudAccount := range tagRule.CloudAccounts {
			cloudAccountIDs = append(cloudAccountIDs, types.StringValue(cloudAccount.ID.String()))
		}
		cloudAccountSet, diags := types.SetValue(types.StringType, cloudAccountIDs)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		state.CloudAccountIDs = cloudAccountSet
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
