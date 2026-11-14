package provider

import (
	"errors"
	"fmt"
	"io/fs"
	"net/mail"
	"os"
	"slices"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

// validateAwsAccountID verifies the account number is numeric, 12 digits.
func validateAwsAccountID(i any, k string) ([]string, []error) {
	v, ok := i.(string)
	if !ok {
		return nil, []error{fmt.Errorf("expected type of %q to be string", k)}
	}
	if len(v) != 12 {
		return nil, []error{fmt.Errorf("%q is not a valid account number", v)}
	}
	if _, err := strconv.ParseUint(v, 10, 64); err != nil {
		return nil, []error{fmt.Errorf("%q is not a valid account number", v)}
	}

	return nil, nil
}

// validateEmailAddress verifies that i contains a valid email address.
func validateEmailAddress(i any, k string) ([]string, []error) {
	v, ok := i.(string)
	if !ok {
		return nil, []error{fmt.Errorf("expected type of %q to be string", k)}
	}
	if _, err := mail.ParseAddress(v); err != nil {
		return nil, []error{fmt.Errorf("%q is not a valid email address", v)}
	}

	return nil, nil
}

// validateFileExist assumes m is a file path and returns nil if the file exist,
// otherwise a diagnostic message is returned.
func validateFileExist(i any, k string) ([]string, []error) {
	v, ok := i.(string)
	if !ok {
		return nil, []error{fmt.Errorf("expected type of %q to be string", k)}
	}

	if _, err := os.Stat(v); err != nil {
		details := "unknown error"
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			details = pathErr.Err.Error()
		}

		return nil, []error{fmt.Errorf("failed to access file: %s", details)}
	}

	return nil, nil
}

// validate numNodes verifies that the num_nodes value is valid. 2 is not allowed
func validateNumNodes(i any, k string) ([]string, []error) {
	v, ok := i.(int)
	if !ok {
		return nil, []error{fmt.Errorf("expected type of %q to be string", k)}
	}
	if v == 2 {
		return nil, []error{fmt.Errorf("num_nodes cannot be 2")}
	}
	if v < 1 {
		return nil, []error{fmt.Errorf("num_nodes must be greater than 0")}
	}
	return nil, nil
}

// validatePermissions verifies that the permissions value is valid.
func validatePermissions(m any, p cty.Path) diag.Diagnostics {
	if m.(string) != "update" {
		return diag.Errorf("invalid permissions value")
	}

	return nil
}

// validateRoleARN verifies that the role ARN is a valid AWS ARN.
func validateRoleARN(m any, p cty.Path) diag.Diagnostics {
	if _, err := arn.Parse(m.(string)); err != nil {
		return diag.Errorf("failed to parse role ARN: %v", err)
	}

	return nil
}

// validatePermissionGroup returns a function that validates that the value is
// one of the permission groups of the feature with the specified name, or one
// of the extra permission groups. If the feature isn't found in the features,
// no values are allowed.
func validatePermissionGroup(features []core.Feature, name string, extraGroups ...core.PermissionGroup) func(any, string) ([]string, []error) {
	var groups []string
	if feature, ok := core.LookupFeatureName(features, name); ok {
		groups = core.PermissionGroupNames(slices.Concat(feature.PermissionGroups, extraGroups))
	}

	return validation.StringInSlice(groups, false)
}

// checkFeaturePermissionGroups returns an error if the feature with the
// specified name isn't found in the features, or if it doesn't support all
// the permission groups.
func checkFeaturePermissionGroups(features []core.Feature, name string, groups []string) error {
	feature, ok := core.LookupFeatureName(features, name)
	if !ok {
		return fmt.Errorf("feature %q is not supported", name)
	}
	for _, group := range groups {
		if !feature.HasPermissionGroup(core.PermissionGroup(group)) {
			return fmt.Errorf("feature %q does not support permission group %q", name, group)
		}
	}

	return nil
}
