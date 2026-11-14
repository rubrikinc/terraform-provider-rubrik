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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// singleBlock returns the block of a converted value: a slice of one element, or
// nil when the value is nil.
func singleBlock[T any](v *T) []T {
	if v == nil {
		return nil
	}
	return []T{*v}
}

// slaDomainModelFromRemote converts an SLA domain read from RSC to a model. The
// prior model is the plan or the state the read is made for. It holds what RSC
// doesn't return, and decides the order of the archival and backup location
// blocks, and which archival frequencies are kept.
func slaDomainModelFromRemote(ctx context.Context, domain gqlsla.Domain, prior slaDomainModel) (slaDomainModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	objectTypes := make([]string, 0, len(domain.ObjectTypes))
	for _, objectType := range domain.ObjectTypes {
		objectTypes = append(objectTypes, string(objectType))
	}
	objectTypeSet, d := types.SetValueFrom(ctx, types.StringType, objectTypes)
	diags.Append(d...)

	configs := domain.ObjectSpecificConfigs
	model := slaDomainModel{
		ID:                               types.StringValue(domain.ID.String()),
		Name:                             types.StringValue(domain.Name),
		Description:                      types.StringValue(domain.Description),
		ObjectTypes:                      objectTypeSet,
		BackupType:                       types.StringValue(string(domain.BackupType)),
		ApplyChangesToExistingSnapshots:  prior.ApplyChangesToExistingSnapshots,
		ApplyChangesToNonPolicySnapshots: prior.ApplyChangesToNonPolicySnapshots,
		Timeouts:                         prior.Timeouts,

		DailySchedule:     singleBlock(toDailyScheduleModel(domain.SnapshotSchedule.Daily)),
		HourlySchedule:    singleBlock(toHourlyScheduleModel(domain.SnapshotSchedule.Hourly)),
		MinuteSchedule:    singleBlock(toMinuteScheduleModel(domain.SnapshotSchedule.Minute)),
		MonthlySchedule:   singleBlock(toMonthlyScheduleModel(domain.SnapshotSchedule.Monthly)),
		QuarterlySchedule: singleBlock(toQuarterlyScheduleModel(domain.SnapshotSchedule.Quarterly)),
		WeeklySchedule:    singleBlock(toWeeklyScheduleModel(domain.SnapshotSchedule.Weekly)),
		YearlySchedule:    singleBlock(toYearlyScheduleModel(domain.SnapshotSchedule.Yearly)),

		AWSDynamoDBConfig:                 toAWSDynamoDBConfigModel(configs.AWSDynamoDBConfig),
		AWSRDSConfig:                      toAWSRDSConfigModel(configs.AWSRDSConfig),
		AzurePostgresFlexibleServerConfig: toAzurePostgresFlexibleServerConfigModel(configs.AzurePostgresFlexibleServerConfig),
		AzureSQLDatabaseConfig:            toAzureSQLConfigModel(configs.AzureSQLDatabaseDBConfig),
		AzureSQLManagedInstanceConfig:     toAzureSQLConfigModel(configs.AzureSQLManagedInstanceDBConfig),
		VMwareVMConfig:                    toVMwareVMConfigModel(configs.VMwareVMConfig),
		SapHanaConfig:                     toSapHanaConfigModel(configs.SapHanaConfig),
		DB2Config:                         toDB2ConfigModel(configs.DB2Config),
		MSSQLConfig:                       toMSSQLConfigModel(configs.MssqlConfig),
		OracleConfig:                      toOracleConfigModel(configs.OracleConfig),
		MongoConfig:                       toMongoConfigModel(configs.MongoConfig),
		ManagedVolumeConfig:               toManagedVolumeConfigModel(configs.ManagedVolumeSlaConfig),
		PostgresDBClusterConfig:           toPostgresDBClusterConfigModel(configs.PostgresDbClusterSlaConfig),
		MySQLDBConfig:                     toMySQLDBConfigModel(configs.MysqldbSlaConfig),
		InformixConfig:                    toInformixConfigModel(configs.InformixSlaConfig),
		GCPCloudSQLConfig:                 toGCPCloudSQLConfigModel(configs.GcpCloudSqlConfig),
		NCDConfig:                         toNCDConfigModel(configs.NcdSlaConfig),

		ReplicationSpec: toReplicationSpecModels(replicationSpecsFromDomain(domain)),
		RetentionLock:   singleBlock(toRetentionLockModel(domain)),
	}

	if blob := configs.AzureBlobConfig; blob != nil {
		model.AzureBlobConfig = []azureBlobConfigModel{{ArchivalLocationID: types.StringValue(blob.BackupLocationID.String())}}
	}

	if limit := domain.LocalRetentionLimit; limit != nil {
		model.LocalRetention = singleBlock(toLocalRetentionModel(&gqlsla.RetentionDuration{
			Duration: limit.Duration,
			Unit:     limit.Unit,
		}))
	}

	archival, err := toArchivalModels(domain, prior.Archival)
	if err != nil {
		diags.AddError("Failed to convert the archival locations", err.Error())
		return model, diags
	}

	// Only keep the frequencies of an archival block when they were configured.
	// This avoids a perpetual diff when the frequencies are derived from the
	// snapshot schedule.
	explicitFrequencies := make(map[string]bool)
	for _, old := range prior.Archival {
		if len(old.Frequency) > 0 {
			explicitFrequencies[old.ArchivalLocationID.ValueString()] = true
		}
	}
	for i := range archival {
		if !explicitFrequencies[archival[i].ArchivalLocationID.ValueString()] {
			archival[i].Frequency = nil
		}
	}
	model.Archival = archival

	// AWS S3 is supported in two ways, either using backup location specs, or
	// using object specific configs if multiple backup locations aren't enabled.
	model.BackupLocation, err = toBackupLocationModels(domain, prior.BackupLocation)
	if err != nil {
		diags.AddError("Failed to convert the backup locations", err.Error())
		return model, diags
	}

	model.SnapshotWindow, err = toSnapshotWindowModels(domain.BackupWindows)
	if err != nil {
		diags.AddError("Failed to convert the snapshot windows", err.Error())
		return model, diags
	}
	model.FirstFullSnapshot, err = toSnapshotWindowModels(domain.FirstFullBackupWindows)
	if err != nil {
		diags.AddError("Failed to convert the first full snapshot windows", err.Error())
		return model, diags
	}

	return model, diags
}

