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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *slaDomainResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "slaDomainResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(ctx),
	}
}

// moveStateV0 moves v0 state from the polaris_sla_domain resource in the
// rubrikinc/polaris provider to the rubrik_sla_domain resource. The SourceSchema
// is an inline literal frozen at the polaris V0 schema, so it remains correct
// when the schema of the rubrik resource changes. It only declares the types and
// whether an attribute is required, optional or computed, which is all the
// framework needs to decode the source state.
func (r *slaDomainResource) moveStateV0(ctx context.Context) resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyApplyChangesToExistingSnapshots:  schema.BoolAttribute{Optional: true},
				keyApplyChangesToNonPolicySnapshots: schema.BoolAttribute{Optional: true},
				keyBackupType:                       schema.StringAttribute{Computed: true},
				keyDescription:                      schema.StringAttribute{Optional: true},
				keyID:                               schema.StringAttribute{Computed: true},
				keyName:                             schema.StringAttribute{Required: true},
				keyObjectTypes:                      schema.SetAttribute{ElementType: types.StringType, Required: true},
			},
			Blocks: map[string]schema.Block{
				keyArchival: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyArchivalLocationID: schema.StringAttribute{Optional: true, Computed: true},
							keyFrequency:          schema.SetAttribute{ElementType: types.StringType, Optional: true},
							keyThreshold:          schema.Int64Attribute{Optional: true},
							keyThresholdUnit:      schema.StringAttribute{Optional: true},
						},
						Blocks: map[string]schema.Block{
							keyArchivalLocationToClusterMapping: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keyArchivalLocationID: schema.StringAttribute{Required: true},
										keyClusterID:          schema.StringAttribute{Optional: true, Computed: true},
										keyClusterName:        schema.StringAttribute{Computed: true},
										keyName:               schema.StringAttribute{Computed: true},
									},
								},
							},
							keyArchivalTiering: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keyColdStorageClass:               schema.StringAttribute{Optional: true},
										keyInstantTiering:                 schema.BoolAttribute{Optional: true},
										keyMinAccessibleDurationInSeconds: schema.Int64Attribute{Optional: true},
										keyTierExistingSnapshots:          schema.BoolAttribute{Optional: true},
									},
								},
							},
						},
					},
				},
				keyAWSDynamoDBConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyKMSAlias: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyAWSRDSConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyLogRetention:     schema.Int64Attribute{Required: true},
							keyLogRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyAzureBlobConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyArchivalLocationID: schema.StringAttribute{Required: true},
						},
					},
				},
				keyAzurePostgresFlexibleServerConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyBackupRetentionInDays: schema.Int64Attribute{Required: true},
						},
					},
				},
				keyAzureSQLDatabaseConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyLogRetention: schema.Int64Attribute{Required: true},
						},
						Blocks: map[string]schema.Block{
							keyLTRConfig: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{},
									Blocks: map[string]schema.Block{
										keyMonthlyRetention: schema.ListNestedBlock{
											NestedObject: schema.NestedBlockObject{
												Attributes: map[string]schema.Attribute{
													keyRetention:     schema.Int64Attribute{Required: true},
													keyRetentionUnit: schema.StringAttribute{Required: true},
												},
											},
										},
										keyWeeklyRetention: schema.ListNestedBlock{
											NestedObject: schema.NestedBlockObject{
												Attributes: map[string]schema.Attribute{
													keyRetention:     schema.Int64Attribute{Required: true},
													keyRetentionUnit: schema.StringAttribute{Required: true},
												},
											},
										},
										keyYearlyRetention: schema.ListNestedBlock{
											NestedObject: schema.NestedBlockObject{
												Attributes: map[string]schema.Attribute{
													keyRetention:     schema.Int64Attribute{Required: true},
													keyRetentionUnit: schema.StringAttribute{Required: true},
													keyWeekOfYear:    schema.Int64Attribute{Required: true},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				keyAzureSQLManagedInstanceConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyLogRetention: schema.Int64Attribute{Required: true},
						},
						Blocks: map[string]schema.Block{
							keyLTRConfig: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{},
									Blocks: map[string]schema.Block{
										keyMonthlyRetention: schema.ListNestedBlock{
											NestedObject: schema.NestedBlockObject{
												Attributes: map[string]schema.Attribute{
													keyRetention:     schema.Int64Attribute{Required: true},
													keyRetentionUnit: schema.StringAttribute{Required: true},
												},
											},
										},
										keyWeeklyRetention: schema.ListNestedBlock{
											NestedObject: schema.NestedBlockObject{
												Attributes: map[string]schema.Attribute{
													keyRetention:     schema.Int64Attribute{Required: true},
													keyRetentionUnit: schema.StringAttribute{Required: true},
												},
											},
										},
										keyYearlyRetention: schema.ListNestedBlock{
											NestedObject: schema.NestedBlockObject{
												Attributes: map[string]schema.Attribute{
													keyRetention:     schema.Int64Attribute{Required: true},
													keyRetentionUnit: schema.StringAttribute{Required: true},
													keyWeekOfYear:    schema.Int64Attribute{Required: true},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				keyBackupLocation: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyArchivalGroupID: schema.StringAttribute{Required: true},
						},
					},
				},
				keyDailySchedule: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:     schema.Int64Attribute{Required: true},
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyDB2Config: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDifferentialFrequency:     schema.Int64Attribute{Optional: true},
							keyDifferentialFrequencyUnit: schema.StringAttribute{Optional: true},
							keyIncrementalFrequency:      schema.Int64Attribute{Optional: true},
							keyIncrementalFrequencyUnit:  schema.StringAttribute{Optional: true},
							keyLogArchivalMethod:         schema.StringAttribute{Optional: true},
							keyLogRetention:              schema.Int64Attribute{Optional: true},
							keyLogRetentionUnit:          schema.StringAttribute{Optional: true},
						},
					},
				},
				keyFirstFullSnapshot: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDuration: schema.Int64Attribute{Required: true},
							keyStartAt:  schema.StringAttribute{Required: true},
						},
					},
				},
				keyGCPCloudSQLConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyLogRetention:     schema.Int64Attribute{Required: true},
							keyLogRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyHourlySchedule: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:     schema.Int64Attribute{Required: true},
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyInformixConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:                schema.Int64Attribute{Optional: true},
							keyFrequencyUnit:            schema.StringAttribute{Optional: true},
							keyIncrementalFrequency:     schema.Int64Attribute{Optional: true},
							keyIncrementalFrequencyUnit: schema.StringAttribute{Optional: true},
							keyIncrementalRetention:     schema.Int64Attribute{Optional: true},
							keyIncrementalRetentionUnit: schema.StringAttribute{Optional: true},
							keyRetention:                schema.Int64Attribute{Optional: true},
							keyRetentionUnit:            schema.StringAttribute{Optional: true},
						},
					},
				},
				keyLocalRetention: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyManagedVolumeConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyLogRetention:     schema.Int64Attribute{Required: true},
							keyLogRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyMinuteSchedule: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:     schema.Int64Attribute{Required: true},
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyMongoConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:     schema.Int64Attribute{Required: true},
							keyFrequencyUnit: schema.StringAttribute{Optional: true},
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyMonthlySchedule: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDayOfMonth:    schema.StringAttribute{Required: true},
							keyFrequency:     schema.Int64Attribute{Required: true},
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Required: true},
						},
					},
				},
				keyMSSQLConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:        schema.Int64Attribute{Required: true},
							keyFrequencyUnit:    schema.StringAttribute{Optional: true},
							keyLogRetention:     schema.Int64Attribute{Required: true},
							keyLogRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyMySQLDBConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:     schema.Int64Attribute{Required: true},
							keyFrequencyUnit: schema.StringAttribute{Optional: true},
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyNCDConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDailyBackupLocations:     schema.ListAttribute{ElementType: types.StringType, Optional: true},
							keyHourlyBackupLocations:    schema.ListAttribute{ElementType: types.StringType, Optional: true},
							keyMinutelyBackupLocations:  schema.ListAttribute{ElementType: types.StringType, Optional: true},
							keyMonthlyBackupLocations:   schema.ListAttribute{ElementType: types.StringType, Optional: true},
							keyQuarterlyBackupLocations: schema.ListAttribute{ElementType: types.StringType, Optional: true},
							keyWeeklyBackupLocations:    schema.ListAttribute{ElementType: types.StringType, Optional: true},
							keyYearlyBackupLocations:    schema.ListAttribute{ElementType: types.StringType, Optional: true},
						},
					},
				},
				keyOracleConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyFrequency:                     schema.Int64Attribute{Required: true},
							keyFrequencyUnit:                 schema.StringAttribute{Optional: true},
							keyHostLogRetention:              schema.Int64Attribute{Optional: true},
							keyHostLogRetentionUnit:          schema.StringAttribute{Optional: true},
							keyLogRetention:                  schema.Int64Attribute{Required: true},
							keyLogRetentionUnit:              schema.StringAttribute{Optional: true},
							keyRetainArchiveLogsIndefinitely: schema.BoolAttribute{Optional: true},
						},
					},
				},
				keyPostgresDBClusterConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyLogRetention:     schema.Int64Attribute{Required: true},
							keyLogRetentionUnit: schema.StringAttribute{Optional: true},
						},
					},
				},
				keyQuarterlySchedule: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDayOfQuarter:      schema.StringAttribute{Required: true},
							keyFrequency:         schema.Int64Attribute{Required: true},
							keyQuarterStartMonth: schema.StringAttribute{Required: true},
							keyRetention:         schema.Int64Attribute{Required: true},
							keyRetentionUnit:     schema.StringAttribute{Required: true},
						},
					},
				},
				keyReplicationSpec: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyAWSCrossAccount: schema.StringAttribute{Optional: true},
							keyAWSRegion:       schema.StringAttribute{Optional: true},
							keyAzureRegion:     schema.StringAttribute{Optional: true},
							keyRetention:       schema.Int64Attribute{Required: true},
							keyRetentionUnit:   schema.StringAttribute{Required: true},
						},
						Blocks: map[string]schema.Block{
							keyCascadingArchival: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keyArchivalLocationID:    schema.StringAttribute{Required: true},
										keyArchivalThreshold:     schema.Int64Attribute{Optional: true},
										keyArchivalThresholdUnit: schema.StringAttribute{Optional: true},
										keyFrequency:             schema.SetAttribute{ElementType: types.StringType, Optional: true},
									},
									Blocks: map[string]schema.Block{
										keyArchivalTiering: schema.ListNestedBlock{
											NestedObject: schema.NestedBlockObject{
												Attributes: map[string]schema.Attribute{
													keyColdStorageClass:               schema.StringAttribute{Optional: true},
													keyInstantTiering:                 schema.BoolAttribute{Optional: true},
													keyMinAccessibleDurationInSeconds: schema.Int64Attribute{Optional: true},
													keyTierExistingSnapshots:          schema.BoolAttribute{Optional: true},
												},
											},
										},
									},
								},
							},
							keyLocalRetention: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keyRetention:     schema.Int64Attribute{Required: true},
										keyRetentionUnit: schema.StringAttribute{Required: true},
									},
								},
							},
							keyReplicationPair: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keySourceCluster: schema.StringAttribute{Required: true},
										keyTargetCluster: schema.StringAttribute{Required: true},
									},
								},
							},
						},
					},
				},
				keyRetentionLock: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyRetentionLockComplianceAcknowledgment: schema.BoolAttribute{Optional: true},
							keyMode:                                  schema.StringAttribute{Required: true},
						},
					},
				},
				keySapHanaConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDifferentialFrequency:     schema.Int64Attribute{Optional: true},
							keyDifferentialFrequencyUnit: schema.StringAttribute{Optional: true},
							keyIncrementalFrequency:      schema.Int64Attribute{Optional: true},
							keyIncrementalFrequencyUnit:  schema.StringAttribute{Optional: true},
							keyLogRetention:              schema.Int64Attribute{Optional: true},
							keyLogRetentionUnit:          schema.StringAttribute{Optional: true},
						},
						Blocks: map[string]schema.Block{
							keyStorageSnapshotConfig: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keyFrequency:     schema.Int64Attribute{Required: true},
										keyFrequencyUnit: schema.StringAttribute{Optional: true},
										keyRetention:     schema.Int64Attribute{Required: true},
										keyRetentionUnit: schema.StringAttribute{Optional: true},
									},
								},
							},
						},
					},
				},
				keySnapshotWindow: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDuration: schema.Int64Attribute{Required: true},
							keyStartAt:  schema.StringAttribute{Required: true},
						},
					},
				},
				keyVMwareVMConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyLogRetention: schema.Int64Attribute{Required: true},
						},
					},
				},
				keyWeeklySchedule: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDayOfWeek:     schema.StringAttribute{Optional: true},
							keyFrequency:     schema.Int64Attribute{Required: true},
							keyRetention:     schema.Int64Attribute{Required: true},
							keyRetentionUnit: schema.StringAttribute{Required: true},
						},
					},
				},
				keyYearlySchedule: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyDayOfYear:      schema.StringAttribute{Required: true},
							keyFrequency:      schema.Int64Attribute{Required: true},
							keyRetention:      schema.Int64Attribute{Required: true},
							keyRetentionUnit:  schema.StringAttribute{Required: true},
							keyYearStartMonth: schema.StringAttribute{Required: true},
						},
					},
				},
				keyTimeouts: timeouts.Block(ctx, timeouts.Opts{Delete: true}),
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "slaDomainResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolarisSLADomain {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state slaDomainModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
