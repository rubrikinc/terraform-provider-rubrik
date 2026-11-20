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
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// This file holds the Framework models of the snapshot schedule blocks of the
// SLA domain resource and data source, and the functions converting them to and
// from the SDK types. The functions are pure: they don't read the resource data
// or the state, so the null and unknown values the schema depends on can be
// tested in isolation.
//
// Every schedule block holds at most one element in the schema, and the
// snapshot schedule of an SLA domain holds at most one schedule of each kind.

// basicScheduleModel holds the attributes shared by all snapshot schedules.
type basicScheduleModel struct {
	Frequency     types.Int64  `tfsdk:"frequency"`
	Retention     types.Int64  `tfsdk:"retention"`
	RetentionUnit types.String `tfsdk:"retention_unit"`
}

// dailyScheduleModel is the daily_schedule block, which holds only the shared
// attributes. The hourly_schedule and minute_schedule blocks do too.
type dailyScheduleModel = basicScheduleModel

type hourlyScheduleModel = basicScheduleModel

type minuteScheduleModel = basicScheduleModel

// weeklyScheduleModel is the weekly_schedule block.
type weeklyScheduleModel struct {
	basicScheduleModel
	DayOfWeek types.String `tfsdk:"day_of_week"`
}

// monthlyScheduleModel is the monthly_schedule block.
type monthlyScheduleModel struct {
	basicScheduleModel
	DayOfMonth types.String `tfsdk:"day_of_month"`
}

// quarterlyScheduleModel is the quarterly_schedule block.
type quarterlyScheduleModel struct {
	basicScheduleModel
	DayOfQuarter      types.String `tfsdk:"day_of_quarter"`
	QuarterStartMonth types.String `tfsdk:"quarter_start_month"`
}

// yearlyScheduleModel is the yearly_schedule block.
type yearlyScheduleModel struct {
	basicScheduleModel
	DayOfYear      types.String `tfsdk:"day_of_year"`
	YearStartMonth types.String `tfsdk:"year_start_month"`
}

// retentionLockModel is the retention_lock block.
type retentionLockModel struct {
	Mode                  types.String `tfsdk:"mode"`
	ComplianceAcknowledge types.Bool   `tfsdk:"compliance_mode_acknowledgment"`
}

// scheduleFrequencies returns the frequencies of the snapshot schedule, from the
// shortest to the longest. They are the frequencies an archival spec archives
// when it has no frequencies of its own.
func scheduleFrequencies(schedule gqlsla.SnapshotSchedule) []gqlsla.RetentionUnit {
	var frequencies []gqlsla.RetentionUnit

	if schedule.Minute != nil {
		frequencies = append(frequencies, gqlsla.Minute)
	}
	if schedule.Hourly != nil {
		frequencies = append(frequencies, gqlsla.Hours)
	}
	if schedule.Daily != nil {
		frequencies = append(frequencies, gqlsla.Days)
	}
	if schedule.Weekly != nil {
		frequencies = append(frequencies, gqlsla.Weeks)
	}
	if schedule.Monthly != nil {
		frequencies = append(frequencies, gqlsla.Months)
	}
	if schedule.Quarterly != nil {
		frequencies = append(frequencies, gqlsla.Quarters)
	}
	if schedule.Yearly != nil {
		frequencies = append(frequencies, gqlsla.Years)
	}

	return frequencies
}

// scheduleAttrTypes returns the attribute types of a schedule model, given the
// attribute types of its schedule specific attributes. They are the basic
// attributes plus the extra attributes.
func scheduleAttrTypes(extra map[string]attr.Type) map[string]attr.Type {
	attrTypes := map[string]attr.Type{
		keyFrequency:     types.Int64Type,
		keyRetention:     types.Int64Type,
		keyRetentionUnit: types.StringType,
	}
	for k, v := range extra {
		attrTypes[k] = v
	}
	return attrTypes
}

// fromBasicSchedule returns the SDK basic schedule of the model.
func fromBasicSchedule(m basicScheduleModel) gqlsla.BasicSnapshotSchedule {
	return gqlsla.BasicSnapshotSchedule{
		Frequency:     int(m.Frequency.ValueInt64()),
		Retention:     int(m.Retention.ValueInt64()),
		RetentionUnit: gqlsla.RetentionUnit(m.RetentionUnit.ValueString()),
	}
}

// toBasicSchedule returns the model of the SDK basic schedule.
func toBasicSchedule(s gqlsla.BasicSnapshotSchedule) basicScheduleModel {
	return basicScheduleModel{
		Frequency:     types.Int64Value(int64(s.Frequency)),
		Retention:     types.Int64Value(int64(s.Retention)),
		RetentionUnit: types.StringValue(string(s.RetentionUnit)),
	}
}

func fromDailyScheduleModel(m dailyScheduleModel) *gqlsla.DailySnapshotSchedule {
	return &gqlsla.DailySnapshotSchedule{BasicSchedule: fromBasicSchedule(m)}
}

func toDailyScheduleModel(s *gqlsla.DailySnapshotSchedule) *dailyScheduleModel {
	if s == nil {
		return nil
	}
	m := toBasicSchedule(s.BasicSchedule)
	return &m
}

