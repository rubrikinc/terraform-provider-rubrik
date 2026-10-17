## What changed

<!-- Bullet list of the specific resources, data sources, list resources, attributes, or schema versions added, modified, or removed -->

-

## Why

<!-- 1-2 sentences: what was missing, what broke, or what the user needs -->

## Testing

<!-- Name the unit and acceptance tests added or updated and what each guards. State which RSC deployment the acceptance tests ran against. -->

- [ ] Unit tests added/updated
- [ ] Acceptance tests added/updated and run (`TF_ACC=1`) against a live RSC deployment
- [ ] New resources cover create, update, and all three import kinds (`ImportCommandWithID`, `ImportBlockWithID`, `ImportBlockWithResourceIdentity`) <!-- remove if no new resource -->
- [ ] `go vet`, `staticcheck`, and `gofmt` pass
- [ ] `go generate ./...` produces no diff
- [ ] `go test ./...` passes

## Docs and changelog

- [ ] Examples added/updated under `examples/` <!-- remove if not applicable -->
- [ ] Changelog entry added to `templates/guides/changelog.md.tmpl`
- [ ] Upgrade guide added/updated <!-- remove if not applicable -->

## Notes for review

<!-- Breaking changes, state upgrades or moves, SDK version (a pseudo-version during development), known gaps — delete section if none -->
