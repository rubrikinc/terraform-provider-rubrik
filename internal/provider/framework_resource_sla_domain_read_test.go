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
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// slaDomainPlan returns a plan of the SLA domain where the given attributes have
// the given values. The attributes which are computed, and not set, are unknown
// as in a real plan.
func slaDomainPlan(t *testing.T, set map[string]tftypes.Value) tfsdk.Plan {
	t.Helper()

	cfg := slaDomainConfig(t, set)
	return tfsdk.Plan(cfg)
}

// oneBlockWith returns the named block with a single element, where the given
// attributes have the given values.
func oneBlockWith(t *testing.T, name string, attrs map[string]tftypes.Value) tftypes.Value {
	t.Helper()

	block := oneBlock(t, name)
	var elems []tftypes.Value
	if err := block.As(&elems); err != nil {
		t.Fatal(err)
	}
	var values map[string]tftypes.Value
	if err := elems[0].As(&values); err != nil {
		t.Fatal(err)
	}
	for k, v := range attrs {
		values[k] = v
	}
	return tftypes.NewValue(block.Type(), []tftypes.Value{tftypes.NewValue(elems[0].Type(), values)})
}

func TestSLADomainStateFromRemoteKeepsUnsetAttributesNull(t *testing.T) {
	ctx := t.Context()
	s := slaDomainSchema(ctx)

	domain := gqlsla.Domain{ID: uuid.New(), Name: "gold", ObjectTypes: []gqlsla.ObjectType{gqlsla.ObjectVSphereVM}}
	domain.SnapshotSchedule.Weekly = &gqlsla.WeeklySnapshotSchedule{
		BasicSchedule: gqlsla.BasicSnapshotSchedule{Frequency: 1, Retention: 1, RetentionUnit: gqlsla.Weeks},
	}

	t.Run("a description and day of week which weren't configured stay null", func(t *testing.T) {
		plan := slaDomainPlan(t, map[string]tftypes.Value{keyWeeklySchedule: oneBlock(t, keyWeeklySchedule)})
		model, diags := slaDomainStateFromRemote(ctx, s, domain, plan)
		if diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if !model.Description.IsNull() {
			t.Errorf("description = %q, want null", model.Description.ValueString())
		}
		if len(model.WeeklySchedule) != 1 || !model.WeeklySchedule[0].DayOfWeek.IsNull() {
			t.Errorf("weekly schedule = %+v, want a null day of week", model.WeeklySchedule)
		}
	})

	t.Run("a description which is set to an empty string is kept", func(t *testing.T) {
		plan := slaDomainPlan(t, map[string]tftypes.Value{
			keyDescription:    tftypes.NewValue(tftypes.String, ""),
			keyWeeklySchedule: oneBlockWith(t, keyWeeklySchedule, map[string]tftypes.Value{keyDayOfWeek: tftypes.NewValue(tftypes.String, "")}),
		})
		model, diags := slaDomainStateFromRemote(ctx, s, domain, plan)
		if diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if model.Description.IsNull() || model.Description.ValueString() != "" {
			t.Errorf("description = %v, want an empty string", model.Description)
		}
		if model.WeeklySchedule[0].DayOfWeek.IsNull() {
			t.Errorf("day of week is null, want an empty string")
		}
	})

	t.Run("a value RSC returns which isn't zero is kept", func(t *testing.T) {
		d := domain
		d.Description = "changed in the UI"
		d.SnapshotSchedule.Weekly = &gqlsla.WeeklySnapshotSchedule{
			BasicSchedule: domain.SnapshotSchedule.Weekly.BasicSchedule,
			DayOfWeek:     gqlsla.Monday,
		}
		plan := slaDomainPlan(t, map[string]tftypes.Value{keyWeeklySchedule: oneBlock(t, keyWeeklySchedule)})
		model, diags := slaDomainStateFromRemote(ctx, s, d, plan)
		if diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if model.Description.ValueString() != "changed in the UI" {
			t.Errorf("description = %v", model.Description)
		}
		if model.WeeklySchedule[0].DayOfWeek.ValueString() != string(gqlsla.Monday) {
			t.Errorf("day of week = %v", model.WeeklySchedule[0].DayOfWeek)
		}
	})

	t.Run("the computed values come from RSC", func(t *testing.T) {
		plan := slaDomainPlan(t, map[string]tftypes.Value{keyWeeklySchedule: oneBlock(t, keyWeeklySchedule)})
		model, diags := slaDomainStateFromRemote(ctx, s, domain, plan)
		if diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if model.ID.ValueString() != domain.ID.String() || model.Name.ValueString() != "gold" {
			t.Errorf("id, name = %v, %v", model.ID, model.Name)
		}
	})
}

