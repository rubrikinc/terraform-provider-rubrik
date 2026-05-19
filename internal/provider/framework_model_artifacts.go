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
)

type awsInstanceProfileModel struct {
	Key  types.String `tfsdk:"key"`
	Name types.String `tfsdk:"name"`
}

func awsInstanceProfileAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyKey:  types.StringType,
		keyName: types.StringType,
	}
}

func awsFromInstanceProfiles(profiles map[string]string) []awsInstanceProfileModel {
	models := make([]awsInstanceProfileModel, 0, len(profiles))
	for key, name := range profiles {
		models = append(models, awsInstanceProfileModel{
			Key:  types.StringValue(key),
			Name: types.StringValue(name),
		})
	}
	return models
}

func awsToInstanceProfiles(ctx context.Context, set types.Set) (map[string]string, diag.Diagnostics) {
	var models []awsInstanceProfileModel
	diags := set.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	profiles := make(map[string]string, len(models))
	for _, m := range models {
		profiles[m.Key.ValueString()] = m.Name.ValueString()
	}
	return profiles, diags
}

type awsRoleModel struct {
	Key         types.String `tfsdk:"key"`
	ARN         types.String `tfsdk:"arn"`
	Permissions types.String `tfsdk:"permissions"`
}

func awsRoleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyKey:         types.StringType,
		keyARN:         types.StringType,
		keyPermissions: types.StringType,
	}
}

func awsFromRoles(roles map[string]string) []awsRoleModel {
	models := make([]awsRoleModel, 0, len(roles))
	for key, arn := range roles {
		models = append(models, awsRoleModel{
			Key: types.StringValue(key),
			ARN: types.StringValue(arn),
		})
	}
	return models
}

func awsToRoles(ctx context.Context, set types.Set) (map[string]string, diag.Diagnostics) {
	var models []awsRoleModel
	diags := set.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	roles := make(map[string]string, len(models))
	for _, m := range models {
		roles[m.Key.ValueString()] = m.ARN.ValueString()
	}
	return roles, diags
}
