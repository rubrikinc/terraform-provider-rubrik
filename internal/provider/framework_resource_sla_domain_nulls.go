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
	"math/big"
	"slices"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// keepNullsFromPrior returns the remote value, where an attribute which RSC
// returns as the zero value of its type is null when it is null in the prior
// value.
//
// RSC doesn't tell an attribute which isn't set from one which is set to "", 0
// or false, and the SDKv2 resource treated them the same. The Framework doesn't:
// an optional attribute which is null in the plan has to be null in the state
// after the apply, and in the state after a refresh to not show a diff. An
// attribute which is set to its zero value in the prior value is kept as it is,
// so is a value which isn't zero, since that is a change made outside Terraform.
//
// The prior value is the plan when creating or updating, and the state when
// reading. Blocks are matched by their position, and the elements of a set can't
// be, so they are left as RSC returned them.
func keepNullsFromPrior(remote, prior tftypes.Value) tftypes.Value {
	if !remote.IsKnown() || remote.IsNull() || !prior.IsKnown() {
		return remote
	}
	if prior.IsNull() {
		if isZeroPrimitive(remote) {
			return prior
		}
		return remote
	}

	switch {
	case remote.Type().Is(tftypes.Object{}) && prior.Type().Is(tftypes.Object{}):
		var remoteAttrs, priorAttrs map[string]tftypes.Value
		if remote.As(&remoteAttrs) != nil || prior.As(&priorAttrs) != nil {
			return remote
		}
		attrs := make(map[string]tftypes.Value, len(remoteAttrs))
		for name, value := range remoteAttrs {
			if priorValue, ok := priorAttrs[name]; ok {
				value = keepNullsFromPrior(value, priorValue)
			}
			attrs[name] = value
		}
		return tftypes.NewValue(remote.Type(), attrs)
	case remote.Type().Is(tftypes.List{}) && prior.Type().Is(tftypes.List{}):
		var remoteElems, priorElems []tftypes.Value
		if remote.As(&remoteElems) != nil || prior.As(&priorElems) != nil {
			return remote
		}
		elems := slices.Clone(remoteElems)
		for i := range min(len(remoteElems), len(priorElems)) {
			elems[i] = keepNullsFromPrior(remoteElems[i], priorElems[i])
		}
		return tftypes.NewValue(remote.Type(), elems)
	}

	return remote
}

// isZeroPrimitive reports whether the value is "", 0 or false.
func isZeroPrimitive(value tftypes.Value) bool {
	switch {
	case value.Type().Is(tftypes.String):
		var s string
		return value.As(&s) == nil && s == ""
	case value.Type().Is(tftypes.Number):
		var n big.Float
		return value.As(&n) == nil && n.Sign() == 0
	case value.Type().Is(tftypes.Bool):
		var b bool
		return value.As(&b) == nil && !b
	}
	return false
}
