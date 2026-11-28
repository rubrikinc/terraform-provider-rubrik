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
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// This file holds the models of the Oracle, SAP HANA, DB2, Informix and NCD
// configs of the SLA domain.
//
// In the SAP HANA, DB2 and Informix configs the durations are optional. A
// duration which isn't set is the zero value in the SDK, and only a duration
// with a number greater than zero is read from the block. When writing the
// block, the unit of every duration is always set, to the default of DAYS when
// the duration isn't set, and the number is null. The default is set by the
// schema when planning, and not when the state is saved, so the state has to
// carry it as well, or every plan would show a difference.

// oracleRetainArchiveLogsIndefinitely is the duration the CDM uses as a
// sentinel for retaining all the archived redo logs indefinitely: minus two
// minutes.
var oracleRetainArchiveLogsIndefinitely = gqlsla.RetentionDuration{Duration: -2, Unit: gqlsla.Minute}

// optionalDurationFromAttributes returns the SDK retention duration of an
// optional duration. The duration is only set when its number is greater than
// zero, otherwise it is the zero value.
func optionalDurationFromAttributes(duration types.Int64, unit types.String) gqlsla.RetentionDuration {
	if duration.ValueInt64() <= 0 {
		return gqlsla.RetentionDuration{}
	}
	return durationFromAttributes(duration, unit)
}

// optionalDurationToAttributes returns the number and the unit attributes of an
// optional SDK retention duration. A duration without a number gives a null
// number and the default unit, DAYS.
func optionalDurationToAttributes(d gqlsla.RetentionDuration) (types.Int64, types.String) {
	if d.Duration <= 0 {
		return types.Int64Null(), types.StringValue(string(gqlsla.Days))
	}
	return durationToAttributes(d)
}

// oracleConfigModel is the oracle_config block.
type oracleConfigModel struct {
	Frequency                     types.Int64  `tfsdk:"frequency"`
	FrequencyUnit                 types.String `tfsdk:"frequency_unit"`
	LogRetention                  types.Int64  `tfsdk:"log_retention"`
	LogRetentionUnit              types.String `tfsdk:"log_retention_unit"`
	HostLogRetention              types.Int64  `tfsdk:"host_log_retention"`
	HostLogRetentionUnit          types.String `tfsdk:"host_log_retention_unit"`
	RetainArchiveLogsIndefinitely types.Bool   `tfsdk:"retain_archive_logs_indefinitely"`
}

// sapHanaStorageSnapshotConfigModel is the storage_snapshot_config block of the
// sap_hana_config block.
type sapHanaStorageSnapshotConfigModel struct {
	Frequency     types.Int64  `tfsdk:"frequency"`
	FrequencyUnit types.String `tfsdk:"frequency_unit"`
	Retention     types.Int64  `tfsdk:"retention"`
	RetentionUnit types.String `tfsdk:"retention_unit"`
}

// sapHanaConfigModel is the sap_hana_config block.
type sapHanaConfigModel struct {
	IncrementalFrequency      types.Int64                         `tfsdk:"incremental_frequency"`
	IncrementalFrequencyUnit  types.String                        `tfsdk:"incremental_frequency_unit"`
	LogRetention              types.Int64                         `tfsdk:"log_retention"`
	LogRetentionUnit          types.String                        `tfsdk:"log_retention_unit"`
	DifferentialFrequency     types.Int64                         `tfsdk:"differential_frequency"`
	DifferentialFrequencyUnit types.String                        `tfsdk:"differential_frequency_unit"`
	StorageSnapshotConfig     []sapHanaStorageSnapshotConfigModel `tfsdk:"storage_snapshot_config"`
}

// db2ConfigModel is the db2_config block.
type db2ConfigModel struct {
	IncrementalFrequency      types.Int64  `tfsdk:"incremental_frequency"`
	IncrementalFrequencyUnit  types.String `tfsdk:"incremental_frequency_unit"`
	LogRetention              types.Int64  `tfsdk:"log_retention"`
	LogRetentionUnit          types.String `tfsdk:"log_retention_unit"`
	DifferentialFrequency     types.Int64  `tfsdk:"differential_frequency"`
	DifferentialFrequencyUnit types.String `tfsdk:"differential_frequency_unit"`
	LogArchivalMethod         types.String `tfsdk:"log_archival_method"`
}

