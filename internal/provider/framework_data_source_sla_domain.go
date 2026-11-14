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
	"errors"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/sla"
)

const dataSourceSLADomainDescription = `
The ´rubrik_sla_domain´ data source is used to access information about RSC SLA
domains. A SLA domain is looked up using either the ID or the name.
`

var (
	_ datasource.DataSource              = &slaDomainDataSource{}
	_ datasource.DataSourceWithConfigure = &slaDomainDataSource{}
)

type slaDomainDataSource struct {
	client *client
	prefix string
}

func newSLADomainDataSource() datasource.DataSource {
	return &slaDomainDataSource{prefix: keyRubrik}
}

func newPolarisSLADomainDataSource() datasource.DataSource {
	return &slaDomainDataSource{prefix: keyPolaris}
}

func (d *slaDomainDataSource) Metadata(ctx context.Context, _ datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "slaDomainDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keySLADomain
}

func (d *slaDomainDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "slaDomainDataSource.Schema")

	res.Schema = slaDomainDataSourceSchema()
	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_sla_domain` data source instead."
	}
}

func (d *slaDomainDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "slaDomainDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

// slaDomainDataSourceSchema returns the schema of the SLA domain data source. The
// SLA domain is looked up with the ID or the name, and everything else is
// computed. It has the same attributes as the SDKv2 schema it replaces.
func slaDomainDataSourceSchema() schema.Schema {
	return schema.Schema{
		Description: description(dataSourceSLADomainDescription),
		Attributes: map[string]schema.Attribute{
			keyArchival: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Archive snapshots to the specified archival location.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyArchivalLocationID: schema.StringAttribute{
							Computed:    true,
							Description: "Archival location ID (UUID).",
						},
						keyArchivalLocationToClusterMapping: schema.ListNestedAttribute{
							Computed:    true,
							Description: "Mapping between archival location and Rubrik cluster.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keyArchivalLocationID: schema.StringAttribute{
										Computed:    true,
										Description: "Archival location ID (UUID).",
									},
									keyClusterID: schema.StringAttribute{
										Computed:    true,
										Description: "Cluster ID (UUID).",
									},
									keyClusterName: schema.StringAttribute{
										Computed:    true,
										Description: "Cluster name.",
									},
									keyName: schema.StringAttribute{
										Computed:    true,
										Description: "Archival location name.",
									},
								},
							},
						},
						keyArchivalTiering: schema.ListNestedAttribute{
							Computed:    true,
							Description: "Archival tiering specification for cold storage.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keyColdStorageClass: schema.StringAttribute{
										Computed:    true,
										Description: "Cold storage class for tiering.",
									},
									keyInstantTiering: schema.BoolAttribute{
										Computed:    true,
										Description: "Enable instant tiering to cold storage.",
									},
									keyMinAccessibleDurationInSeconds: schema.Int64Attribute{
										Computed:    true,
										Description: "Minimum duration in seconds that data must remain accessible before tiering.",
									},
									keyTierExistingSnapshots: schema.BoolAttribute{
										Computed:    true,
										Description: "Whether to tier existing snapshots to cold storage.",
									},
								},
							},
						},
						keyFrequency: schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Effective snapshot frequencies being archived.",
						},
						keyThreshold: schema.Int64Attribute{
							Computed:    true,
							Description: "Threshold specifies the time before archiving the snapshots at the managing location.",
						},
						keyThresholdUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Threshold unit specifies the unit of threshold.",
						},
					},
				},
			},
			keyAWSDynamoDBConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "AWS DynamoDB configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyKMSAlias: schema.StringAttribute{
							Computed:    true,
							Description: "KMS alias for primary backup.",
						},
					},
				},
			},
			keyAWSRDSConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "AWS RDS continuous backups for point-in-time recovery.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention specifies for how long the backups are kept.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit specifies the unit of the log_retention field.",
						},
					},
				},
			},
			keyAzureBlobConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Azure Blob Storage backup location for scheduled snapshots.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyArchivalLocationID: schema.StringAttribute{
							Computed:    true,
							Description: "Archival location ID (UUID).",
						},
					},
				},
			},
			keyAzurePostgresFlexibleServerConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Azure Postgres Flexible Server configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyBackupRetentionInDays: schema.Int64Attribute{
							Computed:    true,
							Description: "Point-in-time restore retention, in days, enforced on the source flexible server.",
						},
					},
				},
			},
			keyAzureSQLDatabaseConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Azure SQL Database continuous backups for point-in-time recovery.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention specifies for how long, in days, the continuous backups are kept.",
						},
						keyLTRConfig: schema.ListNestedAttribute{
							Computed:    true,
							Description: "Long-term retention (LTR) configuration for a V1 (Azure-managed) Azure SQL SLA.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keyMonthlyRetention: schema.ListNestedAttribute{
										Computed:    true,
										Description: "The monthly Azure SQL long-term retention.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Computed:    true,
													Description: "Retention value in the configured retention unit.",
												},
												keyRetentionUnit: schema.StringAttribute{
													Computed:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
												},
											},
										},
									},
									keyWeeklyRetention: schema.ListNestedAttribute{
										Computed:    true,
										Description: "The weekly Azure SQL long-term retention.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Computed:    true,
													Description: "Retention value in the configured retention unit.",
												},
												keyRetentionUnit: schema.StringAttribute{
													Computed:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
												},
											},
										},
									},
									keyYearlyRetention: schema.ListNestedAttribute{
										Computed:    true,
										Description: "The yearly Azure SQL long-term retention.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Computed:    true,
													Description: "Retention value in the configured retention unit.",
												},
												keyRetentionUnit: schema.StringAttribute{
													Computed:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
												},
												keyWeekOfYear: schema.Int64Attribute{
													Computed:    true,
													Description: "The week of the year (1-52) retained as the yearly backup.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			keyAzureSQLManagedInstanceConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Azure SQL MI log backups.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention specifies for how long, in days, the log backups are kept.",
						},
						keyLTRConfig: schema.ListNestedAttribute{
							Computed:    true,
							Description: "Long-term retention (LTR) configuration for a V1 (Azure-managed) Azure SQL SLA.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keyMonthlyRetention: schema.ListNestedAttribute{
										Computed:    true,
										Description: "The monthly Azure SQL long-term retention.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Computed:    true,
													Description: "Retention value in the configured retention unit.",
												},
												keyRetentionUnit: schema.StringAttribute{
													Computed:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
												},
											},
										},
									},
									keyWeeklyRetention: schema.ListNestedAttribute{
										Computed:    true,
										Description: "The weekly Azure SQL long-term retention.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Computed:    true,
													Description: "Retention value in the configured retention unit.",
												},
												keyRetentionUnit: schema.StringAttribute{
													Computed:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
												},
											},
										},
									},
									keyYearlyRetention: schema.ListNestedAttribute{
										Computed:    true,
										Description: "The yearly Azure SQL long-term retention.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Computed:    true,
													Description: "Retention value in the configured retention unit.",
												},
												keyRetentionUnit: schema.StringAttribute{
													Computed:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
												},
												keyWeekOfYear: schema.Int64Attribute{
													Computed:    true,
													Description: "The week of the year (1-52) retained as the yearly backup.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			keyBackupLocation: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Backup locations for AWS S3 object type.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyArchivalGroupID: schema.StringAttribute{
							Computed:    true,
							Description: "Archival group ID (UUID).",
						},
					},
				},
			},
			keyBackupType: schema.StringAttribute{
				Computed:    true,
				Description: "Identifies which system manages the SLA's Azure SQL backups: `NATIVE` for a V1 (Azure-managed / long-term retention) SLA, or the Rubrik-managed value for a V2 SLA.",
			},
			keyDailySchedule: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Daily schedule of the SLA Domain.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Frequency of snapshots (days).",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention of snapshots.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyDB2Config: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Db2 database configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDifferentialFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Differential backup frequency.",
						},
						keyDifferentialFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Differential frequency unit.",
						},
						keyIncrementalFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Incremental backup frequency.",
						},
						keyIncrementalFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Incremental frequency unit.",
						},
						keyLogArchivalMethod: schema.StringAttribute{
							Computed:    true,
							Description: "Log archival method.",
						},
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit.",
						},
					},
				},
			},
			keyDescription: schema.StringAttribute{
				Computed:    true,
				Description: "SLA domain description.",
			},
			keyFirstFullSnapshot: schema.ListNestedAttribute{
				Computed:    true,
				Description: "First full snapshot window.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDuration: schema.Int64Attribute{
							Computed:    true,
							Description: "Duration of the first full snapshot window in hours.",
						},
						keyStartAt: schema.StringAttribute{
							Computed:    true,
							Description: "Start time of the first full snapshot window.",
						},
					},
				},
			},
			keyGCPCloudSQLConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "GCP Cloud SQL configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit.",
						},
					},
				},
			},
			keyHourlySchedule: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Hourly schedule.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Frequency of snapshots (hours).",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "SLA domain ID (UUID). Exactly one of `id` and `name` must be specified.",
				Validators: []validator.String{
					isUUID(),
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyID), path.MatchRoot(keyName)),
				},
			},
			keyInformixConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Informix database configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Log backup frequency.",
						},
						keyFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Frequency unit.",
						},
						keyIncrementalFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Incremental backup frequency.",
						},
						keyIncrementalFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Incremental frequency unit.",
						},
						keyIncrementalRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Incremental backup retention duration.",
						},
						keyIncrementalRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Incremental retention unit.",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyLocalRetention: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Local retention.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit specifies the unit of retention.",
						},
					},
				},
			},
			keyManagedVolumeConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Managed Volume configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit.",
						},
					},
				},
			},
			keyMinuteSchedule: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Minute schedule.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Frequency (minutes).",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyMongoConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "MongoDB database configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Log backup frequency.",
						},
						keyFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Frequency unit.",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyMonthlySchedule: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Monthly schedule.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfMonth: schema.StringAttribute{
							Computed:    true,
							Description: "Day of month.",
						},
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Frequency (months).",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyMSSQLConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "SQL Server database configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Log backup frequency.",
						},
						keyFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Frequency unit.",
						},
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit.",
						},
					},
				},
			},
			keyMySQLDBConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "MySQL database configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Log backup frequency.",
						},
						keyFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Frequency unit.",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "SLA domain name. Exactly one of `id` and `name` must be specified.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyNCDConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "NAS Cloud Direct configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDailyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Target location UUIDs for daily schedule backups.",
						},
						keyHourlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Target location UUIDs for hourly schedule backups.",
						},
						keyMinutelyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Target location UUIDs for per-minute schedule backups.",
						},
						keyMonthlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Target location UUIDs for monthly schedule backups.",
						},
						keyQuarterlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Target location UUIDs for quarterly schedule backups.",
						},
						keyWeeklyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Target location UUIDs for weekly schedule backups.",
						},
						keyYearlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Target location UUIDs for yearly schedule backups.",
						},
					},
				},
			},
			keyObjectTypes: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Object types which can be protected by the SLA domain.",
			},
			keyOracleConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Oracle database configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Log backup frequency.",
						},
						keyFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Frequency unit.",
						},
						keyHostLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Host log retention duration for archived redo logs.",
						},
						keyHostLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Host log retention unit.",
						},
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit.",
						},
					},
				},
			},
			keyPostgresDBClusterConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Postgres DB Cluster configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration for Write-Ahead Logging (WAL) logs.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit.",
						},
					},
				},
			},
			keyQuarterlySchedule: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Quarterly schedule.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfQuarter: schema.StringAttribute{
							Computed:    true,
							Description: "Day of quarter.",
						},
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Frequency (quarters).",
						},
						keyQuarterStartMonth: schema.StringAttribute{
							Computed:    true,
							Description: "Quarter start month.",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyReplicationSpec: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Replicate snapshots to the specified region.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyAWSCrossAccount: schema.StringAttribute{
							Computed:    true,
							Description: "Replication target (RSC cloud account ID) for cross account replication.",
						},
						keyAWSRegion: schema.StringAttribute{
							Computed:    true,
							Description: "AWS region to replicate to.",
						},
						keyAzureRegion: schema.StringAttribute{
							Computed:    true,
							Description: "Azure region to replicate to.",
						},
						keyCascadingArchival: schema.ListNestedAttribute{
							Computed:    true,
							Description: "Cascading archival specifications for replication.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keyArchivalLocationID: schema.StringAttribute{
										Computed:    true,
										Description: "Archival location ID (UUID) for cascading archival.",
									},
									keyArchivalThreshold: schema.Int64Attribute{
										Computed:    true,
										Description: "Archival threshold specifies when to archive replicated snapshots.",
									},
									keyArchivalThresholdUnit: schema.StringAttribute{
										Computed:    true,
										Description: "Archival threshold unit.",
									},
									keyArchivalTiering: schema.ListNestedAttribute{
										Computed:    true,
										Description: "Archival tiering specification for cold storage.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												keyColdStorageClass: schema.StringAttribute{
													Computed:    true,
													Description: "Cold storage class for tiering.",
												},
												keyInstantTiering: schema.BoolAttribute{
													Computed:    true,
													Description: "Enable instant tiering to cold storage.",
												},
												keyMinAccessibleDurationInSeconds: schema.Int64Attribute{
													Computed:    true,
													Description: "Minimum duration in seconds that data must remain accessible before tiering.",
												},
												keyTierExistingSnapshots: schema.BoolAttribute{
													Computed:    true,
													Description: "Whether to tier existing snapshots to cold storage.",
												},
											},
										},
									},
									keyFrequency: schema.SetAttribute{
										ElementType: types.StringType,
										Computed:    true,
										Description: "Frequencies for cascading archival.",
									},
								},
							},
						},
						keyLocalRetention: schema.ListNestedAttribute{
							Computed:    true,
							Description: "Local retention on replication target.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keyRetention: schema.Int64Attribute{
										Computed:    true,
										Description: "Local retention on replication target specifies for how long the snapshots are kept on the replication target before being archived.",
									},
									keyRetentionUnit: schema.StringAttribute{
										Computed:    true,
										Description: "Local retention unit.",
									},
								},
							},
						},
						keyReplicationPair: schema.ListNestedAttribute{
							Computed:    true,
							Description: "Replication pairs specifying source and target clusters.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keySourceCluster: schema.StringAttribute{
										Computed:    true,
										Description: "Source cluster ID (UUID).",
									},
									keyTargetCluster: schema.StringAttribute{
										Computed:    true,
										Description: "Target cluster ID (UUID).",
									},
								},
							},
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit specifies the unit of retention.",
						},
					},
				},
			},
			keyRetentionLock: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Retention lock.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyMode: schema.StringAttribute{
							Computed:    true,
							Description: "Retention lock mode.",
						},
					},
				},
			},
			keySapHanaConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "SAP HANA database configuration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDifferentialFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Differential backup frequency.",
						},
						keyDifferentialFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Differential frequency unit.",
						},
						keyIncrementalFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Incremental backup frequency.",
						},
						keyIncrementalFrequencyUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Incremental frequency unit.",
						},
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention duration.",
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Log retention unit.",
						},
						keyStorageSnapshotConfig: schema.ListNestedAttribute{
							Computed:    true,
							Description: "SAP HANA storage snapshot configuration.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									keyFrequency: schema.Int64Attribute{
										Computed:    true,
										Description: "Storage snapshot frequency.",
									},
									keyFrequencyUnit: schema.StringAttribute{
										Computed:    true,
										Description: "Frequency unit.",
									},
									keyRetention: schema.Int64Attribute{
										Computed:    true,
										Description: "Storage snapshot retention.",
									},
									keyRetentionUnit: schema.StringAttribute{
										Computed:    true,
										Description: "Retention unit.",
									},
								},
							},
						},
					},
				},
			},
			keySnapshotWindow: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Snapshot window.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDuration: schema.Int64Attribute{
							Computed:    true,
							Description: "Duration of the snapshot window in hours.",
						},
						keyStartAt: schema.StringAttribute{
							Computed:    true,
							Description: "Start time of the snapshot window.",
						},
					},
				},
			},
			keyVMwareVMConfig: schema.ListNestedAttribute{
				Computed:    true,
				Description: "VMware vSphere VM log backups.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Log retention specifies for how long, in seconds, the log backups are kept.",
						},
					},
				},
			},
			keyWeeklySchedule: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Weekly schedule.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfWeek: schema.StringAttribute{
							Computed:    true,
							Description: "Day of week.",
						},
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Frequency (weeks).",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
					},
				},
			},
			keyYearlySchedule: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Yearly schedule.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfYear: schema.StringAttribute{
							Computed:    true,
							Description: "Day of year.",
						},
						keyFrequency: schema.Int64Attribute{
							Computed:    true,
							Description: "Frequency (years).",
						},
						keyRetention: schema.Int64Attribute{
							Computed:    true,
							Description: "Retention.",
						},
						keyRetentionUnit: schema.StringAttribute{
							Computed:    true,
							Description: "Retention unit.",
						},
						keyYearStartMonth: schema.StringAttribute{
							Computed:    true,
							Description: "Year start month.",
						},
					},
				},
			},
		},
	}
}

func (d *slaDomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "slaDomainDataSource.Read")

	var config slaDomainDataSourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var domain gqlsla.Domain
	if id := config.ID.ValueString(); id != "" {
		domainID, err := uuid.Parse(id)
		if err != nil {
			res.Diagnostics.AddError("Invalid SLA domain ID", err.Error())
			return
		}
		domain, err = sla.Wrap(polarisClient).DomainByID(ctx, domainID)
		if errors.Is(err, graphql.ErrNotFound) {
			res.Diagnostics.AddError("SLA domain not found", err.Error())
			return
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to read SLA domain", err.Error())
			return
		}
	} else {
		domain, err = sla.Wrap(polarisClient).DomainByName(ctx, config.Name.ValueString())
		if errors.Is(err, graphql.ErrNotFound) {
			res.Diagnostics.AddError("SLA domain not found", err.Error())
			return
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to read SLA domain", err.Error())
			return
		}
	}

	state, diags := slaDomainDataSourceState(ctx, domain)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	res.State = state
}
