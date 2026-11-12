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
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// TestValidateAzureSQLSLA covers the V1/V2 separation rules for Azure SQL DB and
// MI SLAs without needing RSC. A V1 (Azure-managed) SLA carries an LTR config and
// must not specify a backup location, Rubrik schedule, or archival location. A V2
// (Rubrik-managed) SLA omits the LTR config and must specify a backup location
// and a schedule, and must not use the legacy archival block.
func TestValidateAzureSQLSLA(t *testing.T) {
	ltr := &gqlsla.AzureSQLLTRConfig{
		WeeklyBackupRetention: &gqlsla.AzureSQLLTRRetention{
			Retention:     4,
			RetentionUnit: gqlsla.Weeks,
		},
	}
	withSchedule := gqlsla.SnapshotSchedule{Daily: &gqlsla.DailySnapshotSchedule{}}
	var noSchedule gqlsla.SnapshotSchedule
	backupLoc := []gqlsla.BackupLocationSpec{{ArchivalGroupID: uuid.New()}}
	archival := []gqlsla.ArchivalSpec{{}}

	tests := []struct {
		name            string
		config          *gqlsla.AzureDBConfig
		schedule        gqlsla.SnapshotSchedule
		backupLocations []gqlsla.BackupLocationSpec
		archivalSpecs   []gqlsla.ArchivalSpec
		wantErr         string // substring; "" means no error expected
	}{
		{
			name:     "V1 valid: ltr config only, no schedule or backup location",
			config:   &gqlsla.AzureDBConfig{LogRetentionInDays: 7, LTRConfig: ltr},
			schedule: noSchedule,
			wantErr:  "",
		},
		{
			name:            "V1 invalid: ltr config with backup location",
			config:          &gqlsla.AzureDBConfig{LTRConfig: ltr},
			schedule:        noSchedule,
			backupLocations: backupLoc,
			wantErr:         "must not specify a backup_location",
		},
		{
			name:     "V1 invalid: ltr config with Rubrik schedule",
			config:   &gqlsla.AzureDBConfig{LTRConfig: ltr},
			schedule: withSchedule,
			wantErr:  "must not specify a Rubrik snapshot schedule",
		},
		{
			name:          "V1 invalid: ltr config with archival block",
			config:        &gqlsla.AzureDBConfig{LTRConfig: ltr},
			schedule:      noSchedule,
			archivalSpecs: archival,
			wantErr:       "must not specify an archival location",
		},
		{
			name:            "V2 valid: backup location and schedule",
			config:          &gqlsla.AzureDBConfig{LogRetentionInDays: 7},
			schedule:        withSchedule,
			backupLocations: backupLoc,
			wantErr:         "",
		},
		{
			name:     "V2 invalid: missing backup location",
			config:   &gqlsla.AzureDBConfig{LogRetentionInDays: 7},
			schedule: withSchedule,
			wantErr:  "requires a backup_location",
		},
		{
			name:            "V2 invalid: backup location but no schedule",
			config:          &gqlsla.AzureDBConfig{LogRetentionInDays: 7},
			schedule:        noSchedule,
			backupLocations: backupLoc,
			wantErr:         "requires a snapshot schedule",
		},
		{
			name:            "V2 invalid: legacy archival block present",
			config:          &gqlsla.AzureDBConfig{LogRetentionInDays: 7},
			schedule:        withSchedule,
			backupLocations: backupLoc,
			archivalSpecs:   archival,
			wantErr:         "remove the archival block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAzureSQLSLA("Azure SQL Database", tt.config, tt.schedule, tt.backupLocations, tt.archivalSpecs)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("expected no error, got: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("expected error containing %q, got: %q", tt.wantErr, err.Error())
			}
		})
	}
}

