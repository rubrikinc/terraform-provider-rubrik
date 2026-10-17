# Action skeleton

File: `internal/provider/framework_action_<name>.go`.

An action runs an imperative operation that does not map to a resource lifecycle: triggering a refresh, starting an
on-demand job, running a health check. Practitioners trigger it from a resource's `lifecycle { action_trigger { ... } }`
or with `terraform apply -invoke=action.rubrik_<name>.<label>`. Actions need Terraform 1.14 or later, so every action
is optional for users and must never be required for a configuration to work.

**This provider has no actions yet, and `terraform-plugin-testing` v1.14.0 has no action assertions.** Before adding the
first one, confirm with the user that an action is the right shape (not a resource, and not a data source), and agree how
it will be tested. See the testing note below.

## One-time provider wiring (first action only)

Edit `internal/provider/framework_provider.go`:

```go
var (
	_ provider.ProviderWithListResources = &FrameworkProvider{}
	_ provider.ProviderWithActions       = &FrameworkProvider{}
)
```

Add the client for actions in `Configure`, next to the other `...Data` assignments:

```go
res.ActionData = c
```

Add the registration method, next to `ListResources`:

```go
func (p *FrameworkProvider) Actions(ctx context.Context) []func() action.Action {
	tflog.Trace(ctx, "FrameworkProvider.Actions")

	return []func() action.Action{
		new<Name>Action,
	}
}
```

with the import `github.com/hashicorp/terraform-plugin-framework/action`. Later actions only add a line to `Actions`.

## Skeleton

```go
// Copyright <YEAR> Rubrik, Inc.
// [MIT license header — copy from a neighbouring file verbatim]

package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/<sdkpkg>"
)

const action<Name>Description = `
The ´rubrik_<name>´ action <does what> in RSC.
`

var (
	_ action.Action              = &<camelName>Action{}
	_ action.ActionWithConfigure = &<camelName>Action{}
)

type <camelName>Action struct {
	client *client
}

type <camelName>ActionModel struct {
	ID types.String `tfsdk:"id"`
}

func new<Name>Action() action.Action {
	return &<camelName>Action{}
}

func (a *<camelName>Action) Metadata(ctx context.Context, req action.MetadataRequest, res *action.MetadataResponse) {
	tflog.Trace(ctx, "<camelName>Action.Metadata")

	res.TypeName = keyRubrik + "_" + key<Name>
}

func (a *<camelName>Action) Schema(ctx context.Context, _ action.SchemaRequest, res *action.SchemaResponse) {
	tflog.Trace(ctx, "<camelName>Action.Schema")

	res.Schema = schema.Schema{
		Description: description(action<Name>Description),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Required:    true,
				Description: "<What> ID (UUID).",
			},
		},
	}
}

func (a *<camelName>Action) Configure(ctx context.Context, req action.ConfigureRequest, res *action.ConfigureResponse) {
	tflog.Trace(ctx, "<camelName>Action.Configure")

	if req.ProviderData == nil {
		return
	}
	a.client = req.ProviderData.(*client)
}

func (a *<camelName>Action) Invoke(ctx context.Context, req action.InvokeRequest, res *action.InvokeResponse) {
	tflog.Trace(ctx, "<camelName>Action.Invoke")

	var config <camelName>ActionModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := a.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(config.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid <what> ID", err.Error())
		return
	}

	res.SendProgress(action.InvokeProgressEvent{Message: "Starting <operation>"})
	if err := <sdkpkg>.Wrap(polarisClient).<Operation>(ctx, id); err != nil {
		res.Diagnostics.AddError("Failed to <operation>", err.Error())
		return
	}
	res.SendProgress(action.InvokeProgressEvent{Message: "<Operation> completed"})
}
```

## Rules the skeleton encodes

- **Config only.** An action has a config schema and no state. Everything it needs comes from `req.Config`.
- **Idempotent or clearly one-shot.** An action may run on every apply that triggers it. Make a repeated call safe, or
  say in the description that it starts a new operation each time.
- **Wait for completion** when the operation is asynchronous and report progress with `res.SendProgress`, so the user
  sees what is happening during a long apply. Respect `ctx` cancellation.
- **Optional interfaces:** `ActionWithValidateConfig`, `ActionWithConfigValidators`, and `ActionWithModifyPlan` for
  plan-time checks.
- **No `Read` or `Delete`, no identity, no import.**
- **Example:** `examples/actions/rubrik_<name>/action.tf`. tfplugindocs v0.25.0 renders actions to `docs/actions/`. After
  `go generate ./...`, check that the page was created.

## Testing

`terraform-plugin-testing` v1.14.0 does not expose action assertions, so an acceptance test cannot assert that an action
ran. Until the library is upgraded, split the action so the part worth testing needs no Terraform:

1. Keep `Invoke` thin: read config, get the client, call one function, send progress.
2. Put the RSC logic (calling the SDK, waiting, mapping errors) in a function that takes an interface for the SDK call
   it needs. Unit-test that function with a fake. This is the "inject dependencies, prefer fakes" rule, and it is the
   one case where a single-use function is justified.
3. If the action is meant to be used from an `action_trigger`, add an acceptance test that applies a resource with the
   trigger and checks the side effect through the SDK afterwards (`CheckDestroy`-style helper, in the test).

Do not claim an action is acceptance-tested unless a test really checked the side effect.
