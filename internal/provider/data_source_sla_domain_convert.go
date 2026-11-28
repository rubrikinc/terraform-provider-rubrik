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
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// The functions in this file convert an SLA domain read from RSC to the values
// of the SDKv2 sla_domain data source. They were the functions of the SDKv2
// sla_domain resource, and are removed when the data source is migrated.

// toArchival returns a slice holding the archival configuration.
func toArchival(domain gqlsla.Domain, existing []any) ([]any, error) {
	blocks := make(map[string]map[string]any)
	for _, spec := range domain.ArchivalSpecs {
		id := spec.StorageSetting.ID
		if blocks[id] != nil {
			return nil, fmt.Errorf("archival location %q used multiple times", id)
		}

		// Convert archival location to cluster mapping
		var mappings []any
		for _, mapping := range spec.ArchivalLocationToClusterMapping {
			mappings = append(mappings, map[string]any{
				keyClusterID:          mapping.Cluster.ID,
				keyClusterName:        mapping.Cluster.Name,
				keyArchivalLocationID: mapping.Location.ID,
				keyName:               mapping.Location.Name,
			})
		}

		block := map[string]any{
			keyArchivalLocationID:               id,
			keyThreshold:                        spec.Threshold,
			keyThresholdUnit:                    string(spec.ThresholdUnit),
			keyArchivalLocationToClusterMapping: mappings,
		}

		// Convert archival tiering spec
		if spec.ArchivalTieringSpec != nil {
			tieringMap := map[string]any{
				keyInstantTiering:                 spec.ArchivalTieringSpec.InstantTiering,
				keyMinAccessibleDurationInSeconds: spec.ArchivalTieringSpec.MinAccessibleDurationInSeconds,
				keyColdStorageClass:               spec.ArchivalTieringSpec.ColdStorageClass,
				keyTierExistingSnapshots:          spec.ArchivalTieringSpec.TierExistingSnapshots,
			}
			block[keyArchivalTiering] = []any{tieringMap}
		}

		if len(spec.Frequencies) > 0 {
			frequencies := &schema.Set{F: schema.HashString}
			for _, freq := range spec.Frequencies {
				frequencies.Add(string(freq))
			}
			block[keyFrequency] = frequencies
		} else {
			block[keyFrequency] = nil
		}

		blocks[id] = block
	}

	// Preserve order from existing, then add new ones to the end.
	var sorted []any
	for _, old := range existing {
		id := old.(map[string]any)[keyArchivalLocationID].(string)
		if block, ok := blocks[id]; ok {
			sorted = append(sorted, block)
			delete(blocks, id)
		}
	}

	// Add remaining blocks in the order they appear in archivalSpecs.
	for _, spec := range domain.ArchivalSpecs {
		id := spec.StorageSetting.ID
		if _, ok := blocks[id]; !ok {
			continue
		}
		sorted = append(sorted, blocks[id])
	}
	return sorted, nil
}

