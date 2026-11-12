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
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// setConfigFields returns the names of the object specific configs which are set.
func setConfigFields(t *testing.T, configs *gqlsla.ObjectSpecificConfigs) []string {
	t.Helper()

	if configs == nil {
		t.Fatal("object specific configs are nil")
	}
	var set []string
	v := reflect.ValueOf(*configs)
	for i := range v.NumField() {
		if !v.Field(i).IsNil() {
			set = append(set, v.Type().Field(i).Name)
		}
	}
	return set
}

func TestSLADomainParamsConfigWiring(t *testing.T) {
	blobID := uuid.NewString()

	// Each block is set on its own, and has to give exactly its own config. The
	// Managed Volume, Postgres DB Cluster and GCP Cloud SQL blocks share a model,
	// and so do the Mongo and MySQL blocks, so crossing them would compile.
	tests := []struct {
		name string
		set  func(*slaDomainModel)
		want string
	}{
		{"aws_dynamodb_config", func(m *slaDomainModel) { m.AWSDynamoDBConfig = []awsDynamoDBConfigModel{{}} }, "AWSDynamoDBConfig"},
		{"aws_rds_config", func(m *slaDomainModel) { m.AWSRDSConfig = []awsRDSConfigModel{{}} }, "AWSRDSConfig"},
		{"azure_blob_config", func(m *slaDomainModel) {
			m.AzureBlobConfig = []azureBlobConfigModel{{ArchivalLocationID: types.StringValue(blobID)}}
		}, "AzureBlobConfig"},
		{"azure_postgres_flexible_server_config", func(m *slaDomainModel) {
			m.AzurePostgresFlexibleServerConfig = []azurePostgresFlexibleServerConfigModel{{}}
		}, "AzurePostgresFlexibleServerConfig"},
		{"azure_sql_database_config", func(m *slaDomainModel) { m.AzureSQLDatabaseConfig = []azureSQLConfigModel{{}} }, "AzureSQLDatabaseDBConfig"},
		{"azure_sql_managed_instance_config", func(m *slaDomainModel) { m.AzureSQLManagedInstanceConfig = []azureSQLConfigModel{{}} }, "AzureSQLManagedInstanceDBConfig"},
		{"vmware_vm_config", func(m *slaDomainModel) { m.VMwareVMConfig = []vmwareVMConfigModel{{}} }, "VMwareVMConfig"},
		{"sap_hana_config", func(m *slaDomainModel) { m.SapHanaConfig = []sapHanaConfigModel{{}} }, "SapHanaConfig"},
		{"db2_config", func(m *slaDomainModel) { m.DB2Config = []db2ConfigModel{{}} }, "DB2Config"},
		{"mssql_config", func(m *slaDomainModel) { m.MSSQLConfig = []mssqlConfigModel{{}} }, "MssqlConfig"},
		{"oracle_config", func(m *slaDomainModel) { m.OracleConfig = []oracleConfigModel{{}} }, "OracleConfig"},
		{"mongo_config", func(m *slaDomainModel) { m.MongoConfig = []logScheduleConfigModel{{}} }, "MongoConfig"},
		{"managed_volume_config", func(m *slaDomainModel) { m.ManagedVolumeConfig = []logRetentionConfigModel{{}} }, "ManagedVolumeSlaConfig"},
		{"postgres_db_cluster_config", func(m *slaDomainModel) { m.PostgresDBClusterConfig = []logRetentionConfigModel{{}} }, "PostgresDbClusterSlaConfig"},
		{"mysqldb_config", func(m *slaDomainModel) { m.MySQLDBConfig = []logScheduleConfigModel{{}} }, "MysqldbSlaConfig"},
		{"ncd_config", func(m *slaDomainModel) { m.NCDConfig = []ncdConfigModel{{}} }, "NcdSlaConfig"},
		{"informix_config", func(m *slaDomainModel) { m.InformixConfig = []informixConfigModel{{}} }, "InformixSlaConfig"},
		{"gcp_cloud_sql_config", func(m *slaDomainModel) { m.GCPCloudSQLConfig = []logRetentionConfigModel{{}} }, "GcpCloudSqlConfig"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var model slaDomainModel
			tt.set(&model)
			params, err := slaDomainParams(model, nil, slaDomainFeatures{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := setConfigFields(t, params.ObjectSpecificConfigs); !reflect.DeepEqual(got, []string{tt.want}) {
				t.Fatalf("set configs = %v, want [%s]", got, tt.want)
			}
		})
	}

	t.Run("no block sets no config", func(t *testing.T) {
		params, err := slaDomainParams(slaDomainModel{}, nil, slaDomainFeatures{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := setConfigFields(t, params.ObjectSpecificConfigs); len(got) != 0 {
			t.Fatalf("set configs = %v, want none", got)
		}
	})

	t.Run("blocks sharing a model keep their own values", func(t *testing.T) {
		days := func(n int64) logRetentionConfigModel {
			return logRetentionConfigModel{LogRetention: types.Int64Value(n), LogRetentionUnit: types.StringValue("DAYS")}
		}
		schedule := func(frequency, retention int64) logScheduleConfigModel {
			return logScheduleConfigModel{
				Frequency: types.Int64Value(frequency), FrequencyUnit: types.StringValue("HOURS"),
				Retention: types.Int64Value(retention), RetentionUnit: types.StringValue("DAYS"),
			}
		}
		model := slaDomainModel{
			ManagedVolumeConfig:     []logRetentionConfigModel{days(21)},
			PostgresDBClusterConfig: []logRetentionConfigModel{days(22)},
			GCPCloudSQLConfig:       []logRetentionConfigModel{days(23)},
			MongoConfig:             []logScheduleConfigModel{schedule(31, 32)},
			MySQLDBConfig:           []logScheduleConfigModel{schedule(33, 34)},
		}
		params, err := slaDomainParams(model, nil, slaDomainFeatures{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		c := params.ObjectSpecificConfigs
		want := func(got gqlsla.RetentionDuration, duration int, unit gqlsla.RetentionUnit, what string) {
			t.Helper()
			if got != (gqlsla.RetentionDuration{Duration: duration, Unit: unit}) {
				t.Errorf("%s = %+v, want %d %s", what, got, duration, unit)
			}
		}
		want(c.ManagedVolumeSlaConfig.LogRetention, 21, gqlsla.Days, "managed volume log retention")
		want(c.PostgresDbClusterSlaConfig.LogRetention, 22, gqlsla.Days, "postgres log retention")
		want(c.GcpCloudSqlConfig.LogRetention, 23, gqlsla.Days, "gcp cloud sql log retention")
		want(c.MongoConfig.LogFrequency, 31, gqlsla.Hours, "mongo log frequency")
		want(c.MongoConfig.LogRetention, 32, gqlsla.Days, "mongo log retention")
		want(c.MysqldbSlaConfig.LogFrequency, 33, gqlsla.Hours, "mysql log frequency")
		want(c.MysqldbSlaConfig.LogRetention, 34, gqlsla.Days, "mysql log retention")
	})
}

func TestSLADomainParamsSchedules(t *testing.T) {
	basic := basicScheduleModel{Frequency: types.Int64Value(2), Retention: types.Int64Value(3), RetentionUnit: types.StringValue("DAYS")}

	t.Run("an unset schedule is nil", func(t *testing.T) {
		params, err := slaDomainParams(slaDomainModel{DailySchedule: []dailyScheduleModel{basic}}, nil, slaDomainFeatures{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s := params.SnapshotSchedule
		if s.Daily == nil || s.Daily.BasicSchedule.Frequency != 2 {
			t.Fatalf("daily schedule = %+v", s.Daily)
		}
		if s.Hourly != nil || s.Minute != nil || s.Weekly != nil || s.Monthly != nil || s.Quarterly != nil || s.Yearly != nil {
			t.Fatalf("other schedules are set: %+v", s)
		}
	})

	t.Run("each block gives its own schedule", func(t *testing.T) {
		model := slaDomainModel{
			DailySchedule:     []dailyScheduleModel{{Frequency: types.Int64Value(1), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}},
			HourlySchedule:    []hourlyScheduleModel{{Frequency: types.Int64Value(2), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}},
			MinuteSchedule:    []minuteScheduleModel{{Frequency: types.Int64Value(3), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}},
			WeeklySchedule:    []weeklyScheduleModel{{basicScheduleModel: basicScheduleModel{Frequency: types.Int64Value(4), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}}},
			MonthlySchedule:   []monthlyScheduleModel{{basicScheduleModel: basicScheduleModel{Frequency: types.Int64Value(5), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}}},
			QuarterlySchedule: []quarterlyScheduleModel{{basicScheduleModel: basicScheduleModel{Frequency: types.Int64Value(6), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}}},
			YearlySchedule:    []yearlyScheduleModel{{basicScheduleModel: basicScheduleModel{Frequency: types.Int64Value(7), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}}},
		}
		params, err := slaDomainParams(model, nil, slaDomainFeatures{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s := params.SnapshotSchedule
		got := []int{
			s.Daily.BasicSchedule.Frequency, s.Hourly.BasicSchedule.Frequency, s.Minute.BasicSchedule.Frequency,
			s.Weekly.BasicSchedule.Frequency, s.Monthly.BasicSchedule.Frequency,
			s.Quarterly.BasicSchedule.Frequency, s.Yearly.BasicSchedule.Frequency,
		}
		if want := []int{1, 2, 3, 4, 5, 6, 7}; !reflect.DeepEqual(got, want) {
			t.Fatalf("frequencies = %v, want %v", got, want)
		}
	})
}

func TestSLADomainParamsRetentionLock(t *testing.T) {
	lock := func(mode string, acknowledged types.Bool) []retentionLockModel {
		return []retentionLockModel{{Mode: types.StringValue(mode), ComplianceAcknowledge: acknowledged}}
	}

	tests := []struct {
		name     string
		lock     []retentionLockModel
		wantMode gqlsla.RetentionLockMode
		wantLock bool
		wantErr  string
	}{
		{name: "no block", wantMode: "", wantLock: false},
		{name: "compliance acknowledged", lock: lock(string(gqlsla.Compliance), types.BoolValue(true)), wantMode: gqlsla.Compliance, wantLock: true},
		{name: "compliance not acknowledged", lock: lock(string(gqlsla.Compliance), types.BoolValue(false)), wantErr: "compliance_mode_acknowledgment must be set to true"},
		{name: "compliance acknowledgment null", lock: lock(string(gqlsla.Compliance), types.BoolNull()), wantErr: "compliance_mode_acknowledgment must be set to true"},
		{name: "protection needs no acknowledgment", lock: lock(string(gqlsla.Protection), types.BoolNull()), wantMode: gqlsla.Protection, wantLock: true},
		{name: "no lock", lock: lock(string(gqlsla.NoLock), types.BoolNull()), wantMode: gqlsla.NoLock, wantLock: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := slaDomainParams(slaDomainModel{RetentionLock: tt.lock}, nil, slaDomainFeatures{})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if params.RetentionLockMode != tt.wantMode || params.RetentionLock != tt.wantLock {
				t.Fatalf("mode, lock = %q, %t; want %q, %t", params.RetentionLockMode, params.RetentionLock, tt.wantMode, tt.wantLock)
			}
		})
	}
}

func TestSLADomainParamsBackupLocationRouting(t *testing.T) {
	id := uuid.New()
	locations := []backupLocationModel{{ArchivalGroupID: types.StringValue(id.String())}}
	daily := []dailyScheduleModel{{Frequency: types.Int64Value(1), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}}
	ltr := []azureSQLLTRConfigModel{{}}

	tests := []struct {
		name        string
		model       slaDomainModel
		objectTypes []gqlsla.ObjectType
		features    slaDomainFeatures
		wantSpecs   int
		wantS3      bool
		wantErr     string
	}{
		{
			name:        "AWS S3 legacy uses the object specific config",
			model:       slaDomainModel{BackupLocation: locations},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAWSS3},
			wantS3:      true,
		},
		{
			name:        "AWS S3 with multiple backup locations uses the specs",
			model:       slaDomainModel{BackupLocation: locations},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAWSS3},
			features:    slaDomainFeatures{MultipleBackupLocations: true},
			wantSpecs:   1,
		},
		{
			name:        "AWS S3 legacy without a location",
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAWSS3},
			wantErr:     "requires AWS S3 configuration",
		},
		{
			name:        "AWS S3 with multiple backup locations needs one",
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAWSS3},
			features:    slaDomainFeatures{MultipleBackupLocations: true},
			wantErr:     "requires at least one backup location",
		},
		{
			name:        "AWS S3 legacy takes only one location",
			model:       slaDomainModel{BackupLocation: append(locations, locations...)},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAWSS3},
			wantErr:     "multiple backup locations not supported",
		},
		{
			name:        "AWS S3 can't be combined",
			model:       slaDomainModel{BackupLocation: locations},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAWSS3, gqlsla.ObjectAWSEC2EBS},
			features:    slaDomainFeatures{MultipleBackupLocations: true},
			wantErr:     "cannot be combined with other object types",
		},
		{
			name:        "AWS Config uses the specs",
			model:       slaDomainModel{BackupLocation: locations},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAWSConfig},
			wantSpecs:   1,
		},
		{
			name:        "Azure Postgres uses the specs",
			model:       slaDomainModel{BackupLocation: locations},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAzurePostgresFlexibleServer},
			wantSpecs:   1,
		},
		{
			name:        "GCP BigQuery uses the specs",
			model:       slaDomainModel{BackupLocation: locations},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectGCPBigQuery},
			wantSpecs:   1,
		},
		{
			name: "Azure SQL V2 uses the specs when revamped",
			model: slaDomainModel{
				BackupLocation:         locations,
				AzureSQLDatabaseConfig: []azureSQLConfigModel{{}},
				DailySchedule:          daily,
			},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase},
			features:    slaDomainFeatures{AzureSQLRevamp: true},
			wantSpecs:   1,
		},
		{
			name: "Azure SQL V1 with a location is rejected, and not sent as an S3 config",
			model: slaDomainModel{
				BackupLocation:         locations,
				AzureSQLDatabaseConfig: []azureSQLConfigModel{{LTRConfig: ltr}},
			},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase},
			features:    slaDomainFeatures{AzureSQLRevamp: true},
			wantErr:     "must not specify a backup_location",
		},
		{
			name:        "a location without a supporting object type",
			model:       slaDomainModel{BackupLocation: locations},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectVSphereVM},
			wantErr:     "backup_location is not supported by the configured object types",
		},
		{
			name: "Azure SQL doesn't take locations when not revamped",
			model: slaDomainModel{
				BackupLocation:         locations,
				AzureSQLDatabaseConfig: []azureSQLConfigModel{{}},
			},
			objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase},
			wantErr:     "backup_location is not supported by the configured object types",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := slaDomainParams(tt.model, tt.objectTypes, tt.features)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := len(params.BackupLocationSpecs); got != tt.wantSpecs {
				t.Fatalf("backup location specs = %d, want %d", got, tt.wantSpecs)
			}
			if got := params.ObjectSpecificConfigs.AWSS3Config != nil; got != tt.wantS3 {
				t.Fatalf("AWS S3 config set = %t, want %t", got, tt.wantS3)
			}
			if tt.wantS3 && params.ObjectSpecificConfigs.AWSS3Config.ArchivalLocationID != id {
				t.Fatalf("AWS S3 archival location = %s, want %s", params.ObjectSpecificConfigs.AWSS3Config.ArchivalLocationID, id)
			}
			if tt.wantSpecs > 0 && params.BackupLocationSpecs[0].ArchivalGroupID != id {
				t.Fatalf("backup location = %s, want %s", params.BackupLocationSpecs[0].ArchivalGroupID, id)
			}
		})
	}
}