func fromHourlyScheduleModel(m hourlyScheduleModel) *gqlsla.HourlySnapshotSchedule {
	return &gqlsla.HourlySnapshotSchedule{BasicSchedule: fromBasicSchedule(m)}
}

func toHourlyScheduleModel(s *gqlsla.HourlySnapshotSchedule) *hourlyScheduleModel {
	if s == nil {
		return nil
	}
	m := toBasicSchedule(s.BasicSchedule)
	return &m
}

func fromMinuteScheduleModel(m minuteScheduleModel) *gqlsla.MinuteSnapshotSchedule {
	return &gqlsla.MinuteSnapshotSchedule{BasicSchedule: fromBasicSchedule(m)}
}

func toMinuteScheduleModel(s *gqlsla.MinuteSnapshotSchedule) *minuteScheduleModel {
	if s == nil {
		return nil
	}
	m := toBasicSchedule(s.BasicSchedule)
	return &m
}

func fromMonthlyScheduleModel(m monthlyScheduleModel) *gqlsla.MonthlySnapshotSchedule {
	return &gqlsla.MonthlySnapshotSchedule{
		BasicSchedule: fromBasicSchedule(m.basicScheduleModel),
		DayOfMonth:    gqlsla.DayOfMonth(m.DayOfMonth.ValueString()),
	}
}

func toMonthlyScheduleModel(s *gqlsla.MonthlySnapshotSchedule) *monthlyScheduleModel {
	if s == nil {
		return nil
	}
	return &monthlyScheduleModel{
		basicScheduleModel: toBasicSchedule(s.BasicSchedule),
		DayOfMonth:         types.StringValue(string(s.DayOfMonth)),
	}
}

func fromQuarterlyScheduleModel(m quarterlyScheduleModel) *gqlsla.QuarterlySnapshotSchedule {
	return &gqlsla.QuarterlySnapshotSchedule{
		BasicSchedule:     fromBasicSchedule(m.basicScheduleModel),
		DayOfQuarter:      gqlsla.DayOfQuarter(m.DayOfQuarter.ValueString()),
		QuarterStartMonth: gqlsla.Month(m.QuarterStartMonth.ValueString()),
	}
}

func toQuarterlyScheduleModel(s *gqlsla.QuarterlySnapshotSchedule) *quarterlyScheduleModel {
	if s == nil {
		return nil
	}
	return &quarterlyScheduleModel{
		basicScheduleModel: toBasicSchedule(s.BasicSchedule),
		DayOfQuarter:       types.StringValue(string(s.DayOfQuarter)),
		QuarterStartMonth:  types.StringValue(string(s.QuarterStartMonth)),
	}
}

// fromWeeklyScheduleModel returns the SDK weekly schedule of the model. The day
// of the week is only set when it's configured, since it must be left out for
// the M365 Backup Storage SLAs.
func fromWeeklyScheduleModel(m weeklyScheduleModel) *gqlsla.WeeklySnapshotSchedule {
	schedule := &gqlsla.WeeklySnapshotSchedule{BasicSchedule: fromBasicSchedule(m.basicScheduleModel)}
	if day := m.DayOfWeek.ValueString(); day != "" {
		schedule.DayOfWeek = gqlsla.Day(day)
	}
	return schedule
}

func toWeeklyScheduleModel(s *gqlsla.WeeklySnapshotSchedule) *weeklyScheduleModel {
	if s == nil {
		return nil
	}
	return &weeklyScheduleModel{
		basicScheduleModel: toBasicSchedule(s.BasicSchedule),
		DayOfWeek:          types.StringValue(string(s.DayOfWeek)),
	}
}

func fromYearlyScheduleModel(m yearlyScheduleModel) *gqlsla.YearlySnapshotSchedule {
	return &gqlsla.YearlySnapshotSchedule{
		BasicSchedule:  fromBasicSchedule(m.basicScheduleModel),
		DayOfYear:      gqlsla.DayOfYear(m.DayOfYear.ValueString()),
		YearStartMonth: gqlsla.Month(m.YearStartMonth.ValueString()),
	}
}

func toYearlyScheduleModel(s *gqlsla.YearlySnapshotSchedule) *yearlyScheduleModel {
	if s == nil {
		return nil
	}
	return &yearlyScheduleModel{
		basicScheduleModel: toBasicSchedule(s.BasicSchedule),
		DayOfYear:          types.StringValue(string(s.DayOfYear)),
		YearStartMonth:     types.StringValue(string(s.YearStartMonth)),
	}
}

// toRetentionLockModel returns the model of the retention lock of the SLA
// domain, or nil if the domain has no retention lock. The compliance
// acknowledgment is true only for the compliance mode, so the state reflects
// the requirement, and a mode other than compliance and protection is no lock.
func toRetentionLockModel(domain gqlsla.Domain) *retentionLockModel {
	if !domain.RetentionLock {
		return nil
	}

	mode := domain.RetentionLockMode
	if mode != gqlsla.Compliance && mode != gqlsla.Protection {
		mode = gqlsla.NoLock
	}

	return &retentionLockModel{
		Mode:                  types.StringValue(string(mode)),
		ComplianceAcknowledge: types.BoolValue(mode == gqlsla.Compliance),
	}
}