// informixConfigModel is the informix_config block. The log frequency and the
// log retention are named frequency and retention.
type informixConfigModel struct {
	IncrementalFrequency     types.Int64  `tfsdk:"incremental_frequency"`
	IncrementalFrequencyUnit types.String `tfsdk:"incremental_frequency_unit"`
	IncrementalRetention     types.Int64  `tfsdk:"incremental_retention"`
	IncrementalRetentionUnit types.String `tfsdk:"incremental_retention_unit"`
	Frequency                types.Int64  `tfsdk:"frequency"`
	FrequencyUnit            types.String `tfsdk:"frequency_unit"`
	Retention                types.Int64  `tfsdk:"retention"`
	RetentionUnit            types.String `tfsdk:"retention_unit"`
}

// ncdConfigModel is the ncd_config block. Each attribute holds the archival
// location IDs for one schedule.
type ncdConfigModel struct {
	MinutelyBackupLocations  []string `tfsdk:"minutely_backup_locations"`
	HourlyBackupLocations    []string `tfsdk:"hourly_backup_locations"`
	DailyBackupLocations     []string `tfsdk:"daily_backup_locations"`
	WeeklyBackupLocations    []string `tfsdk:"weekly_backup_locations"`
	MonthlyBackupLocations   []string `tfsdk:"monthly_backup_locations"`
	QuarterlyBackupLocations []string `tfsdk:"quarterly_backup_locations"`
	YearlyBackupLocations    []string `tfsdk:"yearly_backup_locations"`
}

// fromOracleConfigModel returns the SDK Oracle config of the block, or nil if
// the block isn't configured. When the archived redo logs are retained
// indefinitely, the host log retention is the sentinel, which replaces any host
// log retention.
func fromOracleConfigModel(config []oracleConfigModel) *gqlsla.OracleConfig {
	if len(config) == 0 {
		return nil
	}

	c := config[0]
	oracleConfig := &gqlsla.OracleConfig{
		Frequency:    durationFromAttributes(c.Frequency, c.FrequencyUnit),
		LogRetention: durationFromAttributes(c.LogRetention, c.LogRetentionUnit),
	}

	if c.HostLogRetention.ValueInt64() > 0 {
		oracleConfig.HostLogRetention = durationFromAttributes(c.HostLogRetention, c.HostLogRetentionUnit)
	}
	if c.RetainArchiveLogsIndefinitely.ValueBool() {
		oracleConfig.HostLogRetention = oracleRetainArchiveLogsIndefinitely
	}

	return oracleConfig
}

// toOracleConfigModel returns the block of the SDK Oracle config, or nil if the
// config is nil. The sentinel host log retention is returned as
// retain_archive_logs_indefinitely, and a host log retention which isn't set is
// returned with the default unit, DAYS.
func toOracleConfigModel(config *gqlsla.OracleConfig) []oracleConfigModel {
	if config == nil {
		return nil
	}

	frequency, frequencyUnit := durationToAttributes(config.Frequency)
	logRetention, logRetentionUnit := durationToAttributes(config.LogRetention)
	model := oracleConfigModel{
		Frequency: frequency, FrequencyUnit: frequencyUnit,
		LogRetention: logRetention, LogRetentionUnit: logRetentionUnit,
		HostLogRetention:              types.Int64Null(),
		HostLogRetentionUnit:          types.StringValue(string(gqlsla.Days)),
		RetainArchiveLogsIndefinitely: types.BoolValue(false),
	}

	switch config.HostLogRetention.Duration {
	case oracleRetainArchiveLogsIndefinitely.Duration:
		model.RetainArchiveLogsIndefinitely = types.BoolValue(true)
	case 0:
		// Not set.
	default:
		model.HostLogRetention, model.HostLogRetentionUnit = durationToAttributes(config.HostLogRetention)
	}

	return []oracleConfigModel{model}
}

