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
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

func TestToReplicationSpecModels(t *testing.T) {
	tests := []struct {
		name             string
		replicationSpecs []gqlsla.ReplicationSpec
		wantAWSRegion    string
		wantAWSAccount   string
		wantAzureRegion  string
		wantRetention    int64
		wantUnit         string
	}{
		{
			name: "AWS replication",
			replicationSpecs: []gqlsla.ReplicationSpec{{
				AWSRegion:         gqlaws.RegionFromName("us-east-1").ToRegionForReplicationEnum(),
				AWSAccount:        "123456789012",
				RetentionDuration: &gqlsla.RetentionDuration{Duration: 30, Unit: gqlsla.Days},
			}},
			wantAWSRegion: "us-east-1", wantAWSAccount: "123456789012", wantRetention: 30, wantUnit: "DAYS",
		},
		{
			name: "Azure replication",
			replicationSpecs: []gqlsla.ReplicationSpec{{
				AzureRegion:       gqlazure.RegionFromName("eastus").ToRegionForReplicationEnum(),
				RetentionDuration: &gqlsla.RetentionDuration{Duration: 90, Unit: gqlsla.Weeks},
			}},
			wantAzureRegion: "eastus", wantRetention: 90, wantUnit: "WEEKS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toReplicationSpecModels(tt.replicationSpecs)
			if len(got) != 1 {
				t.Fatalf("expected 1 spec, got %d", len(got))
			}
			m := got[0]
			if m.AWSRegion.ValueString() != tt.wantAWSRegion || m.AWSCrossAccount.ValueString() != tt.wantAWSAccount ||
				m.AzureRegion.ValueString() != tt.wantAzureRegion || m.Retention.ValueInt64() != tt.wantRetention ||
				m.RetentionUnit.ValueString() != tt.wantUnit {
				t.Errorf("unexpected model: %+v", m)
			}
			// The regions which are not set are empty strings, and not null.
			if m.AWSRegion.IsNull() || m.AzureRegion.IsNull() {
				t.Error("an unset region must be an empty string, not null")
			}
		})
	}

	t.Run("no specs", func(t *testing.T) {
		if got := toReplicationSpecModels(nil); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

func TestToReplicationSpecModelsDetails(t *testing.T) {
	loc := uuid.New()
	got := toReplicationSpecModels([]gqlsla.ReplicationSpec{{
		RetentionDuration: &gqlsla.RetentionDuration{Duration: 7, Unit: gqlsla.Days},
		ReplicationPairs:  []gqlsla.ReplicationPair{{SourceClusterID: "src", TargetClusterID: "dst"}},
		ReplicationLocalRetentionDuration: &gqlsla.RetentionDuration{
			Duration: 3, Unit: gqlsla.Weeks,
		},
		CascadingArchivalSpecs: []gqlsla.CascadingArchivalSpec{{
			ArchivalLocationToClusterMappings: []gqlsla.ArchivalLocationToClusterMapping{
				{ClusterID: uuid.New(), LocationID: loc}, {ClusterID: uuid.New(), LocationID: uuid.New()},
			},
			ArchivalThreshold: &gqlsla.RetentionDuration{Duration: 10, Unit: gqlsla.Days},
			Frequencies:       []gqlsla.RetentionUnit{gqlsla.Weeks, gqlsla.Days},
		}},
	}})

	m := got[0]
	if len(m.ReplicationPair) != 1 || m.ReplicationPair[0].SourceCluster.ValueString() != "src" ||
		m.ReplicationPair[0].TargetCluster.ValueString() != "dst" {
		t.Errorf("unexpected replication pairs: %+v", m.ReplicationPair)
	}
	if len(m.LocalRetention) != 1 || m.LocalRetention[0].Retention.ValueInt64() != 3 ||
		m.LocalRetention[0].RetentionUnit.ValueString() != "WEEKS" {
		t.Errorf("unexpected local retention: %+v", m.LocalRetention)
	}

	if len(m.CascadingArchival) != 1 {
		t.Fatalf("expected 1 cascading archival, got %d", len(m.CascadingArchival))
	}
	ca := m.CascadingArchival[0]
	// The first mapping's archival location is used, as the mappings hold the
	// same location for all clusters.
	if ca.ArchivalLocationID.ValueString() != loc.String() {
		t.Errorf("archival location ID = %s, want %s", ca.ArchivalLocationID.ValueString(), loc)
	}
	if ca.ArchivalThreshold.ValueInt64() != 10 || ca.ArchivalThresholdUnit.ValueString() != "DAYS" {
		t.Errorf("unexpected threshold: %+v", ca)
	}
	if want := []string{"DAYS", "WEEKS"}; !slices.Equal(ca.Frequency, want) {
		t.Errorf("frequency = %v, want %v", ca.Frequency, want)
	}
}

// TestToCascadingArchivalModelsFallback verifies that the deprecated archival
// location ID is used when there is no mapping, and that a missing threshold and
// frequencies are null.
func TestToCascadingArchivalModelsFallback(t *testing.T) {
	loc := uuid.New()

	t.Run("deprecated archival location ID", func(t *testing.T) {
		got := toCascadingArchivalModels([]gqlsla.CascadingArchivalSpec{{ArchivalLocationID: &loc}})
		if got[0].ArchivalLocationID.ValueString() != loc.String() {
			t.Errorf("archival location ID = %s, want %s", got[0].ArchivalLocationID.ValueString(), loc)
		}
	})

	t.Run("no location at all", func(t *testing.T) {
		got := toCascadingArchivalModels([]gqlsla.CascadingArchivalSpec{{}})
		if !got[0].ArchivalLocationID.IsNull() || !got[0].ArchivalThreshold.IsNull() ||
			!got[0].ArchivalThresholdUnit.IsNull() || got[0].Frequency != nil || got[0].ArchivalTiering != nil {
			t.Errorf("unset values must be null, got %+v", got[0])
		}
	})
}

func TestFromReplicationSpecModels(t *testing.T) {
	retention := func(m replicationSpecModel) replicationSpecModel {
		m.Retention = types.Int64Value(30)
		m.RetentionUnit = types.StringValue("DAYS")
		return m
	}

	t.Run("AWS account defaults to SAME", func(t *testing.T) {
		got, err := fromReplicationSpecModels([]replicationSpecModel{retention(replicationSpecModel{
			AWSRegion: types.StringValue("us-east-1"), AWSCrossAccount: types.StringNull(),
			AzureRegion: types.StringNull(),
		})})
		if err != nil {
			t.Fatal(err)
		}
		if got[0].AWSAccount != "SAME" || got[0].AzureSubscription != "" {
			t.Errorf("AWS account %q, Azure subscription %q", got[0].AWSAccount, got[0].AzureSubscription)
		}
		if got[0].AWSRegion != gqlaws.RegionFromName("us-east-1").ToRegionForReplicationEnum() {
			t.Errorf("unexpected AWS region %v", got[0].AWSRegion)
		}
	})

	t.Run("AWS account is kept when set", func(t *testing.T) {
		got, err := fromReplicationSpecModels([]replicationSpecModel{retention(replicationSpecModel{
			AWSRegion: types.StringValue("us-east-1"), AWSCrossAccount: types.StringValue("123456789012"),
			AzureRegion: types.StringNull(),
		})})
		if err != nil {
			t.Fatal(err)
		}
		if got[0].AWSAccount != "123456789012" {
			t.Errorf("AWS account = %q, want 123456789012", got[0].AWSAccount)
		}
	})

	t.Run("AWS account is ignored without a region", func(t *testing.T) {
		got, err := fromReplicationSpecModels([]replicationSpecModel{retention(replicationSpecModel{
			AWSRegion: types.StringValue(""), AWSCrossAccount: types.StringValue("123456789012"),
			AzureRegion: types.StringValue(""),
		})})
		if err != nil {
			t.Fatal(err)
		}
		if got[0].AWSAccount != "" {
			t.Errorf("AWS account = %q, want empty", got[0].AWSAccount)
		}
	})

	t.Run("Azure subscription is always SAME", func(t *testing.T) {
		got, err := fromReplicationSpecModels([]replicationSpecModel{retention(replicationSpecModel{
			AWSRegion: types.StringNull(), AzureRegion: types.StringValue("eastus"),
		})})
		if err != nil {
			t.Fatal(err)
		}
		if got[0].AzureSubscription != "SAME" || got[0].AWSAccount != "" {
			t.Errorf("Azure subscription %q, AWS account %q", got[0].AzureSubscription, got[0].AWSAccount)
		}
	})

	t.Run("unknown regions are errors", func(t *testing.T) {
		_, err := fromReplicationSpecModels([]replicationSpecModel{retention(replicationSpecModel{
			AWSRegion: types.StringValue("mars-north-1"),
		})})
		if err == nil || !strings.Contains(err.Error(), "unknown AWS region: mars-north-1") {
			t.Errorf("expected an unknown AWS region error, got %v", err)
		}
		_, err = fromReplicationSpecModels([]replicationSpecModel{retention(replicationSpecModel{
			AzureRegion: types.StringValue("marsnorth"),
		})})
		if err == nil || !strings.Contains(err.Error(), "unknown Azure region: marsnorth") {
			t.Errorf("expected an unknown Azure region error, got %v", err)
		}
	})

	t.Run("pairs and local retention", func(t *testing.T) {
		got, err := fromReplicationSpecModels([]replicationSpecModel{retention(replicationSpecModel{
			ReplicationPair: []replicationPairModel{{
				SourceCluster: types.StringValue("src"), TargetCluster: types.StringValue("dst"),
			}},
			LocalRetention: []localRetentionModel{{
				Retention: types.Int64Value(3), RetentionUnit: types.StringValue("WEEKS"),
			}},
		})})
		if err != nil {
			t.Fatal(err)
		}
		if want := (gqlsla.ReplicationPair{SourceClusterID: "src", TargetClusterID: "dst"}); got[0].ReplicationPairs[0] != want {
			t.Errorf("pair = %+v, want %+v", got[0].ReplicationPairs[0], want)
		}
		if lr := got[0].ReplicationLocalRetentionDuration; lr == nil || lr.Duration != 3 || lr.Unit != gqlsla.Weeks {
			t.Errorf("local retention = %+v", lr)
		}
	})
}

// TestFromCascadingArchivalModels verifies that the archival location is mapped
// to the target cluster of every replication pair of the replication spec.
func TestFromCascadingArchivalModels(t *testing.T) {
	loc := uuid.New()
	dst1, dst2 := uuid.New(), uuid.New()
	pairs := []gqlsla.ReplicationPair{
		{SourceClusterID: uuid.NewString(), TargetClusterID: dst1.String()},
		{SourceClusterID: uuid.NewString(), TargetClusterID: dst2.String()},
	}

	t.Run("one mapping per replication pair", func(t *testing.T) {
		got, err := fromCascadingArchivalModels([]cascadingArchivalModel{{
			ArchivalLocationID:    types.StringValue(loc.String()),
			ArchivalThreshold:     types.Int64Value(10),
			ArchivalThresholdUnit: types.StringValue("DAYS"),
			Frequency:             []string{"DAYS"},
			ArchivalTiering: []archivalTieringModel{{
				InstantTiering: types.BoolValue(true), ColdStorageClass: types.StringValue("AWS_GLACIER"),
			}},
		}}, pairs)
		if err != nil {
			t.Fatal(err)
		}

		want := []gqlsla.ArchivalLocationToClusterMapping{
			{ClusterID: dst1, LocationID: loc}, {ClusterID: dst2, LocationID: loc},
		}
		if !slices.Equal(got[0].ArchivalLocationToClusterMappings, want) {
			t.Errorf("mappings = %+v, want %+v", got[0].ArchivalLocationToClusterMappings, want)
		}
		if got[0].ArchivalThreshold == nil || got[0].ArchivalThreshold.Duration != 10 {
			t.Errorf("threshold = %+v", got[0].ArchivalThreshold)
		}
		if got[0].ArchivalTieringSpec == nil || !got[0].ArchivalTieringSpec.InstantTiering {
			t.Errorf("tiering = %+v", got[0].ArchivalTieringSpec)
		}
		if !slices.Equal(got[0].Frequencies, []gqlsla.RetentionUnit{gqlsla.Days}) {
			t.Errorf("frequencies = %v", got[0].Frequencies)
		}
	})

	t.Run("no pairs gives no mappings", func(t *testing.T) {
		got, err := fromCascadingArchivalModels([]cascadingArchivalModel{{
			ArchivalLocationID: types.StringValue(loc.String()),
		}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got[0].ArchivalLocationToClusterMappings) != 0 {
			t.Errorf("mappings = %+v, want none", got[0].ArchivalLocationToClusterMappings)
		}
	})

	t.Run("a threshold of zero is not set", func(t *testing.T) {
		got, err := fromCascadingArchivalModels([]cascadingArchivalModel{{
			ArchivalLocationID: types.StringValue(loc.String()), ArchivalThreshold: types.Int64Value(0),
			ArchivalThresholdUnit: types.StringValue("DAYS"),
		}}, pairs)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].ArchivalThreshold != nil {
			t.Errorf("threshold = %+v, want nil", got[0].ArchivalThreshold)
		}
	})

	t.Run("errors", func(t *testing.T) {
		_, err := fromCascadingArchivalModels([]cascadingArchivalModel{{
			ArchivalLocationID: types.StringValue("not-a-uuid"),
		}}, pairs)
		if err == nil || !strings.Contains(err.Error(), "invalid archival location ID") {
			t.Errorf("expected an invalid archival location ID error, got %v", err)
		}

		_, err = fromCascadingArchivalModels([]cascadingArchivalModel{{
			ArchivalLocationID: types.StringValue(loc.String()),
		}}, []gqlsla.ReplicationPair{{TargetClusterID: "not-a-uuid"}})
		if err == nil || !strings.Contains(err.Error(), "invalid target cluster ID") {
			t.Errorf("expected an invalid target cluster ID error, got %v", err)
		}
	})
}

// TestReplicationModelsMatchAttributes decodes the replication models from the
// attribute names the SLA domain schema uses.
func TestReplicationModelsMatchAttributes(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }

	t.Run("pair", func(t *testing.T) {
		var m replicationPairModel
		decodeModel(t, map[string]attr.Type{keySourceCluster: types.StringType, keyTargetCluster: types.StringType},
			map[string]attr.Value{keySourceCluster: str("s"), keyTargetCluster: str("t")}, &m)
		if m.SourceCluster.ValueString() != "s" || m.TargetCluster.ValueString() != "t" {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("cascading archival", func(t *testing.T) {
		tieringType := types.ObjectType{AttrTypes: map[string]attr.Type{
			keyInstantTiering: types.BoolType, keyMinAccessibleDurationInSeconds: types.Int64Type,
			keyColdStorageClass: types.StringType, keyTierExistingSnapshots: types.BoolType,
		}}
		var m cascadingArchivalModel
		decodeModel(t, map[string]attr.Type{
			keyArchivalLocationID: types.StringType, keyArchivalThreshold: types.Int64Type,
			keyArchivalThresholdUnit: types.StringType, keyArchivalTiering: types.ListType{ElemType: tieringType},
			keyFrequency: types.SetType{ElemType: types.StringType},
		}, map[string]attr.Value{
			keyArchivalLocationID: str("l"), keyArchivalThreshold: types.Int64Value(5),
			keyArchivalThresholdUnit: str("DAYS"), keyArchivalTiering: types.ListValueMust(tieringType, nil),
			keyFrequency: types.SetValueMust(types.StringType, []attr.Value{str("DAYS")}),
		}, &m)
		if m.ArchivalThreshold.ValueInt64() != 5 || !slices.Equal(m.Frequency, []string{"DAYS"}) {
			t.Errorf("unexpected model: %+v", m)
		}
	})

	t.Run("replication spec", func(t *testing.T) {
		pairType := types.ObjectType{AttrTypes: map[string]attr.Type{
			keySourceCluster: types.StringType, keyTargetCluster: types.StringType,
		}}
		localType := types.ObjectType{AttrTypes: map[string]attr.Type{
			keyRetention: types.Int64Type, keyRetentionUnit: types.StringType,
		}}
		caType := types.ObjectType{AttrTypes: map[string]attr.Type{
			keyArchivalLocationID: types.StringType, keyArchivalThreshold: types.Int64Type,
			keyArchivalThresholdUnit: types.StringType,
			keyArchivalTiering: types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
				keyInstantTiering: types.BoolType, keyMinAccessibleDurationInSeconds: types.Int64Type,
				keyColdStorageClass: types.StringType, keyTierExistingSnapshots: types.BoolType,
			}}},
			keyFrequency: types.SetType{ElemType: types.StringType},
		}}
		var m replicationSpecModel
		decodeModel(t, map[string]attr.Type{
			keyAWSRegion: types.StringType, keyAWSCrossAccount: types.StringType, keyAzureRegion: types.StringType,
			keyRetention: types.Int64Type, keyRetentionUnit: types.StringType,
			keyReplicationPair: types.ListType{ElemType: pairType}, keyLocalRetention: types.ListType{ElemType: localType},
			keyCascadingArchival: types.ListType{ElemType: caType},
		}, map[string]attr.Value{
			keyAWSRegion: str("us-east-1"), keyAWSCrossAccount: str("SAME"), keyAzureRegion: str(""),
			keyRetention: types.Int64Value(7), keyRetentionUnit: str("DAYS"),
			keyReplicationPair:   types.ListValueMust(pairType, nil),
			keyLocalRetention:    types.ListValueMust(localType, nil),
			keyCascadingArchival: types.ListValueMust(caType, nil),
		}, &m)
		if m.AWSRegion.ValueString() != "us-east-1" || m.Retention.ValueInt64() != 7 {
			t.Errorf("unexpected model: %+v", m)
		}
	})
}