// toReplicationSpec returns a slice holding the replication configuration.
func toReplicationSpec(replicationSpecs []gqlsla.ReplicationSpec) []any {
	var replicationSpec []any
	for _, spec := range replicationSpecs {
		specMap := map[string]any{
			keyAWSRegion:       spec.AWSRegion.Name(),
			keyAWSCrossAccount: spec.AWSAccount,
			keyAzureRegion:     spec.AzureRegion.Name(),
			keyRetention:       spec.RetentionDuration.Duration,
			keyRetentionUnit:   spec.RetentionDuration.Unit,
		}

		var replicationPairs []any
		for _, pair := range spec.ReplicationPairs {
			replicationPairs = append(replicationPairs, map[string]any{
				keySourceCluster: pair.SourceClusterID,
				keyTargetCluster: pair.TargetClusterID,
			})
		}
		specMap[keyReplicationPair] = replicationPairs

		// Add replication local retention (only if present)
		if spec.ReplicationLocalRetentionDuration != nil {
			localRetentionMap := map[string]any{
				keyRetention:     spec.ReplicationLocalRetentionDuration.Duration,
				keyRetentionUnit: spec.ReplicationLocalRetentionDuration.Unit,
			}
			specMap[keyLocalRetention] = []any{localRetentionMap}
		}

		// Add cascading archival specs (only if present)
		if len(spec.CascadingArchivalSpecs) > 0 {
			var cascadingArchival []any
			for _, ca := range spec.CascadingArchivalSpecs {
				caMap := map[string]any{}

				// Extract archival location ID from the mapping
				// The mapping should have the same location ID for all clusters
				if len(ca.ArchivalLocationToClusterMappings) > 0 {
					caMap[keyArchivalLocationID] = ca.ArchivalLocationToClusterMappings[0].LocationID.String()
					//lint:ignore SA1019 Fallback to deprecated field when reading in the else branch below.
				} else if alID := ca.ArchivalLocationID; alID != nil {
					// Fallback to deprecated field if mapping is not present
					caMap[keyArchivalLocationID] = alID.String()
				}

				if ca.ArchivalThreshold != nil {
					caMap[keyArchivalThreshold] = ca.ArchivalThreshold.Duration
					caMap[keyArchivalThresholdUnit] = ca.ArchivalThreshold.Unit
				}

				if ca.ArchivalTieringSpec != nil {
					tieringMap := map[string]any{
						keyInstantTiering:                 ca.ArchivalTieringSpec.InstantTiering,
						keyMinAccessibleDurationInSeconds: ca.ArchivalTieringSpec.MinAccessibleDurationInSeconds,
						keyColdStorageClass:               ca.ArchivalTieringSpec.ColdStorageClass,
						keyTierExistingSnapshots:          ca.ArchivalTieringSpec.TierExistingSnapshots,
					}
					caMap[keyArchivalTiering] = []any{tieringMap}
				}

				if len(ca.Frequencies) > 0 {
					frequencies := &schema.Set{F: schema.HashString}
					for _, freq := range ca.Frequencies {
						frequencies.Add(string(freq))
					}
					caMap[keyFrequency] = frequencies
				}

				cascadingArchival = append(cascadingArchival, caMap)
			}
			specMap[keyCascadingArchival] = cascadingArchival
		}

		replicationSpec = append(replicationSpec, specMap)
	}

	return replicationSpec
}

// toSnapshotWindow returns a slice holding the snapshot window configuration.
func toSnapshotWindow(backupWindows []gqlsla.BackupWindow) ([]any, error) {
	var snapshotWindow []any
	for _, backupWindow := range backupWindows {
		startAt := fmt.Sprintf("%02d:%02d", backupWindow.StartTime.Hour, backupWindow.StartTime.Minute)
		if day := backupWindow.StartTime.DayOfWeek.Day; day != "" {
			wd, err := day.ToWeekday()
			if err != nil {
				return nil, err
			}
			startAt = wd.String()[:3] + ", " + startAt
		}
		snapshotWindow = append(snapshotWindow, map[string]any{
			keyDuration: backupWindow.DurationInHours,
			keyStartAt:  startAt,
		})
	}

	return snapshotWindow, nil
}

// toLocalRetention returns a map holding the source retention configuration or
// nil if the RetentionDuration is nil.
func toLocalRetention(localRetention *gqlsla.RetentionDuration) []any {
	if localRetention == nil {
		return nil
	}
	return []any{map[string]any{
		keyRetention:     localRetention.Duration,
		keyRetentionUnit: string(localRetention.Unit),
	}}
}

func toAWSDynamoDBConfig(dynamoDBConfig *gqlsla.AWSDynamoDBConfig) []any {
	if dynamoDBConfig == nil {
		return nil
	}

	return []any{map[string]any{
		keyKMSAlias: dynamoDBConfig.KMSAliasForPrimaryBackup,
	}}
}

func toAWSRDSConfig(rdsConfig *gqlsla.AWSRDSConfig) []any {
	if rdsConfig == nil {
		return nil
	}

	return []any{map[string]any{
		keyLogRetention:     rdsConfig.LogRetention.Duration,
		keyLogRetentionUnit: rdsConfig.LogRetention.Unit,
	}}
}