// fromSapHanaConfigModel returns the SDK SAP HANA config of the block, or nil if
// the block isn't configured.
func fromSapHanaConfigModel(config []sapHanaConfigModel) *gqlsla.SapHanaConfig {
	if len(config) == 0 {
		return nil
	}

	c := config[0]
	sapHanaConfig := &gqlsla.SapHanaConfig{
		IncrementalFrequency:  optionalDurationFromAttributes(c.IncrementalFrequency, c.IncrementalFrequencyUnit),
		LogRetention:          optionalDurationFromAttributes(c.LogRetention, c.LogRetentionUnit),
		DifferentialFrequency: optionalDurationFromAttributes(c.DifferentialFrequency, c.DifferentialFrequencyUnit),
	}

	if len(c.StorageSnapshotConfig) > 0 {
		snap := c.StorageSnapshotConfig[0]
		sapHanaConfig.StorageSnapshotConfig = &gqlsla.SapHanaStorageSnapshotConfig{
			Frequency: durationFromAttributes(snap.Frequency, snap.FrequencyUnit),
			Retention: durationFromAttributes(snap.Retention, snap.RetentionUnit),
		}
	}

	return sapHanaConfig
}

// toSapHanaConfigModel returns the block of the SDK SAP HANA config, or nil if
// the config is nil. The storage snapshot config is only returned when it has a
// frequency or a retention.
func toSapHanaConfigModel(config *gqlsla.SapHanaConfig) []sapHanaConfigModel {
	if config == nil {
		return nil
	}

	var model sapHanaConfigModel
	model.IncrementalFrequency, model.IncrementalFrequencyUnit = optionalDurationToAttributes(config.IncrementalFrequency)
	model.LogRetention, model.LogRetentionUnit = optionalDurationToAttributes(config.LogRetention)
	model.DifferentialFrequency, model.DifferentialFrequencyUnit = optionalDurationToAttributes(config.DifferentialFrequency)

	if c := config.StorageSnapshotConfig; c != nil && (c.Frequency.Duration > 0 || c.Retention.Duration > 0) {
		frequency, frequencyUnit := durationToAttributes(c.Frequency)
		retention, retentionUnit := durationToAttributes(c.Retention)
		model.StorageSnapshotConfig = []sapHanaStorageSnapshotConfigModel{{
			Frequency: frequency, FrequencyUnit: frequencyUnit,
			Retention: retention, RetentionUnit: retentionUnit,
		}}
	}

	return []sapHanaConfigModel{model}
}

// fromDB2ConfigModel returns the SDK DB2 config of the block, or nil if the
// block isn't configured. The log archival method is only set when it isn't
// empty.
func fromDB2ConfigModel(config []db2ConfigModel) *gqlsla.DB2Config {
	if len(config) == 0 {
		return nil
	}

	c := config[0]
	db2Config := &gqlsla.DB2Config{
		IncrementalFrequency:  optionalDurationFromAttributes(c.IncrementalFrequency, c.IncrementalFrequencyUnit),
		LogRetention:          optionalDurationFromAttributes(c.LogRetention, c.LogRetentionUnit),
		DifferentialFrequency: optionalDurationFromAttributes(c.DifferentialFrequency, c.DifferentialFrequencyUnit),
	}
	if method := c.LogArchivalMethod.ValueString(); method != "" {
		db2Config.LogArchivalMethod = gqlsla.Db2LogArchivalMethod(method)
	}

	return db2Config
}

// toDB2ConfigModel returns the block of the SDK DB2 config, or nil if the config
// is nil. The log archival method is null when it isn't set.
func toDB2ConfigModel(config *gqlsla.DB2Config) []db2ConfigModel {
	if config == nil {
		return nil
	}

	var model db2ConfigModel
	model.IncrementalFrequency, model.IncrementalFrequencyUnit = optionalDurationToAttributes(config.IncrementalFrequency)
	model.LogRetention, model.LogRetentionUnit = optionalDurationToAttributes(config.LogRetention)
	model.DifferentialFrequency, model.DifferentialFrequencyUnit = optionalDurationToAttributes(config.DifferentialFrequency)
	model.LogArchivalMethod = types.StringNull()
	if config.LogArchivalMethod != "" {
		model.LogArchivalMethod = types.StringValue(string(config.LogArchivalMethod))
	}

	return []db2ConfigModel{model}
}

