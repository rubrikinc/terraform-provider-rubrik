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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqltags "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/tags"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/tags"
)

func createCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "createCustomTagsResource")

	conf := newCustomTagsConfig(vendor)

	cloudAccountID, diags := getCloudAccountID(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planTags, diags := getCustomTags(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planOverride, diags := getOverrideTags(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planExclude, diags := getExcludeTags(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTags := make([]core.Tag, 0, len(planTags))
	for key, value := range planTags {
		customerTags = append(customerTags, core.Tag{Key: key, Value: value})
	}

	excludeTags := make([]string, 0, len(planExclude))
	for key := range planExclude {
		excludeTags = append(excludeTags, key)
	}

	if err := tags.Wrap(client).AddCustomerTags(ctx, gqltags.CustomerTags{
		CloudVendor:          vendor,
		CloudAccountID:       cloudAccountID,
		Tags:                 customerTags,
		ExcludedTags:         excludeTags,
		OverrideResourceTags: planOverride,
	}); err != nil {
		res.Diagnostics.AddError("Failed to create custom tags", err.Error())
		return
	}

	id := conf.globalID
	if cloudAccountID != "" {
		id = cloudAccountID
	}

	res.Diagnostics.Append(res.State.Set(ctx, req.Plan.Raw)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), types.StringValue(id))...)
}

// The resource only manages the custom tags and the excluded tags in its state.
// Refresh the values of the managed tags and excluded tags still present in RSC
// and drop the ones which are gone. Custom tags and excluded tags in RSC not
// managed by the resource are ignored.
func readCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "readCustomTagsResource")

	conf := newCustomTagsConfig(vendor)

	cloudAccountID, diags := getCloudAccountID(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateTags, diags := getCustomTags(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateExclude, diags := getExcludeTags(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTags, err := tags.Wrap(client).CustomerTagsByFilter(ctx, gqltags.CustomerTagsFilter{
		CloudVendor:    vendor,
		CloudAccountID: cloudAccountID,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to read custom tags", err.Error())
		return
	}

	newStateTags := make(map[string]string, len(stateTags))
	for _, tag := range customerTags.Tags {
		if _, ok := stateTags[tag.Key]; ok {
			newStateTags[tag.Key] = tag.Value
		}
	}

	newStateExclude := make(map[string]struct{}, len(stateExclude))
	for _, key := range customerTags.ExcludedTags {
		if _, ok := stateExclude[key]; ok {
			newStateExclude[key] = struct{}{}
		}
	}

	res.Diagnostics.Append(setCustomTags(ctx, conf, &res.State, newStateTags)...)
	res.Diagnostics.Append(setExcludeTags(ctx, conf, &res.State, newStateExclude)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, conf.overrideTagsPath,
		types.BoolValue(customerTags.OverrideResourceTags))...)
}

func updateCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "updateCustomTagsResource")

	conf := newCustomTagsConfig(vendor)

	cloudAccountID, diags := getCloudAccountID(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateTags, diags := getCustomTags(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planTags, diags := getCustomTags(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planOverride, diags := getOverrideTags(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateExclude, diags := getExcludeTags(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planExclude, diags := getExcludeTags(ctx, conf, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTags, err := tags.Wrap(client).CustomerTagsByFilter(ctx, gqltags.CustomerTagsFilter{
		CloudVendor:    vendor,
		CloudAccountID: cloudAccountID,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to read custom tags", err.Error())
		return
	}

	// Add tags read from RSC, overwrite with tags in the plan and finally
	// remove tags being removed by the plan.
	newCustomTags := make(map[string]string, len(planTags)+len(customerTags.Tags))
	for _, tag := range customerTags.Tags {
		newCustomTags[tag.Key] = tag.Value
	}
	for key, value := range planTags {
		newCustomTags[key] = value
	}
	for key := range stateTags {
		if _, ok := planTags[key]; !ok {
			delete(newCustomTags, key)
		}
	}

	// Add excluded tags read from RSC, overwrite with excluded tags in the plan
	// and finally remove excluded tags being removed by the plan.
	newExcludeTags := make(map[string]struct{}, len(planExclude)+len(customerTags.ExcludedTags))
	for _, key := range customerTags.ExcludedTags {
		newExcludeTags[key] = struct{}{}
	}
	for key := range planExclude {
		newExcludeTags[key] = struct{}{}
	}
	for key := range stateExclude {
		if _, ok := planExclude[key]; !ok {
			delete(newExcludeTags, key)
		}
	}

	customerTags.Tags = make([]core.Tag, 0, len(newCustomTags))
	for key, value := range newCustomTags {
		customerTags.Tags = append(customerTags.Tags, core.Tag{Key: key, Value: value})
	}

	customerTags.ExcludedTags = make([]string, 0, len(newExcludeTags))
	for key := range newExcludeTags {
		customerTags.ExcludedTags = append(customerTags.ExcludedTags, key)
	}
	customerTags.OverrideResourceTags = planOverride
	if err := tags.Wrap(client).ReplaceCustomerTags(ctx, customerTags); err != nil {
		res.Diagnostics.AddError("Failed to update custom tags", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, req.Plan.Raw)...)
}

func deleteCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "deleteCustomTagsResource")

	conf := newCustomTagsConfig(vendor)

	cloudAccountID, diags := getCloudAccountID(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateTags, diags := getCustomTags(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateExclude, diags := getExcludeTags(ctx, conf, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Read customer tags and remove tags and excluded tags existing in state.
	customerTags, err := tags.Wrap(client).CustomerTagsByFilter(ctx, gqltags.CustomerTagsFilter{
		CloudVendor:    vendor,
		CloudAccountID: cloudAccountID,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to read custom tags", err.Error())
		return
	}

	newTags := make([]core.Tag, 0, len(customerTags.Tags))
	for _, tag := range customerTags.Tags {
		if _, ok := stateTags[tag.Key]; !ok {
			newTags = append(newTags, tag)
		}
	}
	customerTags.Tags = newTags

	newExcludedTags := make([]string, 0, len(customerTags.ExcludedTags))
	for _, key := range customerTags.ExcludedTags {
		if _, ok := stateExclude[key]; !ok {
			newExcludedTags = append(newExcludedTags, key)
		}
	}
	customerTags.ExcludedTags = newExcludedTags

	if err := tags.Wrap(client).ReplaceCustomerTags(ctx, customerTags); err != nil {
		res.Diagnostics.AddError("Failed to replace custom tags", err.Error())
		return
	}
}

// Note, the custom tags resource is designed to only manage the custom tags
// owned by the resource. An import on the other hand will take ownership of
// all custom tags of the scope imported.
//
// The import ID is the cloud account ID of the cloud account to scope the
// import to, or keyGlobal to scope the import to all cloud accounts of the
// cloud vendor. The latter scope is an RSC account level configuration without
// a unique identifier. Any other import ID is rejected, so that a malformed
// cloud account ID fails the import instead of silently taking ownership of
// the wrong scope.
func importCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "importCustomTagsResource")

	conf := newCustomTagsConfig(vendor)

	var cloudAccountID string
	if req.ID != keyGlobal {
		id, err := uuid.Parse(req.ID)
		if err != nil {
			res.Diagnostics.AddError("Invalid import ID", fmt.Sprintf(
				"%q is not a valid import ID, expected a cloud account ID (UUID) or %q: %s",
				req.ID, keyGlobal, err))
			return
		}
		cloudAccountID = id.String()
	}

	customerTags, err := tags.Wrap(client).CustomerTagsByFilter(ctx, gqltags.CustomerTagsFilter{
		CloudVendor:    vendor,
		CloudAccountID: cloudAccountID,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to read custom tags", err.Error())
		return
	}

	stateCloudAccountID := types.StringNull()
	if cloudAccountID != "" {
		stateCloudAccountID = types.StringValue(cloudAccountID)
	}

	stateTags := make(map[string]string, len(customerTags.Tags))
	for _, tag := range customerTags.Tags {
		stateTags[tag.Key] = tag.Value
	}

	stateExclude := make(map[string]struct{}, len(customerTags.ExcludedTags))
	for _, tag := range customerTags.ExcludedTags {
		stateExclude[tag] = struct{}{}
	}

	id := conf.globalID
	if cloudAccountID != "" {
		id = cloudAccountID
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), types.StringValue(id))...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, conf.cloudAccountIDPath, stateCloudAccountID)...)
	res.Diagnostics.Append(setCustomTags(ctx, conf, &res.State, stateTags)...)
	res.Diagnostics.Append(setExcludeTags(ctx, conf, &res.State, stateExclude)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, conf.overrideTagsPath,
		types.BoolValue(customerTags.OverrideResourceTags))...)
}

// customTagsConfig holds the global ID and attribute paths of a custom tags
// resource. GCP names its attributes after labels rather than tags.
type customTagsConfig struct {
	globalID           string
	cloudAccountIDPath path.Path
	customTagsKey      string
	customTagsPath     path.Path
	excludedTagsKey    string
	excludedTagsPath   path.Path
	overrideTagsKey    string
	overrideTagsPath   path.Path
	typeName           string
}

func newCustomTagsConfig(vendor core.CloudVendor) customTagsConfig {
	var conf customTagsConfig
	switch vendor {
	case core.CloudVendorAWS:
		conf = customTagsConfig{
			globalID:           awsCustomTagsID,
			cloudAccountIDPath: path.Root(keyCloudAccountID),
			customTagsKey:      keyCustomTags,
			overrideTagsKey:    keyOverrideResourceTags,
			excludedTagsKey:    keyExcludedTags,
			typeName:           keyAWSCustomTags,
		}
	case core.CloudVendorAzure:
		conf = customTagsConfig{
			globalID:           azureCustomTagsID,
			cloudAccountIDPath: path.Root(keyCloudAccountID),
			customTagsKey:      keyCustomTags,
			overrideTagsKey:    keyOverrideResourceTags,
			excludedTagsKey:    keyExcludedTags,
			typeName:           keyAzureCustomTags,
		}
	case core.CloudVendorGCP:
		conf = customTagsConfig{
			globalID:           gcpCustomLabelsID,
			cloudAccountIDPath: path.Root(keyCloudAccountID),
			customTagsKey:      keyCustomLabels,
			overrideTagsKey:    keyOverrideResourceLabels,
			excludedTagsKey:    keyExcludedLabels,
			typeName:           keyGCPCustomLabels,
		}
	default:
		// The vendor is a constant passed in by an implementation, never user
		// input, so reaching this means a vendor was added without updating
		// this function.
		panic(fmt.Sprintf("unknown vendor: %q", vendor))
	}

	// Derive the paths from the keys.
	conf.customTagsPath = path.Root(conf.customTagsKey)
	conf.overrideTagsPath = path.Root(conf.overrideTagsKey)
	conf.excludedTagsPath = path.Root(conf.excludedTagsKey)

	return conf
}

type configGet interface {
	GetAttribute(ctx context.Context, path path.Path, target any) diag.Diagnostics
}

func getCloudAccountID(ctx context.Context, resConf customTagsConfig, conf configGet) (string, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var cloudAccountID types.String
	diags.Append(conf.GetAttribute(ctx, resConf.cloudAccountIDPath, &cloudAccountID)...)
	if diags.HasError() {
		return "", diags
	}

	return cloudAccountID.ValueString(), diags
}

func getCustomTags(ctx context.Context, resConf customTagsConfig, conf configGet) (map[string]string, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var customTagsMap types.Map
	diags.Append(conf.GetAttribute(ctx, resConf.customTagsPath, &customTagsMap)...)
	if diags.HasError() {
		return nil, diags
	}

	var tagsMap map[string]string
	diags.Append(customTagsMap.ElementsAs(ctx, &tagsMap, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return tagsMap, diags
}

func getExcludeTags(ctx context.Context, resConf customTagsConfig, conf configGet) (map[string]struct{}, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var excludedTagsSet types.Set
	diags.Append(conf.GetAttribute(ctx, resConf.excludedTagsPath, &excludedTagsSet)...)
	if diags.HasError() {
		return nil, diags
	}

	var tagsSlice []string
	diags.Append(excludedTagsSet.ElementsAs(ctx, &tagsSlice, false)...)
	if diags.HasError() {
		return nil, diags
	}

	tagsSet := make(map[string]struct{}, len(tagsSlice))
	for _, tag := range tagsSlice {
		tagsSet[tag] = struct{}{}
	}

	return tagsSet, diags
}

func getOverrideTags(ctx context.Context, resConf customTagsConfig, conf configGet) (bool, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var override types.Bool
	diags.Append(conf.GetAttribute(ctx, resConf.overrideTagsPath, &override)...)
	if diags.HasError() {
		return false, diags
	}

	return override.ValueBool(), diags
}

type configSet interface {
	SetAttribute(ctx context.Context, path path.Path, value any) diag.Diagnostics
}

func setCustomTags(ctx context.Context, resConf customTagsConfig, conf configSet, customTags map[string]string) diag.Diagnostics {
	diags := diag.Diagnostics{}

	stateTagsMap := types.MapNull(types.StringType)
	if len(customTags) > 0 {
		stateTagsMap, diags = types.MapValueFrom(ctx, types.StringType, customTags)
		if diags.HasError() {
			return diags
		}
	}

	diags.Append(conf.SetAttribute(ctx, resConf.customTagsPath, stateTagsMap)...)
	return diags
}

func setExcludeTags(ctx context.Context, resConf customTagsConfig, conf configSet, excludeTags map[string]struct{}) diag.Diagnostics {
	diags := diag.Diagnostics{}

	stateExcludeSet := types.SetNull(types.StringType)
	if len(excludeTags) > 0 {
		stateExclude := make([]string, 0, len(excludeTags))
		for key := range excludeTags {
			stateExclude = append(stateExclude, key)
		}

		stateExcludeSet, diags = types.SetValueFrom(ctx, types.StringType, stateExclude)
		if diags.HasError() {
			return diags
		}
	}

	diags.Append(conf.SetAttribute(ctx, resConf.excludedTagsPath, stateExcludeSet)...)
	return diags
}
