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
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// domainArchivalSpec is the type of an element of the archival specs of an SDK
// domain, which is an anonymous struct.
type domainArchivalSpec = struct {
	Frequencies    []gqlsla.RetentionUnit `json:"frequencies"`
	Threshold      int                    `json:"threshold"`
	ThresholdUnit  gqlsla.RetentionUnit   `json:"thresholdUnit"`
	StorageSetting struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"storageSetting"`
	ArchivalLocationToClusterMapping []struct {
		Cluster struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"cluster"`
		Location struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"location"`
	} `json:"archivalLocationToClusterMapping"`
	ArchivalTieringSpec *struct {
		InstantTiering                 bool                    `json:"isInstantTieringEnabled"`
		MinAccessibleDurationInSeconds int64                   `json:"minAccessibleDurationInSeconds"`
		ColdStorageClass               gqlsla.ColdStorageClass `json:"coldStorageClass"`
		TierExistingSnapshots          bool                    `json:"shouldTierExistingSnapshots"`
	} `json:"archivalTieringSpec"`
}

// newDomainArchivalSpec returns an archival spec of an SDK domain.
func newDomainArchivalSpec(id string, threshold int, unit gqlsla.RetentionUnit) domainArchivalSpec {
	var spec domainArchivalSpec
	spec.StorageSetting.ID = id
	spec.Threshold = threshold
	spec.ThresholdUnit = unit
	return spec
}

func TestToArchivalModels(t *testing.T) {
	loc1, loc2, loc3 := uuid.New().String(), uuid.New().String(), uuid.New().String()
	existing := func(ids ...string) []archivalModel {
		models := make([]archivalModel, 0, len(ids))
		for _, id := range ids {
			models = append(models, archivalModel{ArchivalLocationID: types.StringValue(id)})
		}
		return models
	}

	tests := []struct {
		name          string
		specs         []domainArchivalSpec
		existing      []archivalModel
		wantIDs       []string
		wantThreshold []int64
		wantError     string
	}{
		{name: "empty specs and existing", specs: nil, existing: nil},
		{
			name: "new specs only",
			specs: []domainArchivalSpec{
				newDomainArchivalSpec(loc1, 30, gqlsla.Days), newDomainArchivalSpec(loc2, 90, gqlsla.Weeks),
			},
			wantIDs: []string{loc1, loc2}, wantThreshold: []int64{30, 90},
		},
		{
			name: "preserve existing order",
			specs: []domainArchivalSpec{
				newDomainArchivalSpec(loc1, 30, gqlsla.Days), newDomainArchivalSpec(loc2, 90, gqlsla.Weeks),
			},
			existing: existing(loc2, loc1),
			wantIDs:  []string{loc2, loc1}, wantThreshold: []int64{90, 30},
		},
		{
			name: "add new specs to end",
			specs: []domainArchivalSpec{
				newDomainArchivalSpec(loc1, 30, gqlsla.Days), newDomainArchivalSpec(loc2, 90, gqlsla.Weeks),
				newDomainArchivalSpec(loc3, 12, gqlsla.Months),
			},
			existing: existing(loc2),
			wantIDs:  []string{loc2, loc1, loc3}, wantThreshold: []int64{90, 30, 12},
		},
		{
			name: "remove existing specs",
			specs: []domainArchivalSpec{
				newDomainArchivalSpec(loc1, 30, gqlsla.Days), newDomainArchivalSpec(loc3, 12, gqlsla.Months),
			},
			existing: existing(loc3, loc2, loc1),
			wantIDs:  []string{loc3, loc1}, wantThreshold: []int64{12, 30},
		},
		{
			name: "duplicate location IDs",
			specs: []domainArchivalSpec{
				newDomainArchivalSpec(loc1, 30, gqlsla.Days), newDomainArchivalSpec(loc1, 90, gqlsla.Weeks),
			},
			wantError: "used multiple times",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toArchivalModels(gqlsla.Domain{ArchivalSpecs: tt.specs}, tt.existing)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected an error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("expected %d items, got %d", len(tt.wantIDs), len(got))
			}
			for i := range got {
				if got[i].ArchivalLocationID.ValueString() != tt.wantIDs[i] ||
					got[i].Threshold.ValueInt64() != tt.wantThreshold[i] {
					t.Errorf("item %d: got %s/%d, want %s/%d", i, got[i].ArchivalLocationID.ValueString(),
						got[i].Threshold.ValueInt64(), tt.wantIDs[i], tt.wantThreshold[i])
				}
			}
		})
	}
}

// TestToArchivalModelsDetails verifies the cluster mapping, the tiering and the
// frequencies which RSC returns.
func TestToArchivalModelsDetails(t *testing.T) {
	spec := newDomainArchivalSpec("loc", 30, gqlsla.Days)
	spec.Frequencies = []gqlsla.RetentionUnit{gqlsla.Weeks, gqlsla.Days}
	spec.ArchivalLocationToClusterMapping = append(spec.ArchivalLocationToClusterMapping, struct {
		Cluster struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"cluster"`
		Location struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"location"`
	}{})
	spec.ArchivalLocationToClusterMapping[0].Cluster.ID = "cluster-id"
	spec.ArchivalLocationToClusterMapping[0].Cluster.Name = "cluster-name"
	spec.ArchivalLocationToClusterMapping[0].Location.ID = "loc"
	spec.ArchivalLocationToClusterMapping[0].Location.Name = "location-name"
	spec.ArchivalTieringSpec = &struct {
		InstantTiering                 bool                    `json:"isInstantTieringEnabled"`
		MinAccessibleDurationInSeconds int64                   `json:"minAccessibleDurationInSeconds"`
		ColdStorageClass               gqlsla.ColdStorageClass `json:"coldStorageClass"`
		TierExistingSnapshots          bool                    `json:"shouldTierExistingSnapshots"`
	}{InstantTiering: true, MinAccessibleDurationInSeconds: 3600, ColdStorageClass: gqlsla.ColdStorageClassAWSGlacier}

	got, err := toArchivalModels(gqlsla.Domain{ArchivalSpecs: []domainArchivalSpec{spec}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 block, got %d", len(got))
	}

	m := got[0].ArchivalLocationToClusterMapping
	if len(m) != 1 || m[0].ClusterID.ValueString() != "cluster-id" || m[0].ClusterName.ValueString() != "cluster-name" ||
		m[0].ArchivalLocationID.ValueString() != "loc" || m[0].Name.ValueString() != "location-name" {
		t.Errorf("unexpected mapping: %+v", m)
	}

	if len(got[0].ArchivalTiering) != 1 || !got[0].ArchivalTiering[0].InstantTiering.ValueBool() ||
		got[0].ArchivalTiering[0].MinAccessibleDurationInSeconds.ValueInt64() != 3600 ||
		got[0].ArchivalTiering[0].ColdStorageClass.ValueString() != string(gqlsla.ColdStorageClassAWSGlacier) {
		t.Errorf("unexpected tiering: %+v", got[0].ArchivalTiering)
	}

	// The frequencies are a set in the schema, so they are sorted.
	if want := []string{"DAYS", "WEEKS"}; !slices.Equal(got[0].Frequency, want) {
		t.Errorf("frequency = %v, want %v", got[0].Frequency, want)
	}

	// A spec without tiering and frequencies has none in the block, so the
	// frequencies are derived from the snapshot schedule.
	bare, err := toArchivalModels(gqlsla.Domain{
		ArchivalSpecs: []domainArchivalSpec{newDomainArchivalSpec("loc", 1, gqlsla.Days)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bare[0].ArchivalTiering != nil || bare[0].Frequency != nil {
		t.Errorf("a bare spec must have no tiering and null frequencies, got %+v", bare[0])
	}
}

func TestFromArchivalModels(t *testing.T) {
	loc, cluster := uuid.New(), uuid.New()
	hourly := gqlsla.SnapshotSchedule{
		Hourly: &gqlsla.HourlySnapshotSchedule{},
		Daily:  &gqlsla.DailySnapshotSchedule{},
	}

	t.Run("frequencies are derived from the schedule when not set", func(t *testing.T) {
		got, err := fromArchivalModels([]archivalModel{{
			ArchivalLocationID: types.StringValue(loc.String()), Threshold: types.Int64Value(30),
			ThresholdUnit: types.StringValue("DAYS"),
		}}, hourly)
		if err != nil {
			t.Fatal(err)
		}
		if want := []gqlsla.RetentionUnit{gqlsla.Hours, gqlsla.Days}; !slices.Equal(got[0].Frequencies, want) {
			t.Errorf("frequencies = %v, want %v", got[0].Frequencies, want)
		}
		if got[0].GroupID != loc || got[0].Threshold != 30 || got[0].ThresholdUnit != gqlsla.Days {
			t.Errorf("unexpected spec: %+v", got[0])
		}
	})

	t.Run("specified frequencies override the schedule", func(t *testing.T) {
		got, err := fromArchivalModels([]archivalModel{{
			ArchivalLocationID: types.StringValue(loc.String()), Frequency: []string{"WEEKS"},
		}}, hourly)
		if err != nil {
			t.Fatal(err)
		}
		if want := []gqlsla.RetentionUnit{gqlsla.Weeks}; !slices.Equal(got[0].Frequencies, want) {
			t.Errorf("frequencies = %v, want %v", got[0].Frequencies, want)
		}
	})

	t.Run("an unset archival location ID gives the nil group ID", func(t *testing.T) {
		got, err := fromArchivalModels([]archivalModel{{
			ArchivalLocationID: types.StringUnknown(),
			ArchivalLocationToClusterMapping: []archivalLocationToClusterMappingModel{{
				ArchivalLocationID: types.StringValue(loc.String()), ClusterID: types.StringValue(cluster.String()),
			}},
		}}, hourly)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].GroupID != uuid.Nil {
			t.Errorf("group ID = %s, want the nil UUID", got[0].GroupID)
		}
		want := gqlsla.ArchivalLocationToClusterMapping{ClusterID: cluster, LocationID: loc}
		if len(got[0].ArchivalLocationToClusterMapping) != 1 || got[0].ArchivalLocationToClusterMapping[0] != want {
			t.Errorf("unexpected mapping: %+v", got[0].ArchivalLocationToClusterMapping)
		}
	})

	t.Run("a mapping without a cluster ID gives the nil cluster ID", func(t *testing.T) {
		got, err := fromArchivalModels([]archivalModel{{
			ArchivalLocationToClusterMapping: []archivalLocationToClusterMappingModel{{
				ArchivalLocationID: types.StringValue(loc.String()), ClusterID: types.StringNull(),
			}},
		}}, hourly)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].ArchivalLocationToClusterMapping[0].ClusterID != uuid.Nil {
			t.Error("the cluster ID must be the nil UUID")
		}
	})

	t.Run("tiering", func(t *testing.T) {
		got, err := fromArchivalModels([]archivalModel{{
			ArchivalLocationID: types.StringValue(loc.String()),
			ArchivalTiering: []archivalTieringModel{{
				InstantTiering: types.BoolValue(true), MinAccessibleDurationInSeconds: types.Int64Value(60),
				ColdStorageClass: types.StringValue("AWS_GLACIER"), TierExistingSnapshots: types.BoolValue(true),
			}},
		}}, hourly)
		if err != nil {
			t.Fatal(err)
		}
		want := &gqlsla.ArchivalTieringSpec{
			InstantTiering: true, MinAccessibleDurationInSeconds: 60,
			ColdStorageClass: gqlsla.ColdStorageClassAWSGlacier, TierExistingSnapshots: true,
		}
		if got[0].ArchivalTieringSpec == nil || *got[0].ArchivalTieringSpec != *want {
			t.Errorf("tiering = %+v, want %+v", got[0].ArchivalTieringSpec, want)
		}
	})

	errorTests := []struct {
		name  string
		model archivalModel
		want  string
	}{
		{
			name:  "invalid archival location ID",
			model: archivalModel{ArchivalLocationID: types.StringValue("not-a-uuid")},
			want:  "failed to parse archival_location_id",
		},
		{
			name: "invalid mapping archival location ID",
			model: archivalModel{ArchivalLocationToClusterMapping: []archivalLocationToClusterMappingModel{{
				ArchivalLocationID: types.StringValue("nope"),
			}}},
			want: "failed to parse archival location ID in mapping",
		},
		{
			name: "invalid mapping cluster ID",
			model: archivalModel{ArchivalLocationToClusterMapping: []archivalLocationToClusterMappingModel{{
				ArchivalLocationID: types.StringValue(loc.String()), ClusterID: types.StringValue("nope"),
			}}},
			want: "failed to parse cluster ID in mapping",
		},
	}
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fromArchivalModels([]archivalModel{tt.model}, hourly)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected an error containing %q, got %v", tt.want, err)
			}
		})
	}
}

// TestArchivalModelsMatchAttributes decodes the archival models from the
// attribute names the SLA domain schema uses.
func TestArchivalModelsMatchAttributes(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }
	tieringTypes := map[string]attr.Type{
		keyInstantTiering: types.BoolType, keyMinAccessibleDurationInSeconds: types.Int64Type,
		keyColdStorageClass: types.StringType, keyTierExistingSnapshots: types.BoolType,
	}

	t.Run("tiering", func(t *testing.T) {
		var m archivalTieringModel
		decodeModel(t, tieringTypes, map[string]attr.Value{
			keyInstantTiering: types.BoolValue(true), keyMinAccessibleDurationInSeconds: types.Int64Value(1),
			keyColdStorageClass: str("AWS_GLACIER"), keyTierExistingSnapshots: types.BoolValue(false),
		}, &m)
		if !m.InstantTiering.ValueBool() || m.ColdStorageClass.ValueString() != "AWS_GLACIER" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("mapping", func(t *testing.T) {
		var m archivalLocationToClusterMappingModel
		decodeModel(t, map[string]attr.Type{
			keyClusterID: types.StringType, keyArchivalLocationID: types.StringType,
			keyClusterName: types.StringType, keyName: types.StringType,
		}, map[string]attr.Value{
			keyClusterID: str("c"), keyArchivalLocationID: str("l"), keyClusterName: str("cn"), keyName: str("n"),
		}, &m)
		if m.ClusterID.ValueString() != "c" || m.Name.ValueString() != "n" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("archival", func(t *testing.T) {
		var m archivalModel
		decodeModel(t, map[string]attr.Type{
			keyArchivalLocationID: types.StringType, keyThreshold: types.Int64Type, keyThresholdUnit: types.StringType,
			keyArchivalLocationToClusterMapping: types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
				keyClusterID: types.StringType, keyArchivalLocationID: types.StringType,
				keyClusterName: types.StringType, keyName: types.StringType,
			}}},
			keyArchivalTiering: types.ListType{ElemType: types.ObjectType{AttrTypes: tieringTypes}},
			keyFrequency:       types.SetType{ElemType: types.StringType},
		}, map[string]attr.Value{
			keyArchivalLocationID: str("l"), keyThreshold: types.Int64Value(30), keyThresholdUnit: str("DAYS"),
			keyArchivalLocationToClusterMapping: types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
				keyClusterID: types.StringType, keyArchivalLocationID: types.StringType,
				keyClusterName: types.StringType, keyName: types.StringType,
			}}, nil),
			keyArchivalTiering: types.ListValueMust(types.ObjectType{AttrTypes: tieringTypes}, nil),
			keyFrequency:       types.SetValueMust(types.StringType, []attr.Value{str("DAYS")}),
		}, &m)
		if m.Threshold.ValueInt64() != 30 || !slices.Equal(m.Frequency, []string{"DAYS"}) {
			t.Errorf("unexpected model: %+v", m)
		}
	})
}
