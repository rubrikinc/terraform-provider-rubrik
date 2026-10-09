# Resource skeleton

File: `internal/provider/framework_resource_<name>.go`. Modeled on `framework_resource_custom_role.go`.

Every new resource implements resource identity and import. Do not leave either out.

```go
// Copyright <YEAR> Rubrik, Inc.
// [MIT license header — copy from a neighbouring file verbatim]

package provider

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/<sdkpkg>"
)

const resource<Name>Description = `
The ´rubrik_<name>´ resource manages <what it manages> in RSC.
`

var (
	_ resource.Resource                = &<camelName>Resource{}
	_ resource.ResourceWithIdentity    = &<camelName>Resource{}
	_ resource.ResourceWithImportState = &<camelName>Resource{}
)

type <camelName>Resource struct {
	client *client
}

// <camelName>Model is the resource state. Field order follows the schema.
type <camelName>Model struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

// <camelName>IdentityModel is the resource identity, shared with the list
// resource when there is one.
type <camelName>IdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func new<Name>Resource() resource.Resource {
	return &<camelName>Resource{}
}

func (r *<camelName>Resource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "<camelName>Resource.Metadata")

	res.TypeName = keyRubrik + "_" + key<Name>
}

func (r *<camelName>Resource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "<camelName>Resource.Schema")

	res.Schema = schema.Schema{
		Description: description(resource<Name>Description),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "<What> ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "<What> name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyDescription: schema.StringAttribute{
				Optional:    true,
				Description: "<What> description.",
			},
		},
	}
}

func (r *<camelName>Resource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "<camelName>Resource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "<What> ID (UUID).",
			},
		},
	}
}

func (r *<camelName>Resource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "<camelName>Resource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *<camelName>Resource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "<camelName>Resource.Create")

	var plan <camelName>Model
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := <sdkpkg>.Wrap(polarisClient).Create<Name>(ctx, plan.Name.ValueString(), plan.Description.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Failed to create <what>", err.Error())
		return
	}

	plan.ID = types.StringValue(id.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := <camelName>IdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *<camelName>Resource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "<camelName>Resource.Read")

	var state <camelName>Model
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid <what> ID", err.Error())
		return
	}

	obj, err := <sdkpkg>.Wrap(polarisClient).<Name>ByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read <what>", err.Error())
		return
	}

	// Read must rebuild the full state from the ID alone, since an import has
	// no configuration to fall back on.
	state.Name = types.StringValue(obj.Name)
	if obj.Description != "" || !state.Description.IsNull() {
		state.Description = types.StringValue(obj.Description)
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := <camelName>IdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *<camelName>Resource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "<camelName>Resource.Update")

	var plan <camelName>Model
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state <camelName>Model
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid <what> ID", err.Error())
		return
	}

	if err := <sdkpkg>.Wrap(polarisClient).Update<Name>(ctx, id, plan.Name.ValueString(), plan.Description.ValueString()); err != nil {
		res.Diagnostics.AddError("Failed to update <what>", err.Error())
		return
	}

	plan.ID = state.ID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := <camelName>IdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *<camelName>Resource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "<camelName>Resource.Delete")

	var state <camelName>Model
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid <what> ID", err.Error())
		return
	}

	err = <sdkpkg>.Wrap(polarisClient).Delete<Name>(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to delete <what>", err.Error())
		return
	}
}

func (r *<camelName>Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "<camelName>Resource.ImportState")

	// Handles both `import { id = "..." }` and `import { identity = { id = "..." } }`.
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}
```

## Rules the skeleton encodes

- **Identity:** `Create`, `Read`, and `Update` all set `res.Identity`. The identity attributes are `RequiredForImport`.
- **Import:** `ImportState` is a single pass-through for both import styles. If the ID is not a plain UUID, parse it in
  `ImportState` and document the format in the `id` attribute description. If the resource has no identity-compatible
  form (a composite ID, for instance), say why in a comment and in the PR notes.
- **Not found:** `Read` removes the resource from state on `graphql.ErrNotFound`. `Delete` ignores it. Use `errors.Is`.
- **Optional strings that RSC returns as `""`:** keep them `null` in state when the configuration left them unset, as in
  the `Read` above. Writing `""` over a `null` makes every plan show drift, and an import would differ from the applied
  state. The same applies to optional lists, sets, and maps that RSC returns empty: keep `null` unless the state
  already holds a value.
- **`id` attribute:** computed, with `UseStateForUnknown`.
- **Immutable fields:** add `stringplanmodifier.RequiresReplace()` (or the type's equivalent) and say so in the
  attribute description. Every attribute that is not replaced must be updatable in `Update`. If every attribute
  forces replacement, `Update` still has to exist; make it return an error diagnostic that explains why.
- **Secrets:** `Sensitive: true`. For a value RSC needs on create or update but never returns, use `WriteOnly: true`
  (it cannot be `Computed`, and it is never in state, so it cannot be verified by an import step).
- **Blocks versus attributes:** use nested attributes for new schemas. Use blocks only to stay compatible with an
  existing configuration, as `rubrik_custom_role` does.
- **Description:** use `´` for backticks and see `AGENTS.md` for the `{{` escaping rule. Write the description
  for a user reading the generated page: what the resource manages, notable behaviour on delete, and any RSC feature
  flag or prerequisite.
- **Errors:** user-facing summary in `AddError`, SDK error text as the detail. Do not wrap the SDK error again.
- **Regions:** SDK typed regions, never `string`, and parse with the SDK's `RegionFromAny`.
- **Update on a computed-only change:** copy `plan.ID = state.ID` so the ID never becomes unknown.
