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

	pathCustomTags := path.Root(keyCustomTags)
	pathOverrideTags := path.Root(keyOverrideResourceTags)
	if vendor == core.CloudVendorGCP {
		pathCustomTags = path.Root(keyCustomLabels)
		pathOverrideTags = path.Root(keyOverrideResourceLabels)
	}

	var planTagsMap types.Map
	res.Diagnostics.Append(req.Plan.GetAttribute(ctx, pathCustomTags, &planTagsMap)...)
	if res.Diagnostics.HasError() {
		return
	}

	var planTags map[string]string
	res.Diagnostics.Append(planTagsMap.ElementsAs(ctx, &planTags, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	var planOverride types.Bool
	res.Diagnostics.Append(req.Plan.GetAttribute(ctx, pathOverrideTags, &planOverride)...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTags := make([]core.Tag, 0, len(planTags))
	for key, value := range planTags {
		customerTags = append(customerTags, core.Tag{Key: key, Value: value})
	}

	if err := tags.Wrap(client).AddCustomerTags(ctx, gqltags.CustomerTags{
		CloudVendor:          vendor,
		Tags:                 customerTags,
		OverrideResourceTags: planOverride.ValueBool(),
	}); err != nil {
		res.Diagnostics.AddError("Failed to create custom tags", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, req.Plan.Raw)...)
}

// The resource only manages the custom tags in its state. Refresh the
// values of the managed tags still present in RSC and drop the ones which
// are gone. Custom tags in RSC not managed by the resource are ignored.
func readCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "readCustomTagsResource")

	pathCustomTags := path.Root(keyCustomTags)
	pathOverrideTags := path.Root(keyOverrideResourceTags)
	if vendor == core.CloudVendorGCP {
		pathCustomTags = path.Root(keyCustomLabels)
		pathOverrideTags = path.Root(keyOverrideResourceLabels)
	}

	var stateTagsMap types.Map
	res.Diagnostics.Append(req.State.GetAttribute(ctx, pathCustomTags, &stateTagsMap)...)
	if res.Diagnostics.HasError() {
		return
	}

	var stateTags map[string]string
	res.Diagnostics.Append(stateTagsMap.ElementsAs(ctx, &stateTags, false)...)
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

	newStateTagsMap, diags := types.MapValueFrom(ctx, types.StringType, newStateTags)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, pathCustomTags, newStateTagsMap)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, pathOverrideTags,
		types.BoolValue(customerTags.OverrideResourceTags))...)
}

func updateCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "updateCustomTagsResource")

	pathCustomTags := path.Root(keyCustomTags)
	pathOverrideTags := path.Root(keyOverrideResourceTags)
	if vendor == core.CloudVendorGCP {
		pathCustomTags = path.Root(keyCustomLabels)
		pathOverrideTags = path.Root(keyOverrideResourceLabels)
	}

	var stateTagsMap types.Map
	res.Diagnostics.Append(req.State.GetAttribute(ctx, pathCustomTags, &stateTagsMap)...)
	if res.Diagnostics.HasError() {
		return
	}

	var stateTags map[string]string
	res.Diagnostics.Append(stateTagsMap.ElementsAs(ctx, &stateTags, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	var planTagsMap types.Map
	res.Diagnostics.Append(req.Plan.GetAttribute(ctx, pathCustomTags, &planTagsMap)...)
	if res.Diagnostics.HasError() {
		return
	}

	var planTags map[string]string
	res.Diagnostics.Append(planTagsMap.ElementsAs(ctx, &planTags, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	var overrideTags types.Bool
	res.Diagnostics.Append(req.Plan.GetAttribute(ctx, pathOverrideTags, &overrideTags)...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTags, err := tags.Wrap(client).CustomerTags(ctx, vendor)
	if err != nil {
		res.Diagnostics.AddError("Failed to read custom tags", err.Error())
		return
	}

	newCustomerTags := make(map[string]string, len(customerTags.Tags)+len(planTags))
	for _, tag := range customerTags.Tags {
		_, state := stateTags[tag.Key]
		_, plan := planTags[tag.Key]
		if state && !plan {
			continue
		}
		newCustomerTags[tag.Key] = tag.Value
	}
	for key, value := range planTags {
		newCustomerTags[key] = value
	}

	customerTags.Tags = make([]core.Tag, 0, len(newCustomerTags))
	for key, value := range newCustomerTags {
		customerTags.Tags = append(customerTags.Tags, core.Tag{Key: key, Value: value})
	}

	customerTags.OverrideResourceTags = overrideTags.ValueBool()
	if err := tags.Wrap(client).ReplaceCustomerTags(ctx, customerTags); err != nil {
		res.Diagnostics.AddError("Failed to update custom tags", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, req.Plan.Raw)...)
}

func deleteCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "deleteCustomTagsResource")

	pathCustomTags := path.Root(keyCustomTags)
	if vendor == core.CloudVendorGCP {
		pathCustomTags = path.Root(keyCustomLabels)
	}

	var stateTagsMap types.Map
	res.Diagnostics.Append(req.State.GetAttribute(ctx, pathCustomTags, &stateTagsMap)...)
	if res.Diagnostics.HasError() {
		return
	}

	var stateTags map[string]string
	res.Diagnostics.Append(stateTagsMap.ElementsAs(ctx, &stateTags, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	customerTagKeys := make([]string, 0, len(stateTags))
	for key := range stateTags {
		customerTagKeys = append(customerTagKeys, key)
	}

	if err := tags.Wrap(client).RemoveCustomerTags(ctx, vendor, customerTagKeys); err != nil {
		res.Diagnostics.AddError("Failed to delete custom tags", err.Error())
		return
	}
}

// Note, the custom tags resource is designed to only manage the custom tags
// owned by the resource. An import on the other hand will take ownership of
// all custom tags for a cloud vendor. The import ID is ignored, the resource
// manages an RSC account level configuration without a unique identifier.
func importCustomTagsResource(ctx context.Context, client *polaris.Client, vendor core.CloudVendor, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "importCustomTagsResource")

	pathCustomTags := path.Root(keyCustomTags)
	pathOverrideTags := path.Root(keyOverrideResourceTags)
	if vendor == core.CloudVendorGCP {
		pathCustomTags = path.Root(keyCustomLabels)
		pathOverrideTags = path.Root(keyOverrideResourceLabels)
	}

	customerTags, err := tags.Wrap(client).CustomerTags(ctx, vendor)
	if err != nil {
		res.Diagnostics.AddError("Failed to read custom tags", err.Error())
		return
	}

	stateTags := make(map[string]string, len(customerTags.Tags))
	for _, tag := range customerTags.Tags {
		stateTags[tag.Key] = tag.Value
	}

	stateTagsMap, diags := types.MapValueFrom(ctx, types.StringType, stateTags)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, pathCustomTags, stateTagsMap)...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, pathOverrideTags,
		types.BoolValue(customerTags.OverrideResourceTags))...)
}