// TestValidateAzureSQLObjectTypeFeatureGate verifies the CNP_AZURE_SQL_SLA_REVAMP
// gating: when disabled, the legacy Azure SQL model applies (instant archival
// required, ltr_config rejected); when enabled, the V1/V2 model applies.
func TestValidateAzureSQLObjectTypeFeatureGate(t *testing.T) {
	ltr := &gqlsla.AzureDBConfig{LTRConfig: &gqlsla.AzureSQLLTRConfig{
		WeeklyBackupRetention: &gqlsla.AzureSQLLTRRetention{Retention: 1, RetentionUnit: gqlsla.Weeks},
	}}
	plain := &gqlsla.AzureDBConfig{LogRetentionInDays: 7}
	instantArchival := []gqlsla.ArchivalSpec{{Threshold: 0}}
	delayedArchival := []gqlsla.ArchivalSpec{{Threshold: 1}}
	backupLoc := []gqlsla.BackupLocationSpec{{ArchivalGroupID: uuid.New()}}
	withSchedule := gqlsla.SnapshotSchedule{Daily: &gqlsla.DailySnapshotSchedule{}}
	dbTypes := []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase}
	miTypes := []gqlsla.ObjectType{gqlsla.ObjectAzureSQLManagedInstance}

	t.Run("DB legacy: instant archival accepted", func(t *testing.T) {
		if err := validateAzureSQLDatabaseObjectType(false, dbTypes, plain, gqlsla.SnapshotSchedule{}, nil, instantArchival, nil); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
	t.Run("DB legacy: ltr_config rejected with feature error", func(t *testing.T) {
		err := validateAzureSQLDatabaseObjectType(false, dbTypes, ltr, gqlsla.SnapshotSchedule{}, nil, instantArchival, nil)
		if !errors.Is(err, errLTRConfigRequiresFeature) {
			t.Fatalf("expected errLTRConfigRequiresFeature, got %v", err)
		}
	})
	t.Run("DB legacy: non-instant archival rejected", func(t *testing.T) {
		err := validateAzureSQLDatabaseObjectType(false, dbTypes, plain, gqlsla.SnapshotSchedule{}, nil, delayedArchival, nil)
		if err == nil || !strings.Contains(err.Error(), "instant archival") {
			t.Fatalf("expected instant archival error, got %v", err)
		}
	})
	t.Run("DB revamp: V2 backup location accepted", func(t *testing.T) {
		if err := validateAzureSQLDatabaseObjectType(true, dbTypes, plain, withSchedule, backupLoc, nil, nil); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
	t.Run("MI legacy: plain accepted", func(t *testing.T) {
		if err := validateAzureSQLManagedInstanceObjectType(false, miTypes, plain, nil, gqlsla.SnapshotSchedule{}, nil, nil, nil); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
	t.Run("MI legacy: ltr_config rejected with feature error", func(t *testing.T) {
		err := validateAzureSQLManagedInstanceObjectType(false, miTypes, ltr, nil, gqlsla.SnapshotSchedule{}, nil, nil, nil)
		if !errors.Is(err, errLTRConfigRequiresFeature) {
			t.Fatalf("expected errLTRConfigRequiresFeature, got %v", err)
		}
	})
	t.Run("MI legacy: archival rejected", func(t *testing.T) {
		err := validateAzureSQLManagedInstanceObjectType(false, miTypes, plain, nil, gqlsla.SnapshotSchedule{}, nil, instantArchival, nil)
		if err == nil || !strings.Contains(err.Error(), "does not support archival") {
			t.Fatalf("expected archival error, got %v", err)
		}
	})
}

// TestOnlyAzureSQLObjectTypes verifies that Azure SQL DB and MI may be combined
// with each other but not with any other object type.
// TestValidateAzurePostgresFlexibleServerObjectType covers the rules for an
// Azure Postgres flexible server SLA. Two of them are not expressed in the RSC
// schema: the object type cannot be combined with any other, matching the RSC
// UI, and a backup location is mandatory because these SLAs use
// backupLocationSpecs rather than the legacy archivalSpecs.
func TestValidateAWSConfigObjectType(t *testing.T) {
	config := gqlsla.ObjectAWSConfig
	hourly := func(frequency int) *gqlsla.HourlySnapshotSchedule {
		return &gqlsla.HourlySnapshotSchedule{BasicSchedule: gqlsla.BasicSnapshotSchedule{Frequency: frequency}}
	}
	tests := []struct {
		name                string
		objectTypes         []gqlsla.ObjectType
		schedule            gqlsla.SnapshotSchedule
		backupLocationCount int
		wantErr             string
	}{{
		name:     "DailyOnly",
		schedule: gqlsla.SnapshotSchedule{Daily: &gqlsla.DailySnapshotSchedule{}},
	}, {
		// Mirrors an SLA created in the RSC UI.
		name:                "HourlyWithOneBackupLocation",
		schedule:            gqlsla.SnapshotSchedule{Hourly: hourly(6)},
		backupLocationCount: 1,
	}, {
		name:                "MultipleBackupLocations",
		schedule:            gqlsla.SnapshotSchedule{Hourly: hourly(6)},
		backupLocationCount: 2,
		wantErr:             "at most one backup_location",
	}, {
		name:        "CombinedWithOtherObjectType",
		objectTypes: []gqlsla.ObjectType{config, gqlsla.ObjectAWSEC2EBS},
		schedule:    gqlsla.SnapshotSchedule{Hourly: hourly(6)},
		wantErr:     "cannot be combined with other object types",
	}, {
		name:     "HourlyAtMinimum",
		schedule: gqlsla.SnapshotSchedule{Hourly: hourly(6)},
	}, {
		name:     "HourlyAboveMinimum",
		schedule: gqlsla.SnapshotSchedule{Hourly: hourly(12)},
	}, {
		name:     "HourlyBelowMinimum",
		schedule: gqlsla.SnapshotSchedule{Hourly: hourly(5)},
		wantErr:  "at least 6 hours",
	}, {
		name:     "Minute",
		schedule: gqlsla.SnapshotSchedule{Minute: &gqlsla.MinuteSnapshotSchedule{}},
		wantErr:  "does not support a minute schedule",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectTypes := tt.objectTypes
			if objectTypes == nil {
				objectTypes = []gqlsla.ObjectType{config}
			}
			err := validateAWSConfigObjectType(objectTypes, tt.schedule, tt.backupLocationCount)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAzurePostgresFlexibleServerObjectType(t *testing.T) {
	pg := gqlsla.ObjectAzurePostgresFlexibleServer
	backupLocation := []gqlsla.BackupLocationSpec{{ArchivalGroupID: uuid.MustParse("f6b1f4e8-5d1e-4f4e-9b6f-3a1c2d5e7f90")}}

	tests := []struct {
		name             string
		objectTypes      []gqlsla.ObjectType
		backupLocations  []gqlsla.BackupLocationSpec
		archivalSpecs    []gqlsla.ArchivalSpec
		replicationSpecs []gqlsla.ReplicationSpec
		wantErr          string
	}{{
		name:            "ValidWithBackupLocation",
		objectTypes:     []gqlsla.ObjectType{pg},
		backupLocations: backupLocation,
	}, {
		name:            "CombinedWithOtherObjectType",
		objectTypes:     []gqlsla.ObjectType{pg, gqlsla.ObjectAzureSQLDatabase},
		backupLocations: backupLocation,
		wantErr:         "cannot be combined with other object types",
	}, {
		name:            "MissingBackupLocation",
		objectTypes:     []gqlsla.ObjectType{pg},
		backupLocations: nil,
		wantErr:         "requires a backup_location",
	}, {
		name:            "UsesLegacyArchivalBlock",
		objectTypes:     []gqlsla.ObjectType{pg},
		backupLocations: backupLocation,
		archivalSpecs:   []gqlsla.ArchivalSpec{{Threshold: 0}},
		wantErr:         "remove the archival block",
	}, {
		name:             "Replication",
		objectTypes:      []gqlsla.ObjectType{pg},
		backupLocations:  backupLocation,
		replicationSpecs: []gqlsla.ReplicationSpec{{}},
		wantErr:          "does not support replication",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAzurePostgresFlexibleServerObjectType(
				tt.objectTypes, tt.backupLocations, tt.archivalSpecs, tt.replicationSpecs)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateGCPBigQueryObjectType(t *testing.T) {
	bq := gqlsla.ObjectGCPBigQuery
	basic := func(frequency int) gqlsla.BasicSnapshotSchedule {
		return gqlsla.BasicSnapshotSchedule{Frequency: frequency}
	}
	hourly := func(frequency int) *gqlsla.HourlySnapshotSchedule {
		return &gqlsla.HourlySnapshotSchedule{BasicSchedule: basic(frequency)}
	}
	daily := func(frequency int) *gqlsla.DailySnapshotSchedule {
		return &gqlsla.DailySnapshotSchedule{BasicSchedule: basic(frequency)}
	}
	weekly := func(frequency int) *gqlsla.WeeklySnapshotSchedule {
		return &gqlsla.WeeklySnapshotSchedule{BasicSchedule: basic(frequency)}
	}

	tests := []struct {
		name             string
		objectTypes      []gqlsla.ObjectType
		schedule         gqlsla.SnapshotSchedule
		backupLocations  int
		archivalSpecs    int
		replicationSpecs int
		wantErr          string
	}{{
		name:            "ValidDaily",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Daily: daily(1)},
		backupLocations: 1,
	}, {
		name:            "ValidHourlyAtLimit",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Hourly: hourly(168)},
		backupLocations: 1,
	}, {
		name:            "ValidDailyAtLimit",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Daily: daily(7)},
		backupLocations: 1,
	}, {
		name:            "ValidWeeklyAtLimit",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Weekly: weekly(1)},
		backupLocations: 2,
	}, {
		name:            "ValidMostFrequentScheduleWithinLimit",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Daily: daily(1), Weekly: weekly(2), Monthly: &gqlsla.MonthlySnapshotSchedule{}},
		backupLocations: 1,
	}, {
		name:            "CombinedWithOtherObjectType",
		objectTypes:     []gqlsla.ObjectType{bq, gqlsla.ObjectGCPCloudSQL},
		schedule:        gqlsla.SnapshotSchedule{Daily: daily(1)},
		backupLocations: 1,
		wantErr:         "cannot be combined with other object types",
	}, {
		name:            "MissingBackupLocation",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Daily: daily(1)},
		backupLocations: 0,
		wantErr:         "requires a backup_location",
	}, {
		name:            "UsesLegacyArchivalBlock",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Daily: daily(1)},
		backupLocations: 1,
		archivalSpecs:   1,
		wantErr:         "remove the archival block",
	}, {
		name:             "Replication",
		objectTypes:      []gqlsla.ObjectType{bq},
		schedule:         gqlsla.SnapshotSchedule{Daily: daily(1)},
		backupLocations:  1,
		replicationSpecs: 1,
		wantErr:          "does not support replication",
	}, {
		name:            "MinuteSchedule",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Minute: &gqlsla.MinuteSnapshotSchedule{}, Daily: daily(1)},
		backupLocations: 1,
		wantErr:         "does not support a minute schedule",
	}, {
		name:            "HourlyOverLimit",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Hourly: hourly(169)},
		backupLocations: 1,
		wantErr:         "at least every 7 days",
	}, {
		name:            "DailyOverLimit",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Daily: daily(8)},
		backupLocations: 1,
		wantErr:         "at least every 7 days",
	}, {
		name:            "WeeklyOverLimit",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Weekly: weekly(2)},
		backupLocations: 1,
		wantErr:         "at least every 7 days",
	}, {
		name:            "MonthlyOnly",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Monthly: &gqlsla.MonthlySnapshotSchedule{}},
		backupLocations: 1,
		wantErr:         "at least every 7 days",
	}, {
		name:            "QuarterlyAndYearlyOnly",
		objectTypes:     []gqlsla.ObjectType{bq},
		schedule:        gqlsla.SnapshotSchedule{Quarterly: &gqlsla.QuarterlySnapshotSchedule{}, Yearly: &gqlsla.YearlySnapshotSchedule{}},
		backupLocations: 1,
		wantErr:         "at least every 7 days",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGCPBigQueryObjectType(
				tt.objectTypes, tt.schedule, tt.backupLocations, tt.archivalSpecs, tt.replicationSpecs)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAzurePostgresFlexibleServerConfig(t *testing.T) {
	tests := []struct {
		name          string
		configPresent bool
		objectTypes   []gqlsla.ObjectType
		wantErr       string
	}{{
		name:          "ConfigWithOwnObjectType",
		configPresent: true,
		objectTypes:   []gqlsla.ObjectType{gqlsla.ObjectAzurePostgresFlexibleServer},
	}, {
		name:          "NoConfigOtherObjectType",
		configPresent: false,
		objectTypes:   []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase},
	}, {
		name:          "ConfigWithDifferentObjectType",
		configPresent: true,
		objectTypes:   []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase},
		wantErr:       "is only valid when object_types is",
	}, {
		name:          "ConfigWithNoObjectTypes",
		configPresent: true,
		objectTypes:   []gqlsla.ObjectType{},
		wantErr:       "is only valid when object_types is",
	}, {
		name:          "ConfigWithNilObjectTypes",
		configPresent: true,
		objectTypes:   nil,
		wantErr:       "is only valid when object_types is",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAzurePostgresFlexibleServerConfig(tt.configPresent, tt.objectTypes)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestOnlyAzureSQLObjectTypes(t *testing.T) {
	tests := []struct {
		name        string
		objectTypes []gqlsla.ObjectType
		want        bool
	}{
		{"DB only", []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase}, true},
		{"MI only", []gqlsla.ObjectType{gqlsla.ObjectAzureSQLManagedInstance}, true},
		{"DB + MI", []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase, gqlsla.ObjectAzureSQLManagedInstance}, true},
		{"DB + Blob", []gqlsla.ObjectType{gqlsla.ObjectAzureSQLDatabase, gqlsla.ObjectAzureBlob}, false},
		{"MI + other", []gqlsla.ObjectType{gqlsla.ObjectAzureSQLManagedInstance, "AWS_EC2_EBS_OBJECT_TYPE"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := onlyAzureSQLObjectTypes(tt.objectTypes); got != tt.want {
				t.Fatalf("onlyAzureSQLObjectTypes(%v) = %v, want %v", tt.objectTypes, got, tt.want)
			}
		})
	}
}

// TestScheduleEmpty verifies scheduleEmpty reports true only when no Rubrik
// snapshot schedule of any frequency is set.
func TestScheduleEmpty(t *testing.T) {
	if !scheduleEmpty(gqlsla.SnapshotSchedule{}) {
		t.Fatal("expected empty schedule to report empty")
	}
	schedules := []gqlsla.SnapshotSchedule{
		{Daily: &gqlsla.DailySnapshotSchedule{}},
		{Hourly: &gqlsla.HourlySnapshotSchedule{}},
		{Minute: &gqlsla.MinuteSnapshotSchedule{}},
		{Monthly: &gqlsla.MonthlySnapshotSchedule{}},
		{Quarterly: &gqlsla.QuarterlySnapshotSchedule{}},
		{Weekly: &gqlsla.WeeklySnapshotSchedule{}},
		{Yearly: &gqlsla.YearlySnapshotSchedule{}},
	}
	for _, s := range schedules {
		if scheduleEmpty(s) {
			t.Fatalf("expected non-empty schedule to report non-empty: %+v", s)
		}
	}
}
