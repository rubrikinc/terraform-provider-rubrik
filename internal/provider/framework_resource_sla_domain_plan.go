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
	"errors"
	"math/big"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// errSLADomainBackupServiceChange is returned when the backup service of an
// existing Azure SQL SLA domain is changed.
var errSLADomainBackupServiceChange = errors.New("cannot change the backup service of an existing Azure SQL " +
	"SLA Domain between Azure-managed (V1, with ltr_config) and Rubrik-managed (V2, without ltr_config); create " +
	"a new SLA Domain to use a different backup service")

// ModifyPlan checks the rules which depend on more than one attribute, and which
// SDKv2 checked in CustomizeDiff, so that they fail when planning and not part
// way through an apply. They are not attribute validators, since those only
// see their own attribute, and they can't use the model, since it can't hold
// the values which are unknown while planning. The values are read from the
// plan by path instead, and a rule which depends on a value which isn't known
// yet is left to the plan which is made when applying.
func (r *slaDomainResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	tflog.Trace(ctx, "slaDomainResource.ModifyPlan")

	// Nothing is planned when the SLA domain is destroyed.
	if req.Plan.Raw.IsNull() {
		return
	}

	if err := validateSLADomainPlan(req.Plan.Raw, req.State.Raw); err != nil {
		res.Diagnostics.AddError("Invalid SLA domain", err.Error())
	}
}

// validateSLADomainPlan applies the plan time rules to the plan. The state is
// null when the SLA domain is created.
func validateSLADomainPlan(plan, state tftypes.Value) error {
	planned, ok := slaDomainPlanAttributes(plan)
	if !ok {
		return nil
	}

	objectTypes, known := slaDomainPlannedObjectTypes(planned[keyObjectTypes])
	if !known {
		return nil
	}

	if err := validateAzurePostgresFlexibleServerConfig(blockCount(planned[keyAzurePostgresFlexibleServerConfig]) > 0, objectTypes); err != nil {
		return err
	}

	if slices.Contains(objectTypes, gqlsla.ObjectAWSConfig) {
		schedule := slaDomainPlannedSchedule(planned, false)
		if err := validateAWSConfigObjectType(objectTypes, schedule, blockCount(planned[keyBackupLocation])); err != nil {
			return err
		}
	}

	// The number of backup_location blocks is not known while planning when the
	// blocks are generated from values which are only known after the apply. The
	// rule is then checked when the plan is made again during the apply.
	if slices.Contains(objectTypes, gqlsla.ObjectGCPBigQuery) && planned[keyBackupLocation].IsKnown() {
		schedule := slaDomainPlannedSchedule(planned, true)
		err := validateGCPBigQueryObjectType(objectTypes, schedule, blockCount(planned[keyBackupLocation]),
			blockCount(planned[keyArchival]), blockCount(planned[keyReplicationSpec]))
		if err != nil {
			return err
		}
	}

	// Changing the backup service of an existing SLA domain is rejected. Creating
	// a new SLA domain with any backup service is allowed, and has no state.
	if prior, ok := slaDomainPlanAttributes(state); ok {
		for _, key := range []string{keyAzureSQLDatabaseConfig, keyAzureSQLManagedInstanceConfig} {
			// A config block which is added or removed is not a change of the
			// backup service of the block.
			if blockCount(prior[key]) == 0 || blockCount(planned[key]) == 0 {
				continue
			}
			if hasLTRConfig(prior[key]) != hasLTRConfig(planned[key]) {
				return errSLADomainBackupServiceChange
			}
		}
	}

	return nil
}

// slaDomainPlanAttributes returns the attributes of the plan or state.
func slaDomainPlanAttributes(value tftypes.Value) (map[string]tftypes.Value, bool) {
	var attrs map[string]tftypes.Value
	if !value.IsKnown() || value.IsNull() || value.As(&attrs) != nil {
		return nil, false
	}
	return attrs, true
}

// slaDomainPlannedObjectTypes returns the planned object types. It reports false
// when the set or one of its elements isn't known, since the rules which depend
// on the object types can't be decided then.
func slaDomainPlannedObjectTypes(value tftypes.Value) ([]gqlsla.ObjectType, bool) {
	if !value.IsKnown() || value.IsNull() {
		return nil, false
	}
	var elements []tftypes.Value
	if value.As(&elements) != nil {
		return nil, false
	}
	var objectTypes []gqlsla.ObjectType
	for _, element := range elements {
		var s string
		if !element.IsKnown() || element.IsNull() || element.As(&s) != nil {
			return nil, false
		}
		objectTypes = append(objectTypes, gqlsla.ObjectType(s))
	}
	return objectTypes, true
}

