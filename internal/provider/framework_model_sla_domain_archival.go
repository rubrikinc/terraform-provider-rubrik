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
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// archivalTieringModel is the archival_tiering block of an archival spec. The
// block is also used by the cascading_archival block of a replication spec.
type archivalTieringModel struct {
	InstantTiering                 types.Bool   `tfsdk:"instant_tiering"`
	MinAccessibleDurationInSeconds types.Int64  `tfsdk:"min_accessible_duration_in_seconds"`
	ColdStorageClass               types.String `tfsdk:"cold_storage_class"`
	TierExistingSnapshots          types.Bool   `tfsdk:"tier_existing_snapshots"`
}

// archivalLocationToClusterMappingModel is an element of the
// archival_location_to_cluster_mapping block. The cluster name and the archival
// location name are computed, and are only used when reading from RSC.
type archivalLocationToClusterMappingModel struct {
	ClusterID          types.String `tfsdk:"cluster_id"`
	ArchivalLocationID types.String `tfsdk:"archival_location_id"`
	ClusterName        types.String `tfsdk:"cluster_name"`
	Name               types.String `tfsdk:"name"`
}

// archivalModel is an element of the archival block. The frequency is a set in
// the schema, so it is a slice of strings which can be null when the
// frequencies are derived from the snapshot schedule.
type archivalModel struct {
	ArchivalLocationID               types.String                            `tfsdk:"archival_location_id"`
	Threshold                        types.Int64                             `tfsdk:"threshold"`
	ThresholdUnit                    types.String                            `tfsdk:"threshold_unit"`
	ArchivalLocationToClusterMapping []archivalLocationToClusterMappingModel `tfsdk:"archival_location_to_cluster_mapping"`
	ArchivalTiering                  []archivalTieringModel                  `tfsdk:"archival_tiering"`
	Frequency                        []string                                `tfsdk:"frequency"`
}

// fromArchivalTieringModel returns the SDK archival tiering spec of the block,
// or nil if the block is not configured.
func fromArchivalTieringModel(tiering []archivalTieringModel) *gqlsla.ArchivalTieringSpec {
	if len(tiering) == 0 {
		return nil
	}

	return &gqlsla.ArchivalTieringSpec{
		InstantTiering:                 tiering[0].InstantTiering.ValueBool(),
		MinAccessibleDurationInSeconds: tiering[0].MinAccessibleDurationInSeconds.ValueInt64(),
		ColdStorageClass:               gqlsla.ColdStorageClass(tiering[0].ColdStorageClass.ValueString()),
		TierExistingSnapshots:          tiering[0].TierExistingSnapshots.ValueBool(),
	}
}

// toArchivalTieringModel returns the archival tiering block of the SDK spec, or
// nil if the spec is nil.
func toArchivalTieringModel(spec *gqlsla.ArchivalTieringSpec) []archivalTieringModel {
	if spec == nil {
		return nil
	}

	return []archivalTieringModel{{
		InstantTiering:                 types.BoolValue(spec.InstantTiering),
		MinAccessibleDurationInSeconds: types.Int64Value(spec.MinAccessibleDurationInSeconds),
		ColdStorageClass:               types.StringValue(string(spec.ColdStorageClass)),
		TierExistingSnapshots:          types.BoolValue(spec.TierExistingSnapshots),
	}}
}

// toArchivalTieringModelFromDomain returns the archival tiering block of the
// archival tiering spec held by an SDK domain, or nil if the spec is nil. The
// domain holds the spec in an anonymous struct, which is not an
// ArchivalTieringSpec, so the fields are copied.
func toArchivalTieringModelFromDomain(spec *struct {
	InstantTiering                 bool                    `json:"isInstantTieringEnabled"`
	MinAccessibleDurationInSeconds int64                   `json:"minAccessibleDurationInSeconds"`
	ColdStorageClass               gqlsla.ColdStorageClass `json:"coldStorageClass"`
	TierExistingSnapshots          bool                    `json:"shouldTierExistingSnapshots"`
}) []archivalTieringModel {
	if spec == nil {
		return nil
	}

	return toArchivalTieringModel(&gqlsla.ArchivalTieringSpec{
		InstantTiering:                 spec.InstantTiering,
		MinAccessibleDurationInSeconds: spec.MinAccessibleDurationInSeconds,
		ColdStorageClass:               spec.ColdStorageClass,
		TierExistingSnapshots:          spec.TierExistingSnapshots,
	})
}