func TestSLADomainParamsObjectTypeRules(t *testing.T) {
	hourly := func(frequency int64) []hourlyScheduleModel {
		return []hourlyScheduleModel{{Frequency: types.Int64Value(frequency), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS")}}
	}
	window := []snapshotWindowModel{{Duration: types.Int64Value(4), StartAt: types.StringValue("16:45")}}
	archival := []archivalModel{{ArchivalLocationID: types.StringValue(uuid.NewString()), Threshold: types.Int64Value(1), ThresholdUnit: types.StringValue("DAYS")}}
	replication := []replicationSpecModel{{
		AzureRegion: types.StringValue("eastus"), Retention: types.Int64Value(1), RetentionUnit: types.StringValue("DAYS"),
	}}

	tests := []struct {
		name        string
		model       slaDomainModel
		objectTypes []gqlsla.ObjectType
		wantErr     string
	}{
		{name: "Active Directory hourly below 4", model: slaDomainModel{HourlySchedule: hourly(3)}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectActiveDirectory}, wantErr: "minimum of 4 hours"},
		{name: "Active Directory hourly at 4", model: slaDomainModel{HourlySchedule: hourly(4)}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectActiveDirectory}},
		{name: "Azure Blob needs its config", objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAzureBlob}, wantErr: "requires Azure Blob configuration"},
		{name: "Azure Blob with its config", model: slaDomainModel{AzureBlobConfig: []azureBlobConfigModel{{ArchivalLocationID: types.StringValue(uuid.NewString())}}}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectAzureBlob}},
		{name: "Microsoft 365 snapshot window", model: slaDomainModel{SnapshotWindow: window}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectMicrosoft365}, wantErr: "does not support snapshot windows"},
		{name: "Microsoft 365 first full snapshot window", model: slaDomainModel{FirstFullSnapshot: window}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectMicrosoft365}, wantErr: "does not support first full snapshot windows"},
		{name: "Microsoft 365 hourly below 8", model: slaDomainModel{HourlySchedule: hourly(7)}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectMicrosoft365}, wantErr: "minimum of 8 hours"},
		{name: "Microsoft 365 hourly at 8", model: slaDomainModel{HourlySchedule: hourly(8)}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectMicrosoft365}},
		{name: "OLVM archival", model: slaDomainModel{Archival: archival}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectOLVM}, wantErr: "OLVM object type does not support archival"},
		{name: "Cassandra archival", model: slaDomainModel{Archival: archival}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectCassandra}, wantErr: "Cassandra object type does not support archival"},
		{name: "Cassandra replication", model: slaDomainModel{ReplicationSpec: replication}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectCassandra}, wantErr: "Cassandra object type does not support replication"},
		{name: "MongoDB replication", model: slaDomainModel{ReplicationSpec: replication}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectMongoDB}, wantErr: "MongoDB object type does not support replication"},
		{name: "MongoDB archival is allowed", model: slaDomainModel{Archival: archival}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectMongoDB}},
		{name: "a rule applies to its own object type only", model: slaDomainModel{Archival: archival}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectVSphereVM}},
		{name: "a rule applies when combined", model: slaDomainModel{HourlySchedule: hourly(3)}, objectTypes: []gqlsla.ObjectType{gqlsla.ObjectVSphereVM, gqlsla.ObjectActiveDirectory}, wantErr: "minimum of 4 hours"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := slaDomainParams(tt.model, tt.objectTypes, slaDomainFeatures{})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(params.ObjectTypes, tt.objectTypes) {
				t.Fatalf("object types = %v, want %v", params.ObjectTypes, tt.objectTypes)
			}
		})
	}
}

