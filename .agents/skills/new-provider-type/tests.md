# Test skeletons

File: `internal/provider/framework_<kind>_<name>_test.go`. The test rules in `AGENTS.md` apply. This file shows
the shape.

Test object names and descriptions must make leftovers easy to spot and delete, for example the name
`Terraform Test <Name>` and the description `Acceptance test: Delete Me!`.

Do not add a helper function used fewer than four times. Inline fixtures and checks. A helper that earns its place
gets a name specific to the type.

## Unit test

Use this for conversion, validation, and plan-time logic that needs no RSC. No `TF_ACC` involved.

```go
func Test<Thing>(t *testing.T) {
	tests := []struct {
		name    string
		input   <InputType>
		want    <OutputType>
		wantErr bool
	}{
		{name: "Valid", input: ..., want: ...},
		{name: "Invalid", input: ..., wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := <function>(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
```

Skip a unit test that only restates the code (a string-to-enum mapping, "constant is in list"). Test behaviour that can
break.

## Resource acceptance test

Name: `TestAcc<Name>Resource`. The harness fails any step whose follow-up plan is not empty, so a no-op re-plan is
checked after every apply.

```go
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAcc<Name>Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             <camelName>CheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with only the required
			// attributes.
			Config: `
				resource "rubrik_<name>" "test" {
					name = "Terraform Test <Name>"
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_<name>.test", tfjsonpath.New(keyID), NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_<name>.test", tfjsonpath.New(keyName),
					knownvalue.StringExact("Terraform Test <Name>")),
				statecheck.ExpectIdentity("rubrik_<name>.test", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_<name>.test", tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the resource can be updated in place, and that the
			// ID does not change.
			Config: `
				resource "rubrik_<name>" "test" {
					name        = "Terraform Test <Name> Updated"
					description = "Acceptance test: Delete Me!"
				}
			`,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_<name>.test", plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_<name>.test", tfjsonpath.New(keyName),
					knownvalue.StringExact("Terraform Test <Name> Updated")),
				statecheck.ExpectKnownValue("rubrik_<name>.test", tfjsonpath.New(keyDescription),
					knownvalue.StringExact("Acceptance test: Delete Me!")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_<name>.test", tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName:      "rubrik_<name>.test",
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName:    "rubrik_<name>.test",
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName:    "rubrik_<name>.test",
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
```

Add steps for anything else that can break: each `RequiresReplace` attribute (assert
`plancheck.ResourceActionReplace`), each optional attribute being removed, each validator that rejects a value, and each
state upgrade or move. A step with `ExpectError` and `PlanOnly: true` goes **first** in `Steps`, before anything is
applied, because the final destroy reuses the last step's configuration and that must be valid.

### Import rules

- All three import steps are required. Drop one only when the type cannot support it, and say why in a comment and in
  the PR notes.
- `ImportStateVerifyIgnore` lists attributes that legitimately do not round-trip: write-only and sensitive values RSC
  does not return, and timeouts blocks. Each entry needs a comment saying why. Empty is the default.
- Composite IDs use `ImportStateIdFunc` and an `ImportStateId` the test computes. Inline it unless four or more tests
  share it.
- A singleton resource passes a throwaway `ImportStateId` and asserts the fixed ID, see
  `framework_resource_self_serve_rolling_upgrade_test.go`.
- An import step follows a step that left the resource in state. It uses that step's configuration.

### CheckDestroy

Add to `framework_checkdestroys_test.go`:

```go
// <camelName>CheckDestroy verifies that all rubrik_<name> resources have been
// deleted.
func <camelName>CheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "rubrik_<name>" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = <sdkpkg>.Wrap(polarisClient).<Name>ByID(t.Context(), id)
			if err == nil {
				return fmt.Errorf("<what> %s still exists", id)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}
```

### Gating

`resource.Test` skips itself unless `TF_ACC` is set. Any code that runs before it and calls RSC (`testClient`, fixtures,
`skipUnlessFeatureEnabled`) must call `skipUnlessAcceptanceTest(t)` first, as the existing helpers do. Put
`skipUnlessFeatureEnabled(t, core.FeatureFlag...)` first in a test that depends on an RSC feature flag. Name cloud
tests with the `TestAccAws`, `TestAccAzure`, or `TestAccGcp` prefix (see `AGENTS.md`).

## Data source acceptance test

Name: `TestAcc<Name>DataSource`. Create the object with a resource in the same configuration, then look it up both ways
and compare the data source to the resource. Besides the imports of the resource test it needs `regexp` and
`github.com/hashicorp/terraform-plugin-testing/compare`.

```go
func TestAcc<Name>DataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             <camelName>CheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that an invalid lookup is rejected at plan time. Steps that
			// expect an error go first: the final destroy reuses the last step's
			// configuration, which must be valid.
			Config: `
				data "rubrik_<name>" "invalid" {
					id   = "00000000-0000-0000-0000-000000000000"
					name = "both set"
				}
			`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
		}, {
			// Verify that the data source finds the object by ID and by name.
			Config: `
				resource "rubrik_<name>" "test" {
					name        = "Terraform Test <Name> DS"
					description = "Data source acceptance test: Delete Me!"
				}

				data "rubrik_<name>" "by_id" {
					id = rubrik_<name>.test.id
				}

				data "rubrik_<name>" "by_name" {
					name = rubrik_<name>.test.name
				}
			`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.CompareValuePairs("data.rubrik_<name>.by_id", tfjsonpath.New(keyID),
					"rubrik_<name>.test", tfjsonpath.New(keyID), compare.ValuesSame()),
				statecheck.CompareValuePairs("data.rubrik_<name>.by_name", tfjsonpath.New(keyID),
					"rubrik_<name>.test", tfjsonpath.New(keyID), compare.ValuesSame()),
			},
		}},
	})
}
```

When the object cannot be created by Terraform, create it through the SDK inside the test and remove it with
`t.Cleanup`, as the fixtures in `framework_fixtures_test.go` do (for example `createTestRole`).

## List resource acceptance test

Name: `TestAcc<Name>ListResource`. List tests need Terraform 1.14 or later and use `Query: true`. Imports:
`knownvalue`, `querycheck`, `tfversion`, and `helper/resource` from `terraform-plugin-testing`.

```go
func TestAcc<Name>ListResource(t *testing.T) {
	// Create an object through the SDK so the list has something to find.
	id := createTest<Name>(t, "Terraform Test <Name> List")

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			// Verify that the object is listed, with its identity.
			Query: true,
			Config: `
				provider "polaris" {}

				list "rubrik_<name>" "all" {
					provider = polaris
				}
			`,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_<name>.all", map[string]knownvalue.Check{
					keyID: knownvalue.StringExact(id.String()),
				}),
			},
		}, {
			// Verify that the name filter narrows the result to the object.
			Query: true,
			Config: `
				provider "polaris" {}

				list "rubrik_<name>" "filtered" {
					provider = polaris

					config {
						name = "Terraform Test <Name> List"
					}
				}
			`,
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectIdentity("rubrik_<name>.filtered", map[string]knownvalue.Check{
					keyID: knownvalue.StringExact(id.String()),
				}),
				querycheck.ExpectLength("rubrik_<name>.filtered", 1),
			},
		}},
	})
}
```

The provider block name matches the neighbouring list tests (see `framework_list_resource_custom_role_test.go`); the
test factories serve both `polaris` and `rubrik`. Write `createTest<Name>` inline in the test when it is used once. If
it is needed by four or more tests, move it to `framework_fixtures_test.go` with a feature-specific name and register
cleanup with `t.Cleanup`, calling `skipUnlessAcceptanceTest(t)` first.

## Action tests

See [action.md](action.md). The testing library cannot assert that an action ran, so unit-test the logic behind
`Invoke` with a fake, and cover the trigger path with an acceptance test that checks the side effect.
