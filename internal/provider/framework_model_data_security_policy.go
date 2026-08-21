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
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqldspm "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/dspm"
)

// RSC accepts exactly one filter shape for a data security policy: a top-level
// group joined with AND, holding one or two condition groups, each flat and
// each covering a single resource type.
//
// The schema encodes that shape as the object_filter and identity_filter
// blocks, so the top-level group is never user-visible: it is synthesized on
// write and unwrapped on read.

// filterResourceType is the RSC resource type a filter condition applies to.
// RSC derives it from the filter type and requires each condition group to
// cover exactly one of them.
type filterResourceType string

const (
	filterResourceTypeObject   filterResourceType = "object"
	filterResourceTypeIdentity filterResourceType = "identity"
)

// filterTypePrefixes holds the filter type prefixes RSC registers for each
// resource type. Prefixes rather than an enumeration of filter types: RSC
// registers filter types the SDK's FilterType constants do not name yet, and an
// enumeration would reject valid conditions until the SDK catches up.
var filterTypePrefixes = map[filterResourceType][]string{
	filterResourceTypeObject:   {"SECURITY_DOCUMENT_", "SECURITY_SNAPPABLE_"},
	filterResourceTypeIdentity: {"SECURITY_IDENTITY_", "SECURITY_GPO_"},
}

// resourceTypeOf returns the resource type of the given filter type. The second
// return value is false for a filter type that data security policies cannot
// use, which includes the SECURITY_IDP_ family.
func resourceTypeOf(filterType string) (filterResourceType, bool) {
	for resourceType, prefixes := range filterTypePrefixes {
		for _, prefix := range prefixes {
			if strings.HasPrefix(filterType, prefix) {
				return resourceType, true
			}
		}
	}

	return "", false
}

// conditionModel maps to a single filter condition in the schema.
type conditionModel struct {
	FilterType   types.String `tfsdk:"filter_type"`
	Relationship types.String `tfsdk:"relationship"`
	Values       types.List   `tfsdk:"values"`
}

// conditionGroupModel maps to a flat group of filter conditions in the schema.
// It backs the object_filter, identity_filter and threshold_filter blocks.
type conditionGroupModel struct {
	Operator  types.String     `tfsdk:"op"`
	Condition []conditionModel `tfsdk:"condition"`
}

// toPolicyFilter builds the top-level filter group from the object_filter
// and identity_filter blocks. The object group is always emitted first, so the
// order RSC reads back does not depend on the order the blocks appear in the
// configuration.
func toPolicyFilter(ctx context.Context, object, identity []conditionGroupModel) (gqldspm.GroupConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	root := gqldspm.GroupConfig{Op: gqldspm.LogicalAnd}
	for _, group := range [][]conditionGroupModel{object, identity} {
		gc, d := toFilterGroupConfig(ctx, group)
		diags.Append(d...)
		if diags.HasError() {
			return gqldspm.GroupConfig{}, diags
		}
		if gc != nil {
			root.Filters = append(root.Filters, gqldspm.Node{GroupConfig: gc})
		}
	}

	return root, diags
}

// toFilterGroupConfig converts a condition group block into a flat GroupConfig.
// It returns nil when the block is absent.
func toFilterGroupConfig(ctx context.Context, group []conditionGroupModel) (*gqldspm.GroupConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(group) == 0 {
		return nil, diags
	}

	nodes, d := toConditionNodes(ctx, group[0].Condition)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	return &gqldspm.GroupConfig{
		Op:      gqldspm.LogicalOp(group[0].Operator.ValueString()),
		Filters: nodes,
	}, diags
}

// toThresholdGroupConfig converts the threshold_filter block into a GroupConfig
// holding its single condition. It returns nil when the block is absent.
//
// The block has no operator: it holds one condition, so there is nothing to
// join. AND is sent because RSC rejects a group without an operator at
// evaluation time, and with a single condition AND and OR are equivalent.
func toThresholdGroupConfig(ctx context.Context, threshold []conditionModel) (*gqldspm.GroupConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(threshold) == 0 {
		return nil, diags
	}

	nodes, d := toConditionNodes(ctx, threshold)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	return &gqldspm.GroupConfig{Op: gqldspm.LogicalAnd, Filters: nodes}, diags
}

