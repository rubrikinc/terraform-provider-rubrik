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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
)

func awsTrustPolicyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyRoleKey: types.StringType,
		keyPolicy:  types.StringType,
	}
}

func awsFromTrustPolicies(policies aws.TrustPolicyMap) (types.Set, diag.Diagnostics) {
	policyValues := make([]attr.Value, 0, len(policies))
	for roleKey, policy := range policies {
		obj, d := types.ObjectValue(awsTrustPolicyAttrTypes(), map[string]attr.Value{
			keyRoleKey: types.StringValue(roleKey),
			keyPolicy:  types.StringValue(policy),
		})
		if d.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: awsTrustPolicyAttrTypes()}), d
		}
		policyValues = append(policyValues, obj)
	}

	return types.SetValue(types.ObjectType{AttrTypes: awsTrustPolicyAttrTypes()}, policyValues)
}
