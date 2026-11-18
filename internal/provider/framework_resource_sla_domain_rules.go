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

	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// errLTRConfigRequiresFeature is returned when ltr_config is used while the
// CNP_AZURE_SQL_SLA_REVAMP feature is not enabled for the account.
var errLTRConfigRequiresFeature = errors.New(
	"ltr_config requires the CNP_AZURE_SQL_SLA_REVAMP feature, which is not enabled for this account")

// validateAzureSQLDatabaseObjectType validates an Azure SQL Database SLA. When
// the CNP_AZURE_SQL_SLA_REVAMP feature is enabled it applies the V1/V2 model;
// otherwise it applies the legacy model (a single instant-archival location is
// required, no V1/V2 split) so configurations created before the feature
// continue to work unchanged.
func validateAzureSQLDatabaseObjectType(revamp bool, objectTypes []gqlsla.ObjectType, config *gqlsla.AzureDBConfig, schedule gqlsla.SnapshotSchedule, backupLocations []gqlsla.BackupLocationSpec, archivalSpecs []gqlsla.ArchivalSpec, replicationSpecs []gqlsla.ReplicationSpec) error {
	if config == nil {
		return fmt.Errorf("the Azure SQL Database object type requires Azure SQL Database configuration")
	}
	if len(replicationSpecs) > 0 {
		return fmt.Errorf("the Azure SQL Database object type does not support replication")
	}

	if !revamp {
		// Legacy behavior (feature disabled).
		if config.LTRConfig != nil {
			return errLTRConfigRequiresFeature
		}
		if len(objectTypes) > 1 {
			return fmt.Errorf("the Azure SQL Database object type cannot be combined with other object types")
		}
		if len(archivalSpecs) != 1 || archivalSpecs[0].Threshold != 0 {
			return fmt.Errorf("the Azure SQL Database object type requires an archival location with instant archival enabled")
		}
		return nil
	}

	// Revamp behavior (feature enabled): V1/V2 model.
	if !onlyAzureSQLObjectTypes(objectTypes) {
		return fmt.Errorf("the Azure SQL Database object type can only be combined with Azure SQL Managed Instance")
	}
	return validateAzureSQLSLA("Azure SQL Database", config, schedule, backupLocations, archivalSpecs)
}

// validateAzureSQLManagedInstanceObjectType validates an Azure SQL Managed
// Instance SLA, gated on the CNP_AZURE_SQL_SLA_REVAMP feature in the same way as
// validateAzureSQLDatabaseObjectType. Legacy MI SLAs do not support archival.
func validateAzureSQLManagedInstanceObjectType(revamp bool, objectTypes []gqlsla.ObjectType, miConfig, dbConfig *gqlsla.AzureDBConfig, schedule gqlsla.SnapshotSchedule, backupLocations []gqlsla.BackupLocationSpec, archivalSpecs []gqlsla.ArchivalSpec, replicationSpecs []gqlsla.ReplicationSpec) error {
	if miConfig == nil {
		return fmt.Errorf("the Azure SQL Managed Instance object type requires Azure SQL Managed Instance configuration")
	}
	if len(replicationSpecs) > 0 {
		return fmt.Errorf("the Azure SQL Managed Instance object type does not support replication")
	}

	if !revamp {
		// Legacy behavior (feature disabled).
		if miConfig.LTRConfig != nil {
			return errLTRConfigRequiresFeature
		}
		if dbConfig != nil {
			return fmt.Errorf("the Azure SQL Managed Instance object type cannot be combined with Azure SQL Database configuration")
		}
		if len(archivalSpecs) > 0 {
			return fmt.Errorf("the Azure SQL Managed Instance object type does not support archival locations")
		}
		return nil
	}

	// Revamp behavior (feature enabled): V1/V2 model.
	if !onlyAzureSQLObjectTypes(objectTypes) {
		return fmt.Errorf("the Azure SQL Managed Instance object type can only be combined with Azure SQL Database")
	}
	return validateAzureSQLSLA("Azure SQL Managed Instance", miConfig, schedule, backupLocations, archivalSpecs)
}

