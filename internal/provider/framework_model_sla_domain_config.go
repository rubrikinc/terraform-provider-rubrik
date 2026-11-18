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

// This file holds the models of the object type configs of the SLA domain which
// are made up of a few durations: VMware VM, MSSQL, MongoDB, Managed Volume,
// Azure Postgres Flexible Server, Postgres DB Cluster, MySQL and GCP Cloud SQL.
//
// Which attributes hold a duration differs between the configs, and doesn't
// follow the names of the SDK fields. The Mongo and MySQL configs use
// frequency and retention for their log frequency and log retention, and the
// other configs use log_retention for their log retention.

// durationFromAttributes returns the SDK retention duration of a duration and
// a unit attribute.
func durationFromAttributes(duration types.Int64, unit types.String) gqlsla.RetentionDuration {
	return gqlsla.RetentionDuration{
		Duration: int(duration.ValueInt64()),
		Unit:     gqlsla.RetentionUnit(unit.ValueString()),
	}
}

// durationToAttributes returns the duration and the unit attributes of the SDK
// retention duration.
func durationToAttributes(d gqlsla.RetentionDuration) (types.Int64, types.String) {
	return types.Int64Value(int64(d.Duration)), types.StringValue(string(d.Unit))
}

// vmwareVMConfigModel is the vmware_vm_config block. The log retention is in
// seconds.
type vmwareVMConfigModel struct {
	LogRetention types.Int64 `tfsdk:"log_retention"`
}

// logRetentionConfigModel is the block of the configs which only hold a log
// retention: managed_volume_config, postgres_db_cluster_config and
// gcp_cloud_sql_config.
type logRetentionConfigModel struct {
	LogRetention     types.Int64  `tfsdk:"log_retention"`
	LogRetentionUnit types.String `tfsdk:"log_retention_unit"`
}

// mssqlConfigModel is the mssql_config block.
type mssqlConfigModel struct {
	Frequency        types.Int64  `tfsdk:"frequency"`
	FrequencyUnit    types.String `tfsdk:"frequency_unit"`
	LogRetention     types.Int64  `tfsdk:"log_retention"`
	LogRetentionUnit types.String `tfsdk:"log_retention_unit"`
}

// logScheduleConfigModel is the block of the configs which hold a log frequency
// and a log retention, but name them frequency and retention: mongo_config and
// mysqldb_config.
type logScheduleConfigModel struct {
	Frequency     types.Int64  `tfsdk:"frequency"`
	FrequencyUnit types.String `tfsdk:"frequency_unit"`
	Retention     types.Int64  `tfsdk:"retention"`
	RetentionUnit types.String `tfsdk:"retention_unit"`
}

// azurePostgresFlexibleServerConfigModel is the
// azure_postgres_flexible_server_config block.
type azurePostgresFlexibleServerConfigModel struct {
	BackupRetentionInDays types.Int64 `tfsdk:"backup_retention_in_days"`
}

// fromVMwareVMConfigModel returns the SDK VMware VM config of the block, or nil
// if the block isn't configured.
func fromVMwareVMConfigModel(config []vmwareVMConfigModel) *gqlsla.VMwareVMConfig {
	if len(config) == 0 {
		return nil
	}
	return &gqlsla.VMwareVMConfig{LogRetentionSeconds: config[0].LogRetention.ValueInt64()}
}

// toVMwareVMConfigModel returns the block of the SDK VMware VM config, or nil if
// the config is nil.
func toVMwareVMConfigModel(config *gqlsla.VMwareVMConfig) []vmwareVMConfigModel {
	if config == nil {
		return nil
	}
	return []vmwareVMConfigModel{{LogRetention: types.Int64Value(config.LogRetentionSeconds)}}
}

