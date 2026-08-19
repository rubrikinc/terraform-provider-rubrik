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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/dspm"
	gqldspm "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/dspm"
)

const dataSourceDataSecurityPolicyDescription = `
The ´rubrik_data_security_policy´ data source is used to access information
about a data security policy in RSC. A data security policy is looked up using
either the policy ID or the name.

The filter is returned as the ´object_filter´ and ´identity_filter´ blocks, the
same shape the ´rubrik_data_security_policy´ resource uses. Predefined policies,
and policies created before RSC started enforcing that shape, are exempt from it
and can hold filters the two blocks cannot express. Looking up such a policy
fails with an error describing why.
`

var _ datasource.DataSource = &dataSecurityPolicyDataSource{}

type dataSecurityPolicyDataSource struct {
	client *client
}

type dataSecurityPolicyModel struct {
	ID              types.String          `tfsdk:"id"`
	PolicyID        types.String          `tfsdk:"policy_id"`
	Name            types.String          `tfsdk:"name"`
	Description     types.String          `tfsdk:"description"`
	Category        types.String          `tfsdk:"category"`
	Severity        types.String          `tfsdk:"severity"`
	Enabled         types.Bool            `tfsdk:"enabled"`
	Predefined      types.Bool            `tfsdk:"predefined"`
	ObjectFilter    []conditionGroupModel `tfsdk:"object_filter"`
	IdentityFilter  []conditionGroupModel `tfsdk:"identity_filter"`
	ThresholdFilter []conditionModel      `tfsdk:"threshold_filter"`
}

func newDataSecurityPolicyDataSource() datasource.DataSource {
	return &dataSecurityPolicyDataSource{}
}

func (d *dataSecurityPolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyDataSource.Metadata")

	res.TypeName = keyRubrik + "_" + keyDataSecurityPolicy
}

// computedConditionAttributes returns the computed attributes of a single
// filter condition. It mirrors conditionAttributes in the
// rubrik_data_security_policy resource.
func computedConditionAttributes(resourceTypes ...filterResourceType) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		keyFilterType: schema.StringAttribute{
			Computed:    true,
			Description: filterTypeDescription(resourceTypes...),
		},
		keyRelationship: schema.StringAttribute{
			Computed:    true,
			Description: "Comparison operator.",
		},
		keyValues: schema.ListAttribute{
			ElementType: types.StringType,
			Computed:    true,
			Description: "Filter values.",
		},
	}
}

// computedConditionGroupBlockSchema returns a computed condition group block
// schema for data sources. It mirrors conditionGroupBlockSchema in the
// rubrik_data_security_policy resource.
func computedConditionGroupBlockSchema(description string, resourceTypes ...filterResourceType) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				keyOp: schema.StringAttribute{
					Computed:    true,
					Description: "Logical operator joining the conditions in this block, AND or OR.",
				},
			},
			Blocks: map[string]schema.Block{
				keyCondition: schema.ListNestedBlock{
					Description: "Filter condition.",
					NestedObject: schema.NestedBlockObject{
						Attributes: computedConditionAttributes(resourceTypes...),
					},
				},
			},
		},
	}
}

// computedThresholdFilterBlockSchema returns the computed threshold_filter
// block schema. It mirrors thresholdFilterBlockSchema in the
// rubrik_data_security_policy resource.
func computedThresholdFilterBlockSchema(description string, resourceTypes ...filterResourceType) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{
			Attributes: computedConditionAttributes(resourceTypes...),
		},
	}
}

func (d *dataSecurityPolicyDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceDataSecurityPolicyDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Data security policy ID (UUID).",
			},
			keyPolicyID: schema.StringAttribute{
				Optional:    true,
				Description: "Data security policy ID (UUID).",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Description: "Name of the data security policy.",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyPolicyID)),
					isNotWhiteSpace(),
				},
			},
			keyDescription: schema.StringAttribute{
				Computed:    true,
				Description: "Description of the data security policy.",
			},
			keyCategory: schema.StringAttribute{
				Computed:    true,
				Description: "Category of the data security policy.",
			},
			keySeverity: schema.StringAttribute{
				Computed:    true,
				Description: "Severity of the data security policy.",
			},
			keyEnabled: schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the data security policy is enabled.",
			},
			keyPredefined: schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the data security policy is predefined.",
			},
		},
		Blocks: map[string]schema.Block{
			keyObjectFilter: computedConditionGroupBlockSchema(
				"Object conditions for the data security policy.",
				filterResourceTypeObject,
			),
			keyIdentityFilter: computedConditionGroupBlockSchema(
				"Identity conditions for the data security policy.",
				filterResourceTypeIdentity,
			),
			keyThresholdFilter: computedThresholdFilterBlockSchema(
				"Threshold condition deciding how many matches raise a violation.",
				filterResourceTypeObject, filterResourceTypeIdentity,
			),
		},
	}
}

func (d *dataSecurityPolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *dataSecurityPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "dataSecurityPolicyDataSource.Read")

	var config dataSecurityPolicyModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var policy gqldspm.Policy
	if !config.PolicyID.IsNull() {
		id, err := uuid.Parse(config.PolicyID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid policy ID", err.Error())
			return
		}

		policy, err = dspm.Wrap(polarisClient).PolicyByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read data security policy", err.Error())
			return
		}
	} else {
		policy, err = dspm.Wrap(polarisClient).PolicyByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read data security policy", err.Error())
			return
		}
	}

	state := dataSecurityPolicyModel{
		ID:          types.StringValue(policy.ID.String()),
		PolicyID:    types.StringValue(policy.ID.String()),
		Name:        types.StringValue(policy.Name),
		Description: types.StringValue(policy.Description),
		Category:    types.StringValue(string(policy.Category)),
		Severity:    types.StringValue(string(policy.Severity)),
		Enabled:     types.BoolValue(policy.Enabled),
		Predefined:  types.BoolValue(policy.Predefined),
	}

	object, identity, diags := fromPolicyFilter(ctx, policy.Filter)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.ObjectFilter = object
	state.IdentityFilter = identity

	thresholdFilter, diags := fromThresholdGroupConfig(ctx, policy.ThresholdFilter)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.ThresholdFilter = thresholdFilter

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