// replicationSpecsFromDomain converts the replication specs of an SLA domain to
// the SDK type the replication models are converted from.
func replicationSpecsFromDomain(domain gqlsla.Domain) []gqlsla.ReplicationSpec {
	var replicationSpecs []gqlsla.ReplicationSpec
	for _, spec := range domain.ReplicationSpecs {
		var replicationPairs []gqlsla.ReplicationPair
		for _, pair := range spec.ReplicationPairs {
			replicationPairs = append(replicationPairs, gqlsla.ReplicationPair{
				SourceClusterID: pair.SourceCluster.ID,
				TargetClusterID: pair.TargetCluster.ID,
			})
		}

		var cascadingArchivalSpecs []gqlsla.CascadingArchivalSpec
		for _, cas := range spec.CascadingArchivalSpecs {
			cascadingSpec := gqlsla.CascadingArchivalSpec{
				ArchivalThreshold: cas.ArchivalThreshold,
				Frequencies:       cas.Frequencies,
			}

			if cas.ArchivalLocation != nil {
				if locationID, err := uuid.Parse(cas.ArchivalLocation.ID); err == nil {
					//lint:ignore SA1019 Allow deprecated field when reading
					cascadingSpec.ArchivalLocationID = &locationID
				}
			}

			if cas.ArchivalTieringSpec != nil {
				cascadingSpec.ArchivalTieringSpec = &gqlsla.ArchivalTieringSpec{
					InstantTiering:                 cas.ArchivalTieringSpec.InstantTiering,
					MinAccessibleDurationInSeconds: cas.ArchivalTieringSpec.MinAccessibleDurationInSeconds,
					ColdStorageClass:               cas.ArchivalTieringSpec.ColdStorageClass,
					TierExistingSnapshots:          cas.ArchivalTieringSpec.TierExistingSnapshots,
				}
			}

			// A mapping with an ID which isn't a UUID is skipped.
			for _, mapping := range cas.ArchivalLocationToClusterMapping {
				clusterID, err := uuid.Parse(mapping.Cluster.ID)
				if err != nil {
					continue
				}
				locationID, err := uuid.Parse(mapping.Location.ID)
				if err != nil {
					continue
				}
				cascadingSpec.ArchivalLocationToClusterMappings = append(cascadingSpec.ArchivalLocationToClusterMappings,
					gqlsla.ArchivalLocationToClusterMapping{ClusterID: clusterID, LocationID: locationID})
			}

			cascadingArchivalSpecs = append(cascadingArchivalSpecs, cascadingSpec)
		}

		replicationSpecs = append(replicationSpecs, gqlsla.ReplicationSpec{
			AWSRegion:   spec.AWSRegion,
			AWSAccount:  spec.AWS.AccountID,
			AzureRegion: spec.AzureRegion,
			RetentionDuration: &gqlsla.RetentionDuration{
				Duration: spec.RetentionDuration.Duration,
				Unit:     spec.RetentionDuration.Unit,
			},
			ReplicationLocalRetentionDuration: spec.ReplicationLocalRetentionDuration,
			CascadingArchivalSpecs:            cascadingArchivalSpecs,
			ReplicationPairs:                  replicationPairs,
		})
	}

	return replicationSpecs
}

// slaDomainStateFromRemote converts an SLA domain read from RSC to the state of
// the resource. The attributes which RSC returns as zero values are null when
// they are null in the prior, so an optional attribute which wasn't configured
// doesn't change.
//
// The prior is the plan when creating and updating, and the state when reading.
func slaDomainStateFromRemote(ctx context.Context, s schema.Schema, domain gqlsla.Domain, prior tfsdk.Plan) (slaDomainModel, diag.Diagnostics) {
	var priorModel slaDomainModel
	diags := prior.Get(ctx, &priorModel)
	if diags.HasError() {
		return slaDomainModel{}, diags
	}

	model, d := slaDomainModelFromRemote(ctx, domain, priorModel)
	diags.Append(d...)
	if diags.HasError() {
		return model, diags
	}

	// The remote value is converted to a framework object, so the zero values
	// can be compared with the prior in the type neutral form.
	remote := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	diags.Append(remote.Set(ctx, &model)...)
	if diags.HasError() {
		return model, diags
	}

	// Right after an import there is no configuration to compare with.
	kept := keepNullsFromPrior(remote.Raw, prior.Raw)
	if isImportedPrior(prior.Raw) {
		var err error
		if kept, err = nullOptionalZeroValues(s, remote.Raw); err != nil {
			diags.AddError("Failed to convert the SLA domain", err.Error())
			return model, diags
		}
	}
	diags.Append(tfsdk.State{Schema: s, Raw: kept}.Get(ctx, &model)...)
	return model, diags
}
