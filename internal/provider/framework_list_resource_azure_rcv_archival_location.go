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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
)

const listResourceAzureRCVArchivalLocationDescription = `
The ´rubrik_azure_rcv_archival_location´ list resource lists RCV Azure archival
locations in RSC.
`

var (
	_ list.ListResource              = &azureRCVArchivalLocationListResource{}
	_ list.ListResourceWithConfigure = &azureRCVArchivalLocationListResource{}
)

type azureRCVArchivalLocationListResource struct {
	client *client
	prefix string
}

type azureRCVArchivalLocationListConfigModel struct {
	Name types.String `tfsdk:"name"`
}

func newAzureRCVArchivalLocationListResource() list.ListResource {
	return &azureRCVArchivalLocationListResource{prefix: keyRubrik}
}

func newPolarisAzureRCVArchivalLocationListResource() list.ListResource {
	return &azureRCVArchivalLocationListResource{prefix: keyPolaris}
}

func (r *azureRCVArchivalLocationListResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationListResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureRCVArchivalLocation
}

func (r *azureRCVArchivalLocationListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResourceAzureRCVArchivalLocationDescription),
		Attributes: map[string]listschema.Attribute{
			keyName: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter archival locations by name. Matches names starting with the given value.",
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_azure_rcv_archival_location` list resource instead."
	}
}

func (r *azureRCVArchivalLocationListResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureRCVArchivalLocationListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "azureRCVArchivalLocationListResource.List")

	var config azureRCVArchivalLocationListConfigModel
	diags := req.Config.Get(ctx, &config)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		diags.AddError("RSC client error", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	targetMappings, err := archival.Wrap(polarisClient).RCVAzureTargetMappings(ctx, config.Name.ValueString())
	if err != nil {
		diags.AddError("Failed to list RCV Azure archival locations", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	// The encryption key of an archival location isn't part of the target
	// mapping, it is listed with the encryption details of the archival groups.
	// RSC only lists the groups encrypted with a key from a key manager.
	var encryptionDetails []gqlarchival.ArchivalGroupEncryptionDetails
	if req.IncludeResource {
		encryptionDetails, err = archival.Wrap(polarisClient).ArchivalGroupEncryptionDetails(ctx, "")
		if err != nil {
			diags.AddError("Failed to list RCV Azure archival location encryption details", err.Error())
			stream.Results = list.ListResultsStreamDiagnostics(diags)
			return
		}
	}

	// The resource model carries a timeouts.Value whose zero value is a null
	// object with no attribute types. Setting it against the resource schema
	// would fail type conversion, so derive a correctly-typed null from the
	// schema's timeouts attribute.
	nullTimeouts := timeouts.Value{Object: types.ObjectNull(nil)}
	if obj, ok := req.ResourceSchema.Type().(types.ObjectType); ok {
		if t, ok := obj.AttrTypes[keyTimeouts].(types.ObjectType); ok {
			nullTimeouts = timeouts.Value{Object: types.ObjectNull(t.AttrTypes)}
		}
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, targetMapping := range targetMappings {
			if int64(i) >= req.Limit {
				return
			}

			result := req.NewListResult(ctx)
			result.DisplayName = targetMapping.Name

			identity := azureRCVArchivalLocationIdentityModel{
				ID: types.StringValue(targetMapping.ID.String()),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				model := newAzureRCVArchivalLocationModel(targetMapping, nullTimeouts)
				for _, details := range encryptionDetails {
					if details.ID != targetMapping.ID || details.KMSKey == nil || details.KMSKey.Key == nil ||
						details.KMSKey.Key.AzureKey == nil {
						continue
					}

					key := []azureRCVKeyVaultKeyModel{{
						KeyVaultID: types.StringValue(details.KMSKey.KeyManagerID),
						KeyName:    types.StringValue(details.KMSKey.Key.AzureKey.KeyName),
						KeyVersion: types.StringValue(details.KMSKey.Key.AzureKey.KeyVersion),
					}}
					keyList, keyDiags := types.ListValueFrom(ctx, azureRCVKeyVaultKeyType, key)
					result.Diagnostics.Append(keyDiags...)
					model.KeyVaultKey = keyList
				}
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}
			}

			if !push(result) {
				return
			}
		}
	}
}
