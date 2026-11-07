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
	"slices"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// replicationPairModel is an element of the replication_pair block.
type replicationPairModel struct {
	SourceCluster types.String `tfsdk:"source_cluster"`
	TargetCluster types.String `tfsdk:"target_cluster"`
}

// cascadingArchivalModel is an element of the cascading_archival block of a
// replication spec. The archival location is archived to from the target
// clusters of the replication pairs of the replication spec.
type cascadingArchivalModel struct {
	ArchivalLocationID    types.String           `tfsdk:"archival_location_id"`
	ArchivalThreshold     types.Int64            `tfsdk:"archival_threshold"`
	ArchivalThresholdUnit types.String           `tfsdk:"archival_threshold_unit"`
	ArchivalTiering       []archivalTieringModel `tfsdk:"archival_tiering"`
	Frequency             []string               `tfsdk:"frequency"`
}

// replicationSpecModel is an element of the replication_spec block.
type replicationSpecModel struct {
	AWSRegion         types.String             `tfsdk:"aws_region"`
	AWSCrossAccount   types.String             `tfsdk:"aws_cross_account"`
	AzureRegion       types.String             `tfsdk:"azure_region"`
	Retention         types.Int64              `tfsdk:"retention"`
	RetentionUnit     types.String             `tfsdk:"retention_unit"`
	ReplicationPair   []replicationPairModel   `tfsdk:"replication_pair"`
	LocalRetention    []localRetentionModel    `tfsdk:"local_retention"`
	CascadingArchival []cascadingArchivalModel `tfsdk:"cascading_archival"`
}

// sameAccount is the value of the AWS account and the Azure subscription of a
// replication spec which replicates within the same account or subscription.
const sameAccount = "SAME"

// fromReplicationSpecModels returns the SDK replication specs of the
// replication_spec blocks.
func fromReplicationSpecModels(specs []replicationSpecModel) ([]gqlsla.ReplicationSpec, error) {
	var replicationSpecs []gqlsla.ReplicationSpec
	for _, spec := range specs {
		retention := &gqlsla.RetentionDuration{
			Duration: int(spec.Retention.ValueInt64()),
			Unit:     gqlsla.RetentionUnit(spec.RetentionUnit.ValueString()),
		}

		// A cross account replication to AWS defaults to the same account.
		var awsRegion gqlaws.Region
		var awsCrossAccount string
		if name := spec.AWSRegion.ValueString(); name != "" {
			awsRegion = gqlaws.RegionFromName(name)
			if awsRegion == gqlaws.RegionUnknown {
				return nil, fmt.Errorf("unknown AWS region: %s", name)
			}
			awsCrossAccount = spec.AWSCrossAccount.ValueString()
			if awsCrossAccount == "" {
				awsCrossAccount = sameAccount
			}
		}

		// Replication to Azure is always within the same subscription.
		var azureRegion gqlazure.Region
		var azureCrossSubscription string
		if name := spec.AzureRegion.ValueString(); name != "" {
			azureRegion = gqlazure.RegionFromName(name)
			if azureRegion == gqlazure.RegionUnknown {
				return nil, fmt.Errorf("unknown Azure region: %s", name)
			}
			azureCrossSubscription = sameAccount
		}

		var replicationPairs []gqlsla.ReplicationPair
		for _, pair := range spec.ReplicationPair {
			replicationPairs = append(replicationPairs, gqlsla.ReplicationPair{
				SourceClusterID: pair.SourceCluster.ValueString(),
				TargetClusterID: pair.TargetCluster.ValueString(),
			})
		}

		cascadingArchivalSpecs, err := fromCascadingArchivalModels(spec.CascadingArchival, replicationPairs)
		if err != nil {
			return nil, err
		}

		replicationSpecs = append(replicationSpecs, gqlsla.ReplicationSpec{
			AWSRegion:                         awsRegion.ToRegionForReplicationEnum(),
			AWSAccount:                        awsCrossAccount,
			AzureRegion:                       azureRegion.ToRegionForReplicationEnum(),
			AzureSubscription:                 azureCrossSubscription,
			RetentionDuration:                 retention,
			ReplicationPairs:                  replicationPairs,
			ReplicationLocalRetentionDuration: fromLocalRetentionModel(firstOrNil(spec.LocalRetention)),
			CascadingArchivalSpecs:            cascadingArchivalSpecs,
		})
	}

	return replicationSpecs, nil
}

