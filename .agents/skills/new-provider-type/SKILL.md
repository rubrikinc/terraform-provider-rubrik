---
name: new-provider-type
description: Scaffold a new Terraform resource, data source, list resource, or action in internal/provider using the Plugin Framework, with registration, identity and import, examples, a changelog entry, and tests. Use whenever adding any new type to the provider.
---

Usage: `new-provider-type <kind> <name> [description]`

- `kind`: one of `resource`, `data-source`, `list-resource`, or `action`
- `name`: the type name in snake_case without the provider prefix, for example `aws_cnp_account`
- `description`: optional one-line description of what the type manages or does

If an argument is missing, ask for it.

Scaffold a new provider type. Follow every step in order. Always use the Plugin Framework; never create `resource_*.go`
or `data_source_*.go` files.

If the type already exists in the SDKv2 provider (`resource_<name>.go` or `data_source_<name>.go`), stop and use
the `migrate-to-framework` skill instead.

## Placeholders

| Placeholder | Example | Used for |
|---|---|---|
| `<name>` | `custom_role` | File names, type name, example directories |
| `<Name>` | `CustomRole` | Exported-style Go names: `new<Name>Resource`, `key<Name>` |
| `<camelName>` | `customRole` | Unexported Go names: `<camelName>Resource`, `<camelName>Model` |

The Terraform type name is `rubrik_<name>`. The type has no `prefix` field and no `newPolaris...` constructor: those
exist only for types that predate the rename.

## 0. Gather what you need first

- The SDK wrapper that exposes the RSC operations (find the SDK source with
  `go list -m -json github.com/rubrikinc/rubrik-polaris-sdk-for-go | jq -r .Dir`). If the SDK does not have them yet,
  stop and say so: SDK work ships first, as its own PR, and the provider pins the SDK by pseudo-version meanwhile.
- The attribute list, which are required, optional, computed, immutable, or secret.
- For a resource: whether RSC has a list or query API. If it does, a list resource is part of the change. If it does
  not, say so in the PR notes.
- Read one neighbouring type of the same kind and follow it where this skill is silent. Good references:
  `framework_resource_custom_role.go`, `framework_data_source_role.go`, `framework_list_resource_custom_role.go`.

## 1. Add the field names

Add a `key*` constant to `internal/provider/names.go` for every schema field and for the type itself
(`key<Name> = "<name>"`). Grep for the value first and reuse existing constants. Never use string literals in a schema.

## 2. Create the type file

| Kind | File | Reference |
|---|---|---|
| `resource` | `internal/provider/framework_resource_<name>.go` | [resource.md](resource.md) |
| `data-source` | `internal/provider/framework_data_source_<name>.go` | [data-source.md](data-source.md) |
| `list-resource` | `internal/provider/framework_list_resource_<name>.go` | [list-resource.md](list-resource.md) |
| `action` | `internal/provider/framework_action_<name>.go` | [action.md](action.md) |

Read only the reference file for the kind you are creating. Every file starts with the MIT license header (copy it
from a neighbouring file, current year), has a `description` constant wrapped in `description()`, an interface
assertion block, and `tflog.Trace(ctx, "<camelName><Kind>.<Method>")` as the first line of every method.

Follow the Code Rules and the helper-function rule in `AGENTS.md`: inline logic that is used fewer than four
times.

## 3. Register the type

Add the constructor to the matching list in `internal/provider/framework_provider.go`, keeping the list's existing
alphabetical order:

- `Resources` for `new<Name>Resource`
- `DataSources` for `new<Name>DataSource`
- `ListResources` for `new<Name>ListResource`
- `Actions` for `new<Name>Action` (see [action.md](action.md) for the one-time provider wiring)

## 4. Write the tests

Create `internal/provider/framework_<kind>_<name>_test.go` from [tests.md](tests.md). A resource always gets the three
import steps. A resource also gets a `<camelName>CheckDestroy` in `framework_checkdestroys_test.go`.

## 5. Add the examples

The generated docs embed these files. Every file must be valid HCL that a user could copy.

| Kind | Files |
|---|---|
| `resource` | `examples/resources/rubrik_<name>/resource.tf`, `import.sh`, `import-by-string-id.tf`, `import-by-identity.tf` |
| `data-source` | `examples/data-sources/rubrik_<name>/data-source.tf` |
| `list-resource` | `examples/list-resources/rubrik_<name>/list-resource.tfquery.hcl` |
| `action` | `examples/actions/rubrik_<name>/action.tf` |

Copy the shape of the existing example for the same kind, for instance `examples/resources/rubrik_custom_role/`. Add a
file under `templates/` only when the generated page needs hand-written text the schema description cannot carry.

## 6. Add the changelog entry

Add an entry to `templates/guides/changelog.md.tmpl` under the unreleased version heading (ask which version if it is
unclear), using the format and priority order in `AGENTS.md`. A new resource and its list resource share one
entry.

## 7. Verify

Run the `tf-checks` skill. It regenerates the docs, so commit the resulting `docs/` changes. Then, if RSC credentials are
available, run the new acceptance tests:

```bash
TF_ACC=1 go test -count=1 -timeout=120m -run '^TestAcc<Name>' -v ./internal/provider
```

Do not claim the acceptance tests pass unless you ran them.

## Optional interfaces

Add these only when the type needs them, and add each to the interface assertion block:

| Interface | When |
|---|---|
| `ResourceWithValidateConfig` / `ResourceWithConfigValidators` | Plan-time rules that need no RSC call. Put the rules in a function that takes the model so a unit test can call it |
| `ResourceWithModifyPlan` | Plan-time behaviour the schema plan modifiers cannot express |
| `ResourceWithUpgradeState` | After bumping the schema `Version`. Keep the old schema |
| `ResourceWithMoveState` | Only when migrating a `polaris_` type, see the `migrate-to-framework` skill |

Ephemeral resources and provider-defined functions are not covered by this skill. Ask the user before adding one.

## Summary of files

| File | Purpose |
|---|---|
| `internal/provider/names.go` | New `key*` constants |
| `internal/provider/framework_<kind>_<name>.go` | The type |
| `internal/provider/framework_provider.go` | Registration |
| `internal/provider/framework_<kind>_<name>_test.go` | Tests |
| `internal/provider/framework_checkdestroys_test.go` | `CheckDestroy` for a new resource |
| `examples/...` | Docs examples |
| `templates/guides/changelog.md.tmpl` | Changelog entry |
| `docs/...` | Regenerated by `go generate ./...` |
