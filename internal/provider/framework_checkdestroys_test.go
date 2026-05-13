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
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/access"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
)

// awsAccountCheckDestroy verifies that all aws_account resources have been
// deleted.
func awsAccountCheckDestroy(ctx context.Context) func(*terraform.State) error {
	return func(s *terraform.State) error {
		client, err := testClient(ctx)
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_aws_account" && rs.Type != "rubrik_aws_account" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = aws.Wrap(client).AccountByID(ctx, id)
			if err == nil {
				return fmt.Errorf("aws account %s still exists", id)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// awsCnpAccountCheckDestroy verifies that all aws_cnp_account resources have
// been deleted.
func awsCnpAccountCheckDestroy(ctx context.Context) func(*terraform.State) error {
	return func(s *terraform.State) error {
		client, err := testClient(ctx)
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_aws_cnp_account" && rs.Type != "rubrik_aws_cnp_account" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = aws.Wrap(client).AccountByID(ctx, id)
			if err == nil {
				return fmt.Errorf("aws_cnp_account %s still exists", id)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// customRoleCheckDestroy verifies that all custom_role resources have been
// deleted.
func customRoleCheckDestroy(ctx context.Context) func(*terraform.State) error {
	return func(s *terraform.State) error {
		client, err := testClient(ctx)
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_custom_role" && rs.Type != "rubrik_custom_role" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = access.Wrap(client).RoleByID(ctx, id)
			if err == nil {
				return fmt.Errorf("custom role %s still exists", id)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// roleAssignmentCheckDestroy verifies that the specific roles managed by each
// role_assignment resource have been unassigned. Roles outside the resource's
// management are ignored. Users or SSO groups not found are ignored.
func roleAssignmentCheckDestroy(ctx context.Context) func(*terraform.State) error {
	return func(s *terraform.State) error {
		client, err := testClient(ctx)
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_role_assignment" && rs.Type != "rubrik_role_assignment" {
				continue
			}

			// Collect the managed role IDs from the state.
			managedRoleIDs := make(map[uuid.UUID]struct{})
			if v, ok := rs.Primary.Attributes[keyRoleID]; ok && v != "" {
				id, err := uuid.Parse(v)
				if err != nil {
					return err
				}
				managedRoleIDs[id] = struct{}{}
			}
			if countStr, ok := rs.Primary.Attributes[keyRoleIDs+".#"]; ok {
				count, err := strconv.Atoi(countStr)
				if err != nil {
					return err
				}
				for i := 0; i < count; i++ {
					v := rs.Primary.Attributes[fmt.Sprintf("%s.%d", keyRoleIDs, i)]
					id, err := uuid.Parse(v)
					if err != nil {
						return err
					}
					managedRoleIDs[id] = struct{}{}
				}
			}

			// Try as user.
			user, err := access.Wrap(client).UserByID(ctx, rs.Primary.ID)
			if err == nil {
				for _, role := range user.Roles {
					if _, ok := managedRoleIDs[role.ID]; ok {
						return fmt.Errorf("role %q still assigned to user %q", role.ID, rs.Primary.ID)
					}
				}
				continue
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}

			// Try as SSO group.
			group, err := access.Wrap(client).SSOGroupByID(ctx, rs.Primary.ID)
			if err == nil {
				for _, role := range group.Roles {
					if _, ok := managedRoleIDs[role.ID]; ok {
						return fmt.Errorf("role %q still assigned to SSO group %q", role.ID, rs.Primary.ID)
					}
				}
				continue
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// userCheckDestroy verifies that all user resources have been deleted.
func userCheckDestroy(ctx context.Context) func(*terraform.State) error {
	return func(s *terraform.State) error {
		client, err := testClient(ctx)
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_user" && rs.Type != "rubrik_user" {
				continue
			}

			_, err := access.Wrap(client).UserByID(ctx, rs.Primary.ID)
			if err == nil {
				return fmt.Errorf("user %s still exists", rs.Primary.ID)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}
