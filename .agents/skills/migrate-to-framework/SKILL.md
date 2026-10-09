---
name: migrate-to-framework
description: Migrate an existing SDKv2 resource or data source to the Plugin Framework, keeping the deprecated polaris_ alias and adding a _move.go so state can move from the polaris provider. Use when converting a resource_*.go or data_source_*.go file.
---

Usage: `migrate-to-framework <name>`, where `name` is the type name in snake_case without the provider prefix of an
existing SDKv2 type, for example `aws_exocompute`. If it is missing, ask for it.

Migrate one SDKv2 type to the Plugin Framework. Users of the deprecated `polaris` provider move to this provider, so every SDKv2 type
needs a Framework implementation here and a state migration path. Migrate one type per PR (a
resource and its cluster-attachment sibling may go together if they share code).

Existing users must not notice, except for the deprecation message on the `polaris_` name. Preserve attribute names,
types, defaults, block versus attribute shape, and replace-on-change behaviour. Anything that cannot be preserved goes in
the PR notes and in the upgrade guide.

## Placeholders

`<name>`, `<Name>`, and `<camelName>` are as in the `new-provider-type` skill. `<polarisKey>` is the `keyPolaris<Name>` constant
the SDKv2 type is registered under in `provider.go`.

## 1. Understand the SDKv2 type

- Read `resource_<name>.go` (or `data_source_<name>.go`) and any `resource_<name>_v0.go` / `_v1.go` files.
- Record every attribute and block with its type, `Required` / `Optional` / `Computed`, `ForceNew`, `Default`,
  `Sensitive`, validators, and description.
- Record `SchemaVersion` and every `StateUpgraders` entry. Record how the ID is built and parsed (a composite ID needs
  extra care).
- Read the existing acceptance test, if any. Its intent must survive the rewrite.
- Look at the closest migrated type and follow it: `framework_resource_user.go`, `framework_resource_user_move.go`, and
  `framework_resource_user_upgrade.go` (a type with schema versions), or `framework_resource_custom_role.go` and
  `framework_resource_custom_role_move.go` (a single version).

## 2. Create the Framework type

Use the `new-provider-type` skill for the skeleton of the right kind, with these differences:

- **Two constructors and a `prefix` field.** `new<Name>Resource` returns `&<camelName>Resource{prefix: keyRubrik}` and
  `newPolaris<Name>Resource` returns `&<camelName>Resource{prefix: keyPolaris}`. Same for data sources and list
  resources.
- **`Metadata`:** `res.TypeName = r.prefix + "_" + key<Name>`.
- **Deprecation:** at the end of `Schema`, for the alias only:

  ```go
  if r.prefix == keyPolaris {
  	res.Schema.DeprecationMessage = "use `rubrik_<name>` instead."
  }
  ```

  For a data source or list resource, say so in the message: "use the `rubrik_<name>` data source instead." or "use
  the `rubrik_<name>` list resource instead."
- **Schema version.** Set `Version` to the SDKv2 `SchemaVersion`, so existing state is accepted as is.
- **Same behaviour.** Port validation, plan-time behaviour, and error handling. Do not "improve" semantics in the same
  PR.
- **Identity and import:** implement `ResourceWithIdentity` and `ResourceWithImportState` as in the `new-provider-type` skill,
  keeping the SDKv2 import ID format working. If the SDKv2 ID is composite, `ImportState` parses it and the identity
  carries the parts.

## 3. Keep state upgrades for existing `rubrik_` and `polaris_` state

For every `StateUpgraders` entry in the SDKv2 type add an entry to `UpgradeState` (`ResourceWithUpgradeState`) in
`framework_resource_<name>_upgrade.go`, with the prior schema copied from the SDKv2 schema. See
`framework_resource_user_upgrade.go`.

## 4. Add the state mover

Create `framework_resource_<name>_move.go` implementing `ResourceWithMoveState`, with **one `StateMover` per source
schema version** of the `polaris_<name>` resource in the `rubrikinc/polaris` provider (version 0 up to the SDKv2
`SchemaVersion`). Each mover:

- sets `SourceSchema` to the SDKv2 schema of that version (attributes only declare type and required/optional/computed,
  see the examples)
- returns early unless `req.SourceProviderAddress` ends in `rubrikinc/polaris` or `rubrikinc/rubrik`,
  `req.SourceTypeName == keyPolaris+"_"+key<Name>`, and `req.SourceSchemaVersion` matches
- copies the source state into the target model, applying the same transformation the state upgrader would for older
  versions

Data sources have no state, so they need no mover or upgrader.

## 5. Register the Framework type and remove the SDKv2 one

- Add `new<Name>Resource` and `newPolaris<Name>Resource` to `Resources` in `framework_provider.go` (and likewise for
  `DataSources` and `ListResources`), keeping alphabetical order.
- Remove the `<polarisKey>: resource<Name>()` entry from the map passed to `withDeprecatedPolarisAlias` in `provider.go`.
  That one entry registers both `rubrik_<name>` and the deprecated `polaris_<name>`, so removing it removes both SDKv2
  registrations. The type must never be registered in both providers.
- Delete the SDKv2 files (`resource_<name>.go`, `_v0`, `_v1`, its test) in the same commit. Remove `keyPolaris<Name>` from
  `names.go` if nothing uses it any more, and any `key*` constant that only the SDKv2 file used.

## 6. Tests

Rewrite the acceptance test from `tests.md` in the `new-provider-type` skill:

- Keep the intent of every existing step.
- Add the three import steps (`ImportCommandWithID`, `ImportBlockWithID`, `ImportBlockWithResourceIdentity`). A migration
  that loses or lacks import coverage is not complete.
- Add `ExpectIdentity` and `ExpectIdentityValueMatchesState` to the create and update steps.
- Cover the move. If the harness cannot run the previous provider, write a unit test of each `StateMover` with a
  hand-built source state, asserting the resulting model. Cover each source schema version.
- Cover each `UpgradeState` entry the same way.
- Name cloud tests with the `TestAccAws`, `TestAccAzure`, or `TestAccGcp` prefix so CI selects them.

## 7. Examples, docs, and changelog

- Move any example files from `examples/` to match the Framework layout (add `import.sh`, `import-by-string-id.tf`, and
  `import-by-identity.tf` for a resource).
- Add a changelog entry (see `AGENTS.md`), for example
  `* Migrate the rubrik_<name> resource to the Plugin Framework. The resource now supports import by identity, and state
  can be moved from polaris_<name> with a moved block. [[docs](../resources/<name>.md)]`. Add an upgrade guide section if
  anything user-visible changed.

## 8. Verify

Run the `tf-checks` skill, then run the acceptance tests for the type if credentials are available. Do not claim they pass unless
you ran them. If the real provider behaviour could differ from the SDKv2 behaviour, say so in the PR notes.