func TestSLADomainParamsFields(t *testing.T) {
	model := slaDomainModel{
		Name:           types.StringValue("gold"),
		Description:    types.StringValue("the gold SLA"),
		LocalRetention: []localRetentionModel{{Retention: types.Int64Value(9), RetentionUnit: types.StringValue("DAYS")}},
		SnapshotWindow: []snapshotWindowModel{{Duration: types.Int64Value(4), StartAt: types.StringValue("Mon, 15:30")}},
		FirstFullSnapshot: []snapshotWindowModel{
			{Duration: types.Int64Value(6), StartAt: types.StringValue("16:45")},
		},
	}
	params, err := slaDomainParams(model, []gqlsla.ObjectType{gqlsla.ObjectVSphereVM}, slaDomainFeatures{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.Name != "gold" || params.Description != "the gold SLA" {
		t.Errorf("name, description = %q, %q", params.Name, params.Description)
	}
	if params.LocalRetentionLimit == nil || *params.LocalRetentionLimit != (gqlsla.RetentionDuration{Duration: 9, Unit: gqlsla.Days}) {
		t.Errorf("local retention = %+v", params.LocalRetentionLimit)
	}
	if len(params.BackupWindows) != 1 || params.BackupWindows[0].DurationInHours != 4 {
		t.Errorf("snapshot windows = %+v", params.BackupWindows)
	}
	if len(params.FirstFullBackupWindows) != 1 || params.FirstFullBackupWindows[0].DurationInHours != 6 {
		t.Errorf("first full snapshot windows = %+v", params.FirstFullBackupWindows)
	}

	t.Run("no local retention is nil", func(t *testing.T) {
		params, err := slaDomainParams(slaDomainModel{}, nil, slaDomainFeatures{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if params.LocalRetentionLimit != nil {
			t.Fatalf("local retention = %+v, want nil", params.LocalRetentionLimit)
		}
	})
}
