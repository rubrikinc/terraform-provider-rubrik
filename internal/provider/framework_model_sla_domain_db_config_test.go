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
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// TestOptionalDurationAttributes covers the rule shared by the SAP HANA, DB2 and
// Informix configs: only a number greater than zero gives a duration, and a
// duration which isn't set gives a null number with the default unit.
func TestOptionalDurationAttributes(t *testing.T) {
	t.Run("from", func(t *testing.T) {
		tests := []struct {
			name     string
			duration types.Int64
			unit     types.String
			want     gqlsla.RetentionDuration
		}{
			{"set", types.Int64Value(3), types.StringValue("WEEKS"), gqlsla.RetentionDuration{Duration: 3, Unit: gqlsla.Weeks}},
			{"null", types.Int64Null(), types.StringValue("WEEKS"), gqlsla.RetentionDuration{}},
			{"unknown", types.Int64Unknown(), types.StringValue("WEEKS"), gqlsla.RetentionDuration{}},
			{"zero", types.Int64Value(0), types.StringValue("WEEKS"), gqlsla.RetentionDuration{}},
			{"negative", types.Int64Value(-1), types.StringValue("WEEKS"), gqlsla.RetentionDuration{}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := optionalDurationFromAttributes(tt.duration, tt.unit); got != tt.want {
					t.Errorf("got %+v, want %+v", got, tt.want)
				}
			})
		}
	})

	t.Run("to", func(t *testing.T) {
		duration, unit := optionalDurationToAttributes(gqlsla.RetentionDuration{Duration: 3, Unit: gqlsla.Weeks})
		if duration.ValueInt64() != 3 || unit.ValueString() != "WEEKS" {
			t.Errorf("got %s %s", duration, unit)
		}

		// A duration without a number has a null number, and the unit of the
		// schema default. A unit from RSC is not used.
		duration, unit = optionalDurationToAttributes(gqlsla.RetentionDuration{Duration: 0, Unit: gqlsla.Weeks})
		if !duration.IsNull() || unit.ValueString() != "DAYS" {
			t.Errorf("got %s %s, want null and DAYS", duration, unit)
		}
	})
}

