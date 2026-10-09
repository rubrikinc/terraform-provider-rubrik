# Data source skeleton

File: `internal/provider/framework_data_source_<name>.go`. Modeled on `framework_data_source_role.go`.

```go
// Copyright <YEAR> Rubrik, Inc.
// [MIT license header — copy from a neighbouring file verbatim]

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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/<sdkpkg>"
)

const dataSource<Name>Description = `
The ´rubrik_<name>´ data source is used to access information about <what> in RSC.
<What> is looked up using either the ID or the name.
`

var _ datasource.DataSource = &<camelName>DataSource{}

type <camelName>DataSource struct {
	client *client
}

// <camelName>DataSourceModel is the data source state. The lookup keys are
// optional, everything else is computed.
type <camelName>DataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func new<Name>DataSource() datasource.DataSource {
	return &<camelName>DataSource{}
}

func (d *<camelName>DataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "<camelName>DataSource.Metadata")

	res.TypeName = keyRubrik + "_" + key<Name>
}

func (d *<camelName>DataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "<camelName>DataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSource<Name>Description),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "<What> ID (UUID).",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyName)),
					isUUID(),
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "<What> name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyDescription: schema.StringAttribute{
				Computed:    true,
				Description: "<What> description.",
			},
		},
	}
}

func (d *<camelName>DataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "<camelName>DataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *<camelName>DataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "<camelName>DataSource.Read")

	var config <camelName>DataSourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var obj <sdkpkg>.<Type>
	if !config.ID.IsNull() {
		id, err := uuid.Parse(config.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid <what> ID", err.Error())
			return
		}
		obj, err = <sdkpkg>.Wrap(polarisClient).<Name>ByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read <what>", err.Error())
			return
		}
	} else {
		obj, err = <sdkpkg>.Wrap(polarisClient).<Name>ByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read <what>", err.Error())
			return
		}
	}

	state := <camelName>DataSourceModel{
		ID:          types.StringValue(obj.ID.String()),
		Name:        types.StringValue(obj.Name),
		Description: types.StringValue(obj.Description),
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
```

## Rules the skeleton encodes

- **No state side effects.** A data source only reads. Never create, modify, or refresh anything in RSC.
- **Lookup keys:** use `stringvalidator.ExactlyOneOf` (or `AtLeastOneOf` / `ConflictsWith`) so an invalid combination is
  a plan-time error, not a runtime one. Mark a key both `Optional` and `Computed` when the data source also returns it.
- **Not found is an error** by default. Only return an empty or null result if the data source is explicitly a "maybe"
  lookup, and say so in the description.
- **Multi-match:** a name lookup that can match more than one object must fail with a message that tells the user to
  look up by ID. Never silently pick the first match.
- **Computed collections:** return sets or lists with a stable order (sort before setting) so the plan is not noisy.
- **No import, no identity.** Data sources have neither.
- A data source that only reshapes provider-side data (no RSC call, such as the permission-group data sources) still
  implements `Read` with the same structure, but `Configure` and the client are not needed.