func toAzureSQLConfig(sqlConfig *gqlsla.AzureDBConfig) []any {
	if sqlConfig == nil {
		return nil
	}

	return []any{map[string]any{
		keyLogRetention: sqlConfig.LogRetentionInDays,
		keyLTRConfig:    toLTRConfig(sqlConfig.LTRConfig),
	}}
}

// toLTRConfig converts the SDK LTR config into the ltr_config schema block.
func toLTRConfig(ltr *gqlsla.AzureSQLLTRConfig) []any {
	if ltr == nil {
		return nil
	}

	block := map[string]any{
		keyWeeklyRetention:  toLTRRetention(ltr.WeeklyBackupRetention),
		keyMonthlyRetention: toLTRRetention(ltr.MonthlyBackupRetention),
	}
	if ltr.YearlyBackupRetention != nil {
		block[keyYearlyRetention] = []any{map[string]any{
			keyRetention:     ltr.YearlyBackupRetention.Retention.Retention,
			keyRetentionUnit: string(ltr.YearlyBackupRetention.Retention.RetentionUnit),
			keyWeekOfYear:    ltr.YearlyBackupRetention.WeekOfYear,
		}}
	}

	return []any{block}
}

// toLTRRetention converts a single SDK LTR retention into a schema block.
func toLTRRetention(r *gqlsla.AzureSQLLTRRetention) []any {
	if r == nil {
		return nil
	}

	return []any{map[string]any{
		keyRetention:     r.Retention,
		keyRetentionUnit: string(r.RetentionUnit),
	}}
}

func toVMwareVMConfig(vmwareConfig *gqlsla.VMwareVMConfig) []any {
	if vmwareConfig == nil {
		return nil
	}

	return []any{map[string]any{
		keyLogRetention: int(vmwareConfig.LogRetentionSeconds),
	}}
}

// toAzurePostgresFlexibleServerConfig returns the resource data representation
// of the Azure Postgres flexible server configuration.
func toAzurePostgresFlexibleServerConfig(config *gqlsla.AzurePostgresFlexibleServerConfig) []any {
	if config == nil {
		return nil
	}

	return []any{map[string]any{
		keyBackupRetentionInDays: config.BackupRetentionInDays,
	}}
}

func toBackupLocations(slaDomain gqlsla.Domain, existing []any) ([]any, error) {
	blocks := make(map[string]map[string]any)
	for _, spec := range slaDomain.BackupLocationSpecs {
		id := spec.ArchivalGroup.ID
		if blocks[id] != nil {
			return nil, fmt.Errorf("archival location %q used multiple times", id)
		}
		blocks[id] = map[string]any{
			keyArchivalGroupID: id,
		}
	}

	// Preserve order from existing, then add new ones to the end.
	var sorted []any
	for _, old := range existing {
		id := old.(map[string]any)[keyArchivalGroupID].(string)
		if block, ok := blocks[id]; ok {
			sorted = append(sorted, block)
			delete(blocks, id)
		}
	}

	// Add remaining blocks in the order they appear in backupLocationSpecs.
	for _, spec := range slaDomain.BackupLocationSpecs {
		id := spec.ArchivalGroup.ID
		if _, ok := blocks[id]; !ok {
			continue
		}
		sorted = append(sorted, blocks[id])
	}

	// AWS S3 fallback when multiple backup locations are not enabled.
	if len(sorted) == 0 && slaDomain.ObjectSpecificConfigs.AWSS3Config != nil {
		sorted = append(sorted, map[string]any{
			keyArchivalGroupID: slaDomain.ObjectSpecificConfigs.AWSS3Config.ArchivalLocationID.String(),
		})
	}
	return sorted, nil
}

func toDailySchedule(slaDomain gqlsla.Domain) []any {
	if slaDomain.SnapshotSchedule.Daily == nil {
		return nil
	}

	return []any{map[string]any{
		keyFrequency:     slaDomain.SnapshotSchedule.Daily.BasicSchedule.Frequency,
		keyRetention:     slaDomain.SnapshotSchedule.Daily.BasicSchedule.Retention,
		keyRetentionUnit: slaDomain.SnapshotSchedule.Daily.BasicSchedule.RetentionUnit,
	}}
}

