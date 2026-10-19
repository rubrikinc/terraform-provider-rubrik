# List resource skeleton

File: `internal/provider/framework_list_resource_<name>.go`. Modeled on `framework_list_resource_custom_role.go`.

A list resource backs `terraform query` (`.tfquery.hcl` files) and generates import blocks and configuration for
existing objects. It needs Terraform 1.14 or later and reuses the resource's identity and model, so the resource must
exist first. Add a list resource for every new resource whose RSC API can list objects.

```go
// Copyright <YEAR> Rubrik, Inc.
// [MIT license header — copy from a neighbouring file verbatim]

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/<sdkpkg>"
)

const listResource<Name>Description = `
The ´rubrik_<name>´ list resource lists <what> in RSC.
`

var (
	_ list.ListResource              = &<camelName>ListResource{}
	_ list.ListResourceWithConfigure = &<camelName>ListResource{}
)

type <camelName>ListResource struct {
	client *client
}

// <camelName>ListConfigModel holds the optional filters of the list block.
type <camelName>ListConfigModel struct {
	Name types.String `tfsdk:"name"`
}

func new<Name>ListResource() list.ListResource {
	return &<camelName>ListResource{}
}

func (r *<camelName>ListResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "<camelName>ListResource.Metadata")

	// Must match the resource's type name.
	res.TypeName = keyRubrik + "_" + key<Name>
}

func (r *<camelName>ListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "<camelName>ListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResource<Name>Description),
		Attributes: map[string]listschema.Attribute{
			keyName: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter by name. Matches objects whose name contains the given value (case-insensitive).",
			},
		},
	}
}

func (r *<camelName>ListResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "<camelName>ListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *<camelName>ListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "<camelName>ListResource.List")

	var config <camelName>ListConfigModel
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

	objs, err := <sdkpkg>.Wrap(polarisClient).<Name>s(ctx, config.Name.ValueString())
	if err != nil {
		diags.AddError("Failed to list <what>", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, obj := range objs {
			if int64(i) >= req.Limit {
				return
			}

			result := req.NewListResult(ctx)
			result.DisplayName = obj.Name

			identity := <camelName>IdentityModel{ID: types.StringValue(obj.ID.String())}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				model := <camelName>Model{
					ID:          types.StringValue(obj.ID.String()),
					Name:        types.StringValue(obj.Name),
					Description: types.StringValue(obj.Description),
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
```

## Rules the skeleton encodes

- **Identity on every result.** Each result sets `result.Identity`. A list result without an identity cannot be turned
  into an import block.
- **Display name.** Set `result.DisplayName` to something a human recognises (the object name).
- **`IncludeResource`.** Only build the full resource model when `req.IncludeResource` is true. It must match what the
  resource's `Read` would put in state, so the generated configuration is correct. When the list API returns less than
  `Read` does, either make the extra call per object in this branch only, or leave the attribute unset and document it.
- **`Limit`.** Honour `req.Limit`. Stop when `push` returns false.
- **Filters** are optional attributes on the list block. Filter in RSC when the API supports it, otherwise in Go after
  the call. Do not add filters nobody has asked for.
- **No `prefix` field.** The `prefix` field and `newPolaris...` constructor exist only for types that predate the rename.
- Register in `ListResources` in `framework_provider.go`. The example file is
  `examples/list-resources/rubrik_<name>/list-resource.tfquery.hcl`.
