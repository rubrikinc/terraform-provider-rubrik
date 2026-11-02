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
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/aws"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqlgcp "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/gcp"
)

const dataSourceAccountDescription = `
The ´rubrik_account´ data source is used to access information about the RSC account.

-> **Note:** The ´fqdn´ and ´name´ fields are read from the local RSC credentials and
   not from RSC.
`

var _ datasource.DataSource = &accountDataSource{}

type accountDataSource struct {
	client *client
	prefix string
}

// accountDataSourceModel is the data source state. Every attribute is
// computed.
type accountDataSourceModel struct {
	ID         types.String        `tfsdk:"id"`
	AWS        []accountCloudModel `tfsdk:"aws"`
	Azure      []accountCloudModel `tfsdk:"azure"`
	Features   []string            `tfsdk:"features"`
	FQDN       types.String        `tfsdk:"fqdn"`
	GCP        []accountCloudModel `tfsdk:"gcp"`
	Name       types.String        `tfsdk:"name"`
	Operations []string            `tfsdk:"operations"`
	Workloads  []string            `tfsdk:"workloads"`
}

// accountCloudModel holds the features supported for a cloud vendor.
type accountCloudModel struct {
	Features []accountFeatureModel `tfsdk:"features"`
}

// accountFeatureModel holds a feature and its permission groups.
type accountFeatureModel struct {
	Name             types.String `tfsdk:"name"`
	PermissionGroups []string     `tfsdk:"permission_groups"`
}

func newAccountDataSource() datasource.DataSource {
	return &accountDataSource{prefix: keyRubrik}
}

func newPolarisAccountDataSource() datasource.DataSource {
	return &accountDataSource{prefix: keyPolaris}
}

func (d *accountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "accountDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAccount
}