// TestOracleConfigModel ports the SDKv2 tests of the retain archive logs
// indefinitely sentinel.
func TestOracleConfigModel(t *testing.T) {
	base := oracleConfigModel{
		Frequency: types.Int64Value(1), FrequencyUnit: types.StringValue("DAYS"),
		LogRetention: types.Int64Value(7), LogRetentionUnit: types.StringValue("DAYS"),
		HostLogRetention: types.Int64Null(), HostLogRetentionUnit: types.StringValue("DAYS"),
		RetainArchiveLogsIndefinitely: types.BoolNull(),
	}

	t.Run("not configured", func(t *testing.T) {
		if fromOracleConfigModel(nil) != nil || toOracleConfigModel(nil) != nil {
			t.Fatal("nil must convert to nil")
		}
	})

	t.Run("retain archive logs indefinitely is the sentinel", func(t *testing.T) {
		m := base
		m.RetainArchiveLogsIndefinitely = types.BoolValue(true)
		got := fromOracleConfigModel([]oracleConfigModel{m})
		if got == nil {
			t.Fatal("expected a config")
		}
		if got.HostLogRetention.Duration != -2 || got.HostLogRetention.Unit != gqlsla.Minute {
			t.Errorf("host log retention = %+v, want -2 MINUTE", got.HostLogRetention)
		}
	})

	t.Run("the sentinel replaces a host log retention", func(t *testing.T) {
		m := base
		m.HostLogRetention = types.Int64Value(30)
		m.RetainArchiveLogsIndefinitely = types.BoolValue(true)
		got := fromOracleConfigModel([]oracleConfigModel{m})
		if got.HostLogRetention != oracleRetainArchiveLogsIndefinitely {
			t.Errorf("host log retention = %+v, want the sentinel", got.HostLogRetention)
		}
	})

	t.Run("a host log retention", func(t *testing.T) {
		m := base
		m.HostLogRetention = types.Int64Value(30)
		m.HostLogRetentionUnit = types.StringValue("WEEKS")
		got := fromOracleConfigModel([]oracleConfigModel{m})
		if want := (gqlsla.RetentionDuration{Duration: 30, Unit: gqlsla.Weeks}); got.HostLogRetention != want {
			t.Errorf("host log retention = %+v, want %+v", got.HostLogRetention, want)
		}
	})

	t.Run("no host log retention", func(t *testing.T) {
		got := fromOracleConfigModel([]oracleConfigModel{base})
		if got.HostLogRetention != (gqlsla.RetentionDuration{}) {
			t.Errorf("host log retention = %+v, want none", got.HostLogRetention)
		}
		if got.Frequency.Duration != 1 || got.LogRetention.Duration != 7 {
			t.Errorf("unexpected config: %+v", got)
		}
	})

	t.Run("to reads the sentinel as retain archive logs indefinitely", func(t *testing.T) {
		got := toOracleConfigModel(&gqlsla.OracleConfig{HostLogRetention: gqlsla.RetentionDuration{Duration: -2, Unit: gqlsla.Minute}})
		if len(got) != 1 || !got[0].RetainArchiveLogsIndefinitely.ValueBool() {
			t.Fatalf("got %+v, want retain_archive_logs_indefinitely", got)
		}
		// The host log retention is absent, with the default unit.
		if !got[0].HostLogRetention.IsNull() || got[0].HostLogRetentionUnit.ValueString() != "DAYS" {
			t.Errorf("host log retention = %s %s, want null and DAYS", got[0].HostLogRetention, got[0].HostLogRetentionUnit)
		}
	})

	t.Run("to reads a host log retention", func(t *testing.T) {
		got := toOracleConfigModel(&gqlsla.OracleConfig{HostLogRetention: gqlsla.RetentionDuration{Duration: 30, Unit: gqlsla.Weeks}})
		if got[0].RetainArchiveLogsIndefinitely.ValueBool() || got[0].RetainArchiveLogsIndefinitely.IsNull() {
			t.Errorf("retain_archive_logs_indefinitely must be false, and not null")
		}
		if got[0].HostLogRetention.ValueInt64() != 30 || got[0].HostLogRetentionUnit.ValueString() != "WEEKS" {
			t.Errorf("host log retention = %s %s", got[0].HostLogRetention, got[0].HostLogRetentionUnit)
		}
	})

	t.Run("to without a host log retention", func(t *testing.T) {
		got := toOracleConfigModel(&gqlsla.OracleConfig{})
		if !got[0].HostLogRetention.IsNull() || got[0].HostLogRetentionUnit.ValueString() != "DAYS" ||
			got[0].RetainArchiveLogsIndefinitely.ValueBool() {
			t.Errorf("unexpected model: %+v", got[0])
		}
	})
}