// fromLogRetentionConfigModel returns the SDK log retention of the block, and
// false if the block isn't configured.
func fromLogRetentionConfigModel(config []logRetentionConfigModel) (gqlsla.RetentionDuration, bool) {
	if len(config) == 0 {
		return gqlsla.RetentionDuration{}, false
	}
	return durationFromAttributes(config[0].LogRetention, config[0].LogRetentionUnit), true
}

// toLogRetentionConfigModel returns the block of the SDK log retention.
func toLogRetentionConfigModel(retention gqlsla.RetentionDuration) []logRetentionConfigModel {
	duration, unit := durationToAttributes(retention)
	return []logRetentionConfigModel{{LogRetention: duration, LogRetentionUnit: unit}}
}

// fromManagedVolumeConfigModel returns the SDK Managed Volume config of the
// block, or nil if the block isn't configured.
func fromManagedVolumeConfigModel(config []logRetentionConfigModel) *gqlsla.ManagedVolumeSlaConfig {
	retention, ok := fromLogRetentionConfigModel(config)
	if !ok {
		return nil
	}
	return &gqlsla.ManagedVolumeSlaConfig{LogRetention: retention}
}

// toManagedVolumeConfigModel returns the block of the SDK Managed Volume config,
// or nil if the config is nil.
func toManagedVolumeConfigModel(config *gqlsla.ManagedVolumeSlaConfig) []logRetentionConfigModel {
	if config == nil {
		return nil
	}
	return toLogRetentionConfigModel(config.LogRetention)
}

// fromPostgresDBClusterConfigModel returns the SDK Postgres DB cluster config of
// the block, or nil if the block isn't configured.
func fromPostgresDBClusterConfigModel(config []logRetentionConfigModel) *gqlsla.PostgresDbClusterSlaConfig {
	retention, ok := fromLogRetentionConfigModel(config)
	if !ok {
		return nil
	}
	return &gqlsla.PostgresDbClusterSlaConfig{LogRetention: retention}
}

// toPostgresDBClusterConfigModel returns the block of the SDK Postgres DB
// cluster config, or nil if the config is nil.
func toPostgresDBClusterConfigModel(config *gqlsla.PostgresDbClusterSlaConfig) []logRetentionConfigModel {
	if config == nil {
		return nil
	}
	return toLogRetentionConfigModel(config.LogRetention)
}

// fromGCPCloudSQLConfigModel returns the SDK GCP Cloud SQL config of the block,
// or nil if the block isn't configured.
func fromGCPCloudSQLConfigModel(config []logRetentionConfigModel) *gqlsla.GcpCloudSqlConfig {
	retention, ok := fromLogRetentionConfigModel(config)
	if !ok {
		return nil
	}
	return &gqlsla.GcpCloudSqlConfig{LogRetention: retention}
}

// toGCPCloudSQLConfigModel returns the block of the SDK GCP Cloud SQL config, or
// nil if the config is nil.
func toGCPCloudSQLConfigModel(config *gqlsla.GcpCloudSqlConfig) []logRetentionConfigModel {
	if config == nil {
		return nil
	}
	return toLogRetentionConfigModel(config.LogRetention)
}

// fromMSSQLConfigModel returns the SDK MSSQL config of the block, or nil if the
// block isn't configured.
func fromMSSQLConfigModel(config []mssqlConfigModel) *gqlsla.MssqlConfig {
	if len(config) == 0 {
		return nil
	}
	return &gqlsla.MssqlConfig{
		Frequency:    durationFromAttributes(config[0].Frequency, config[0].FrequencyUnit),
		LogRetention: durationFromAttributes(config[0].LogRetention, config[0].LogRetentionUnit),
	}
}

// toMSSQLConfigModel returns the block of the SDK MSSQL config, or nil if the
// config is nil.
func toMSSQLConfigModel(config *gqlsla.MssqlConfig) []mssqlConfigModel {
	if config == nil {
		return nil
	}
	frequency, frequencyUnit := durationToAttributes(config.Frequency)
	logRetention, logRetentionUnit := durationToAttributes(config.LogRetention)
	return []mssqlConfigModel{{
		Frequency: frequency, FrequencyUnit: frequencyUnit,
		LogRetention: logRetention, LogRetentionUnit: logRetentionUnit,
	}}
}

