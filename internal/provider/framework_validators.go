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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

// isUUID returns a validator that checks if a string value is a valid UUID.
func isUUID() validator.String {
	return isUUIDValidator{}
}

type isUUIDValidator struct{}

func (v isUUIDValidator) Description(_ context.Context) string {
	return "value must be a valid UUID"
}

func (v isUUIDValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isUUIDValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if _, err := uuid.Parse(req.ConfigValue.ValueString()); err != nil {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid UUID",
			fmt.Sprintf("%q is not a valid UUID: %s", req.ConfigValue.ValueString(), err))
	}
}

// aesKeySize is the only AES key size RSC accepts, in bytes.
const aesKeySize = 32

// isAESKey returns a validator that checks if a string value is a base64
// encoded AES-256 key. It mirrors the RSC server-side check, so a bad key is
// caught at plan time rather than rejected on apply.
func isAESKey() validator.String {
	return isAESKeyValidator{}
}

type isAESKeyValidator struct{}

func (v isAESKeyValidator) Description(_ context.Context) string {
	return "value must be a base64 encoded AES-256 key"
}

func (v isAESKeyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isAESKeyValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	key, err := base64.StdEncoding.DecodeString(req.ConfigValue.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid AES key",
			fmt.Sprintf("the key is not valid base64: %s. Generate one with "+
				"`openssl rand -base64 32`.", err))
		return
	}

	if len(key) != aesKeySize {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid AES key",
			fmt.Sprintf("the key decodes to %d bytes, RSC requires exactly %d. Note that RSC calls "+
				"this field `encryptionKeyInDer`, but it takes a base64 encoded AES-256 key, not a "+
				"DER encoded key. Generate one with `openssl rand -base64 32`.", len(key), aesKeySize))
		return
	}

	if bytes.Equal(key, make([]byte, len(key))) {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid AES key",
			"the key is all zero bytes, which RSC rejects as too weak.")
	}
}

// isNotWhiteSpace returns a validator that checks if a string value is not
// empty or contains only whitespace.
func isNotWhiteSpace() validator.String {
	return isNotWhiteSpaceValidator{}
}

type isNotWhiteSpaceValidator struct{}

func (v isNotWhiteSpaceValidator) Description(_ context.Context) string {
	return "value must not be empty or contain only whitespace"
}

func (v isNotWhiteSpaceValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isNotWhiteSpaceValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if strings.TrimSpace(req.ConfigValue.ValueString()) == "" {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid Value",
			"value must not be empty or contain only whitespace")
	}
}

// isFilterTypeFor returns a validator that checks a data security policy filter
// type belongs to one of the given resource types. RSC requires every condition
// in a group to cover the same resource type, so this keeps a misplaced
// condition from reaching the API as an opaque format error. A filter type that
// does not belong to any known resource type passes, leaving RSC to reject it.
func isFilterTypeFor(resourceTypes ...filterResourceType) validator.String {
	return isFilterTypeForValidator{resourceTypes: resourceTypes}
}

type isFilterTypeForValidator struct {
	resourceTypes []filterResourceType
}

func (v isFilterTypeForValidator) Description(_ context.Context) string {
	names := make([]string, 0, len(v.resourceTypes))
	for _, resourceType := range v.resourceTypes {
		names = append(names, string(resourceType))
	}

	return fmt.Sprintf("value must be a %s condition type", strings.Join(names, " or "))
}

func (v isFilterTypeForValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v isFilterTypeForValidator) ValidateString(_ context.Context, req validator.StringRequest, res *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// An unrecognized filter type is left to RSC to reject, rather than being
	// rejected here against a prefix list that can fall behind the filter types
	// RSC registers.
	filterType := req.ConfigValue.ValueString()
	resourceType, ok := resourceTypeOf(filterType)
	if !ok {
		return
	}

	for _, allowed := range v.resourceTypes {
		if resourceType == allowed {
			return
		}
	}

	blockName := keyObjectFilter
	if resourceType == filterResourceTypeIdentity {
		blockName = keyIdentityFilter
	}
	res.Diagnostics.AddAttributeError(req.Path, "Filter type in the wrong block", fmt.Sprintf(
		"%q is an %s condition and cannot be used here. Move it to a %s block.",
		filterType, resourceType, blockName))
}

// permissionGroupValidator returns a validator that checks if a string value
// is one of the permission groups of the feature with the specified name, or
// one of the extra permission groups. If the feature isn't found in the
// features, no values are allowed.
func permissionGroupValidator(features []core.Feature, name string, extraGroups ...core.PermissionGroup) validator.String {
	var groups []string
	if feature, ok := core.LookupFeatureName(features, name); ok {
		groups = core.PermissionGroupNames(slices.Concat(feature.PermissionGroups, extraGroups))
	}

	return stringvalidator.OneOf(groups...)
}

// featurePermissionGroupsValidator returns a validator that checks that the
// permission groups of a feature object are supported by the feature. The
// object must have name and permission_groups attributes. An empty set of
// permission groups, and the extra permission groups, are accepted for any
// feature. A null or unknown object, name or set passes (nothing to validate
// yet), as does a name which isn't found in the features, since the validator
// of the name attribute reports it.
func featurePermissionGroupsValidator(features []core.Feature, extraGroups ...core.PermissionGroup) validator.Object {
	return featurePermissionGroupsObjectValidator{features: features, extraGroups: extraGroups}
}

type featurePermissionGroupsObjectValidator struct {
	features    []core.Feature
	extraGroups []core.PermissionGroup
}

func (v featurePermissionGroupsObjectValidator) Description(_ context.Context) string {
	return "permission groups must be supported by the feature"
}

func (v featurePermissionGroupsObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v featurePermissionGroupsObjectValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, res *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()
	name, ok := attrs[keyName].(types.String)
	if !ok || name.IsNull() || name.IsUnknown() {
		return
	}
	if _, ok := core.LookupFeatureName(v.features, name.ValueString()); !ok {
		return
	}
	groupSet, ok := attrs[keyPermissionGroups].(types.Set)
	if !ok || groupSet.IsNull() || groupSet.IsUnknown() {
		return
	}

	var groups []string
	for _, elem := range groupSet.Elements() {
		group, ok := elem.(types.String)
		if !ok || group.IsNull() || group.IsUnknown() || slices.Contains(v.extraGroups, core.PermissionGroup(group.ValueString())) {
			continue
		}
		groups = append(groups, group.ValueString())
	}
	if err := checkFeaturePermissionGroups(v.features, name.ValueString(), groups); err != nil {
		res.Diagnostics.AddAttributeError(req.Path, "Invalid permission group", err.Error())
	}
}

// setMustContain returns a validator that checks a set of strings contains the
// given value. A null or unknown set passes (nothing to validate yet).
func setMustContain(value string) validator.Set {
	return setMustContainValidator{value: value}
}

type setMustContainValidator struct {
	value string
}

func (v setMustContainValidator) Description(_ context.Context) string {
	return fmt.Sprintf("set must contain %q", v.value)
}

func (v setMustContainValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v setMustContainValidator) ValidateSet(ctx context.Context, req validator.SetRequest, res *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var values []string
	res.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &values, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	for _, value := range values {
		if value == v.value {
			return
		}
	}

	res.Diagnostics.AddAttributeError(req.Path, "Missing required value",
		fmt.Sprintf("%q must be included in the set", v.value))
}