func toHourlySchedule(slaDomain gqlsla.Domain) []any {
	if slaDomain.SnapshotSchedule.Hourly == nil {
		return nil
	}

	return []any{map[string]any{
		keyFrequency:     slaDomain.SnapshotSchedule.Hourly.BasicSchedule.Frequency,
		keyRetention:     slaDomain.SnapshotSchedule.Hourly.BasicSchedule.Retention,
		keyRetentionUnit: slaDomain.SnapshotSchedule.Hourly.BasicSchedule.RetentionUnit,
	}}
}

func toMinuteSchedule(slaDomain gqlsla.Domain) []any {
	if slaDomain.SnapshotSchedule.Minute == nil {
		return nil
	}

	return []any{map[string]any{
		keyFrequency:     slaDomain.SnapshotSchedule.Minute.BasicSchedule.Frequency,
		keyRetention:     slaDomain.SnapshotSchedule.Minute.BasicSchedule.Retention,
		keyRetentionUnit: slaDomain.SnapshotSchedule.Minute.BasicSchedule.RetentionUnit,
	}}
}

func toMonthlySchedule(slaDomain gqlsla.Domain) []any {
	if slaDomain.SnapshotSchedule.Monthly == nil {
		return nil
	}

	return []any{map[string]any{
		keyFrequency:     slaDomain.SnapshotSchedule.Monthly.BasicSchedule.Frequency,
		keyRetention:     slaDomain.SnapshotSchedule.Monthly.BasicSchedule.Retention,
		keyRetentionUnit: slaDomain.SnapshotSchedule.Monthly.BasicSchedule.RetentionUnit,
		keyDayOfMonth:    slaDomain.SnapshotSchedule.Monthly.DayOfMonth,
	}}
}

func toQuarterlySchedule(slaDomain gqlsla.Domain) []any {
	if slaDomain.SnapshotSchedule.Quarterly == nil {
		return nil
	}

	return []any{map[string]any{
		keyFrequency:         slaDomain.SnapshotSchedule.Quarterly.BasicSchedule.Frequency,
		keyRetention:         slaDomain.SnapshotSchedule.Quarterly.BasicSchedule.Retention,
		keyRetentionUnit:     slaDomain.SnapshotSchedule.Quarterly.BasicSchedule.RetentionUnit,
		keyDayOfQuarter:      slaDomain.SnapshotSchedule.Quarterly.DayOfQuarter,
		keyQuarterStartMonth: slaDomain.SnapshotSchedule.Quarterly.QuarterStartMonth,
	}}
}

func toWeeklySchedule(slaDomain gqlsla.Domain) []any {
	if slaDomain.SnapshotSchedule.Weekly == nil {
		return nil
	}

	return []any{map[string]any{
		keyFrequency:     slaDomain.SnapshotSchedule.Weekly.BasicSchedule.Frequency,
		keyRetention:     slaDomain.SnapshotSchedule.Weekly.BasicSchedule.Retention,
		keyRetentionUnit: slaDomain.SnapshotSchedule.Weekly.BasicSchedule.RetentionUnit,
		keyDayOfWeek:     slaDomain.SnapshotSchedule.Weekly.DayOfWeek,
	}}
}

func toYearlySchedule(slaDomain gqlsla.Domain) []any {
	if slaDomain.SnapshotSchedule.Yearly == nil {
		return nil
	}

	return []any{map[string]any{
		keyFrequency:      slaDomain.SnapshotSchedule.Yearly.BasicSchedule.Frequency,
		keyRetention:      slaDomain.SnapshotSchedule.Yearly.BasicSchedule.Retention,
		keyRetentionUnit:  slaDomain.SnapshotSchedule.Yearly.BasicSchedule.RetentionUnit,
		keyDayOfYear:      slaDomain.SnapshotSchedule.Yearly.DayOfYear,
		keyYearStartMonth: slaDomain.SnapshotSchedule.Yearly.YearStartMonth,
	}}
}
