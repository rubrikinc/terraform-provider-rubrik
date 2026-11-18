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
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// v1AzureSQLConfig is a V1 (Azure-managed) Azure SQL config with every part of
// the long-term retention set.
func v1AzureSQLConfig() *gqlsla.AzureDBConfig {
	return &gqlsla.AzureDBConfig{
		LogRetentionInDays: 14,
		LTRConfig: &gqlsla.AzureSQLLTRConfig{
			WeeklyBackupRetention:  &gqlsla.AzureSQLLTRRetention{Retention: 4, RetentionUnit: gqlsla.Weeks},
			MonthlyBackupRetention: &gqlsla.AzureSQLLTRRetention{Retention: 12, RetentionUnit: gqlsla.Months},
			YearlyBackupRetention: &gqlsla.AzureSQLYearlyLTRRetention{
				Retention:  gqlsla.AzureSQLLTRRetention{Retention: 5, RetentionUnit: gqlsla.Years},
				WeekOfYear: 20,
			},
		},
	}
}

func TestToAzureSQLConfigModel(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		if got := toAzureSQLConfigModel(nil); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	// The cases of the SDKv2 TestToAzureSQLConfig.
	t.Run("with log retention", func(t *testing.T) {
		got := toAzureSQLConfigModel(&gqlsla.AzureDBConfig{LogRetentionInDays: 14})
		if len(got) != 1 || got[0].LogRetention.ValueInt64() != 14 {
			t.Fatalf("got %+v, want a block with a log retention of 14", got)
		}
		// A V2 config has no ltr_config.
		if got[0].LTRConfig != nil {
			t.Errorf("a V2 config must have no ltr_config, got %+v", got[0].LTRConfig)
		}
	})
	t.Run("with zero log retention", func(t *testing.T) {
		got := toAzureSQLConfigModel(&gqlsla.AzureDBConfig{LogRetentionInDays: 0})
		if len(got) != 1 || got[0].LogRetention.IsNull() || got[0].LogRetention.ValueInt64() != 0 {
			t.Fatalf("got %+v, want a log retention of 0 which is not null", got)
		}
	})

	t.Run("V1 with the whole long term retention", func(t *testing.T) {
		got := toAzureSQLConfigModel(v1AzureSQLConfig())
		ltr := got[0].LTRConfig
		if len(ltr) != 1 {
			t.Fatalf("expected an ltr_config, got %+v", ltr)
		}
		if w := ltr[0].WeeklyRetention; len(w) != 1 || w[0].Retention.ValueInt64() != 4 || w[0].RetentionUnit.ValueString() != "WEEKS" {
			t.Errorf("weekly retention: %+v", w)
		}
		if m := ltr[0].MonthlyRetention; len(m) != 1 || m[0].Retention.ValueInt64() != 12 || m[0].RetentionUnit.ValueString() != "MONTHS" {
			t.Errorf("monthly retention: %+v", m)
		}
		if y := ltr[0].YearlyRetention; len(y) != 1 || y[0].Retention.ValueInt64() != 5 ||
			y[0].RetentionUnit.ValueString() != "YEARS" || y[0].WeekOfYear.ValueInt64() != 20 {
			t.Errorf("yearly retention: %+v", y)
		}
	})

	t.Run("V1 with only some of the long term retention", func(t *testing.T) {
		got := toAzureSQLConfigModel(&gqlsla.AzureDBConfig{
			LTRConfig: &gqlsla.AzureSQLLTRConfig{
				WeeklyBackupRetention: &gqlsla.AzureSQLLTRRetention{Retention: 7, RetentionUnit: gqlsla.Days},
			},
		})
		ltr := got[0].LTRConfig
		if len(ltr) != 1 || len(ltr[0].WeeklyRetention) != 1 || ltr[0].MonthlyRetention != nil || ltr[0].YearlyRetention != nil {
			t.Errorf("only the weekly retention must be set, got %+v", ltr)
		}
	})

	t.Run("V1 with an empty ltr_config is still V1", func(t *testing.T) {
		got := toAzureSQLConfigModel(&gqlsla.AzureDBConfig{LTRConfig: &gqlsla.AzureSQLLTRConfig{}})
		if len(got[0].LTRConfig) != 1 {
			t.Fatalf("an empty LTR config must still give an ltr_config block, got %+v", got[0].LTRConfig)
		}
		if !azureSQLConfigHasLTRConfig(got) {
			t.Error("an empty ltr_config still marks the SLA as V1")
		}
	})
}

func TestFromAzureSQLConfigModel(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		if got := fromAzureSQLConfigModel(nil); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("V2 has no LTR config", func(t *testing.T) {
		got := fromAzureSQLConfigModel([]azureSQLConfigModel{{LogRetention: types.Int64Value(7)}})
		if got == nil || got.LogRetentionInDays != 7 {
			t.Fatalf("got %+v, want a log retention of 7", got)
		}
		if got.LTRConfig != nil {
			t.Errorf("a block without ltr_config must give no LTR config, which marks the SLA as V2, got %+v", got.LTRConfig)
		}
	})

	t.Run("an empty ltr_config gives an empty LTR config, which is V1", func(t *testing.T) {
		got := fromAzureSQLConfigModel([]azureSQLConfigModel{{
			LogRetention: types.Int64Value(7), LTRConfig: []azureSQLLTRConfigModel{{}},
		}})
		if got == nil || got.LTRConfig == nil {
			t.Fatalf("an ltr_config block must give an LTR config, got %+v", got)
		}
		if got.LTRConfig.WeeklyBackupRetention != nil || got.LTRConfig.MonthlyBackupRetention != nil ||
			got.LTRConfig.YearlyBackupRetention != nil {
			t.Errorf("no retention was set, got %+v", got.LTRConfig)
		}
	})
}

