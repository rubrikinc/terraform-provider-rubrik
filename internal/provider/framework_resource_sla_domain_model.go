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
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// slaDomainModel is the model of the SLA domain resource. A block which holds at
// most one element is a slice of at most one element.
type slaDomainModel struct {
	ID                                types.String                             `tfsdk:"id"`
	Name                              types.String                             `tfsdk:"name"`
	Description                       types.String                             `tfsdk:"description"`
	ObjectTypes                       types.Set                                `tfsdk:"object_types"`
	BackupType                        types.String                             `tfsdk:"backup_type"`
	ApplyChangesToExistingSnapshots   types.Bool                               `tfsdk:"apply_changes_to_existing_snapshots"`
	ApplyChangesToNonPolicySnapshots  types.Bool                               `tfsdk:"apply_changes_to_non_policy_snapshots"`
	Archival                          []archivalModel                          `tfsdk:"archival"`
	AWSDynamoDBConfig                 []awsDynamoDBConfigModel                 `tfsdk:"aws_dynamodb_config"`
	AWSRDSConfig                      []awsRDSConfigModel                      `tfsdk:"aws_rds_config"`
	AzureBlobConfig                   []azureBlobConfigModel                   `tfsdk:"azure_blob_config"`
	AzurePostgresFlexibleServerConfig []azurePostgresFlexibleServerConfigModel `tfsdk:"azure_postgres_flexible_server_config"`
	AzureSQLDatabaseConfig            []azureSQLConfigModel                    `tfsdk:"azure_sql_database_config"`
	AzureSQLManagedInstanceConfig     []azureSQLConfigModel                    `tfsdk:"azure_sql_managed_instance_config"`
	BackupLocation                    []backupLocationModel                    `tfsdk:"backup_location"`
	DailySchedule                     []dailyScheduleModel                     `tfsdk:"daily_schedule"`
	DB2Config                         []db2ConfigModel                         `tfsdk:"db2_config"`
	FirstFullSnapshot                 []snapshotWindowModel                    `tfsdk:"first_full_snapshot"`
	GCPCloudSQLConfig                 []logRetentionConfigModel                `tfsdk:"gcp_cloud_sql_config"`
	HourlySchedule                    []hourlyScheduleModel                    `tfsdk:"hourly_schedule"`
	InformixConfig                    []informixConfigModel                    `tfsdk:"informix_config"`
	LocalRetention                    []localRetentionModel                    `tfsdk:"local_retention"`
	ManagedVolumeConfig               []logRetentionConfigModel                `tfsdk:"managed_volume_config"`
	MinuteSchedule                    []minuteScheduleModel                    `tfsdk:"minute_schedule"`
	MongoConfig                       []logScheduleConfigModel                 `tfsdk:"mongo_config"`
	MonthlySchedule                   []monthlyScheduleModel                   `tfsdk:"monthly_schedule"`
	MSSQLConfig                       []mssqlConfigModel                       `tfsdk:"mssql_config"`
	MySQLDBConfig                     []logScheduleConfigModel                 `tfsdk:"mysqldb_config"`
	NCDConfig                         []ncdConfigModel                         `tfsdk:"ncd_config"`
	OracleConfig                      []oracleConfigModel                      `tfsdk:"oracle_config"`
	PostgresDBClusterConfig           []logRetentionConfigModel                `tfsdk:"postgres_db_cluster_config"`
	QuarterlySchedule                 []quarterlyScheduleModel                 `tfsdk:"quarterly_schedule"`
	ReplicationSpec                   []replicationSpecModel                   `tfsdk:"replication_spec"`
	RetentionLock                     []retentionLockModel                     `tfsdk:"retention_lock"`
	SapHanaConfig                     []sapHanaConfigModel                     `tfsdk:"sap_hana_config"`
	SnapshotWindow                    []snapshotWindowModel                    `tfsdk:"snapshot_window"`
	VMwareVMConfig                    []vmwareVMConfigModel                    `tfsdk:"vmware_vm_config"`
	WeeklySchedule                    []weeklyScheduleModel                    `tfsdk:"weekly_schedule"`
	YearlySchedule                    []yearlyScheduleModel                    `tfsdk:"yearly_schedule"`
	Timeouts                          timeouts.Value                           `tfsdk:"timeouts"`
}