// TestSLADomainStateFromRemoteNestedBlocks reads an SLA domain in the shape RSC
// returns it, with attributes of nested blocks which RSC returns as zero values.
func TestSLADomainStateFromRemoteNestedBlocks(t *testing.T) {
	ctx := t.Context()
	s := slaDomainSchema(ctx)

	archivalID := uuid.NewString()
	region, err := json.Marshal(gqlazure.RegionFromName("eastus").ToRegionForReplicationEnum())
	if err != nil {
		t.Fatal(err)
	}
	var domain gqlsla.Domain
	err = json.Unmarshal([]byte(`{
		"id": "`+uuid.NewString()+`",
		"name": "gold",
		"objectTypes": ["VSPHERE_OBJECT_TYPE"],
		"archivalSpecs": [{
			"frequencies": ["DAYS"], "threshold": 1, "thresholdUnit": "DAYS",
			"storageSetting": {"id": "`+archivalID+`", "name": "location"},
			"archivalTieringSpec": {"isInstantTieringEnabled": false, "minAccessibleDurationInSeconds": 0,
				"coldStorageClass": "", "shouldTierExistingSnapshots": false}
		}],
		"replicationSpecsV2": [{
			"azureRegion": `+string(region)+`,
			"retentionDuration": {"duration": 1, "unit": "DAYS"},
			"replicationPairs": []
		}]
	}`), &domain)
	if err != nil {
		t.Fatal(err)
	}

	str := func(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	archival := oneBlockWith(t, keyArchival, map[string]tftypes.Value{
		keyArchivalLocationID: str(archivalID),
		keyArchivalTiering:    nestedBlock(t, keyArchival, keyArchivalTiering, nil),
	})
	replication := oneBlockWith(t, keyReplicationSpec, map[string]tftypes.Value{
		keyAzureRegion: str("eastus"),
	})

	model, diags := slaDomainStateFromRemote(ctx, s, domain, slaDomainPlan(t, map[string]tftypes.Value{
		keyArchival:        archival,
		keyReplicationSpec: replication,
	}))
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if len(model.ReplicationSpec) != 1 {
		t.Fatalf("replication specs = %+v", model.ReplicationSpec)
	}
	spec := model.ReplicationSpec[0]
	if !spec.AWSRegion.IsNull() || !spec.AWSCrossAccount.IsNull() {
		t.Errorf("aws_region, aws_cross_account = %v, %v; want null", spec.AWSRegion, spec.AWSCrossAccount)
	}
	if spec.AzureRegion.ValueString() != "eastus" {
		t.Errorf("azure_region = %v", spec.AzureRegion)
	}

	if len(model.Archival) != 1 || len(model.Archival[0].ArchivalTiering) != 1 {
		t.Fatalf("archival = %+v", model.Archival)
	}
	tier := model.Archival[0].ArchivalTiering[0]
	if !tier.InstantTiering.IsNull() || !tier.MinAccessibleDurationInSeconds.IsNull() ||
		!tier.ColdStorageClass.IsNull() || !tier.TierExistingSnapshots.IsNull() {
		t.Errorf("archival tiering = %+v; want every attribute null", tier)
	}
}

// nestedBlock returns the block of a block, with a single element where the given
// attributes have the given values.
func nestedBlock(t *testing.T, parent, name string, attrs map[string]tftypes.Value) tftypes.Value {
	t.Helper()

	outer := oneBlock(t, parent)
	var elems []tftypes.Value
	if err := outer.As(&elems); err != nil {
		t.Fatal(err)
	}
	var outerValues map[string]tftypes.Value
	if err := elems[0].As(&outerValues); err != nil {
		t.Fatal(err)
	}
	innerType := outerValues[name].Type().(tftypes.List)
	innerElem := innerType.ElementType.(tftypes.Object)
	values := make(map[string]tftypes.Value, len(innerElem.AttributeTypes))
	for attrName, attrType := range innerElem.AttributeTypes {
		values[attrName] = zeroValue(attrType)
	}
	for k, v := range attrs {
		values[k] = v
	}
	return tftypes.NewValue(innerType, []tftypes.Value{tftypes.NewValue(innerElem, values)})
}
