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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

// moveCustomTagsResourceV0 moves v0 state from the polaris_aws_custom_tags,
// polaris_azure_custom_tags and polaris_gcp_custom_labels resources in the
// rubrikinc/polaris provider to the rubrik_aws_custom_tags,
// rubrik_azure_custom_tags and rubrik_gcp_custom_labels resources.
func moveCustomTagsResourceV0(vendor core.CloudVendor) resource.StateMover {
	type resourceConfig struct {
		customTagsKey   string
		overrideTagsKey string
		typeName        string
	}

	var conf resourceConfig
	switch vendor {
	case core.CloudVendorAWS:
		conf = resourceConfig{
			customTagsKey:   keyCustomTags,
			overrideTagsKey: keyOverrideResourceTags,
			typeName:        keyAWSCustomTags,
		}
	case core.CloudVendorAzure:
		conf = resourceConfig{
			customTagsKey:   keyCustomTags,
			overrideTagsKey: keyOverrideResourceTags,
			typeName:        keyAzureCustomTags,
		}
	case core.CloudVendorGCP:
		conf = resourceConfig{
			customTagsKey:   keyCustomLabels,
			overrideTagsKey: keyOverrideResourceLabels,
			typeName:        keyGCPCustomLabels,
		}
	default:
		// The vendor is a constant passed in by an implementation, never user
		// input, so reaching this means a vendor was added without updating
		// this function. We cannot handle this as an error since we don't know
		// which resource the mover is for, it's determined by the vendor
		// constant.
		panic(fmt.Sprintf("unknown vendor: %q", vendor))
	}

	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				conf.customTagsKey: schema.MapAttribute{
					ElementType: types.StringType,
					Required:    true,
				},
				conf.overrideTagsKey: schema.BoolAttribute{
					Optional: true,
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "moveCustomTagsResourceV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+conf.typeName {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var id types.String
			res.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root(keyID), &id)...)
			var customTagsMap types.Map
			res.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root(conf.customTagsKey), &customTagsMap)...)
			var overrideTags types.Bool
			res.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root(conf.overrideTagsKey), &overrideTags)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.SetAttribute(ctx, path.Root(keyID), id)...)
			res.Diagnostics.Append(res.TargetState.SetAttribute(ctx, path.Root(conf.customTagsKey), customTagsMap)...)
			res.Diagnostics.Append(res.TargetState.SetAttribute(ctx, path.Root(conf.overrideTagsKey), overrideTags)...)
		},
	}
}
