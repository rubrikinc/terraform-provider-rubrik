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
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// This file holds the models of the azure_sql_database_config and
// azure_sql_managed_instance_config blocks. They have the same schema, so they
// share a model. An Azure SQL SLA is V1 (Azure-managed) when the block has an
// ltr_config, and V2 (Rubrik-managed) when it doesn't, so the difference between
// a missing ltr_config and an empty one is significant.

// azureSQLLTRRetentionModel is a weekly_retention or monthly_retention block of
// the ltr_config block.
type azureSQLLTRRetentionModel struct {
	Retention     types.Int64  `tfsdk:"retention"`
	RetentionUnit types.String `tfsdk:"retention_unit"`
}

// azureSQLLTRYearlyRetentionModel is the yearly_retention block of the
// ltr_config block.
type azureSQLLTRYearlyRetentionModel struct {
	Retention     types.Int64  `tfsdk:"retention"`
	RetentionUnit types.String `tfsdk:"retention_unit"`
	WeekOfYear    types.Int64  `tfsdk:"week_of_year"`
}

// azureSQLLTRConfigModel is the ltr_config block.
type azureSQLLTRConfigModel struct {
	WeeklyRetention  []azureSQLLTRRetentionModel       `tfsdk:"weekly_retention"`
	MonthlyRetention []azureSQLLTRRetentionModel       `tfsdk:"monthly_retention"`
	YearlyRetention  []azureSQLLTRYearlyRetentionModel `tfsdk:"yearly_retention"`
}

// azureSQLConfigModel is the azure_sql_database_config and
// azure_sql_managed_instance_config blocks.
type azureSQLConfigModel struct {
	LogRetention types.Int64              `tfsdk:"log_retention"`
	LTRConfig    []azureSQLLTRConfigModel `tfsdk:"ltr_config"`
}

// fromAzureSQLConfigModel returns the SDK Azure SQL config of the block, or nil
// if the block isn't configured. The LTR config is nil when the block has no
// ltr_config, which marks the SLA as V2.
func fromAzureSQLConfigModel(config []azureSQLConfigModel) *gqlsla.AzureDBConfig {
	if len(config) == 0 {
		return nil
	}

	return &gqlsla.AzureDBConfig{
		LogRetentionInDays: int(config[0].LogRetention.ValueInt64()),
		LTRConfig:          fromAzureSQLLTRConfigModel(config[0].LTRConfig),
	}
}

// fromAzureSQLLTRConfigModel returns the SDK LTR config of the ltr_config
// block, or nil when there is no ltr_config.
func fromAzureSQLLTRConfigModel(block []azureSQLLTRConfigModel) *gqlsla.AzureSQLLTRConfig {
	if len(block) == 0 {
		return nil
	}

	ltr := block[0]
	config := &gqlsla.AzureSQLLTRConfig{
		WeeklyBackupRetention:  fromAzureSQLLTRRetentionModel(ltr.WeeklyRetention),
		MonthlyBackupRetention: fromAzureSQLLTRRetentionModel(ltr.MonthlyRetention),
	}

	if len(ltr.YearlyRetention) > 0 {
		y := ltr.YearlyRetention[0]
		config.YearlyBackupRetention = &gqlsla.AzureSQLYearlyLTRRetention{
			Retention: gqlsla.AzureSQLLTRRetention{
				Retention:     int(y.Retention.ValueInt64()),
				RetentionUnit: gqlsla.RetentionUnit(y.RetentionUnit.ValueString()),
			},
			WeekOfYear: int(y.WeekOfYear.ValueInt64()),
		}
	}

	return config
}

// fromAzureSQLLTRRetentionModel returns the SDK LTR retention of a weekly or
// monthly retention block, or nil if the block isn't configured.
func fromAzureSQLLTRRetentionModel(block []azureSQLLTRRetentionModel) *gqlsla.AzureSQLLTRRetention {
	if len(block) == 0 {
		return nil
	}

	return &gqlsla.AzureSQLLTRRetention{
		Retention:     int(block[0].Retention.ValueInt64()),
		RetentionUnit: gqlsla.RetentionUnit(block[0].RetentionUnit.ValueString()),
	}
}

// toAzureSQLConfigModel returns the block of the SDK Azure SQL config, or nil if
// the config is nil.
func toAzureSQLConfigModel(config *gqlsla.AzureDBConfig) []azureSQLConfigModel {
	if config == nil {
		return nil
	}

	return []azureSQLConfigModel{{
		LogRetention: types.Int64Value(int64(config.LogRetentionInDays)),
		LTRConfig:    toAzureSQLLTRConfigModel(config.LTRConfig),
	}}
}

// toAzureSQLLTRConfigModel returns the ltr_config block of the SDK LTR config,
// or nil if the config is nil.
func toAzureSQLLTRConfigModel(ltr *gqlsla.AzureSQLLTRConfig) []azureSQLLTRConfigModel {
	if ltr == nil {
		return nil
	}

	block := azureSQLLTRConfigModel{
		WeeklyRetention:  toAzureSQLLTRRetentionModel(ltr.WeeklyBackupRetention),
		MonthlyRetention: toAzureSQLLTRRetentionModel(ltr.MonthlyBackupRetention),
	}
	if y := ltr.YearlyBackupRetention; y != nil {
		block.YearlyRetention = []azureSQLLTRYearlyRetentionModel{{
			Retention:     types.Int64Value(int64(y.Retention.Retention)),
			RetentionUnit: types.StringValue(string(y.Retention.RetentionUnit)),
			WeekOfYear:    types.Int64Value(int64(y.WeekOfYear)),
		}}
	}

	return []azureSQLLTRConfigModel{block}
}

// toAzureSQLLTRRetentionModel returns the weekly or monthly retention block of
// the SDK LTR retention, or nil if the retention is nil.
func toAzureSQLLTRRetentionModel(r *gqlsla.AzureSQLLTRRetention) []azureSQLLTRRetentionModel {
	if r == nil {
		return nil
	}

	return []azureSQLLTRRetentionModel{{
		Retention:     types.Int64Value(int64(r.Retention)),
		RetentionUnit: types.StringValue(string(r.RetentionUnit)),
	}}
}

// azureSQLConfigHasLTRConfig reports whether an Azure SQL config block has an
// ltr_config, which makes the SLA a V1 (Azure-managed) SLA.
func azureSQLConfigHasLTRConfig(config []azureSQLConfigModel) bool {
	return len(config) > 0 && len(config[0].LTRConfig) > 0
}