// validateAzureSQLSLA enforces the V1/V2 separation for Azure SQL Database and
// Managed Instance SLAs. A V1 (Azure-managed) SLA carries an LTR config and must
// not specify a Rubrik backup location, snapshot schedule, or archival location.
// A V2 (Rubrik-managed) SLA omits the LTR config and must specify a backup
// location and a snapshot schedule. The archival location for Azure SQL now
// lives in backup_location (renamed from "archival location"), not the top-level
// archival block.
func validateAzureSQLSLA(name string, config *gqlsla.AzureDBConfig, schedule gqlsla.SnapshotSchedule, backupLocations []gqlsla.BackupLocationSpec, archivalSpecs []gqlsla.ArchivalSpec) error {
	if config.LTRConfig != nil {
		// V1 (Azure-managed, long-term retention).
		if len(backupLocations) > 0 {
			return fmt.Errorf("%s V1 (Azure-managed) SLA with ltr_config must not specify a backup_location", name)
		}
		if !scheduleEmpty(schedule) {
			return fmt.Errorf("%s V1 (Azure-managed) SLA with ltr_config must not specify a Rubrik snapshot schedule", name)
		}
		if len(archivalSpecs) > 0 {
			return fmt.Errorf("%s V1 (Azure-managed) SLA with ltr_config must not specify an archival location", name)
		}
		return nil
	}

	// V2 (Rubrik-managed).
	if len(archivalSpecs) > 0 {
		return fmt.Errorf("%s stores its backup location in backup_location, not the archival block; remove the archival block", name)
	}
	if len(backupLocations) == 0 {
		return fmt.Errorf("%s V2 (Rubrik-managed) SLA requires a backup_location; set ltr_config for a V1 (Azure-managed) SLA", name)
	}
	if scheduleEmpty(schedule) {
		return fmt.Errorf("%s V2 (Rubrik-managed) SLA requires a snapshot schedule", name)
	}
	return nil
}

// validateAzurePostgresFlexibleServerConfig validates that the Azure Postgres
// flexible server configuration block is only used with its own object type,
// enforcing what the field documents.
//
// This is called from CustomizeDiff so that it fails during plan rather than
// part way through an apply. It cannot be a schema ValidateFunc: those receive
// only their own attribute's value, so they cannot see object_types, and a block
// cannot carry one at all.
func validateAzurePostgresFlexibleServerConfig(configPresent bool, objectTypes []gqlsla.ObjectType) error {
	if !configPresent {
		return nil
	}
	if !slices.Contains(objectTypes, gqlsla.ObjectAzurePostgresFlexibleServer) {
		return fmt.Errorf("%s is only valid when %s is %s", keyAzurePostgresFlexibleServerConfig,
			keyObjectTypes, gqlsla.ObjectAzurePostgresFlexibleServer)
	}

	return nil
}

// minAWSConfigHourlyFrequency is the smallest hourly snapshot frequency RSC
// accepts for an SLA carrying the AWS Config object type.
const minAWSConfigHourlyFrequency = 6

// validateAWSConfigObjectType validates an SLA carrying the AWS Config (Cloud
// Applications) object type.
//
// RSC rejects minute schedules, hourly frequencies below 6 hours and more than
// one backup location for this object type when the SLA is created or updated.
// The backup location itself is optional. The object type cannot be combined
// with any other, matching the RSC UI, which does not allow it.
//
// This is called from CustomizeDiff so that it fails during plan rather than
// part way through an apply.
func validateAWSConfigObjectType(objectTypes []gqlsla.ObjectType, schedule gqlsla.SnapshotSchedule, backupLocationCount int) error {
	if len(objectTypes) > 1 {
		return fmt.Errorf("the AWS Config object type cannot be combined with other object types")
	}
	if backupLocationCount > 1 {
		return fmt.Errorf("the AWS Config object type supports at most one backup_location")
	}
	if schedule.Minute != nil {
		return fmt.Errorf("the AWS Config object type does not support a minute schedule")
	}
	if schedule.Hourly != nil && schedule.Hourly.BasicSchedule.Frequency < minAWSConfigHourlyFrequency {
		return fmt.Errorf("the AWS Config object type requires an hourly frequency of at least %d hours",
			minAWSConfigHourlyFrequency)
	}

	return nil
}

