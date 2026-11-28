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
	"fmt"
	"slices"

	"github.com/google/uuid"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// slaDomainFeatures are the RSC feature flags which change how an SLA domain is
// created and updated.
type slaDomainFeatures struct {
	// MultipleBackupLocations is CNP_AWS_S3_MULTIPLE_BACKUP_LOCATIONS_ENABLED.
	// AWS S3 is supported in two modes. The old mode uses a single backup
	// location with an object specific config, and the new mode uses multiple
	// backup locations.
	MultipleBackupLocations bool

	// AzureSQLRevamp is CNP_AZURE_SQL_SLA_REVAMP, which introduces the V1/V2
	// Azure SQL SLA model. When it is not enabled the legacy Azure SQL behavior
	// is kept, so existing configurations are not broken.
	AzureSQLRevamp bool
}

// fromFirstBlock converts the only element of a block, or returns nil when the
// block isn't configured.
func fromFirstBlock[M, S any](block []M, from func(M) *S) *S {
	if len(block) == 0 {
		return nil
	}
	return from(block[0])
}

// slaDomainParams converts an SLA domain model to the parameters for creating
// or updating the SLA domain. It validates the model against the object types,
// which needs no RSC. The feature flags are read by the caller.
func slaDomainParams(model slaDomainModel, objectTypes []gqlsla.ObjectType, features slaDomainFeatures) (gqlsla.CreateDomainParams, error) {
	// Parse snapshot schedule. Unspecified time frame schedules are nil.
	schedule := gqlsla.SnapshotSchedule{
		Daily:     fromFirstBlock(model.DailySchedule, fromDailyScheduleModel),
		Hourly:    fromFirstBlock(model.HourlySchedule, fromHourlyScheduleModel),
		Minute:    fromFirstBlock(model.MinuteSchedule, fromMinuteScheduleModel),
		Monthly:   fromFirstBlock(model.MonthlySchedule, fromMonthlyScheduleModel),
		Quarterly: fromFirstBlock(model.QuarterlySchedule, fromQuarterlyScheduleModel),
		Weekly:    fromFirstBlock(model.WeeklySchedule, fromWeeklyScheduleModel),
		Yearly:    fromFirstBlock(model.YearlySchedule, fromYearlyScheduleModel),
	}

	archivalSpecs, err := fromArchivalModels(model.Archival, schedule)
	if err != nil {
		return gqlsla.CreateDomainParams{}, err
	}
	blobConfig, err := fromAzureBlobConfigModel(model.AzureBlobConfig)
	if err != nil {
		return gqlsla.CreateDomainParams{}, err
	}
	firstFullSnapshotWindows, err := fromSnapshotWindowModels(model.FirstFullSnapshot)
	if err != nil {
		return gqlsla.CreateDomainParams{}, err
	}
	snapshotWindows, err := fromSnapshotWindowModels(model.SnapshotWindow)
	if err != nil {
		return gqlsla.CreateDomainParams{}, err
	}
	replicationSpecs, err := fromReplicationSpecModels(model.ReplicationSpec)
	if err != nil {
		return gqlsla.CreateDomainParams{}, err
	}

	azureSQLConfig := fromAzureSQLConfigModel(model.AzureSQLDatabaseConfig)
	azureSQLMIConfig := fromAzureSQLConfigModel(model.AzureSQLManagedInstanceConfig)

	var retentionLockMode gqlsla.RetentionLockMode
	if rl := firstOrNil(model.RetentionLock); rl != nil {
		retentionLockMode = gqlsla.RetentionLockMode(rl.Mode.ValueString())

		// Validate that COMPLIANCE mode requires acknowledgment.
		if retentionLockMode == gqlsla.Compliance && !rl.ComplianceAcknowledge.ValueBool() {
			return gqlsla.CreateDomainParams{}, errors.New("compliance_mode_acknowledgment must be set to true when " +
				"using COMPLIANCE mode. This acknowledges that snapshots protected under compliance mode cannot be " +
				"deleted before the scheduled expiry date. Compliance mode is recommended to meet regulations and " +
				"governance mode is recommended to only protect data")
		}
	}

	// backup_location is routed to one of two places depending on the object
	// type. AWS Config, Azure Postgres Flexible Server, Azure SQL (V1/V2 model)
	// and GCP BigQuery store their backup location in the SLA-level backup
	// location specs. AWS S3 uses the specs when the multiple backup locations
	// feature is enabled and the legacy object specific config otherwise. No
	// other object type accepts a backup location.
	//
	// Azure SQL V1 (Azure-managed) SLAs must not carry a backup location at
	// all. They are routed to the specs together with V2 so validateAzureSQLSLA
	// sees the location and rejects it, rather than it silently being sent as
	// an AWS S3 config.
	hasAWSConfig := slices.Contains(objectTypes, gqlsla.ObjectAWSConfig)
	hasAWSS3 := slices.Contains(objectTypes, gqlsla.ObjectAWSS3)
	hasAzurePostgres := slices.Contains(objectTypes, gqlsla.ObjectAzurePostgresFlexibleServer)
	hasAzureSQL := slices.Contains(objectTypes, gqlsla.ObjectAzureSQLDatabase) ||
		slices.Contains(objectTypes, gqlsla.ObjectAzureSQLManagedInstance)
	hasGCPBigQuery := slices.Contains(objectTypes, gqlsla.ObjectGCPBigQuery)

	var backupLocations []gqlsla.BackupLocationSpec
	var awsS3Config *gqlsla.AWSS3Config
	switch {
	case hasAWSS3 && !features.MultipleBackupLocations:
		if awsS3Config, err = fromAWSS3ConfigModels(model.BackupLocation); err != nil {
			return gqlsla.CreateDomainParams{}, err
		}
	case hasAWSConfig || hasAWSS3 || hasAzurePostgres || (hasAzureSQL && features.AzureSQLRevamp) || hasGCPBigQuery:
		backupLocations = fromBackupLocationModels(model.BackupLocation)
	case len(model.BackupLocation) > 0:
		return gqlsla.CreateDomainParams{}, fmt.Errorf("%s is not supported by the configured object types", keyBackupLocation)
	}

	// Per object type validation.
	for _, objectType := range objectTypes {
		switch objectType {
		case gqlsla.ObjectActiveDirectory:
			if schedule.Hourly != nil && schedule.Hourly.BasicSchedule.Frequency < 4 {
				return gqlsla.CreateDomainParams{}, errors.New("the Active Directory object type requires minimum of 4 hours SLA")
			}
		case gqlsla.ObjectAzureSQLDatabase:
			if err := validateAzureSQLDatabaseObjectType(features.AzureSQLRevamp, objectTypes, azureSQLConfig, schedule, backupLocations, archivalSpecs, replicationSpecs); err != nil {
				return gqlsla.CreateDomainParams{}, err
			}
		case gqlsla.ObjectAzureSQLManagedInstance:
			if err := validateAzureSQLManagedInstanceObjectType(features.AzureSQLRevamp, objectTypes, azureSQLMIConfig, azureSQLConfig, schedule, backupLocations, archivalSpecs, replicationSpecs); err != nil {
				return gqlsla.CreateDomainParams{}, err
			}
		case gqlsla.ObjectAzurePostgresFlexibleServer:
			if err := validateAzurePostgresFlexibleServerObjectType(objectTypes, backupLocations, archivalSpecs, replicationSpecs); err != nil {
				return gqlsla.CreateDomainParams{}, err
			}
		case gqlsla.ObjectAzureBlob:
			if blobConfig == nil {
				return gqlsla.CreateDomainParams{}, errors.New("the Azure Blob object type requires Azure Blob configuration")
			}
		case gqlsla.ObjectAWSS3:
			if len(objectTypes) > 1 {
				return gqlsla.CreateDomainParams{}, errors.New("the AWS S3 object type cannot be combined with other object types")
			}
			if features.MultipleBackupLocations && len(backupLocations) == 0 {
				return gqlsla.CreateDomainParams{}, errors.New("the AWS S3 object type requires at least one backup location")
			}
			if !features.MultipleBackupLocations && awsS3Config == nil {
				return gqlsla.CreateDomainParams{}, errors.New("the AWS S3 object type requires AWS S3 configuration")
			}
		case gqlsla.ObjectMicrosoft365:
			if len(snapshotWindows) > 0 {
				return gqlsla.CreateDomainParams{}, errors.New("the Microsoft 365 object type does not support snapshot windows")
			}
			if len(firstFullSnapshotWindows) > 0 {
				return gqlsla.CreateDomainParams{}, errors.New("the Microsoft 365 object type does not support first full snapshot windows")
			}
			if schedule.Hourly != nil && schedule.Hourly.BasicSchedule.Frequency < 8 {
				return gqlsla.CreateDomainParams{}, errors.New("the Microsoft 365 object type requires minimum of 8 hours SLA")
			}
		case gqlsla.ObjectOLVM:
			if len(archivalSpecs) > 0 {
				return gqlsla.CreateDomainParams{}, errors.New("the OLVM object type does not support archival locations")
			}
		case gqlsla.ObjectCassandra:
			if len(archivalSpecs) > 0 {
				return gqlsla.CreateDomainParams{}, errors.New("the Cassandra object type does not support archival locations")
			}
			if len(replicationSpecs) > 0 {
				return gqlsla.CreateDomainParams{}, errors.New("the Cassandra object type does not support replication")
			}
		case gqlsla.ObjectMongoDB:
			if len(replicationSpecs) > 0 {
				return gqlsla.CreateDomainParams{}, errors.New("the MongoDB object type does not support replication")
			}
		}
	}

	return gqlsla.CreateDomainParams{
		ArchivalSpecs:          archivalSpecs,
		BackupLocationSpecs:    backupLocations,
		BackupWindows:          snapshotWindows,
		Description:            model.Description.ValueString(),
		FirstFullBackupWindows: firstFullSnapshotWindows,
		LocalRetentionLimit:    fromLocalRetentionModel(firstOrNil(model.LocalRetention)),
		Name:                   model.Name.ValueString(),
		ObjectSpecificConfigs: &gqlsla.ObjectSpecificConfigs{
			AWSDynamoDBConfig:                 fromAWSDynamoDBConfigModel(model.AWSDynamoDBConfig),
			AWSS3Config:                       awsS3Config,
			AWSRDSConfig:                      fromAWSRDSConfigModel(model.AWSRDSConfig),
			AzureBlobConfig:                   blobConfig,
			AzurePostgresFlexibleServerConfig: fromAzurePostgresFlexibleServerConfigModel(model.AzurePostgresFlexibleServerConfig),
			AzureSQLDatabaseDBConfig:          azureSQLConfig,
			AzureSQLManagedInstanceDBConfig:   azureSQLMIConfig,
			VMwareVMConfig:                    fromVMwareVMConfigModel(model.VMwareVMConfig),
			SapHanaConfig:                     fromSapHanaConfigModel(model.SapHanaConfig),
			DB2Config:                         fromDB2ConfigModel(model.DB2Config),
			MssqlConfig:                       fromMSSQLConfigModel(model.MSSQLConfig),
			OracleConfig:                      fromOracleConfigModel(model.OracleConfig),
			MongoConfig:                       fromMongoConfigModel(model.MongoConfig),
			ManagedVolumeSlaConfig:            fromManagedVolumeConfigModel(model.ManagedVolumeConfig),
			PostgresDbClusterSlaConfig:        fromPostgresDBClusterConfigModel(model.PostgresDBClusterConfig),
			MysqldbSlaConfig:                  fromMySQLDBConfigModel(model.MySQLDBConfig),
			NcdSlaConfig:                      fromNCDConfigModel(model.NCDConfig),
			InformixSlaConfig:                 fromInformixConfigModel(model.InformixConfig),
			GcpCloudSqlConfig:                 fromGCPCloudSQLConfigModel(model.GCPCloudSQLConfig),
		},
		ObjectTypes:       objectTypes,
		ReplicationSpecs:  replicationSpecs,
		RetentionLock:     retentionLockMode != "" && retentionLockMode != gqlsla.NoLock,
		RetentionLockMode: retentionLockMode,
		SnapshotSchedule:  schedule,
	}, nil
}

// slaDomainUpdateParams returns the parameters for updating the SLA domain with
// the specified ID.
func slaDomainUpdateParams(id uuid.UUID, params gqlsla.CreateDomainParams, applyToExisting, applyToNonPolicy bool) gqlsla.UpdateDomainParams {
	// When updating a data center archival, RSC requires the group ID to be set
	// to nil. The archival specs are copied, since they are shared with the
	// parameters of the caller.
	params.ArchivalSpecs = slices.Clone(params.ArchivalSpecs)
	for i, spec := range params.ArchivalSpecs {
		if len(spec.ArchivalLocationToClusterMapping) > 0 {
			params.ArchivalSpecs[i].GroupID = uuid.Nil
		}
	}

	return gqlsla.UpdateDomainParams{
		ID:                              id,
		ShouldApplyToExistingSnapshots:  &gqlsla.BoolValue{Value: applyToExisting},
		ShouldApplyToNonPolicySnapshots: &gqlsla.BoolValue{Value: applyToNonPolicy},
		CreateDomainParams:              params,
	}
}