// TestAzureSQLConfigModelRoundTrip verifies that converting an SDK config to a
// block and back gives the same config, for V1 and V2.
func TestAzureSQLConfigModelRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		config *gqlsla.AzureDBConfig
	}{
		{name: "V2", config: &gqlsla.AzureDBConfig{LogRetentionInDays: 14}},
		{name: "V1 full", config: v1AzureSQLConfig()},
		{name: "V1 empty", config: &gqlsla.AzureDBConfig{LTRConfig: &gqlsla.AzureSQLLTRConfig{}}},
		{
			name: "V1 monthly only",
			config: &gqlsla.AzureDBConfig{
				LogRetentionInDays: 3,
				LTRConfig: &gqlsla.AzureSQLLTRConfig{
					MonthlyBackupRetention: &gqlsla.AzureSQLLTRRetention{Retention: 6, RetentionUnit: gqlsla.Months},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fromAzureSQLConfigModel(toAzureSQLConfigModel(tt.config))
			if !reflect.DeepEqual(got, tt.config) {
				t.Errorf("got %+v, want %+v", got, tt.config)
			}
		})
	}
}

// TestAzureSQLConfigHasLTRConfig is the equivalent of the SDKv2
// TestConfigHasLTRConfig.
func TestAzureSQLConfigHasLTRConfig(t *testing.T) {
	tests := []struct {
		name   string
		config []azureSQLConfigModel
		want   bool
	}{
		{"V1 with ltr_config", []azureSQLConfigModel{{LogRetention: types.Int64Value(7), LTRConfig: []azureSQLLTRConfigModel{{}}}}, true},
		{"V2 without ltr_config", []azureSQLConfigModel{{LogRetention: types.Int64Value(7)}}, false},
		{"V2 with an empty ltr_config list", []azureSQLConfigModel{{LTRConfig: []azureSQLLTRConfigModel{}}}, false},
		{"empty block", []azureSQLConfigModel{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := azureSQLConfigHasLTRConfig(tt.config); got != tt.want {
				t.Fatalf("azureSQLConfigHasLTRConfig(%+v) = %v, want %v", tt.config, got, tt.want)
			}
		})
	}
}

// TestAzureSQLModelsMatchAttributes decodes the models from the attribute names
// the SLA domain schema uses.
func TestAzureSQLModelsMatchAttributes(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }

	retentionType := types.ObjectType{AttrTypes: map[string]attr.Type{
		keyRetention: types.Int64Type, keyRetentionUnit: types.StringType,
	}}
	yearlyType := types.ObjectType{AttrTypes: map[string]attr.Type{
		keyRetention: types.Int64Type, keyRetentionUnit: types.StringType, keyWeekOfYear: types.Int64Type,
	}}
	ltrType := types.ObjectType{AttrTypes: map[string]attr.Type{
		keyWeeklyRetention:  types.ListType{ElemType: retentionType},
		keyMonthlyRetention: types.ListType{ElemType: retentionType},
		keyYearlyRetention:  types.ListType{ElemType: yearlyType},
	}}

	t.Run("retention", func(t *testing.T) {
		var m azureSQLLTRRetentionModel
		decodeModel(t, map[string]attr.Type{keyRetention: types.Int64Type, keyRetentionUnit: types.StringType},
			map[string]attr.Value{keyRetention: types.Int64Value(4), keyRetentionUnit: str("WEEKS")}, &m)
		if m.Retention.ValueInt64() != 4 || m.RetentionUnit.ValueString() != "WEEKS" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("yearly retention", func(t *testing.T) {
		var m azureSQLLTRYearlyRetentionModel
		decodeModel(t, yearlyType.AttrTypes, map[string]attr.Value{
			keyRetention: types.Int64Value(5), keyRetentionUnit: str("YEARS"), keyWeekOfYear: types.Int64Value(20),
		}, &m)
		if m.WeekOfYear.ValueInt64() != 20 {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("ltr config", func(t *testing.T) {
		var m azureSQLLTRConfigModel
		decodeModel(t, ltrType.AttrTypes, map[string]attr.Value{
			keyWeeklyRetention: types.ListValueMust(retentionType, []attr.Value{types.ObjectValueMust(
				retentionType.AttrTypes, map[string]attr.Value{keyRetention: types.Int64Value(4), keyRetentionUnit: str("WEEKS")})}),
			keyMonthlyRetention: types.ListValueMust(retentionType, nil),
			keyYearlyRetention:  types.ListValueMust(yearlyType, nil),
		}, &m)
		if len(m.WeeklyRetention) != 1 || m.WeeklyRetention[0].Retention.ValueInt64() != 4 {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("azure sql config", func(t *testing.T) {
		var m azureSQLConfigModel
		decodeModel(t, map[string]attr.Type{
			keyLogRetention: types.Int64Type, keyLTRConfig: types.ListType{ElemType: ltrType},
		}, map[string]attr.Value{
			keyLogRetention: types.Int64Value(14), keyLTRConfig: types.ListValueMust(ltrType, nil),
		}, &m)
		if m.LogRetention.ValueInt64() != 14 {
			t.Errorf("unexpected model: %+v", m)
		}
	})
}