// validateGCPBigQueryObjectType validates an SLA carrying the GCP BigQuery
// object type.
//
// RSC rejects minute schedules for this object type, and requires the most
// frequent schedule to take a snapshot at least every 7 days, since a longer
// interval outlasts the BigQuery change history window and turns every
// incremental backup into a full backup. Monthly, quarterly and yearly
// schedules are always further apart than that, so the SLA needs an hourly,
// daily or weekly schedule within the limit.
//
// The other rules are not expressed in the RSC schema. The object type cannot
// be combined with any other, matching the RSC UI, which does not allow it. A
// backup location is mandatory and the archival block is not used, because
// BigQuery backs up directly to its backup locations, which are carried in
// backupLocationSpecs rather than the legacy archivalSpecs.
//
// This is called from CustomizeDiff so that it fails during plan rather than
// part way through an apply.
func validateGCPBigQueryObjectType(objectTypes []gqlsla.ObjectType, schedule gqlsla.SnapshotSchedule, backupLocationCount, archivalCount, replicationCount int) error {
	if len(objectTypes) > 1 {
		return fmt.Errorf("the GCP BigQuery object type cannot be combined with other object types")
	}
	if archivalCount > 0 {
		return fmt.Errorf("the GCP BigQuery object type stores its backup location in backup_location, not the " +
			"archival block; remove the archival block")
	}
	if backupLocationCount == 0 {
		return fmt.Errorf("the GCP BigQuery object type requires a backup_location")
	}
	if replicationCount > 0 {
		return fmt.Errorf("the GCP BigQuery object type does not support replication")
	}
	if schedule.Minute != nil {
		return fmt.Errorf("the GCP BigQuery object type does not support a minute schedule")
	}

	// The interval is measured by the most frequent schedule, in hours.
	const maxIntervalHours = 7 * 24
	var intervals []int
	if schedule.Hourly != nil {
		intervals = append(intervals, schedule.Hourly.BasicSchedule.Frequency)
	}
	if schedule.Daily != nil {
		intervals = append(intervals, 24*schedule.Daily.BasicSchedule.Frequency)
	}
	if schedule.Weekly != nil {
		intervals = append(intervals, 7*24*schedule.Weekly.BasicSchedule.Frequency)
	}
	hasLongSchedule := schedule.Monthly != nil || schedule.Quarterly != nil || schedule.Yearly != nil
	if len(intervals) == 0 && !hasLongSchedule {
		return nil
	}
	if len(intervals) == 0 || slices.Min(intervals) > maxIntervalHours {
		return fmt.Errorf("the GCP BigQuery object type requires a snapshot at least every 7 days; add an " +
			"hourly, daily or weekly schedule with an interval of at most 7 days")
	}

	return nil
}

// validateAzurePostgresFlexibleServerObjectType validates an Azure Postgres
// flexible server SLA.
//
// Two of these rules are not expressed in the RSC schema. The object type
// cannot be combined with any other, matching the RSC UI, which does not allow
// it. A backup location is mandatory because the Postgres object adapter
// supports multiple backup locations but not multiple archival, so these SLAs
// carry their location in backupLocationSpecs rather than the legacy
// archivalSpecs.
func validateAzurePostgresFlexibleServerObjectType(objectTypes []gqlsla.ObjectType, backupLocations []gqlsla.BackupLocationSpec, archivalSpecs []gqlsla.ArchivalSpec, replicationSpecs []gqlsla.ReplicationSpec) error {
	if len(objectTypes) > 1 {
		return fmt.Errorf("the Azure Postgres Flexible Server object type cannot be combined with other object types")
	}
	if len(archivalSpecs) > 0 {
		return fmt.Errorf("the Azure Postgres Flexible Server object type stores its backup location in " +
			"backup_location, not the archival block; remove the archival block")
	}
	if len(backupLocations) == 0 {
		return fmt.Errorf("the Azure Postgres Flexible Server object type requires a backup_location")
	}
	if len(replicationSpecs) > 0 {
		return fmt.Errorf("the Azure Postgres Flexible Server object type does not support replication")
	}

	return nil
}

// onlyAzureSQLObjectTypes reports whether every object type in the list is an
// Azure SQL Database or Azure SQL Managed Instance. These two may be combined in
// a single SLA (matching the UI) but not with any other object type.
func onlyAzureSQLObjectTypes(objectTypes []gqlsla.ObjectType) bool {
	for _, ot := range objectTypes {
		switch ot {
		case gqlsla.ObjectAzureSQLDatabase, gqlsla.ObjectAzureSQLManagedInstance:
		default:
			return false
		}
	}
	return true
}

// scheduleEmpty reports whether no Rubrik snapshot schedule is configured.
func scheduleEmpty(s gqlsla.SnapshotSchedule) bool {
	return s.Daily == nil && s.Hourly == nil && s.Minute == nil && s.Monthly == nil &&
		s.Quarterly == nil && s.Weekly == nil && s.Yearly == nil
}