// fromLogScheduleConfigModel returns the SDK log frequency and log retention of
// the block, and false if the block isn't configured.
func fromLogScheduleConfigModel(config []logScheduleConfigModel) (frequency, retention gqlsla.RetentionDuration, ok bool) {
	if len(config) == 0 {
		return gqlsla.RetentionDuration{}, gqlsla.RetentionDuration{}, false
	}
	return durationFromAttributes(config[0].Frequency, config[0].FrequencyUnit),
		durationFromAttributes(config[0].Retention, config[0].RetentionUnit), true
}

// toLogScheduleConfigModel returns the block of the SDK log frequency and log
// retention.
func toLogScheduleConfigModel(frequency, retention gqlsla.RetentionDuration) []logScheduleConfigModel {
	frequencyDuration, frequencyUnit := durationToAttributes(frequency)
	retentionDuration, retentionUnit := durationToAttributes(retention)
	return []logScheduleConfigModel{{
		Frequency: frequencyDuration, FrequencyUnit: frequencyUnit,
		Retention: retentionDuration, RetentionUnit: retentionUnit,
	}}
}

// fromMongoConfigModel returns the SDK Mongo config of the block, or nil if the
// block isn't configured.
func fromMongoConfigModel(config []logScheduleConfigModel) *gqlsla.MongoConfig {
	frequency, retention, ok := fromLogScheduleConfigModel(config)
	if !ok {
		return nil
	}
	return &gqlsla.MongoConfig{LogFrequency: frequency, LogRetention: retention}
}

// toMongoConfigModel returns the block of the SDK Mongo config, or nil if the
// config is nil.
func toMongoConfigModel(config *gqlsla.MongoConfig) []logScheduleConfigModel {
	if config == nil {
		return nil
	}
	return toLogScheduleConfigModel(config.LogFrequency, config.LogRetention)
}

// fromMySQLDBConfigModel returns the SDK MySQL config of the block, or nil if
// the block isn't configured.
func fromMySQLDBConfigModel(config []logScheduleConfigModel) *gqlsla.MysqldbSlaConfig {
	frequency, retention, ok := fromLogScheduleConfigModel(config)
	if !ok {
		return nil
	}
	return &gqlsla.MysqldbSlaConfig{LogFrequency: frequency, LogRetention: retention}
}

// toMySQLDBConfigModel returns the block of the SDK MySQL config, or nil if the
// config is nil.
func toMySQLDBConfigModel(config *gqlsla.MysqldbSlaConfig) []logScheduleConfigModel {
	if config == nil {
		return nil
	}
	return toLogScheduleConfigModel(config.LogFrequency, config.LogRetention)
}

// fromAzurePostgresFlexibleServerConfigModel returns the SDK Azure Postgres
// flexible server config of the block, or nil if the block isn't configured.
func fromAzurePostgresFlexibleServerConfigModel(config []azurePostgresFlexibleServerConfigModel) *gqlsla.AzurePostgresFlexibleServerConfig {
	if len(config) == 0 {
		return nil
	}
	return &gqlsla.AzurePostgresFlexibleServerConfig{
		BackupRetentionInDays: int(config[0].BackupRetentionInDays.ValueInt64()),
	}
}

// toAzurePostgresFlexibleServerConfigModel returns the block of the SDK Azure
// Postgres flexible server config, or nil if the config is nil.
func toAzurePostgresFlexibleServerConfigModel(config *gqlsla.AzurePostgresFlexibleServerConfig) []azurePostgresFlexibleServerConfigModel {
	if config == nil {
		return nil
	}
	return []azurePostgresFlexibleServerConfigModel{{
		BackupRetentionInDays: types.Int64Value(int64(config.BackupRetentionInDays)),
	}}
}