// toConditionNodes converts filter conditions into leaf nodes.
func toConditionNodes(ctx context.Context, conditions []conditionModel) ([]gqldspm.Node, diag.Diagnostics) {
	var diags diag.Diagnostics

	nodes := make([]gqldspm.Node, 0, len(conditions))
	for _, condition := range conditions {
		// A relationship such as EXISTS or IS_EMPTY takes no values, in which
		// case the values attribute is left out. RSC expects an empty list
		// rather than null on the wire, so the slice is never left nil.
		values := []string{}
		if !condition.Values.IsNull() {
			diags.Append(condition.Values.ElementsAs(ctx, &values, false)...)
			if diags.HasError() {
				return nil, diags
			}
		}

		nodes = append(nodes, gqldspm.Node{
			Config: &gqldspm.Config{
				Type:         gqldspm.FilterType(condition.FilterType.ValueString()),
				Relationship: gqldspm.Relationship(condition.Relationship.ValueString()),
				Values:       values,
			},
		})
	}

	return nodes, diags
}

// fromPolicyFilter splits the top-level filter group returned by RSC into
// the object_filter and identity_filter blocks. Either may be empty, but not
// both.
//
// Predefined and legacy policies are exempt from the RSC format rules, so a
// policy can hold a filter that the schema cannot represent. Such a filter is
// reported as an error naming the rule it breaks rather than being silently
// flattened, which would produce a permanent diff.
func fromPolicyFilter(ctx context.Context, root *gqldspm.GroupConfig) (object, identity []conditionGroupModel, diags diag.Diagnostics) {
	if root == nil {
		diags.Append(unsupportedFilterError("the policy has no filter"))
		return nil, nil, diags
	}
	if root.Op != gqldspm.LogicalAnd {
		diags.Append(unsupportedFilterError(fmt.Sprintf(
			"the top-level group joins its condition groups with %s, but only AND is supported", root.Op)))
		return nil, nil, diags
	}
	if len(root.Filters) < 1 || len(root.Filters) > 2 {
		diags.Append(unsupportedFilterError(fmt.Sprintf(
			"the top-level group holds %d condition groups, but only one or two are supported", len(root.Filters))))
		return nil, nil, diags
	}

	for _, node := range root.Filters {
		if node.GroupConfig == nil {
			diags.Append(unsupportedFilterError(
				"the top-level group holds a bare condition, but every entry must be a condition group"))
			return nil, nil, diags
		}

		group, resourceType, d := fromConditionGroupConfig(ctx, node.GroupConfig)
		diags.Append(d...)
		if diags.HasError() {
			return nil, nil, diags
		}

		switch {
		case resourceType == filterResourceTypeObject && object == nil:
			object = group
		case resourceType == filterResourceTypeIdentity && identity == nil:
			identity = group
		default:
			diags.Append(unsupportedFilterError(fmt.Sprintf(
				"the top-level group holds two %s condition groups, but only one of each resource type is supported",
				resourceType)))
			return nil, nil, diags
		}
	}

	return object, identity, diags
}

// fromConditionGroupConfig converts a flat condition group returned by RSC into
// a condition group block, and reports the single resource type its conditions
// cover.
func fromConditionGroupConfig(ctx context.Context, gc *gqldspm.GroupConfig) ([]conditionGroupModel, filterResourceType, diag.Diagnostics) {
	group, diags := fromFilterGroupConfig(ctx, gc)
	if diags.HasError() {
		return nil, "", diags
	}
	if len(gc.Filters) == 0 {
		diags.Append(unsupportedFilterError("a condition group is empty"))
		return nil, "", diags
	}

	var resourceType filterResourceType
	for _, node := range gc.Filters {
		filterType := string(node.Config.Type)
		nodeResourceType, ok := resourceTypeOf(filterType)
		if !ok {
			diags.Append(unsupportedFilterError(fmt.Sprintf(
				"a condition group holds the condition type %s, which data security policies do not support", filterType)))
			return nil, "", diags
		}
		if resourceType != "" && nodeResourceType != resourceType {
			diags.Append(unsupportedFilterError(
				"a condition group mixes object and identity conditions, but each group must cover a single resource type"))
			return nil, "", diags
		}
		resourceType = nodeResourceType
	}

	return group, resourceType, diags
}

