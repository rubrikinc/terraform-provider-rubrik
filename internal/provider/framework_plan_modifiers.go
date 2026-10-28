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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
)

// requiresReplaceUnlessImported returns a plan modifier for write-only key
// blocks which requires the resource to be replaced on any change to the
// block, except when the block is added to a resource that already uses that
// kind of key. RSC never returns the key, so after an import the block is
// missing from the state while the configuration has it. The state's
// encryption_type tells that the resource is encrypted with the kind of key
// the block holds, and the block is then added to the state with an in-place
// update. The provider can't check that the configured key is the one RSC
// holds, it trusts the configuration.
//
// Adding the block to a resource using another kind of key, removing it, and
// changing the version all still require replacement.
func requiresReplaceUnlessImported(encryptionType gqlarchival.TargetEncryptionType) planmodifier.List {
	return requiresReplaceUnlessImportedModifier{encryptionType: encryptionType}
}

type requiresReplaceUnlessImportedModifier struct {
	encryptionType gqlarchival.TargetEncryptionType
}

func (m requiresReplaceUnlessImportedModifier) Description(context.Context) string {
	return "Requires replacement when the block is changed, unless it is added to an imported resource " +
		"already using this kind of key."
}

func (m requiresReplaceUnlessImportedModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m requiresReplaceUnlessImportedModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, res *planmodifier.ListResponse) {
	// Nothing to replace on create, and a destroy has no plan.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	if req.PlanValue.Equal(req.StateValue) {
		return
	}

	var stateEncryptionType types.String
	res.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(keyEncryptionType), &stateEncryptionType)...)
	if res.Diagnostics.HasError() {
		return
	}

	// A list left out of the configuration is either null or empty, both mean
	// that the list has no elements.
	hadElements := !req.StateValue.IsNull() && len(req.StateValue.Elements()) > 0
	hasElements := !req.PlanValue.IsNull() && !req.PlanValue.IsUnknown() && len(req.PlanValue.Elements()) > 0
	addedToImported := !hadElements && hasElements && stateEncryptionType.ValueString() == string(m.encryptionType)
	res.RequiresReplace = !addedToImported
}

// requiresReplaceOnAddOrRemove returns a plan modifier for list attributes and
// blocks which requires the resource to be replaced when the list goes from
// empty to non-empty, or from non-empty to empty. Changes to the elements of a
// non-empty list don't require replacement, which makes it suitable for blocks
// where a change to the contents is applied in place but the block can't be
// added to, or removed from, an existing resource.
//
// This differs from listplanmodifier.RequiresReplace, which requires
// replacement for any change to the list.
func requiresReplaceOnAddOrRemove() planmodifier.List {
	return requiresReplaceOnAddOrRemoveModifier{}
}

type requiresReplaceOnAddOrRemoveModifier struct{}

func (requiresReplaceOnAddOrRemoveModifier) Description(context.Context) string {
	return "Requires replacement when the block is added or removed."
}

func (m requiresReplaceOnAddOrRemoveModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (requiresReplaceOnAddOrRemoveModifier) PlanModifyList(_ context.Context, req planmodifier.ListRequest, res *planmodifier.ListResponse) {
	// Nothing to replace on create, and a destroy has no plan.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	// An unknown plan value isn't known to be added or removed.
	if req.PlanValue.IsUnknown() {
		return
	}

	// A list left out of the configuration is either null or empty, both mean
	// that the list has no elements.
	hadElements := !req.StateValue.IsNull() && len(req.StateValue.Elements()) > 0
	hasElements := !req.PlanValue.IsNull() && len(req.PlanValue.Elements()) > 0
	if hadElements != hasElements {
		res.RequiresReplace = true
	}
}
