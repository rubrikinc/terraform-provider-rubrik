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
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// defaultSLADomainDeleteTimeout is how long to wait for an SLA domain to be
// deleted, when no timeout is configured.
const defaultSLADomainDeleteTimeout = 10 * time.Minute

// slaDomainSchema returns the schema of the SLA domain resource. It has the same
// attributes and blocks as the SDKv2 schema it replaces.
func slaDomainSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: description(resourceSLADomainDescription),
		Attributes: map[string]schema.Attribute{
			keyApplyChangesToExistingSnapshots: schema.BoolAttribute{
				Optional:    true,
				Description: "Apply changes to existing snapshots when updating the SLA domain.",
			},
			keyApplyChangesToNonPolicySnapshots: schema.BoolAttribute{
				Optional:    true,
				Description: "Apply changes to non-policy snapshots when updating the SLA domain.",
			},
			keyBackupType: schema.StringAttribute{
				Computed:    true,
				Description: "Identifies which system manages the SLA's Azure SQL backups: `NATIVE` for a V1 (Azure-managed / long-term retention) SLA, or the Rubrik-managed value for a V2 SLA. Read-only.",
			},
			keyDescription: schema.StringAttribute{
				Optional:    true,
				Description: "SLA Domain description.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyID: schema.StringAttribute{
				Computed:      true,
				Description:   "SLA Domain ID (UUID).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "SLA Domain name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyObjectTypes: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Object types which can be protected by the SLA Domain. Possible values are `ACTIVE_DIRECTORY_OBJECT_TYPE`, `ATLASSIAN_JIRA_OBJECT_TYPE`, `AWS_CONFIG_OBJECT_TYPE`, `AWS_DYNAMODB_OBJECT_TYPE`, `AWS_EC2_EBS_OBJECT_TYPE`, `AWS_RDS_OBJECT_TYPE`, `AWS_S3_OBJECT_TYPE`, `AZURE_AD_OBJECT_TYPE`, `AZURE_BLOB_OBJECT_TYPE`, `AZURE_DEVOPS_OBJECT_TYPE`, `AZURE_OBJECT_TYPE`, `AZURE_POSTGRES_FLEXIBLE_SERVER_OBJECT_TYPE`, `AZURE_SQL_DATABASE_OBJECT_TYPE`, `AZURE_SQL_MANAGED_INSTANCE_OBJECT_TYPE`, `CASSANDRA_OBJECT_TYPE`, `D365_OBJECT_TYPE`, `DB2_OBJECT_TYPE`, `EXCHANGE_OBJECT_TYPE`, `FILESET_OBJECT_TYPE`, `GCP_BIGQUERY_OBJECT_TYPE`, `GCP_CLOUD_SQL_OBJECT_TYPE`, `GCP_OBJECT_TYPE`, `GOOGLE_WORKSPACE_OBJECT_TYPE`, `HYPERV_OBJECT_TYPE`, `INFORMIX_INSTANCE_OBJECT_TYPE`, `K8S_OBJECT_TYPE`, `KUPR_OBJECT_TYPE`, `M365_BACKUP_STORAGE_OBJECT_TYPE`, `MANAGED_VOLUME_OBJECT_TYPE`, `MONGO_OBJECT_TYPE`, `MONGODB_OBJECT_TYPE`, `MSSQL_OBJECT_TYPE`, `MYSQLDB_OBJECT_TYPE`, `NAS_OBJECT_TYPE`, `NCD_OBJECT_TYPE`, `NUTANIX_OBJECT_TYPE`, `O365_OBJECT_TYPE`, `OKTA_OBJECT_TYPE`, `OLVM_OBJECT_TYPE`, `OPENSTACK_OBJECT_TYPE`, `ORACLE_OBJECT_TYPE`, `POSTGRES_DB_CLUSTER_OBJECT_TYPE`, `PROXMOX_OBJECT_TYPE`, `SALESFORCE_OBJECT_TYPE`, `SAP_HANA_OBJECT_TYPE`, `SNAPMIRROR_CLOUD_OBJECT_TYPE`, `VCD_OBJECT_TYPE`, `VOLUME_GROUP_OBJECT_TYPE`, and `VSPHERE_OBJECT_TYPE`.",
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(gqlsla.AllObjectTypesAsStrings()...)),
				},
			},
		},
		Blocks: map[string]schema.Block{
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Delete: true,
				DeleteDescription: fmt.Sprintf("How long to wait for the SLA Domain to be deleted. Default is `%s`.",
					defaultSLADomainDeleteTimeout),
			}),
			keyArchival: schema.ListNestedBlock{
				Description: "Archive snapshots to the specified archival location. Note, if `instant_archive` is enabled, `threshold` and `threshold_unit` are ignored.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyArchivalLocationID: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Archival location ID (UUID).",
							Validators: []validator.String{
								isUUID(),
							},
						},
						keyFrequency: schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Override which snapshot frequencies to archive. When not specified, frequencies are derived from the snapshot schedule and will not be visible in state. Use the rubrik_sla_domain data source to see the effective frequencies. Possible values are `MINUTES`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS`, `YEARS`.",
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(stringvalidator.OneOf(string(gqlsla.Minute), string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years))),
							},
						},
						keyThreshold: schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Default:     int64default.StaticInt64(0),
							Description: "Threshold specifies the time before archiving the snapshots at the managing location. The archival location retains the snapshots according to the SLA Domain schedule.",
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
							},
						},
						keyThresholdUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Threshold unit specifies the unit of `threshold`. Possible values are `DAYS`, `WEEKS`, `MONTHS` and `YEARS`. Default value is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
					Blocks: map[string]schema.Block{
						keyArchivalLocationToClusterMapping: schema.ListNestedBlock{
							Description: "Mapping between archival location and Rubrik cluster. Each mapping specifies which cluster should be used for archiving to a specific location.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									keyArchivalLocationID: schema.StringAttribute{
										Required:    true,
										Description: "Archival location ID (UUID).",
										Validators: []validator.String{
											isUUID(),
										},
									},
									keyClusterID: schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Cluster ID (UUID).",
										Validators: []validator.String{
											isUUID(),
										},
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
						keyArchivalTiering: schema.ListNestedBlock{
							Description: "Archival tiering specification for cold storage.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									keyColdStorageClass: schema.StringAttribute{
										Optional:    true,
										Description: "Cold storage class for tiering. Possible values are `AZURE_ARCHIVE`, `AWS_GLACIER`, `AWS_GLACIER_DEEP_ARCHIVE`.",
										Validators: []validator.String{
											stringvalidator.OneOf(string(gqlsla.ColdStorageClassAzureArchive), string(gqlsla.ColdStorageClassAWSGlacier), string(gqlsla.ColdStorageClassAWSGlacierDeepArchive)),
										},
									},
									keyInstantTiering: schema.BoolAttribute{
										Optional:    true,
										Description: "Enable instant tiering to cold storage.",
									},
									keyMinAccessibleDurationInSeconds: schema.Int64Attribute{
										Optional:    true,
										Description: "Minimum duration in seconds that data must remain accessible before tiering.",
										Validators: []validator.Int64{
											int64validator.AtLeast(0),
										},
									},
									keyTierExistingSnapshots: schema.BoolAttribute{
										Optional:    true,
										Description: "Whether to tier existing snapshots to cold storage.",
									},
								},
							},
						},
					},
				},
			},
			keyAWSDynamoDBConfig: schema.ListNestedBlock{
				Description: "",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKMSAlias: schema.StringAttribute{
							Optional:    true,
							Description: "KMS alias for primary backup. Ensure the specified KMS key exists in the respective regions of the DynamoDB tables this SLA will be applied to. Avoid deleting it, as it will be used for data decryption during archival and recovery.",
							Validators: []validator.String{
								stringvalidator.RegexMatches(regexp.MustCompile(`^alias\/[a-zA-Z0-9:/_-]+$`), "KMS alias must be in the format `alias/<name>`"),
							},
						},
					},
				},
			},
			keyAWSRDSConfig: schema.ListNestedBlock{
				Description: "AWS RDS continuous backups for point-in-time recovery. If continuous backup isn't specified, AWS provides 1 day of continuous backup by default for Aurora databases, which can be changed but not disable.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention specifies for how long the backups are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit specifies the unit of the `log_retention` field. Possible values are `DAYS`, `WEEKS`, `MONTHS` and `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Years)),
							},
						},
					},
				},
			},
			keyAzureBlobConfig: schema.ListNestedBlock{
				Description: "Azure Blob Storage backup location for scheduled snapshots. To avoid early deletion fees, retain snapshots in cool tier archival locations for at least 30 days.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyArchivalLocationID: schema.StringAttribute{
							Required:    true,
							Description: "Archival location ID (UUID).",
							Validators: []validator.String{
								isUUID(),
							},
						},
					},
				},
			},
			keyAzurePostgresFlexibleServerConfig: schema.ListNestedBlock{
				Description: "Azure Postgres Flexible Server configuration. Only valid when `object_types` is exactly `AZURE_POSTGRES_FLEXIBLE_SERVER_OBJECT_TYPE`.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyBackupRetentionInDays: schema.Int64Attribute{
							Required:    true,
							Description: "Point-in-time restore retention, in days, that RSC enforces on the source flexible server. Must be between 7 and 35. Omit the whole block to leave the server's existing Azure-side retention untouched.",
							Validators: []validator.Int64{
								int64validator.Between(7, 35),
							},
						},
					},
				},
			},
			keyAzureSQLDatabaseConfig: schema.ListNestedBlock{
				Description: "Azure SQL Database continuous backups for point-in-time recovery. Continuous backups are stored in the source database. A V1 (Azure-managed) SLA also specifies `ltr_config`; a V2 (Rubrik-managed) SLA omits it and specifies a backup location and snapshot schedule. Note, the changes will be applied during the next maintenance window.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention specifies for how long, in days, the continuous backups are kept.",
							Validators: []validator.Int64{
								int64validator.Between(1, 35),
							},
						},
					},
					Blocks: map[string]schema.Block{
						keyLTRConfig: schema.ListNestedBlock{
							Description: "Long-term retention (LTR) configuration for a V1 (Azure-managed) Azure SQL SLA. When set, the SLA manages Azure native LTR backups and must not specify a Rubrik backup location or snapshot schedule. When omitted, the SLA is a V2 (Rubrik-managed) SLA.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{},
								Blocks: map[string]schema.Block{
									keyMonthlyRetention: schema.ListNestedBlock{
										Description: "The monthly Azure SQL long-term retention.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Required:    true,
													Description: "Retention value in the configured retention unit. Azure accepts 0, or 7 to 3650 days.",
													Validators:  []validator.Int64{int64validator.AtLeast(0)},
												},
												keyRetentionUnit: schema.StringAttribute{
													Required:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
													Validators:  []validator.String{stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Years))},
												},
											},
										},
									},
									keyWeeklyRetention: schema.ListNestedBlock{
										Description: "The weekly Azure SQL long-term retention.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Required:    true,
													Description: "Retention value in the configured retention unit. Azure accepts 0, or 7 to 3650 days.",
													Validators:  []validator.Int64{int64validator.AtLeast(0)},
												},
												keyRetentionUnit: schema.StringAttribute{
													Required:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
													Validators:  []validator.String{stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Years))},
												},
											},
										},
									},
									keyYearlyRetention: schema.ListNestedBlock{
										Description: "The yearly Azure SQL long-term retention.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Required:    true,
													Description: "Retention value in the configured retention unit. Azure accepts 0, or 7 to 3650 days.",
													Validators:  []validator.Int64{int64validator.AtLeast(0)},
												},
												keyRetentionUnit: schema.StringAttribute{
													Required:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
													Validators:  []validator.String{stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Years))},
												},
												keyWeekOfYear: schema.Int64Attribute{
													Required:    true,
													Description: "The week of the year (1-52) to retain as the yearly backup.",
													Validators:  []validator.Int64{int64validator.Between(1, 52)},
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
			keyAzureSQLManagedInstanceConfig: schema.ListNestedBlock{
				Description: "Azure SQL MI log backups. A V1 (Azure-managed) SLA also specifies `ltr_config`; a V2 (Rubrik-managed) SLA omits it and specifies a backup location and snapshot schedule. Note, the changes will be applied during the next maintenance window.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention specifies for how long, in days, the log backups are kept.",
							Validators: []validator.Int64{
								int64validator.Between(1, 35),
							},
						},
					},
					Blocks: map[string]schema.Block{
						keyLTRConfig: schema.ListNestedBlock{
							Description: "Long-term retention (LTR) configuration for a V1 (Azure-managed) Azure SQL SLA. When set, the SLA manages Azure native LTR backups and must not specify a Rubrik backup location or snapshot schedule. When omitted, the SLA is a V2 (Rubrik-managed) SLA.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{},
								Blocks: map[string]schema.Block{
									keyMonthlyRetention: schema.ListNestedBlock{
										Description: "The monthly Azure SQL long-term retention.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Required:    true,
													Description: "Retention value in the configured retention unit. Azure accepts 0, or 7 to 3650 days.",
													Validators:  []validator.Int64{int64validator.AtLeast(0)},
												},
												keyRetentionUnit: schema.StringAttribute{
													Required:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
													Validators:  []validator.String{stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Years))},
												},
											},
										},
									},
									keyWeeklyRetention: schema.ListNestedBlock{
										Description: "The weekly Azure SQL long-term retention.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Required:    true,
													Description: "Retention value in the configured retention unit. Azure accepts 0, or 7 to 3650 days.",
													Validators:  []validator.Int64{int64validator.AtLeast(0)},
												},
												keyRetentionUnit: schema.StringAttribute{
													Required:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
													Validators:  []validator.String{stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Years))},
												},
											},
										},
									},
									keyYearlyRetention: schema.ListNestedBlock{
										Description: "The yearly Azure SQL long-term retention.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												keyRetention: schema.Int64Attribute{
													Required:    true,
													Description: "Retention value in the configured retention unit. Azure accepts 0, or 7 to 3650 days.",
													Validators:  []validator.Int64{int64validator.AtLeast(0)},
												},
												keyRetentionUnit: schema.StringAttribute{
													Required:    true,
													Description: "Unit for the retention value. One of DAYS, WEEKS, MONTHS or YEARS.",
													Validators:  []validator.String{stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Years))},
												},
												keyWeekOfYear: schema.Int64Attribute{
													Required:    true,
													Description: "The week of the year (1-52) to retain as the yearly backup.",
													Validators:  []validator.Int64{int64validator.Between(1, 52)},
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
			keyBackupLocation: schema.ListNestedBlock{
				Description: "",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyArchivalGroupID: schema.StringAttribute{
							Required:    true,
							Description: "Archival group ID (UUID).",
							Validators: []validator.String{
								isUUID(),
							},
						},
					},
				},
			},
			keyDailySchedule: schema.ListNestedBlock{
				Description: "Take snapshots with frequency specified in days.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Frequency in days.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Retention unit specifies the unit of the `retention` field. Possible values are `DAYS`, `WEEKS` and `MONTHS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months)),
							},
						},
					},
				},
			},
			keyDB2Config: schema.ListNestedBlock{
				Description: "Db2 database configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDifferentialFrequency: schema.Int64Attribute{
							Optional:    true,
							Description: "Differential backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyDifferentialFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Differential frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyIncrementalFrequency: schema.Int64Attribute{
							Optional:    true,
							Description: "Incremental backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyIncrementalFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Incremental frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyLogArchivalMethod: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("LOGARCHMETH1"),
							Description: "Log archival method. Possible values are `LOGARCHMETH1`, `LOGARCHMETH2`. Default is `LOGARCHMETH1`.",
							Validators: []validator.String{
								stringvalidator.OneOf("LOGARCHMETH1", "LOGARCHMETH2"),
							},
						},
						keyLogRetention: schema.Int64Attribute{
							Optional:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyFirstFullSnapshot: schema.ListNestedBlock{
				Description: "Specifies the snapshot window where the first full snapshot will be taken. If not specified it will be at first opportunity.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDuration: schema.Int64Attribute{
							Required:    true,
							Description: "Duration of snapshot window in hours.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyStartAt: schema.StringAttribute{
							Required:    true,
							Description: "Start of the snapshot window. Should be given as `DAY, HH:MM`, e.g: `Mon, 15:30`.",
							Validators: []validator.String{
								isStartAt(true),
							},
						},
					},
				},
			},
			keyGCPCloudSQLConfig: schema.ListNestedBlock{
				Description: "GCP Cloud SQL configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyHourlySchedule: schema.ListNestedBlock{
				Description: "Take snapshots with frequency specified in hours.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Frequency in hours.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Retention unit specifies the unit of the `retention` field. Possible values are `HOURS`, `DAYS`, `WEEKS` and `MONTHS`. Default value is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months)),
							},
						},
					},
				},
			},
			keyInformixConfig: schema.ListNestedBlock{
				Description: "Informix database configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Optional:    true,
							Description: "Log backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyIncrementalFrequency: schema.Int64Attribute{
							Optional:    true,
							Description: "Incremental backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyIncrementalFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Incremental frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyIncrementalRetention: schema.Int64Attribute{
							Optional:    true,
							Description: "Incremental backup retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyIncrementalRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Incremental retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyRetention: schema.Int64Attribute{
							Optional:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyLocalRetention: schema.ListNestedBlock{
				Description: "",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Retention unit specifies the unit of `retention`. Possible values are `MINUTE`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Minute), string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
							},
						},
					},
				},
			},
			keyManagedVolumeConfig: schema.ListNestedBlock{
				Description: "Managed Volume configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyMinuteSchedule: schema.ListNestedBlock{
				Description: "Take snapshots with frequency specified in minutes.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Frequency in minutes.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Retention unit specifies the unit of the `retention` field. Possible values are `HOURS`, `DAYS` and `WEEKS`. Default value is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks)),
							},
						},
					},
				},
			},
			keyMongoConfig: schema.ListNestedBlock{
				Description: "MongoDB database configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Log backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyMonthlySchedule: schema.ListNestedBlock{
				Description: "Take snapshots with frequency specified in months.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfMonth: schema.StringAttribute{
							Required:    true,
							Description: "Day of month. Possible values are `FIRST_DAY`, `FIFTEENTH` and `LAST_DAY`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.FirstDay, string(gqlsla.FifteenthDay), gqlsla.LastDay),
							},
						},
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Frequency in months.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Required:    true,
							Description: "Retention unit specifies the unit of `retention`. Possible values are `MINUTE`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Minute), string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
							},
						},
					},
				},
			},
			keyMSSQLConfig: schema.ListNestedBlock{
				Description: "SQL Server database configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Log backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyMySQLDBConfig: schema.ListNestedBlock{
				Description: "MySQL database configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Log backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyNCDConfig: schema.ListNestedBlock{
				Description: "NAS Cloud Direct configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDailyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Target location UUIDs for daily schedule backups.",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(isUUID()),
							},
						},
						keyHourlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Target location UUIDs for hourly schedule backups.",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(isUUID()),
							},
						},
						keyMinutelyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Target location UUIDs for per-minute schedule backups.",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(isUUID()),
							},
						},
						keyMonthlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Target location UUIDs for monthly schedule backups.",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(isUUID()),
							},
						},
						keyQuarterlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Target location UUIDs for quarterly schedule backups.",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(isUUID()),
							},
						},
						keyWeeklyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Target location UUIDs for weekly schedule backups.",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(isUUID()),
							},
						},
						keyYearlyBackupLocations: schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Target location UUIDs for yearly schedule backups.",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(isUUID()),
							},
						},
					},
				},
			},
			keyOracleConfig: schema.ListNestedBlock{
				Description: "Oracle database configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Log backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyHostLogRetention: schema.Int64Attribute{
							Optional:    true,
							Description: "Host log retention duration for archived redo logs.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyHostLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Host log retention unit. Possible values are `MINUTES`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyRetainArchiveLogsIndefinitely: schema.BoolAttribute{
							Optional:    true,
							Description: "When true, Oracle archive logs are retained indefinitely on the host and never deleted. Mutually exclusive with `host_log_retention`.",
						},
					},
				},
			},
			keyPostgresDBClusterConfig: schema.ListNestedBlock{
				Description: "Postgres DB Cluster configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention duration for Write-Ahead Logging (WAL) logs.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
				},
			},
			keyQuarterlySchedule: schema.ListNestedBlock{
				Description: "Take snapshots with frequency specified in quarters.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfQuarter: schema.StringAttribute{
							Required:    true,
							Description: "Day of quarter. Possible values are `FIRST_DAY` and `LAST_DAY`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.FirstDay, gqlsla.LastDay),
							},
						},
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Frequency in quarters.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyQuarterStartMonth: schema.StringAttribute{
							Required:    true,
							Description: "Quarter start month. Possible values are `JANUARY`, `FEBRUARY`, `MARCH`, `APRIL`, `MAY`, `JUNE`, `JULY`, `AUGUST`, `SEPTEMBER`, `OCTOBER`, `NOVEMBER` and `DECEMBER`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.January), string(gqlsla.February), string(gqlsla.March), string(gqlsla.April), string(gqlsla.May), string(gqlsla.June), string(gqlsla.July), string(gqlsla.August), string(gqlsla.September), string(gqlsla.October), string(gqlsla.November), string(gqlsla.December)),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Required:    true,
							Description: "Retention unit specifies the unit of `retention`. Possible values are `MINUTE`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Minute), string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
							},
						},
					},
				},
			},
			keyReplicationSpec: schema.ListNestedBlock{
				Description: "Replication specification for the SLA Domain. ",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyAWSCrossAccount: schema.StringAttribute{
							Optional:    true,
							Description: "Replication targetRSC cloud account ID) for cross account replication. Set to empyt string for same account replication.",
						},
						keyAWSRegion: schema.StringAttribute{
							Optional:    true,
							Description: "AWS region to replicate to. Should be specified in the standard AWS style, e.g. `us-west-2`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlaws.AllRegionNames()...),
							},
						},
						keyAzureRegion: schema.StringAttribute{
							Optional:    true,
							Description: "Azure region to replicate to. Should be specified in the standard Azure style, e.g. `eastus`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlazure.AllRegionNames()...),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Required:    true,
							Description: "Retention unit specifies the unit of `retention`. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
							},
						},
					},
					Blocks: map[string]schema.Block{
						keyCascadingArchival: schema.ListNestedBlock{
							Description: "Cascading archival specifications for replication.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									keyArchivalLocationID: schema.StringAttribute{
										Required:    true,
										Description: "Archival location ID (UUID) for cascading archival.",
										Validators: []validator.String{
											isUUID(),
										},
									},
									keyArchivalThreshold: schema.Int64Attribute{
										Optional:    true,
										Description: "Archival threshold specifies when to archive replicated snapshots.",
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
									keyArchivalThresholdUnit: schema.StringAttribute{
										Optional:    true,
										Description: "Archival threshold unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
										Validators: []validator.String{
											stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
										},
									},
									keyFrequency: schema.SetAttribute{
										ElementType: types.StringType,
										Optional:    true,
										Description: "Frequencies for cascading archival. Possible values are `MINUTE`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS`, `YEARS`.",
										Validators: []validator.Set{
											setvalidator.ValueStringsAre(stringvalidator.OneOf(string(gqlsla.Minute), string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years))),
										},
									},
								},
								Blocks: map[string]schema.Block{
									keyArchivalTiering: schema.ListNestedBlock{
										Description: "Archival tiering specification for cold storage.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												keyColdStorageClass: schema.StringAttribute{
													Optional:    true,
													Description: "Cold storage class for tiering. Possible values are `AZURE_ARCHIVE`, `AWS_GLACIER`, `AWS_GLACIER_DEEP_ARCHIVE`.",
													Validators: []validator.String{
														stringvalidator.OneOf(string(gqlsla.ColdStorageClassAzureArchive), string(gqlsla.ColdStorageClassAWSGlacier), string(gqlsla.ColdStorageClassAWSGlacierDeepArchive)),
													},
												},
												keyInstantTiering: schema.BoolAttribute{
													Optional:    true,
													Description: "Enable instant tiering to cold storage.",
												},
												keyMinAccessibleDurationInSeconds: schema.Int64Attribute{
													Optional:    true,
													Description: "Minimum duration in seconds that data must remain accessible before tiering.",
													Validators: []validator.Int64{
														int64validator.AtLeast(0),
													},
												},
												keyTierExistingSnapshots: schema.BoolAttribute{
													Optional:    true,
													Description: "Whether to tier existing snapshots to cold storage.",
												},
											},
										},
									},
								},
							},
						},
						keyLocalRetention: schema.ListNestedBlock{
							Description: "Local retention on replication target.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									keyRetention: schema.Int64Attribute{
										Required:    true,
										Description: "Local retention on replication target specifies for how long the snapshots are kept on the replication target before being archived.",
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
									keyRetentionUnit: schema.StringAttribute{
										Required:    true,
										Description: "Local retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
										Validators: []validator.String{
											stringvalidator.OneOf(string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
										},
									},
								},
							},
						},
						keyReplicationPair: schema.ListNestedBlock{
							Description: "Replication pairs specifying source and target clusters.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									keySourceCluster: schema.StringAttribute{
										Required:    true,
										Description: "Source cluster ID (UUID).",
										Validators: []validator.String{
											isUUID(),
										},
									},
									keyTargetCluster: schema.StringAttribute{
										Required:    true,
										Description: "Target cluster ID (UUID).",
										Validators: []validator.String{
											isUUID(),
										},
									},
								},
							},
						},
					},
				},
			},
			keyRetentionLock: schema.ListNestedBlock{
				Description: "Enable retention lock. Retention lock prevents data from being accidentally or maliciously modified or deleted during the retention period",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyRetentionLockComplianceAcknowledgment: schema.BoolAttribute{
							Optional:    true,
							Description: "Acknowledgment that snapshots protected under compliance mode cannot be deleted before the scheduled expiry date. This field must be set to `true` when using `COMPLIANCE` mode. Compliance mode is recommended to meet regulations and governance mode is recommended to only protect data. Default value is `false`.\n\n!> **Warning:** Snapshots protected under compliance mode cannot be deleted before the scheduled expiry date.",
						},
						keyMode: schema.StringAttribute{
							Required:    true,
							Description: "Retention lock mode. Possible values are `COMPLIANCE` and `GOVERNANCE`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Compliance), string(gqlsla.Protection)),
							},
						},
					},
				},
			},
			keySapHanaConfig: schema.ListNestedBlock{
				Description: "SAP HANA database configuration.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDifferentialFrequency: schema.Int64Attribute{
							Optional:    true,
							Description: "Differential backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyDifferentialFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Differential frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyIncrementalFrequency: schema.Int64Attribute{
							Optional:    true,
							Description: "Incremental backup frequency.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyIncrementalFrequencyUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Incremental frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
						keyLogRetention: schema.Int64Attribute{
							Optional:    true,
							Description: "Log retention duration.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyLogRetentionUnit: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("DAYS"),
							Description: "Log retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
							},
						},
					},
					Blocks: map[string]schema.Block{
						keyStorageSnapshotConfig: schema.ListNestedBlock{
							Description: "SAP HANA storage snapshot configuration.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									keyFrequency: schema.Int64Attribute{
										Required:    true,
										Description: "Storage snapshot frequency.",
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
									keyFrequencyUnit: schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Default:     stringdefault.StaticString("DAYS"),
										Description: "Frequency unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
										Validators: []validator.String{
											stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
										},
									},
									keyRetention: schema.Int64Attribute{
										Required:    true,
										Description: "Storage snapshot retention.",
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
									keyRetentionUnit: schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Default:     stringdefault.StaticString("DAYS"),
										Description: "Retention unit. Possible values are `DAYS`, `WEEKS`, `MONTHS`, `YEARS`. Default is `DAYS`.",
										Validators: []validator.String{
											stringvalidator.OneOf(gqlsla.AllRetentionUnitsAsStrings()...),
										},
									},
								},
							},
						},
					},
				},
			},
			keySnapshotWindow: schema.ListNestedBlock{
				Description: "Specifies an optional snapshot window.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDuration: schema.Int64Attribute{
							Required:    true,
							Description: "Duration of the snapshot window in hours.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyStartAt: schema.StringAttribute{
							Required:    true,
							Description: "Start of the snapshot window. Should be given as `HH:MM`, e.g: `15:30`.",
							Validators: []validator.String{
								isStartAt(false),
							},
						},
					},
				},
			},
			keyVMwareVMConfig: schema.ListNestedBlock{
				Description: "VMware vSphere VM log backups.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyLogRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Log retention specifies for how long, in seconds, the log backups are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
					},
				},
			},
			keyWeeklySchedule: schema.ListNestedBlock{
				Description: "Take snapshots with frequency specified in weeks.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfWeek: schema.StringAttribute{
							Optional:    true,
							Description: "Day of week. Possible values are `MONDAY`, `TUESDAY`, `WEDNESDAY`, `THURSDAY`, `FRIDAY`, `SATURDAY` and `SUNDAY`. Note: For M365 Backup Storage SLAs, this field should be omitted.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Monday), string(gqlsla.Tuesday), string(gqlsla.Wednesday), string(gqlsla.Thursday), string(gqlsla.Friday), string(gqlsla.Saturday), string(gqlsla.Sunday)),
							},
						},
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Frequency in weeks.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Required:    true,
							Description: "Retention unit specifies the unit of `retention`. Possible values are `MINUTE`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Minute), string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
							},
						},
					},
				},
			},
			keyYearlySchedule: schema.ListNestedBlock{
				Description:   "Take snapshots with frequency specified in years.",
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
				Validators:    []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyDayOfYear: schema.StringAttribute{
							Required:    true,
							Description: "Day of year. Possible values are `FIRST_DAY` and `LAST_DAY`.",
							Validators: []validator.String{
								stringvalidator.OneOf("FIRST_DAY", "LAST_DAY"),
							},
						},
						keyFrequency: schema.Int64Attribute{
							Required:    true,
							Description: "Frequency (years).",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetention: schema.Int64Attribute{
							Required:    true,
							Description: "Retention specifies for how long the snapshots are kept.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						keyRetentionUnit: schema.StringAttribute{
							Required:    true,
							Description: "Retention unit specifies the unit of `retention`. Possible values are `MINUTE`, `HOURS`, `DAYS`, `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(gqlsla.Minute), string(gqlsla.Hours), string(gqlsla.Days), string(gqlsla.Weeks), string(gqlsla.Months), string(gqlsla.Quarters), string(gqlsla.Years)),
							},
						},
						keyYearStartMonth: schema.StringAttribute{
							Required:    true,
							Description: "Year start month. Possible values are `JANUARY`, `FEBRUARY`, `MARCH`, `APRIL`, `MAY`, `JUNE`, `JULY`, `AUGUST`, `SEPTEMBER`, `OCTOBER`, `NOVEMBER` and `DECEMBER`.",
							Validators: []validator.String{
								stringvalidator.OneOf("JANUARY", "FEBRUARY", "MARCH", "APRIL", "MAY", "JUNE", "JULY", "AUGUST", "SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER"),
							},
						},
					},
				},
			},
		},
	}
}