func TestSapHanaConfigModel(t *testing.T) {
	if fromSapHanaConfigModel(nil) != nil || toSapHanaConfigModel(nil) != nil {
		t.Fatal("nil must convert to nil")
	}

	t.Run("nothing set is the default unit and null numbers", func(t *testing.T) {
		got := toSapHanaConfigModel(&gqlsla.SapHanaConfig{})
		m := got[0]
		if !m.IncrementalFrequency.IsNull() || !m.LogRetention.IsNull() || !m.DifferentialFrequency.IsNull() {
			t.Errorf("the numbers must be null, got %+v", m)
		}
		for _, unit := range []types.String{m.IncrementalFrequencyUnit, m.LogRetentionUnit, m.DifferentialFrequencyUnit} {
			if unit.ValueString() != "DAYS" {
				t.Errorf("the unit must be DAYS, got %s", unit)
			}
		}
		if m.StorageSnapshotConfig != nil {
			t.Errorf("no storage snapshot config expected, got %+v", m.StorageSnapshotConfig)
		}
	})

	t.Run("all set", func(t *testing.T) {
		sdk := &gqlsla.SapHanaConfig{
			IncrementalFrequency:  gqlsla.RetentionDuration{Duration: 1, Unit: gqlsla.Days},
			LogRetention:          gqlsla.RetentionDuration{Duration: 2, Unit: gqlsla.Weeks},
			DifferentialFrequency: gqlsla.RetentionDuration{Duration: 3, Unit: gqlsla.Months},
			StorageSnapshotConfig: &gqlsla.SapHanaStorageSnapshotConfig{
				Frequency: gqlsla.RetentionDuration{Duration: 4, Unit: gqlsla.Days},
				Retention: gqlsla.RetentionDuration{Duration: 5, Unit: gqlsla.Years},
			},
		}
		got := toSapHanaConfigModel(sdk)
		if back := fromSapHanaConfigModel(got); !reflect.DeepEqual(back, sdk) {
			t.Errorf("round trip: got %+v, want %+v", back, sdk)
		}
	})

	t.Run("an empty storage snapshot config is dropped", func(t *testing.T) {
		got := toSapHanaConfigModel(&gqlsla.SapHanaConfig{StorageSnapshotConfig: &gqlsla.SapHanaStorageSnapshotConfig{}})
		if got[0].StorageSnapshotConfig != nil {
			t.Errorf("an empty storage snapshot config must be dropped, got %+v", got[0].StorageSnapshotConfig)
		}
	})

	t.Run("a storage snapshot config with only a retention is kept", func(t *testing.T) {
		got := toSapHanaConfigModel(&gqlsla.SapHanaConfig{StorageSnapshotConfig: &gqlsla.SapHanaStorageSnapshotConfig{
			Retention: gqlsla.RetentionDuration{Duration: 5, Unit: gqlsla.Years},
		}})
		if len(got[0].StorageSnapshotConfig) != 1 || got[0].StorageSnapshotConfig[0].Retention.ValueInt64() != 5 {
			t.Errorf("got %+v", got[0].StorageSnapshotConfig)
		}
	})

	t.Run("unset durations in the block give no duration", func(t *testing.T) {
		got := fromSapHanaConfigModel([]sapHanaConfigModel{{
			IncrementalFrequency: types.Int64Null(), IncrementalFrequencyUnit: types.StringValue("WEEKS"),
			LogRetention: types.Int64Null(), LogRetentionUnit: types.StringValue("DAYS"),
			DifferentialFrequency: types.Int64Null(), DifferentialFrequencyUnit: types.StringValue("DAYS"),
		}})
		if !reflect.DeepEqual(got, &gqlsla.SapHanaConfig{}) {
			t.Errorf("got %+v, want an empty config", got)
		}
	})
}