// fromInformixConfigModel returns the SDK Informix config of the block, or nil
// if the block isn't configured.
func fromInformixConfigModel(config []informixConfigModel) *gqlsla.InformixSlaConfig {
	if len(config) == 0 {
		return nil
	}

	c := config[0]
	return &gqlsla.InformixSlaConfig{
		IncrementalFrequency: optionalDurationFromAttributes(c.IncrementalFrequency, c.IncrementalFrequencyUnit),
		IncrementalRetention: optionalDurationFromAttributes(c.IncrementalRetention, c.IncrementalRetentionUnit),
		LogFrequency:         optionalDurationFromAttributes(c.Frequency, c.FrequencyUnit),
		LogRetention:         optionalDurationFromAttributes(c.Retention, c.RetentionUnit),
	}
}

// toInformixConfigModel returns the block of the SDK Informix config, or nil if
// the config is nil.
func toInformixConfigModel(config *gqlsla.InformixSlaConfig) []informixConfigModel {
	if config == nil {
		return nil
	}

	var model informixConfigModel
	model.IncrementalFrequency, model.IncrementalFrequencyUnit = optionalDurationToAttributes(config.IncrementalFrequency)
	model.IncrementalRetention, model.IncrementalRetentionUnit = optionalDurationToAttributes(config.IncrementalRetention)
	model.Frequency, model.FrequencyUnit = optionalDurationToAttributes(config.LogFrequency)
	model.Retention, model.RetentionUnit = optionalDurationToAttributes(config.LogRetention)

	return []informixConfigModel{model}
}

// parseNCDBackupLocations returns the UUIDs of the archival location IDs. An ID
// which isn't a UUID is skipped, and the others are kept.
func parseNCDBackupLocations(locations []string) []uuid.UUID {
	var uuids []uuid.UUID
	for _, location := range locations {
		if id, err := uuid.Parse(location); err == nil {
			uuids = append(uuids, id)
		}
	}
	return uuids
}

// formatNCDBackupLocations returns the archival location IDs of the UUIDs, or
// nil when there are none.
func formatNCDBackupLocations(uuids []uuid.UUID) []string {
	if len(uuids) == 0 {
		return nil
	}
	locations := make([]string, 0, len(uuids))
	for _, id := range uuids {
		locations = append(locations, id.String())
	}
	return locations
}

// fromNCDConfigModel returns the SDK NCD config of the block, or nil if the
// block isn't configured.
func fromNCDConfigModel(config []ncdConfigModel) *gqlsla.NcdSlaConfig {
	if len(config) == 0 {
		return nil
	}

	c := config[0]
	return &gqlsla.NcdSlaConfig{
		MinutelyBackupLocations:  parseNCDBackupLocations(c.MinutelyBackupLocations),
		HourlyBackupLocations:    parseNCDBackupLocations(c.HourlyBackupLocations),
		DailyBackupLocations:     parseNCDBackupLocations(c.DailyBackupLocations),
		WeeklyBackupLocations:    parseNCDBackupLocations(c.WeeklyBackupLocations),
		MonthlyBackupLocations:   parseNCDBackupLocations(c.MonthlyBackupLocations),
		QuarterlyBackupLocations: parseNCDBackupLocations(c.QuarterlyBackupLocations),
		YearlyBackupLocations:    parseNCDBackupLocations(c.YearlyBackupLocations),
	}
}

// toNCDConfigModel returns the block of the SDK NCD config, or nil if the config
// is nil. A schedule without archival locations is null.
func toNCDConfigModel(config *gqlsla.NcdSlaConfig) []ncdConfigModel {
	if config == nil {
		return nil
	}

	return []ncdConfigModel{{
		MinutelyBackupLocations:  formatNCDBackupLocations(config.MinutelyBackupLocations),
		HourlyBackupLocations:    formatNCDBackupLocations(config.HourlyBackupLocations),
		DailyBackupLocations:     formatNCDBackupLocations(config.DailyBackupLocations),
		WeeklyBackupLocations:    formatNCDBackupLocations(config.WeeklyBackupLocations),
		MonthlyBackupLocations:   formatNCDBackupLocations(config.MonthlyBackupLocations),
		QuarterlyBackupLocations: formatNCDBackupLocations(config.QuarterlyBackupLocations),
		YearlyBackupLocations:    formatNCDBackupLocations(config.YearlyBackupLocations),
	}}
}