// fromCascadingArchivalModels returns the SDK cascading archival specs of the
// cascading_archival blocks. The archival location is mapped to the target
// cluster of each of the replication pairs of the replication spec.
func fromCascadingArchivalModels(cascading []cascadingArchivalModel, pairs []gqlsla.ReplicationPair) ([]gqlsla.CascadingArchivalSpec, error) {
	var specs []gqlsla.CascadingArchivalSpec
	for _, ca := range cascading {
		archivalLocationID, err := uuid.Parse(ca.ArchivalLocationID.ValueString())
		if err != nil {
			return nil, fmt.Errorf("invalid archival location ID: %w", err)
		}

		spec := gqlsla.CascadingArchivalSpec{}

		// Build the archival location to cluster mapping for each target cluster.
		// This is the recommended approach instead of the deprecated archival
		// location ID.
		for _, pair := range pairs {
			targetClusterID, err := uuid.Parse(pair.TargetClusterID)
			if err != nil {
				return nil, fmt.Errorf("invalid target cluster ID: %w", err)
			}
			spec.ArchivalLocationToClusterMappings = append(spec.ArchivalLocationToClusterMappings,
				gqlsla.ArchivalLocationToClusterMapping{ClusterID: targetClusterID, LocationID: archivalLocationID})
		}

		if threshold := ca.ArchivalThreshold.ValueInt64(); threshold > 0 {
			spec.ArchivalThreshold = &gqlsla.RetentionDuration{
				Duration: int(threshold),
				Unit:     gqlsla.RetentionUnit(ca.ArchivalThresholdUnit.ValueString()),
			}
		}

		spec.ArchivalTieringSpec = fromArchivalTieringModel(ca.ArchivalTiering)

		for _, freq := range ca.Frequency {
			spec.Frequencies = append(spec.Frequencies, gqlsla.RetentionUnit(freq))
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// toReplicationSpecModels returns the replication_spec blocks of the SDK
// replication specs.
func toReplicationSpecModels(replicationSpecs []gqlsla.ReplicationSpec) []replicationSpecModel {
	var models []replicationSpecModel
	for _, spec := range replicationSpecs {
		model := replicationSpecModel{
			AWSRegion:       types.StringValue(spec.AWSRegion.Name()),
			AWSCrossAccount: types.StringValue(spec.AWSAccount),
			AzureRegion:     types.StringValue(spec.AzureRegion.Name()),
			Retention:       types.Int64Value(int64(spec.RetentionDuration.Duration)),
			RetentionUnit:   types.StringValue(string(spec.RetentionDuration.Unit)),
		}

		for _, pair := range spec.ReplicationPairs {
			model.ReplicationPair = append(model.ReplicationPair, replicationPairModel{
				SourceCluster: types.StringValue(pair.SourceClusterID),
				TargetCluster: types.StringValue(pair.TargetClusterID),
			})
		}

		// The local retention is only present when the spec has one.
		if lr := toLocalRetentionModel(spec.ReplicationLocalRetentionDuration); lr != nil {
			model.LocalRetention = []localRetentionModel{*lr}
		}

		model.CascadingArchival = toCascadingArchivalModels(spec.CascadingArchivalSpecs)
		models = append(models, model)
	}

	return models
}

// toCascadingArchivalModels returns the cascading_archival blocks of the SDK
// cascading archival specs.
func toCascadingArchivalModels(specs []gqlsla.CascadingArchivalSpec) []cascadingArchivalModel {
	var models []cascadingArchivalModel
	for _, ca := range specs {
		model := cascadingArchivalModel{
			ArchivalThreshold:     types.Int64Null(),
			ArchivalThresholdUnit: types.StringNull(),
			ArchivalLocationID:    types.StringNull(),
		}

		// The mapping holds the same archival location for all clusters, so the
		// first mapping is used, with the deprecated archival location ID as the
		// fallback when there is no mapping.
		if len(ca.ArchivalLocationToClusterMappings) > 0 {
			model.ArchivalLocationID = types.StringValue(ca.ArchivalLocationToClusterMappings[0].LocationID.String())
			//lint:ignore SA1019 Fallback to deprecated field when reading in the else branch below.
		} else if alID := ca.ArchivalLocationID; alID != nil {
			model.ArchivalLocationID = types.StringValue(alID.String())
		}

		if ca.ArchivalThreshold != nil {
			model.ArchivalThreshold = types.Int64Value(int64(ca.ArchivalThreshold.Duration))
			model.ArchivalThresholdUnit = types.StringValue(string(ca.ArchivalThreshold.Unit))
		}

		model.ArchivalTiering = toArchivalTieringModel(ca.ArchivalTieringSpec)

		for _, freq := range ca.Frequencies {
			model.Frequency = append(model.Frequency, string(freq))
		}
		slices.Sort(model.Frequency)

		models = append(models, model)
	}

	return models
}

// firstOrNil returns a pointer to the first element, or nil for an empty slice.
// A block which holds at most one element is a slice in the model.
func firstOrNil[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}