// blockCount returns the number of elements of a block, which is 0 when the block
// isn't configured, or isn't known.
func blockCount(value tftypes.Value) int {
	var elements []tftypes.Value
	if !value.IsKnown() || value.IsNull() || value.As(&elements) != nil {
		return 0
	}
	return len(elements)
}

// firstBlockAttribute returns an attribute of the only element of a block.
func firstBlockAttribute(value tftypes.Value, name string) (tftypes.Value, bool) {
	var elements []tftypes.Value
	if !value.IsKnown() || value.IsNull() || value.As(&elements) != nil || len(elements) == 0 {
		return tftypes.Value{}, false
	}
	var attrs map[string]tftypes.Value
	if elements[0].As(&attrs) != nil {
		return tftypes.Value{}, false
	}
	attr, ok := attrs[name]
	return attr, ok
}

// plannedFrequency returns the planned frequency of a schedule block, and
// whether it is known. The rules are checked again when the plan is made during
// the apply, for a frequency which isn't known yet.
func plannedFrequency(value tftypes.Value) (int, bool) {
	frequency, ok := firstBlockAttribute(value, keyFrequency)
	if !ok || !frequency.IsKnown() || frequency.IsNull() {
		return 0, false
	}
	var n big.Float
	if frequency.As(&n) != nil {
		return 0, false
	}
	i, _ := n.Int64()
	return int(i), true
}

// slaDomainPlannedSchedule returns the parts of the planned snapshot schedule
// which the AWS Config and GCP BigQuery rules check. The AWS Config rules only
// look at the minute and hourly schedules, the GCP BigQuery rules at all of them.
//
// The two rules treat a frequency which isn't known yet differently, as they did
// in SDKv2. The AWS Config rules compare the hourly frequency with a minimum, so
// an hourly schedule without a known frequency is left out. The GCP BigQuery
// rules need to know a schedule is there, so it is kept with the frequency 0,
// which passes them.
func slaDomainPlannedSchedule(planned map[string]tftypes.Value, bigQuery bool) gqlsla.SnapshotSchedule {
	var schedule gqlsla.SnapshotSchedule
	if blockCount(planned[keyMinuteSchedule]) > 0 {
		schedule.Minute = &gqlsla.MinuteSnapshotSchedule{}
	}

	// basic returns the basic schedule of a block, which isn't there when the
	// block isn't, and when the frequency isn't known for the AWS Config rules.
	basic := func(key string) (gqlsla.BasicSnapshotSchedule, bool) {
		if blockCount(planned[key]) == 0 {
			return gqlsla.BasicSnapshotSchedule{}, false
		}
		frequency, known := plannedFrequency(planned[key])
		return gqlsla.BasicSnapshotSchedule{Frequency: frequency}, known || bigQuery
	}
	if b, ok := basic(keyHourlySchedule); ok {
		schedule.Hourly = &gqlsla.HourlySnapshotSchedule{BasicSchedule: b}
	}
	if !bigQuery {
		return schedule
	}

	if b, ok := basic(keyDailySchedule); ok {
		schedule.Daily = &gqlsla.DailySnapshotSchedule{BasicSchedule: b}
	}
	if b, ok := basic(keyWeeklySchedule); ok {
		schedule.Weekly = &gqlsla.WeeklySnapshotSchedule{BasicSchedule: b}
	}
	if blockCount(planned[keyMonthlySchedule]) > 0 {
		schedule.Monthly = &gqlsla.MonthlySnapshotSchedule{}
	}
	if blockCount(planned[keyQuarterlySchedule]) > 0 {
		schedule.Quarterly = &gqlsla.QuarterlySnapshotSchedule{}
	}
	if blockCount(planned[keyYearlySchedule]) > 0 {
		schedule.Yearly = &gqlsla.YearlySnapshotSchedule{}
	}

	return schedule
}

// hasLTRConfig reports whether an Azure SQL config block has an ltr_config, which
// makes the SLA a V1 (Azure-managed) SLA.
func hasLTRConfig(value tftypes.Value) bool {
	ltr, ok := firstBlockAttribute(value, keyLTRConfig)
	return ok && blockCount(ltr) > 0
}
