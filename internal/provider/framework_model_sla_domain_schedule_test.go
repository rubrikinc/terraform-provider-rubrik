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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// decodeModel decodes the attribute values into the model, the way the
// framework decodes a block element. It fails the test if a tfsdk tag of the
// model doesn't match an attribute, or the other way around, which compiles but
// only fails when a model is decoded.
func decodeModel(t *testing.T, attrTypes map[string]attr.Type, values map[string]attr.Value, model any) {
	t.Helper()

	obj, diags := types.ObjectValue(attrTypes, values)
	if diags.HasError() {
		t.Fatalf("invalid object: %v", diags)
	}
	if diags := obj.As(t.Context(), model, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("the model does not match the attributes: %v", diags)
	}
}

func basicScheduleValues(frequency, retention int64, unit string) map[string]attr.Value {
	return map[string]attr.Value{
		keyFrequency:     types.Int64Value(frequency),
		keyRetention:     types.Int64Value(retention),
		keyRetentionUnit: types.StringValue(unit),
	}
}

func withValues(base map[string]attr.Value, extra map[string]attr.Value) map[string]attr.Value {
	for k, v := range extra {
		base[k] = v
	}
	return base
}

// TestScheduleModelsMatchAttributes decodes every schedule model from the
// attribute names the SLA domain schema uses.
func TestScheduleModelsMatchAttributes(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }

	t.Run("daily", func(t *testing.T) {
		var m dailyScheduleModel
		decodeModel(t, scheduleAttrTypes(nil), basicScheduleValues(1, 7, "DAYS"), &m)
		if m.Frequency.ValueInt64() != 1 || m.Retention.ValueInt64() != 7 || m.RetentionUnit.ValueString() != "DAYS" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("weekly", func(t *testing.T) {
		var m weeklyScheduleModel
		decodeModel(t, scheduleAttrTypes(map[string]attr.Type{keyDayOfWeek: types.StringType}),
			withValues(basicScheduleValues(1, 4, "WEEKS"), map[string]attr.Value{keyDayOfWeek: str("MONDAY")}), &m)
		if m.DayOfWeek.ValueString() != "MONDAY" || m.Frequency.ValueInt64() != 1 {
			t.Errorf("the embedded attributes were not decoded: %+v", m)
		}
	})

	t.Run("monthly", func(t *testing.T) {
		var m monthlyScheduleModel
		decodeModel(t, scheduleAttrTypes(map[string]attr.Type{keyDayOfMonth: types.StringType}),
			withValues(basicScheduleValues(1, 12, "MONTHS"), map[string]attr.Value{keyDayOfMonth: str("LAST_DAY")}), &m)
		if m.DayOfMonth.ValueString() != "LAST_DAY" || m.Retention.ValueInt64() != 12 {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("quarterly", func(t *testing.T) {
		var m quarterlyScheduleModel
		decodeModel(t, scheduleAttrTypes(map[string]attr.Type{
			keyDayOfQuarter: types.StringType, keyQuarterStartMonth: types.StringType,
		}), withValues(basicScheduleValues(1, 1, "YEARS"), map[string]attr.Value{
			keyDayOfQuarter: str("FIRST_DAY"), keyQuarterStartMonth: str("JANUARY"),
		}), &m)
		if m.DayOfQuarter.ValueString() != "FIRST_DAY" || m.QuarterStartMonth.ValueString() != "JANUARY" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("yearly", func(t *testing.T) {
		var m yearlyScheduleModel
		decodeModel(t, scheduleAttrTypes(map[string]attr.Type{
			keyDayOfYear: types.StringType, keyYearStartMonth: types.StringType,
		}), withValues(basicScheduleValues(1, 5, "YEARS"), map[string]attr.Value{
			keyDayOfYear: str("LAST_DAY"), keyYearStartMonth: str("DECEMBER"),
		}), &m)
		if m.DayOfYear.ValueString() != "LAST_DAY" || m.YearStartMonth.ValueString() != "DECEMBER" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("retention lock", func(t *testing.T) {
		var m retentionLockModel
		decodeModel(t, map[string]attr.Type{keyMode: types.StringType, keyRetentionLockComplianceAcknowledgment: types.BoolType},
			map[string]attr.Value{keyMode: str("COMPLIANCE"), keyRetentionLockComplianceAcknowledgment: types.BoolValue(true)}, &m)
		if m.Mode.ValueString() != "COMPLIANCE" || !m.ComplianceAcknowledge.ValueBool() {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("snapshot window", func(t *testing.T) {
		var m snapshotWindowModel
		decodeModel(t, map[string]attr.Type{keyDuration: types.Int64Type, keyStartAt: types.StringType},
			map[string]attr.Value{keyDuration: types.Int64Value(2), keyStartAt: str("Mon, 09:00")}, &m)
		if m.Duration.ValueInt64() != 2 || m.StartAt.ValueString() != "Mon, 09:00" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("local retention", func(t *testing.T) {
		var m localRetentionModel
		decodeModel(t, map[string]attr.Type{keyRetention: types.Int64Type, keyRetentionUnit: types.StringType},
			map[string]attr.Value{keyRetention: types.Int64Value(7), keyRetentionUnit: str("DAYS")}, &m)
		if m.Retention.ValueInt64() != 7 || m.RetentionUnit.ValueString() != "DAYS" {
			t.Errorf("unexpected model: %+v", m)
		}
	})
}

// TestScheduleModelsRoundTrip verifies that converting each SDK schedule to a
// model and back gives the same schedule.
func TestScheduleModelsRoundTrip(t *testing.T) {
	basic := gqlsla.BasicSnapshotSchedule{Frequency: 2, Retention: 14, RetentionUnit: gqlsla.Days}

	t.Run("daily", func(t *testing.T) {
		in := &gqlsla.DailySnapshotSchedule{BasicSchedule: basic}
		if got := fromDailyScheduleModel(*toDailyScheduleModel(in)); *got != *in {
			t.Errorf("got %+v, want %+v", got, in)
		}
	})
	t.Run("hourly", func(t *testing.T) {
		in := &gqlsla.HourlySnapshotSchedule{BasicSchedule: basic}
		if got := fromHourlyScheduleModel(*toHourlyScheduleModel(in)); *got != *in {
			t.Errorf("got %+v, want %+v", got, in)
		}
	})
	t.Run("minute", func(t *testing.T) {
		in := &gqlsla.MinuteSnapshotSchedule{BasicSchedule: basic}
		if got := fromMinuteScheduleModel(*toMinuteScheduleModel(in)); *got != *in {
			t.Errorf("got %+v, want %+v", got, in)
		}
	})
	t.Run("weekly", func(t *testing.T) {
		in := &gqlsla.WeeklySnapshotSchedule{BasicSchedule: basic, DayOfWeek: gqlsla.Day("TUESDAY")}
		if got := fromWeeklyScheduleModel(*toWeeklyScheduleModel(in)); *got != *in {
			t.Errorf("got %+v, want %+v", got, in)
		}
	})
	t.Run("monthly", func(t *testing.T) {
		in := &gqlsla.MonthlySnapshotSchedule{BasicSchedule: basic, DayOfMonth: gqlsla.DayOfMonth(gqlsla.LastDay)}
		if got := fromMonthlyScheduleModel(*toMonthlyScheduleModel(in)); *got != *in {
			t.Errorf("got %+v, want %+v", got, in)
		}
	})
	t.Run("quarterly", func(t *testing.T) {
		in := &gqlsla.QuarterlySnapshotSchedule{
			BasicSchedule: basic, DayOfQuarter: gqlsla.DayOfQuarter("LAST_DAY"), QuarterStartMonth: gqlsla.Month("MARCH"),
		}
		if got := fromQuarterlyScheduleModel(*toQuarterlyScheduleModel(in)); *got != *in {
			t.Errorf("got %+v, want %+v", got, in)
		}
	})
	t.Run("yearly", func(t *testing.T) {
		in := &gqlsla.YearlySnapshotSchedule{
			BasicSchedule: basic, DayOfYear: gqlsla.DayOfYear("FIRST_DAY"), YearStartMonth: gqlsla.Month("JULY"),
		}
		if got := fromYearlyScheduleModel(*toYearlyScheduleModel(in)); *got != *in {
			t.Errorf("got %+v, want %+v", got, in)
		}
	})
}

func TestScheduleModelsNil(t *testing.T) {
	if toDailyScheduleModel(nil) != nil || toHourlyScheduleModel(nil) != nil || toMinuteScheduleModel(nil) != nil ||
		toWeeklyScheduleModel(nil) != nil || toMonthlyScheduleModel(nil) != nil ||
		toQuarterlyScheduleModel(nil) != nil || toYearlyScheduleModel(nil) != nil {
		t.Error("a nil schedule must convert to a nil model")
	}
}

// TestFromWeeklyScheduleModelDayOfWeek verifies that the day of the week is only
// set when it's configured, since it must be left out for the M365 Backup
// Storage SLAs.
func TestFromWeeklyScheduleModelDayOfWeek(t *testing.T) {
	basic := basicScheduleModel{
		Frequency: types.Int64Value(1), Retention: types.Int64Value(4), RetentionUnit: types.StringValue("WEEKS"),
	}

	tests := []struct {
		name string
		day  types.String
		want gqlsla.Day
	}{
		{name: "set", day: types.StringValue("FRIDAY"), want: "FRIDAY"},
		{name: "null", day: types.StringNull(), want: ""},
		{name: "empty", day: types.StringValue(""), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fromWeeklyScheduleModel(weeklyScheduleModel{basicScheduleModel: basic, DayOfWeek: tt.day})
			if got.DayOfWeek != tt.want {
				t.Errorf("day of week = %q, want %q", got.DayOfWeek, tt.want)
			}
		})
	}
}

func TestToRetentionLockModel(t *testing.T) {
	tests := []struct {
		name        string
		domain      gqlsla.Domain
		wantNil     bool
		wantMode    string
		wantAcknowl bool
	}{
		{name: "no lock", domain: gqlsla.Domain{RetentionLock: false}, wantNil: true},
		{
			name:     "compliance is acknowledged",
			domain:   gqlsla.Domain{RetentionLock: true, RetentionLockMode: gqlsla.Compliance},
			wantMode: string(gqlsla.Compliance), wantAcknowl: true,
		},
		{
			name:     "protection is not acknowledged",
			domain:   gqlsla.Domain{RetentionLock: true, RetentionLockMode: gqlsla.Protection},
			wantMode: string(gqlsla.Protection), wantAcknowl: false,
		},
		{
			name:     "an unknown mode is no lock",
			domain:   gqlsla.Domain{RetentionLock: true, RetentionLockMode: gqlsla.RetentionLockMode("OTHER")},
			wantMode: string(gqlsla.NoLock), wantAcknowl: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toRetentionLockModel(tt.domain)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected a retention lock, got nil")
			}
			if got.Mode.ValueString() != tt.wantMode || got.ComplianceAcknowledge.ValueBool() != tt.wantAcknowl {
				t.Errorf("got mode %q acknowledged %t, want %q %t", got.Mode.ValueString(),
					got.ComplianceAcknowledge.ValueBool(), tt.wantMode, tt.wantAcknowl)
			}
		})
	}
}
