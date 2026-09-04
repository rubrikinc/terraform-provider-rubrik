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

	paths := newCustomTagsPaths(vendor)

	planTags, diags := getCustomTags(ctx, paths, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planOverride, diags := getOverrideTags(ctx, paths, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planExclude, diags := getExcludeTags(ctx, paths, req.Plan)
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
		Tags:                 customerTags,
		ExcludedTags:         excludeTags,
		OverrideResourceTags: planOverride,
	}); err != nil {
		res.Diagnostics.AddError("Failed to create custom tags", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, req.Plan.Raw)...)
}

// The resource only manages the custom tags and the excluded tags in its state.
// Refresh the values of the managed tags and excluded tags still present in RSC
// and drop the ones which are gone. Custom tags and excluded tags in RSC not
// managed by the resource are ignored.
func readCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "readCustomTagsResource")

	paths := newCustomTagsPaths(vendor)

	stateTags, diags := getCustomTags(ctx, paths, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateExclude, diags := getExcludeTags(ctx, paths, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTags, err := tags.Wrap(client).CustomerTags(ctx, vendor)
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

	res.Diagnostics.Append(setCustomTags(ctx, paths, &res.State, newStateTags)...)
	res.Diagnostics.Append(setExcludeTags(ctx, paths, &res.State, newStateExclude)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, paths.overrideTags,
		types.BoolValue(customerTags.OverrideResourceTags))...)
}

func updateCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "updateCustomTagsResource")

	paths := newCustomTagsPaths(vendor)

	stateTags, diags := getCustomTags(ctx, paths, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planTags, diags := getCustomTags(ctx, paths, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planOverride, diags := getOverrideTags(ctx, paths, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateExclude, diags := getExcludeTags(ctx, paths, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	planExclude, diags := getExcludeTags(ctx, paths, req.Plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTags, err := tags.Wrap(client).CustomerTags(ctx, vendor)
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

	paths := newCustomTagsPaths(vendor)

	stateTags, diags := getCustomTags(ctx, paths, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateExclude, diags := getExcludeTags(ctx, paths, req.State)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Read customer tags and remove tags and excluded tags existing in state.
	customerTags, err := tags.Wrap(client).CustomerTags(ctx, vendor)
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
// all custom tags for a cloud vendor. The import ID is ignored, the resource
// manages an RSC account level configuration without a unique identifier.
func importCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "importCustomTagsResource")

	paths := newCustomTagsPaths(vendor)

	customerTags, err := tags.Wrap(client).CustomerTags(ctx, vendor)
	if err != nil {
		res.Diagnostics.AddError("Failed to read custom tags", err.Error())
		return
	}

	stateTags := make(map[string]string, len(customerTags.Tags))
	for _, tag := range customerTags.Tags {
		stateTags[tag.Key] = tag.Value
	}

	stateExclude := make(map[string]struct{}, len(customerTags.ExcludedTags))
	for _, tag := range customerTags.ExcludedTags {
		stateExclude[tag] = struct{}{}
	}

	res.Diagnostics.Append(setCustomTags(ctx, paths, &res.State, stateTags)...)
	res.Diagnostics.Append(setExcludeTags(ctx, paths, &res.State, stateExclude)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, paths.overrideTags,
		types.BoolValue(customerTags.OverrideResourceTags))...)
}

// customTagsPaths holds the attribute paths of a custom tags resource. GCP
// names its attributes after labels rather than tags.
type customTagsPaths struct {
	customTags   path.Path
	overrideTags path.Path
	excludedTags path.Path
}

func newCustomTagsPaths(vendor core.CloudVendor) customTagsPaths {
	if vendor == core.CloudVendorGCP {
		return customTagsPaths{
			customTags:   path.Root(keyCustomLabels),
			overrideTags: path.Root(keyOverrideResourceLabels),
			excludedTags: path.Root(keyExcludedLabels),
		}
	}

	return customTagsPaths{
		customTags:   path.Root(keyCustomTags),
		overrideTags: path.Root(keyOverrideResourceTags),
		excludedTags: path.Root(keyExcludedTags),
	}
}

type configGet interface {
	GetAttribute(ctx context.Context, path path.Path, target any) diag.Diagnostics
}

func getCustomTags(ctx context.Context, paths customTagsPaths, conf configGet) (map[string]string, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var customTagsMap types.Map
	diags.Append(conf.GetAttribute(ctx, paths.customTags, &customTagsMap)...)
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

func getExcludeTags(ctx context.Context, paths customTagsPaths, conf configGet) (map[string]struct{}, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var excludedTagsSet types.Set
	diags.Append(conf.GetAttribute(ctx, paths.excludedTags, &excludedTagsSet)...)
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

func getOverrideTags(ctx context.Context, paths customTagsPaths, conf configGet) (bool, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var override types.Bool
	diags.Append(conf.GetAttribute(ctx, paths.overrideTags, &override)...)
	if diags.HasError() {
		return false, diags
	}

	return override.ValueBool(), diags
}

type configSet interface {
	SetAttribute(ctx context.Context, path path.Path, value any) diag.Diagnostics
}

func setCustomTags(ctx context.Context, paths customTagsPaths, conf configSet, customTags map[string]string) diag.Diagnostics {
	diags := diag.Diagnostics{}

	stateTagsMap := types.MapNull(types.StringType)
	if len(customTags) > 0 {
		stateTagsMap, diags = types.MapValueFrom(ctx, types.StringType, customTags)
		if diags.HasError() {
			return diags
		}
	}

	diags.Append(conf.SetAttribute(ctx, paths.customTags, stateTagsMap)...)
	return diags
}

func setExcludeTags(ctx context.Context, paths customTagsPaths, conf configSet, excludeTags map[string]struct{}) diag.Diagnostics {
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

	diags.Append(conf.SetAttribute(ctx, paths.excludedTags, stateExcludeSet)...)
	return diags
}