func TestDB2ConfigModel(t *testing.T) {
	if fromDB2ConfigModel(nil) != nil || toDB2ConfigModel(nil) != nil {
		t.Fatal("nil must convert to nil")
	}

	t.Run("nothing set", func(t *testing.T) {
		got := toDB2ConfigModel(&gqlsla.DB2Config{})
		m := got[0]
		if !m.IncrementalFrequency.IsNull() || !m.LogRetention.IsNull() || !m.DifferentialFrequency.IsNull() ||
			!m.LogArchivalMethod.IsNull() {
			t.Errorf("the numbers and the log archival method must be null, got %+v", m)
		}
		if m.IncrementalFrequencyUnit.ValueString() != "DAYS" || m.LogRetentionUnit.ValueString() != "DAYS" ||
			m.DifferentialFrequencyUnit.ValueString() != "DAYS" {
			t.Errorf("the units must be DAYS, got %+v", m)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		sdk := &gqlsla.DB2Config{
			IncrementalFrequency:  gqlsla.RetentionDuration{Duration: 1, Unit: gqlsla.Days},
			LogRetention:          gqlsla.RetentionDuration{Duration: 2, Unit: gqlsla.Weeks},
			DifferentialFrequency: gqlsla.RetentionDuration{Duration: 3, Unit: gqlsla.Months},
			LogArchivalMethod:     gqlsla.Db2LogArchivalMethod2,
		}
		if back := fromDB2ConfigModel(toDB2ConfigModel(sdk)); !reflect.DeepEqual(back, sdk) {
			t.Errorf("got %+v, want %+v", back, sdk)
		}
	})

	t.Run("the log archival method is only set when it isn't empty", func(t *testing.T) {
		for _, method := range []types.String{types.StringNull(), types.StringValue(""), types.StringUnknown()} {
			got := fromDB2ConfigModel([]db2ConfigModel{{LogArchivalMethod: method}})
			if got.LogArchivalMethod != "" {
				t.Errorf("method %s: got %q, want none", method, got.LogArchivalMethod)
			}
		}
		got := fromDB2ConfigModel([]db2ConfigModel{{LogArchivalMethod: types.StringValue("LOGARCHMETH1")}})
		if got.LogArchivalMethod != gqlsla.Db2LogArchivalMethod1 {
			t.Errorf("got %q", got.LogArchivalMethod)
		}
	})
}

func TestInformixConfigModel(t *testing.T) {
	if fromInformixConfigModel(nil) != nil || toInformixConfigModel(nil) != nil {
		t.Fatal("nil must convert to nil")
	}

	// All four durations have a different number and unit, so a swap is seen.
	sdk := &gqlsla.InformixSlaConfig{
		IncrementalFrequency: gqlsla.RetentionDuration{Duration: 1, Unit: gqlsla.Days},
		IncrementalRetention: gqlsla.RetentionDuration{Duration: 2, Unit: gqlsla.Weeks},
		LogFrequency:         gqlsla.RetentionDuration{Duration: 3, Unit: gqlsla.Months},
		LogRetention:         gqlsla.RetentionDuration{Duration: 4, Unit: gqlsla.Years},
	}

	t.Run("round trip", func(t *testing.T) {
		if back := fromInformixConfigModel(toInformixConfigModel(sdk)); !reflect.DeepEqual(back, sdk) {
			t.Errorf("got %+v, want %+v", back, sdk)
		}
	})

	// The log frequency and the log retention are named frequency and
	// retention in the block.
	t.Run("log frequency and retention names", func(t *testing.T) {
		m := toInformixConfigModel(sdk)[0]
		if m.Frequency.ValueInt64() != 3 || m.FrequencyUnit.ValueString() != "MONTHS" ||
			m.Retention.ValueInt64() != 4 || m.RetentionUnit.ValueString() != "YEARS" ||
			m.IncrementalFrequency.ValueInt64() != 1 || m.IncrementalRetention.ValueInt64() != 2 {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("nothing set", func(t *testing.T) {
		m := toInformixConfigModel(&gqlsla.InformixSlaConfig{})[0]
		if !m.IncrementalFrequency.IsNull() || !m.IncrementalRetention.IsNull() || !m.Frequency.IsNull() || !m.Retention.IsNull() {
			t.Errorf("the numbers must be null, got %+v", m)
		}
		for _, unit := range []types.String{m.IncrementalFrequencyUnit, m.IncrementalRetentionUnit, m.FrequencyUnit, m.RetentionUnit} {
			if unit.ValueString() != "DAYS" {
				t.Errorf("the unit must be DAYS, got %s", unit)
			}
		}
	})
}

func TestNCDConfigModel(t *testing.T) {
	if fromNCDConfigModel(nil) != nil || toNCDConfigModel(nil) != nil {
		t.Fatal("nil must convert to nil")
	}

	a, b, c := uuid.New(), uuid.New(), uuid.New()

	t.Run("each schedule is kept apart", func(t *testing.T) {
		got := fromNCDConfigModel([]ncdConfigModel{{
			MinutelyBackupLocations:  []string{a.String()},
			HourlyBackupLocations:    []string{b.String()},
			DailyBackupLocations:     []string{c.String()},
			WeeklyBackupLocations:    []string{a.String(), b.String()},
			MonthlyBackupLocations:   []string{b.String()},
			QuarterlyBackupLocations: []string{c.String()},
			YearlyBackupLocations:    []string{a.String()},
		}})
		want := &gqlsla.NcdSlaConfig{
			MinutelyBackupLocations: []uuid.UUID{a}, HourlyBackupLocations: []uuid.UUID{b},
			DailyBackupLocations: []uuid.UUID{c}, WeeklyBackupLocations: []uuid.UUID{a, b},
			MonthlyBackupLocations: []uuid.UUID{b}, QuarterlyBackupLocations: []uuid.UUID{c},
			YearlyBackupLocations: []uuid.UUID{a},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		if back := toNCDConfigModel(got); !slices.Equal(back[0].WeeklyBackupLocations, []string{a.String(), b.String()}) ||
			!slices.Equal(back[0].YearlyBackupLocations, []string{a.String()}) {
			t.Errorf("to: got %+v", back[0])
		}
	})

	// An ID which isn't a UUID is skipped, and the other IDs are kept. This
	// differs from the backup locations, where one bad ID drops them all.
	t.Run("an invalid ID is skipped and the others are kept", func(t *testing.T) {
		got := fromNCDConfigModel([]ncdConfigModel{{
			DailyBackupLocations: []string{a.String(), "not-a-uuid", b.String()},
		}})
		if want := []uuid.UUID{a, b}; !slices.Equal(got.DailyBackupLocations, want) {
			t.Errorf("got %v, want %v", got.DailyBackupLocations, want)
		}
	})

	t.Run("only invalid IDs give none", func(t *testing.T) {
		got := fromNCDConfigModel([]ncdConfigModel{{DailyBackupLocations: []string{"nope"}}})
		if got.DailyBackupLocations != nil {
			t.Errorf("got %v, want nil", got.DailyBackupLocations)
		}
	})

	t.Run("schedules without locations are null", func(t *testing.T) {
		got := toNCDConfigModel(&gqlsla.NcdSlaConfig{DailyBackupLocations: []uuid.UUID{a}})[0]
		if got.MinutelyBackupLocations != nil || got.YearlyBackupLocations != nil {
			t.Errorf("the schedules without locations must be null, got %+v", got)
		}
		if !slices.Equal(got.DailyBackupLocations, []string{a.String()}) {
			t.Errorf("daily = %v", got.DailyBackupLocations)
		}
	})

	t.Run("an empty config gives an empty block", func(t *testing.T) {
		got := toNCDConfigModel(&gqlsla.NcdSlaConfig{})
		if len(got) != 1 {
			t.Fatalf("expected one block, got %+v", got)
		}
	})
}

// TestDBConfigModelsMatchAttributes decodes the models from the attribute names
// the SLA domain schema uses.
func TestDBConfigModelsMatchAttributes(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }
	i := func(n int64) attr.Value { return types.Int64Value(n) }
	listOf := func(vals ...string) attr.Value {
		elems := make([]attr.Value, 0, len(vals))
		for _, v := range vals {
			elems = append(elems, str(v))
		}
		return types.ListValueMust(types.StringType, elems)
	}

	t.Run("oracle", func(t *testing.T) {
		var m oracleConfigModel
		decodeModel(t, map[string]attr.Type{
			keyFrequency: types.Int64Type, keyFrequencyUnit: types.StringType,
			keyLogRetention: types.Int64Type, keyLogRetentionUnit: types.StringType,
			keyHostLogRetention: types.Int64Type, keyHostLogRetentionUnit: types.StringType,
			keyRetainArchiveLogsIndefinitely: types.BoolType,
		}, map[string]attr.Value{
			keyFrequency: i(1), keyFrequencyUnit: str("DAYS"), keyLogRetention: i(2), keyLogRetentionUnit: str("DAYS"),
			keyHostLogRetention: i(3), keyHostLogRetentionUnit: str("DAYS"), keyRetainArchiveLogsIndefinitely: types.BoolValue(false),
		}, &m)
		if m.HostLogRetention.ValueInt64() != 3 {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("sap hana", func(t *testing.T) {
		snapType := types.ObjectType{AttrTypes: map[string]attr.Type{
			keyFrequency: types.Int64Type, keyFrequencyUnit: types.StringType,
			keyRetention: types.Int64Type, keyRetentionUnit: types.StringType,
		}}
		var m sapHanaConfigModel
		decodeModel(t, map[string]attr.Type{
			keyIncrementalFrequency: types.Int64Type, keyIncrementalFrequencyUnit: types.StringType,
			keyLogRetention: types.Int64Type, keyLogRetentionUnit: types.StringType,
			keyDifferentialFrequency: types.Int64Type, keyDifferentialFrequencyUnit: types.StringType,
			keyStorageSnapshotConfig: types.ListType{ElemType: snapType},
		}, map[string]attr.Value{
			keyIncrementalFrequency: i(1), keyIncrementalFrequencyUnit: str("DAYS"),
			keyLogRetention: i(2), keyLogRetentionUnit: str("DAYS"),
			keyDifferentialFrequency: i(3), keyDifferentialFrequencyUnit: str("DAYS"),
			keyStorageSnapshotConfig: types.ListValueMust(snapType, nil),
		}, &m)
		if m.DifferentialFrequency.ValueInt64() != 3 {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("db2", func(t *testing.T) {
		var m db2ConfigModel
		decodeModel(t, map[string]attr.Type{
			keyIncrementalFrequency: types.Int64Type, keyIncrementalFrequencyUnit: types.StringType,
			keyLogRetention: types.Int64Type, keyLogRetentionUnit: types.StringType,
			keyDifferentialFrequency: types.Int64Type, keyDifferentialFrequencyUnit: types.StringType,
			keyLogArchivalMethod: types.StringType,
		}, map[string]attr.Value{
			keyIncrementalFrequency: i(1), keyIncrementalFrequencyUnit: str("DAYS"),
			keyLogRetention: i(2), keyLogRetentionUnit: str("DAYS"),
			keyDifferentialFrequency: i(3), keyDifferentialFrequencyUnit: str("DAYS"),
			keyLogArchivalMethod: str("LOGARCHMETH1"),
		}, &m)
		if m.LogArchivalMethod.ValueString() != "LOGARCHMETH1" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("informix", func(t *testing.T) {
		var m informixConfigModel
		decodeModel(t, map[string]attr.Type{
			keyIncrementalFrequency: types.Int64Type, keyIncrementalFrequencyUnit: types.StringType,
			keyIncrementalRetention: types.Int64Type, keyIncrementalRetentionUnit: types.StringType,
			keyFrequency: types.Int64Type, keyFrequencyUnit: types.StringType,
			keyRetention: types.Int64Type, keyRetentionUnit: types.StringType,
		}, map[string]attr.Value{
			keyIncrementalFrequency: i(1), keyIncrementalFrequencyUnit: str("DAYS"),
			keyIncrementalRetention: i(2), keyIncrementalRetentionUnit: str("DAYS"),
			keyFrequency: i(3), keyFrequencyUnit: str("DAYS"), keyRetention: i(4), keyRetentionUnit: str("DAYS"),
		}, &m)
		if m.Retention.ValueInt64() != 4 {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("ncd", func(t *testing.T) {
		listType := types.ListType{ElemType: types.StringType}
		var m ncdConfigModel
		decodeModel(t, map[string]attr.Type{
			keyMinutelyBackupLocations: listType, keyHourlyBackupLocations: listType, keyDailyBackupLocations: listType,
			keyWeeklyBackupLocations: listType, keyMonthlyBackupLocations: listType,
			keyQuarterlyBackupLocations: listType, keyYearlyBackupLocations: listType,
		}, map[string]attr.Value{
			keyMinutelyBackupLocations: listOf("a"), keyHourlyBackupLocations: listOf(), keyDailyBackupLocations: listOf(),
			keyWeeklyBackupLocations: listOf(), keyMonthlyBackupLocations: listOf(),
			keyQuarterlyBackupLocations: listOf(), keyYearlyBackupLocations: listOf("y"),
		}, &m)
		if !slices.Equal(m.MinutelyBackupLocations, []string{"a"}) || !slices.Equal(m.YearlyBackupLocations, []string{"y"}) {
			t.Errorf("unexpected model: %+v", m)
		}
	})
}