// fromFilterGroupConfig converts a flat group returned by RSC into a condition
// group block. It returns nil when the group is absent.
func fromFilterGroupConfig(ctx context.Context, gc *gqldspm.GroupConfig) ([]conditionGroupModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	if gc == nil {
		return nil, diags
	}

	conditions, d := fromConditionNodes(ctx, gc.Filters)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	return []conditionGroupModel{{
		Operator:  types.StringValue(string(gc.Op)),
		Condition: conditions,
	}}, diags
}

// fromThresholdGroupConfig converts the threshold filter returned by RSC into
// the threshold_filter block. It returns nil when the policy has no threshold
// filter.
//
// The block holds a single condition, matching the RSC policy editor. RSC
// itself does not enforce that, so a policy created outside Terraform can carry
// several threshold conditions. Such a policy is reported as an error rather
// than silently reduced to its first condition, which would change which
// objects the policy flags.
func fromThresholdGroupConfig(ctx context.Context, gc *gqldspm.GroupConfig) ([]conditionModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	if gc == nil || len(gc.Filters) == 0 {
		return nil, diags
	}
	if len(gc.Filters) > 1 {
		diags.Append(unsupportedThresholdFilterError(fmt.Sprintf(
			"it holds %d conditions, but the block supports a single condition", len(gc.Filters))))
		return nil, diags
	}
	if gc.Filters[0].Config == nil {
		diags.Append(unsupportedThresholdFilterError(
			"it holds a group of conditions, but the block supports a single condition"))
		return nil, diags
	}

	return fromConditionNodes(ctx, gc.Filters)
}

// fromConditionNodes converts the leaf nodes of a flat group returned by RSC
// into filter conditions.
func fromConditionNodes(ctx context.Context, nodes []gqldspm.Node) ([]conditionModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	conditions := make([]conditionModel, 0, len(nodes))
	for _, node := range nodes {
		if node.Config == nil {
			diags.Append(unsupportedFilterError(
				"a condition group holds a nested group, but every entry must be a single condition"))
			return nil, diags
		}

		// A condition using a relationship that takes no values reads back
		// without values. It maps to a null list, matching the omitted values
		// attribute in the configuration, rather than to an empty list, which
		// would produce a permanent diff.
		values := types.ListNull(types.StringType)
		if len(node.Config.Values) > 0 {
			v, d := types.ListValueFrom(ctx, types.StringType, node.Config.Values)
			diags.Append(d...)
			if diags.HasError() {
				return nil, diags
			}
			values = v
		}

		conditions = append(conditions, conditionModel{
			FilterType:   types.StringValue(string(node.Config.Type)),
			Relationship: types.StringValue(string(node.Config.Relationship)),
			Values:       values,
		})
	}

	return conditions, diags
}

// unsupportedThresholdFilterError returns the diagnostic for a policy whose
// threshold filter the schema cannot represent.
func unsupportedThresholdFilterError(reason string) diag.Diagnostic {
	return diag.NewErrorDiagnostic("Unsupported data security policy threshold filter", fmt.Sprintf(
		"The threshold filter cannot be represented by this provider: %s.\n\n"+
			"The %s block holds a single condition, matching the RSC data security policy editor. RSC does "+
			"not enforce that, so a policy created outside of the editor can hold additional threshold "+
			"conditions.",
		reason, keyThresholdFilter))
}

// unsupportedFilterError returns the diagnostic for a policy whose filter the
// schema cannot represent.
func unsupportedFilterError(reason string) diag.Diagnostic {
	return diag.NewErrorDiagnostic("Unsupported data security policy filter", fmt.Sprintf(
		"The filter cannot be represented by this provider: %s.\n\n"+
			"RSC accepts only a top-level AND group holding one or two flat condition groups, one per "+
			"resource type. Predefined policies and policies created before RSC started enforcing that "+
			"shape are exempt, so they can hold filters that the %s and %s blocks cannot express.",
		reason, keyObjectFilter, keyIdentityFilter))
}
