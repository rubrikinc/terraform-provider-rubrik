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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// backupLocationModel is an element of the backup_location block.
type backupLocationModel struct {
	ArchivalGroupID types.String `tfsdk:"archival_group_id"`
}

// awsDynamoDBConfigModel is the aws_dynamodb_config block.
type awsDynamoDBConfigModel struct {
	KMSAlias types.String `tfsdk:"kms_alias"`
}

// awsRDSConfigModel is the aws_rds_config block.
type awsRDSConfigModel struct {
	LogRetention     types.Int64  `tfsdk:"log_retention"`
	LogRetentionUnit types.String `tfsdk:"log_retention_unit"`
}

// azureBlobConfigModel is the azure_blob_config block.
type azureBlobConfigModel struct {
	ArchivalLocationID types.String `tfsdk:"archival_location_id"`
}

// fromBackupLocationModels returns the SDK backup location specs of the
// backup_location blocks. An archival group ID which is not a UUID gives no
// specs at all, and not only none for that block. The schema validates the ID,
// so this isn't reachable from a configuration, and the behavior is kept as it
// was.
func fromBackupLocationModels(locations []backupLocationModel) []gqlsla.BackupLocationSpec {
	var specs []gqlsla.BackupLocationSpec
	for _, location := range locations {
		groupID, err := uuid.Parse(location.ArchivalGroupID.ValueString())
		if err != nil {
			return nil
		}
		specs = append(specs, gqlsla.BackupLocationSpec{ArchivalGroupID: groupID})
	}
	return specs
}

// toBackupLocationModels returns the backup_location blocks of the SDK domain.
// The blocks are in the same order as the existing blocks, with the blocks
// which are not in the existing blocks added last in the order they have in the
// domain. An archival location which is used more than once is an error.
//
// When the domain has no backup location specs, but has an AWS S3 config, the
// archival location of the config is returned as the only block. This is the
// case when multiple backup locations are not enabled.
func toBackupLocationModels(domain gqlsla.Domain, existing []backupLocationModel) ([]backupLocationModel, error) {
	blocks := make(map[string]backupLocationModel)
	for _, spec := range domain.BackupLocationSpecs {
		id := spec.ArchivalGroup.ID
		if _, ok := blocks[id]; ok {
			return nil, fmt.Errorf("archival location %q used multiple times", id)
		}
		blocks[id] = backupLocationModel{ArchivalGroupID: types.StringValue(id)}
	}

	// Preserve the order of the existing blocks, then add the new ones last.
	var sorted []backupLocationModel
	for _, old := range existing {
		id := old.ArchivalGroupID.ValueString()
		if block, ok := blocks[id]; ok {
			sorted = append(sorted, block)
			delete(blocks, id)
		}
	}
	for _, spec := range domain.BackupLocationSpecs {
		id := spec.ArchivalGroup.ID
		if block, ok := blocks[id]; ok {
			sorted = append(sorted, block)
			delete(blocks, id)
		}
	}

	// AWS S3 fallback when multiple backup locations are not enabled.
	if len(sorted) == 0 && domain.ObjectSpecificConfigs.AWSS3Config != nil {
		sorted = append(sorted, backupLocationModel{
			ArchivalGroupID: types.StringValue(domain.ObjectSpecificConfigs.AWSS3Config.ArchivalLocationID.String()),
		})
	}

	return sorted, nil
}

// fromAWSDynamoDBConfigModel returns the SDK AWS DynamoDB config of the block,
// or nil if the block isn't configured.
func fromAWSDynamoDBConfigModel(config []awsDynamoDBConfigModel) *gqlsla.AWSDynamoDBConfig {
	if len(config) == 0 {
		return nil
	}
	return &gqlsla.AWSDynamoDBConfig{KMSAliasForPrimaryBackup: config[0].KMSAlias.ValueString()}
}

// toAWSDynamoDBConfigModel returns the aws_dynamodb_config block of the SDK
// config, or nil if the config is nil.
func toAWSDynamoDBConfigModel(config *gqlsla.AWSDynamoDBConfig) []awsDynamoDBConfigModel {
	if config == nil {
		return nil
	}
	return []awsDynamoDBConfigModel{{KMSAlias: types.StringValue(config.KMSAliasForPrimaryBackup)}}
}

// fromAWSRDSConfigModel returns the SDK AWS RDS config of the block, or nil if
// the block isn't configured.
func fromAWSRDSConfigModel(config []awsRDSConfigModel) *gqlsla.AWSRDSConfig {
	if len(config) == 0 {
		return nil
	}
	return &gqlsla.AWSRDSConfig{
		LogRetention: gqlsla.RetentionDuration{
			Duration: int(config[0].LogRetention.ValueInt64()),
			Unit:     gqlsla.RetentionUnit(config[0].LogRetentionUnit.ValueString()),
		},
	}
}

// toAWSRDSConfigModel returns the aws_rds_config block of the SDK config, or
// nil if the config is nil.
func toAWSRDSConfigModel(config *gqlsla.AWSRDSConfig) []awsRDSConfigModel {
	if config == nil {
		return nil
	}
	return []awsRDSConfigModel{{
		LogRetention:     types.Int64Value(int64(config.LogRetention.Duration)),
		LogRetentionUnit: types.StringValue(string(config.LogRetention.Unit)),
	}}
}

// fromAWSS3ConfigModels returns the SDK AWS S3 config of the backup locations.
// AWS S3 keeps its backup location in the config when multiple backup
// locations aren't enabled, and supports only one. It returns nil when there
// are no backup locations.
func fromAWSS3ConfigModels(locations []backupLocationModel) (*gqlsla.AWSS3Config, error) {
	if len(locations) == 0 {
		return nil, nil
	}
	if len(locations) > 1 {
		return nil, fmt.Errorf("multiple backup locations not supported")
	}
	groupID, err := uuid.Parse(locations[0].ArchivalGroupID.ValueString())
	if err != nil {
		return nil, err
	}
	return &gqlsla.AWSS3Config{ArchivalLocationID: groupID}, nil
}

// fromAzureBlobConfigModel returns the SDK Azure Blob config of the block, or
// nil if the block isn't configured. The continuous backup retention is always
// one day.
func fromAzureBlobConfigModel(config []azureBlobConfigModel) (*gqlsla.AzureBlobConfig, error) {
	if len(config) == 0 {
		return nil, nil
	}
	archivalLocationID, err := uuid.Parse(config[0].ArchivalLocationID.ValueString())
	if err != nil {
		return nil, err
	}
	return &gqlsla.AzureBlobConfig{
		BackupLocationID:                archivalLocationID,
		ContinuousBackupRetentionInDays: 1,
	}, nil
}