// fromArchivalModels returns the SDK archival specs of the archival blocks. The
// frequencies of a block which has none are derived from the snapshot schedule.
func fromArchivalModels(archivals []archivalModel, schedule gqlsla.SnapshotSchedule) ([]gqlsla.ArchivalSpec, error) {
	var archivalSpecs []gqlsla.ArchivalSpec
	for _, archival := range archivals {
		var groupID uuid.UUID
		if alID := archival.ArchivalLocationID.ValueString(); len(alID) > 0 {
			var err error
			groupID, err = uuid.Parse(alID)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %s", keyArchivalLocationID, err)
			}
		}

		// Parse archival location to cluster mapping.
		var mappings []gqlsla.ArchivalLocationToClusterMapping
		for _, mapping := range archival.ArchivalLocationToClusterMapping {
			locationID, err := uuid.Parse(mapping.ArchivalLocationID.ValueString())
			if err != nil {
				return nil, fmt.Errorf("failed to parse archival location ID in mapping: %s", err)
			}

			var clusterID uuid.UUID
			if clusterIDStr := mapping.ClusterID.ValueString(); clusterIDStr != "" {
				clusterID, err = uuid.Parse(clusterIDStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse cluster ID in mapping: %s", err)
				}
			}

			mappings = append(mappings, gqlsla.ArchivalLocationToClusterMapping{
				ClusterID:  clusterID,
				LocationID: locationID,
			})
		}

		// Use the frequencies if they are specified, otherwise derive them from
		// the snapshot schedule.
		var frequencies []gqlsla.RetentionUnit
		if len(archival.Frequency) > 0 {
			for _, freq := range archival.Frequency {
				frequencies = append(frequencies, gqlsla.RetentionUnit(freq))
			}
		} else {
			frequencies = scheduleFrequencies(schedule)
		}

		archivalSpecs = append(archivalSpecs, gqlsla.ArchivalSpec{
			GroupID:                          groupID,
			Frequencies:                      frequencies,
			Threshold:                        int(archival.Threshold.ValueInt64()),
			ThresholdUnit:                    gqlsla.RetentionUnit(archival.ThresholdUnit.ValueString()),
			ArchivalLocationToClusterMapping: mappings,
			ArchivalTieringSpec:              fromArchivalTieringModel(archival.ArchivalTiering),
		})
	}

	return archivalSpecs, nil
}

// toArchivalModels returns the archival blocks of the SDK domain. The blocks are
// in the same order as the existing blocks, with the blocks which are not in
// the existing blocks added last in the order they have in the domain. This
// keeps the state from changing when RSC returns the specs in another order.
// An archival location which is used more than once is an error.
func toArchivalModels(domain gqlsla.Domain, existing []archivalModel) ([]archivalModel, error) {
	blocks := make(map[string]archivalModel)
	for _, spec := range domain.ArchivalSpecs {
		id := spec.StorageSetting.ID
		if _, ok := blocks[id]; ok {
			return nil, fmt.Errorf("archival location %q used multiple times", id)
		}

		// The archival location to cluster mapping holds the cluster name and
		// the archival location name, which RSC returns.
		var mappings []archivalLocationToClusterMappingModel
		for _, mapping := range spec.ArchivalLocationToClusterMapping {
			mappings = append(mappings, archivalLocationToClusterMappingModel{
				ClusterID:          types.StringValue(mapping.Cluster.ID),
				ClusterName:        types.StringValue(mapping.Cluster.Name),
				ArchivalLocationID: types.StringValue(mapping.Location.ID),
				Name:               types.StringValue(mapping.Location.Name),
			})
		}

		// A nil frequency means the frequencies are derived from the snapshot
		// schedule.
		var frequencies []string
		for _, freq := range spec.Frequencies {
			frequencies = append(frequencies, string(freq))
		}
		slices.Sort(frequencies)

		blocks[id] = archivalModel{
			ArchivalLocationID:               types.StringValue(id),
			Threshold:                        types.Int64Value(int64(spec.Threshold)),
			ThresholdUnit:                    types.StringValue(string(spec.ThresholdUnit)),
			ArchivalLocationToClusterMapping: mappings,
			ArchivalTiering:                  toArchivalTieringModelFromDomain(spec.ArchivalTieringSpec),
			Frequency:                        frequencies,
		}
	}

	// Preserve the order of the existing blocks, then add the new ones last.
	var sorted []archivalModel
	for _, old := range existing {
		id := old.ArchivalLocationID.ValueString()
		if block, ok := blocks[id]; ok {
			sorted = append(sorted, block)
			delete(blocks, id)
		}
	}
	for _, spec := range domain.ArchivalSpecs {
		id := spec.StorageSetting.ID
		if block, ok := blocks[id]; ok {
			sorted = append(sorted, block)
			delete(blocks, id)
		}
	}

	return sorted, nil
}
