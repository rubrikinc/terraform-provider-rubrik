---
name: tf-checks
description: Run the local CI check suite for the Terraform provider. Use before pushing changes.
---

Run the local CI check suite for the Terraform provider. It mirrors the CI stages.

Usage: `tf-checks [package]`. The optional `package` is a Go package path to check, for example
`./internal/provider/...`. It defaults to `./...`. Wherever a command below says `<package>`, use that value.

## Ensure Dependencies are Installed

Ensure that the `staticcheck` tool is available. If it is not installed it can be run or installed with
`go install honnef.co/go/tools/cmd/staticcheck@v0.7.0`.

## Run Checks

1. Run `go mod tidy` and then `git diff --exit-code go.mod go.sum` to verify dependencies are in sync. Report pass/fail.
2. Run `gofmt -l .` and report any unformatted files.
3. Run `go vet -tags=cdm <package>` and report pass/fail. The `cdm` tag includes the tag-gated test files so they are
checked too.
4. Run `staticcheck -tags=cdm <package>` and report pass/fail.
5. Run `go generate ./...`, then `git status --porcelain docs/ examples/ templates/` and report any change. Generated docs
must be committed, so a diff here means the docs are out of sync with the code. `go generate` needs a `terraform`
binary. If it fails because none is available, report that instead of skipping the check silently.
6. Run `env -u TF_ACC go test <package>` and report pass/fail. Acceptance tests are skipped unless `TF_ACC` is set, and
`TF_ACC` may already be set in the environment, so unset it for this run, as CI does. Acceptance tests create real RSC
objects. Run them only when the user asks.

## Report Results

Report the results of each check as a table. If any check failed, report the command output.
