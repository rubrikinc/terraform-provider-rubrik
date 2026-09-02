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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/access"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/dspm"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/sla"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/tags"
)

// awsAccountCheckDestroy verifies that all aws_account resources have been
// deleted.
func awsAccountCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_aws_account" && rs.Type != "rubrik_aws_account" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = aws.Wrap(polarisClient).AccountByID(t.Context(), id)
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
func awsCnpAccountCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_aws_cnp_account" && rs.Type != "rubrik_aws_cnp_account" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = aws.Wrap(polarisClient).AccountByID(t.Context(), id)
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

// awsCnpAccountAttachmentsCheckDestroy verifies that all
// aws_cnp_account_attachments resources have been deleted. Since attachments
// share their lifecycle with the parent aws_cnp_account, the parent account
// being gone implies the attachments are gone too.
func awsCnpAccountAttachmentsCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_aws_cnp_account_attachments" && rs.Type != "rubrik_aws_cnp_account_attachments" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = aws.Wrap(polarisClient).AccountByID(t.Context(), id)
			if err == nil {
				return fmt.Errorf("aws_cnp_account_attachments %s still exists", id)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// azureSubscriptionCheckDestroy verifies that all azure_subscription resources
// have been deleted.
func azureSubscriptionCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_azure_subscription" && rs.Type != "rubrik_azure_subscription" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = azure.Wrap(polarisClient).SubscriptionByID(t.Context(), id)
			if err == nil {
				return fmt.Errorf("azure subscription %s still exists", id)
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
func customRoleCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_custom_role" && rs.Type != "rubrik_custom_role" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = access.Wrap(polarisClient).RoleByID(t.Context(), id)
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

// customTagsCheckDestroy verifies that the custom tags managed by all custom
// tags resources of the specified cloud vendor have been removed from RSC.
//
// Note, only the custom tags managed by the resources are checked. The custom
// tags for a cloud vendor is global RSC state shared with other resources and
// other users, so the vendor's set of custom tags is not expected to be empty.
func customTagsCheckDestroy(t *testing.T, vendor core.CloudVendor) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		var customTagsKey string
		var resourceTypes []string
		switch vendor {
		case core.CloudVendorAWS:
			customTagsKey = keyCustomTags
			resourceTypes = []string{"rubrik_aws_custom_tags", "polaris_aws_custom_tags"}
		case core.CloudVendorAzure:
			customTagsKey = keyCustomTags
			resourceTypes = []string{"rubrik_azure_custom_tags", "polaris_azure_custom_tags"}

		case core.CloudVendorGCP:
			customTagsKey = keyCustomLabels
			resourceTypes = []string{"rubrik_gcp_custom_labels", "polaris_gcp_custom_labels"}
		default:
			return fmt.Errorf("unknown vendor: %s", vendor)
		}

		customerTags, err := tags.Wrap(polarisClient).CustomerTags(t.Context(), vendor)
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if !slices.Contains(resourceTypes, rs.Type) {
				continue
			}

			for attr := range rs.Primary.Attributes {
				tagKey, ok := strings.CutPrefix(attr, customTagsKey+".")
				if !ok || tagKey == "%" {
					continue
				}
				if slices.ContainsFunc(customerTags.Tags, func(tag core.Tag) bool {
					return tag.Key == tagKey
				}) {
					return fmt.Errorf("custom tag %q still exists", tagKey)
				}
			}
		}

		return nil
	}
}

// roleAssignmentCheckDestroy verifies that the specific roles managed by each
// role_assignment resource have been unassigned. Roles outside the resource's
// management are ignored. Users or SSO groups not found are ignored.
func roleAssignmentCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
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
			user, err := access.Wrap(polarisClient).UserByID(t.Context(), rs.Primary.ID)
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
			group, err := access.Wrap(polarisClient).SSOGroupByID(t.Context(), rs.Primary.ID)
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

// dataSecurityPolicyCheckDestroy verifies that all data_security_policy
// resources have been deleted.
func dataSecurityPolicyCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "rubrik_data_security_policy" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = dspm.Wrap(polarisClient).PolicyByID(t.Context(), id)
			if err == nil {
				return fmt.Errorf("data security policy %s still exists", id)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// tagRuleCheckDestroy verifies that all tag_rule resources have been deleted.
func tagRuleCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_tag_rule" && rs.Type != "rubrik_tag_rule" {
				continue
			}

			id, err := uuid.Parse(rs.Primary.ID)
			if err != nil {
				return err
			}

			_, err = sla.Wrap(polarisClient).TagRuleByID(t.Context(), id)
			if err == nil {
				return fmt.Errorf("tag rule %s still exists", id)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// ssoGroupCheckDestroy verifies that all sso_group resources have been
// deleted.
func ssoGroupCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_sso_group" && rs.Type != "rubrik_sso_group" {
				continue
			}

			_, err := access.Wrap(polarisClient).SSOGroupByID(t.Context(), rs.Primary.ID)
			if err == nil {
				return fmt.Errorf("SSO group %s still exists", rs.Primary.ID)
			}
			if !errors.Is(err, graphql.ErrNotFound) {
				return err
			}
		}

		return nil
	}
}

// userCheckDestroy verifies that all user resources have been deleted.
func userCheckDestroy(t *testing.T) func(*terraform.State) error {
	t.Helper()
	polarisClient := testClient(t)

	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "polaris_user" && rs.Type != "rubrik_user" {
				continue
			}

			_, err := access.Wrap(polarisClient).UserByID(t.Context(), rs.Primary.ID)
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
