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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cluster"
)

// clusterSettingsModel mirrors gqlcluster.UpgradeDetails for use as a TF data
// source result.
type clusterSettingsModel struct {
	ClusterID               types.String `tfsdk:"cluster_id"`
	Name                    types.String `tfsdk:"name"`
	Version                 types.String `tfsdk:"version"`
	FastUpgradePreferred    types.Bool   `tfsdk:"fast_upgrade_preferred"`
	RollingUpgradeSupported types.Bool   `tfsdk:"rolling_upgrade_supported"`
	UpgradeStatusV2         types.Object `tfsdk:"upgrade_status_v2"`
	LastUpgradeDuration     types.Object `tfsdk:"last_upgrade_duration"`
}

// uiStatusAttributesAttrTypes describes the upgrade_status_v2.ui_status_attributes
// nested object schema.
func uiStatusAttributesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keySourceVersion: types.StringType,
		keyTargetVersion: types.StringType,
		keyProgress:      types.Float64Type,
		keyErrorMsg:      types.StringType,
		keyUpgradeMode:   types.StringType,
	}
}

// upgradeStatusV2AttrTypes describes the upgrade_status_v2 nested object schema.
func upgradeStatusV2AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyRSCClusterUpgradeStatus: types.StringType,
		keyUIStatusAttributes:      types.ObjectType{AttrTypes: uiStatusAttributesAttrTypes()},
	}
}

// lastUpgradeDurationAttrTypes describes the last_upgrade_duration nested
// object schema.
func lastUpgradeDurationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyFastUpgradeDuration:    types.Int64Type,
		keyRollingUpgradeDuration: types.Int64Type,
	}
}

// fromUpgradeDetails converts a gqlcluster.UpgradeDetails to a
// clusterSettingsModel for use in TF state.
func fromUpgradeDetails(ctx context.Context, details gqlcluster.UpgradeDetails) (clusterSettingsModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := clusterSettingsModel{
		ClusterID:               types.StringValue(details.ID.String()),
		Name:                    types.StringValue(details.Name),
		Version:                 types.StringNull(),
		FastUpgradePreferred:    types.BoolNull(),
		RollingUpgradeSupported: types.BoolNull(),
		UpgradeStatusV2:         types.ObjectNull(upgradeStatusV2AttrTypes()),
		LastUpgradeDuration:     types.ObjectNull(lastUpgradeDurationAttrTypes()),
	}

	info := details.CDMInfo
	if info == nil {
		return model, diags
	}

	model.Version = types.StringValue(info.Version)
	model.FastUpgradePreferred = types.BoolValue(info.FastUpgradePreferred)
	model.RollingUpgradeSupported = types.BoolValue(info.IsRUSupported)

	if v2 := info.UpgradeStatusV2; v2 != nil {
		attrs, attrDiags := types.ObjectValue(uiStatusAttributesAttrTypes(), map[string]attr.Value{
			keySourceVersion: types.StringValue(v2.UIStatusAttributes.SourceVersion),
			keyTargetVersion: types.StringValue(v2.UIStatusAttributes.TargetVersion),
			keyProgress:      types.Float64Value(v2.UIStatusAttributes.Progress),
			keyErrorMsg:      types.StringValue(v2.UIStatusAttributes.ErrorMsg),
			keyUpgradeMode:   types.StringValue(v2.UIStatusAttributes.UpgradeMode),
		})
		diags.Append(attrDiags...)
		obj, objDiags := types.ObjectValue(upgradeStatusV2AttrTypes(), map[string]attr.Value{
			keyRSCClusterUpgradeStatus: types.StringValue(string(v2.RSCClusterUpgradeStatus)),
			keyUIStatusAttributes:      attrs,
		})
		diags.Append(objDiags...)
		model.UpgradeStatusV2 = obj
	}

	if dur := info.LastUpgradeDuration; dur != nil {
		obj, objDiags := types.ObjectValue(lastUpgradeDurationAttrTypes(), map[string]attr.Value{
			keyFastUpgradeDuration:    types.Int64Value(dur.FastUpgradeDuration),
			keyRollingUpgradeDuration: types.Int64Value(dur.RollingUpgradeDuration),
		})
		diags.Append(objDiags...)
		model.LastUpgradeDuration = obj
	}

	return model, diags
}