func (d *accountDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "accountDataSource.Schema")

	// The cloud vendor attributes all have the same shape.
	cloudFeatures := func(vendor string) schema.ListNestedAttribute {
		return schema.ListNestedAttribute{
			Computed: true,
			Description: vendor + " cloud vendor information including supported features, and their permission " +
				"groups.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					keyFeatures: schema.ListNestedAttribute{
						Computed:    true,
						Description: "Features supported for this cloud vendor with their permission groups.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								keyName: schema.StringAttribute{
									Computed:    true,
									Description: "RSC feature name.",
								},
								keyPermissionGroups: schema.ListAttribute{
									ElementType: types.StringType,
									Computed:    true,
									Description: "Permission groups available for the feature.",
								},
							},
						},
					},
				},
			},
		}
	}

	res.Schema = schema.Schema{
		Description: description(dataSourceAccountDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 hash of the features, the fully qualified domain name and the name.",
			},
			keyAws:   cloudFeatures("AWS"),
			keyAzure: cloudFeatures("Azure"),
			keyFeatures: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Features enabled for the RSC account.",
			},
			keyFQDN: schema.StringAttribute{
				Computed:    true,
				Description: "Fully qualified domain name of the RSC account.",
			},
			keyGcp: cloudFeatures("GCP"),
			keyName: schema.StringAttribute{
				Computed:    true,
				Description: "RSC account name.",
			},
			keyOperations: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Valid operations that can be performed by the RSC account.",
			},
			keyWorkloads: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Valid workload hierarchy types (snappable types) that can be used in the RSC account.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_account` data source instead."
	}
}

func (d *accountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "accountDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *accountDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "accountDataSource.Read")

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	coreClient := core.Wrap(polarisClient.GQL)

	accountFeatures, err := coreClient.EnabledFeaturesForAccount(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to read account features", err.Error())
		return
	}
	accountFQDN := strings.ToLower(polarisClient.Account.AccountFQDN())
	accountName := strings.ToLower(polarisClient.Account.AccountName())

	// The features attribute is a set, so duplicates are removed. The order
	// of the features used for the ID hash below is the order from RSC.
	features := make([]string, 0, len(accountFeatures))
	for _, accountFeature := range accountFeatures {
		features = append(features, accountFeature.Name)
	}
	slices.Sort(features)
	features = slices.Compact(features)

	operations, err := coreClient.ValuesByEnum(ctx, "Operation")
	if err != nil {
		res.Diagnostics.AddError("Failed to read operations", err.Error())
		return
	}
	slices.Sort(operations)

	workloads, err := coreClient.ValuesByEnum(ctx, "WorkloadLevelHierarchy")
	if err != nil {
		res.Diagnostics.AddError("Failed to read workloads", err.Error())
		return
	}
	slices.Sort(workloads)

	// Populate the cloud vendor features with permission groups.
	var awsFeatures []core.Feature
	for _, name := range core.AWSAccountAllowlistNames() {
		awsFeatures = append(awsFeatures, core.Feature{Name: name})
	}
	awsPermGroups, err := gqlaws.Wrap(polarisClient.GQL).AllPermissionsGroupsByFeature(ctx, awsFeatures,
		gqlaws.ServiceTypeNonBaaS)
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS permission groups", err.Error())
		return
	}

	var azureFeatures []core.Feature
	for _, name := range core.AzureSubscriptionAllowlistNames() {
		azureFeatures = append(azureFeatures, core.Feature{Name: name})
	}
	azurePermGroups, err := gqlazure.Wrap(polarisClient.GQL).AllPermissionsGroupsByFeature(ctx, azureFeatures)
	if err != nil {
		res.Diagnostics.AddError("Failed to read Azure permission groups", err.Error())
		return
	}

	var gcpFeatures []core.Feature
	for _, name := range core.GCPProjectAllowlistNames() {
		gcpFeatures = append(gcpFeatures, core.Feature{Name: name})
	}
	gcpPermGroups, err := gqlgcp.Wrap(polarisClient.GQL).AllPermissionsGroupsByFeature(ctx, gcpFeatures)
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP permission groups", err.Error())
		return
	}

	hash := sha256.New()
	for _, accountFeature := range accountFeatures {
		hash.Write([]byte(accountFeature.Name))
	}
	hash.Write([]byte(accountFQDN))
	hash.Write([]byte(accountName))
	for _, operation := range operations {
		hash.Write([]byte(operation))
	}
	for _, workload := range workloads {
		hash.Write([]byte(workload))
	}

	state := accountDataSourceModel{
		ID:         types.StringValue(fmt.Sprintf("%x", hash.Sum(nil))),
		AWS:        []accountCloudModel{{Features: sortedAWSFeatures(awsPermGroups)}},
		Azure:      []accountCloudModel{{Features: sortedAzureFeatures(azurePermGroups)}},
		Features:   features,
		FQDN:       types.StringValue(accountFQDN),
		GCP:        []accountCloudModel{{Features: sortedGCPFeatures(gcpPermGroups)}},
		Name:       types.StringValue(accountName),
		Operations: operations,
		Workloads:  workloads,
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

// sortedAWSFeatures sorts AWS permission groups by feature name and permission
// group name.
func sortedAWSFeatures(permGroups []gqlaws.FeaturePermissionGroups) []accountFeatureModel {
	result := make([]accountFeatureModel, 0, len(permGroups))
	for _, fp := range permGroups {
		groups := make([]string, 0, len(fp.PermissionGroups))
		for _, pg := range fp.PermissionGroups {
			groups = append(groups, string(pg.PermissionGroup))
		}
		sort.Strings(groups)
		result = append(result, accountFeatureModel{Name: types.StringValue(fp.Feature), PermissionGroups: groups})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name.ValueString() < result[j].Name.ValueString()
	})
	return result
}

// sortedAzureFeatures sorts Azure permission groups by feature name and
// permission group name.
func sortedAzureFeatures(permGroups []gqlazure.FeaturePermissionGroups) []accountFeatureModel {
	result := make([]accountFeatureModel, 0, len(permGroups))
	for _, fp := range permGroups {
		groups := make([]string, 0, len(fp.PermissionGroups))
		for _, pg := range fp.PermissionGroups {
			groups = append(groups, string(pg.PermissionGroup))
		}
		sort.Strings(groups)
		result = append(result, accountFeatureModel{Name: types.StringValue(fp.Feature), PermissionGroups: groups})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name.ValueString() < result[j].Name.ValueString()
	})
	return result
}

// sortedGCPFeatures sorts GCP permission groups by feature name and permission
// group name.
func sortedGCPFeatures(permGroups []gqlgcp.FeaturePermissionGroups) []accountFeatureModel {
	result := make([]accountFeatureModel, 0, len(permGroups))
	for _, fp := range permGroups {
		groups := make([]string, 0, len(fp.PermissionGroups))
		for _, pg := range fp.PermissionGroups {
			groups = append(groups, string(pg.PermissionGroup))
		}
		sort.Strings(groups)
		result = append(result, accountFeatureModel{Name: types.StringValue(fp.Feature), PermissionGroups: groups})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name.ValueString() < result[j].Name.ValueString()
	})
	return result
}
