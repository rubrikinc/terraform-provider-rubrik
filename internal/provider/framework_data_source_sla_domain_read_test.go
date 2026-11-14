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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// slaDomainFromRSC returns an SLA domain in the shape RSC returns it: with
// archival frequencies, a replication spec with a local retention and a
// cascading archival, and the blocks which the SDKv2 data source never set.
func slaDomainFromRSC(t *testing.T) (gqlsla.Domain, string) {
	t.Helper()

	archivalID := uuid.NewString()
	region, err := json.Marshal(gqlazure.RegionFromName("eastus").ToRegionForReplicationEnum())
	if err != nil {
		t.Fatal(err)
	}
	var domain gqlsla.Domain
	err = json.Unmarshal([]byte(`{
		"id": "`+uuid.NewString()+`",
		"name": "gold",
		"description": "the gold SLA",
		"objectTypes": ["VSPHERE_OBJECT_TYPE", "ORACLE_OBJECT_TYPE"],
		"isRetentionLockedSla": true,
		"retentionLockMode": "COMPLIANCE",
		"archivalSpecs": [{
			"frequencies": ["WEEKS", "DAYS"], "threshold": 1, "thresholdUnit": "DAYS",
			"storageSetting": {"id": "`+archivalID+`", "name": "location"}
		}],
		"objectSpecificConfigs": {
			"oracleConfig": {
				"frequency": {"duration": 1, "unit": "HOURS"},
				"logRetention": {"duration": 7, "unit": "DAYS"},
				"hostLogRetention": {"duration": 3, "unit": "DAYS"}
			},
			"managedVolumeSlaConfig": {"logRetention": {"duration": 5, "unit": "DAYS"}},
			"vmwareVmConfig": {"logRetentionSeconds": 3600}
		},
		"replicationSpecsV2": [{
			"azureRegion": `+string(region)+`,
			"retentionDuration": {"duration": 1, "unit": "DAYS"},
			"replicationLocalRetentionDuration": {"duration": 2, "unit": "DAYS"},
			"replicationPairs": [],
			"cascadingArchivalSpecs": [{
				"archivalThreshold": {"duration": 3, "unit": "DAYS"},
				"frequency": ["DAYS"],
				"archivalLocation": {"id": "`+uuid.NewString()+`", "name": "cascade"}
			}]
		}]
	}`), &domain)
	if err != nil {
		t.Fatal(err)
	}
	domain.SnapshotSchedule.Daily = &gqlsla.DailySnapshotSchedule{
		BasicSchedule: gqlsla.BasicSnapshotSchedule{Frequency: 1, Retention: 7, RetentionUnit: gqlsla.Days},
	}

	return domain, archivalID
}

func TestSLADomainDataSourceFromRemote(t *testing.T) {
	domain, archivalID := slaDomainFromRSC(t)

	model, diags := slaDomainDataSourceFromRemote(t.Context(), domain)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	t.Run("what the SDKv2 data source returned", func(t *testing.T) {
		if model.ID.ValueString() != domain.ID.String() || model.Name.ValueString() != "gold" ||
			model.Description.ValueString() != "the gold SLA" {
			t.Errorf("id, name, description = %v, %v, %v", model.ID, model.Name, model.Description)
		}
		if got := len(model.ObjectTypes.Elements()); got != 2 {
			t.Errorf("object types = %d, want 2", got)
		}
		if len(model.DailySchedule) != 1 || model.DailySchedule[0].Retention.ValueInt64() != 7 {
			t.Errorf("daily schedule = %+v", model.DailySchedule)
		}
		if len(model.VMwareVMConfig) != 1 {
			t.Errorf("vmware vm config = %+v, want one block", model.VMwareVMConfig)
		}
		if len(model.ReplicationSpec) != 1 || model.ReplicationSpec[0].AzureRegion.ValueString() != "eastus" ||
			model.ReplicationSpec[0].Retention.ValueInt64() != 1 {
			t.Errorf("replication spec = %+v", model.ReplicationSpec)
		}
	})

	t.Run("the archival frequencies are returned, since there is no configuration to compare with", func(t *testing.T) {
		if len(model.Archival) != 1 || model.Archival[0].ArchivalLocationID.ValueString() != archivalID {
			t.Fatalf("archival = %+v", model.Archival)
		}
		// The SDKv2 data source returned them from RSC, and they are sorted
		// since they are a set.
		if got := model.Archival[0].Frequency; len(got) != 2 || got[0] != "DAYS" || got[1] != "WEEKS" {
			t.Errorf("archival frequencies = %v, want [DAYS WEEKS]", got)
		}
	})

	t.Run("what the SDKv2 data source didn't set", func(t *testing.T) {
		if len(model.ReplicationSpec[0].LocalRetention) != 0 {
			t.Errorf("replication local retention = %+v, want none", model.ReplicationSpec[0].LocalRetention)
		}
		if len(model.ReplicationSpec[0].CascadingArchival) != 0 {
			t.Errorf("cascading archival = %+v, want none", model.ReplicationSpec[0].CascadingArchival)
		}
	})
}

// TestSLADomainDataSourceModelMatchesSchema sets the converted SLA domain as the
// state of the data source and reads it back. The framework reports a struct
// field without an attribute, and an attribute without a struct field, so a
// wrong or missing tfsdk tag fails the test, which a model which compiles
// doesn't.
func TestSLADomainDataSourceModelMatchesSchema(t *testing.T) {
	ctx := t.Context()
	domain, _ := slaDomainFromRSC(t)

	state, diags := slaDomainDataSourceState(ctx, domain)
	if diags.HasError() {
		t.Fatalf("the model doesn't match the schema: %v", diags)
	}
	var read slaDomainDataSourceModel
	if diags := state.Get(ctx, &read); diags.HasError() {
		t.Fatalf("the state doesn't decode: %v", diags)
	}
	if read.ID.ValueString() != domain.ID.String() || read.Name.ValueString() != "gold" {
		t.Errorf("id, name = %v, %v", read.ID, read.Name)
	}
	if len(read.Archival) != 1 || len(read.Archival[0].Frequency) != 2 {
		t.Errorf("archival = %+v", read.Archival)
	}

	// A block which isn't in the domain is an empty list, as in the SDKv2 data
	// source, and not null, which fails length() in a configuration.
	var attrs map[string]tftypes.Value
	if err := state.Raw.As(&attrs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{keyHourlySchedule, keyAWSRDSConfig, keyNCDConfig, keyRetentionLock} {
		if attrs[name].IsNull() {
			t.Errorf("%s is null, want an empty list", name)
		}
		if elems := listElements(t, attrs[name]); len(elems) != 0 && name != keyRetentionLock {
			t.Errorf("%s has %d elements, want none", name, len(elems))
		}
	}
	if elems := listElements(t, attrs[keyDailySchedule]); len(elems) != 1 {
		t.Errorf("%s has %d elements, want 1", keyDailySchedule, len(elems))
	}
}

func listElements(t *testing.T, v tftypes.Value) []tftypes.Value {
	t.Helper()

	var elems []tftypes.Value
	if err := v.As(&elems); err != nil {
		t.Fatal(err)
	}
	return elems
}

func TestEmptyListsForNull(t *testing.T) {
	str := tftypes.String
	list := tftypes.List{ElementType: str}
	set := tftypes.Set{ElementType: str}
	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"s": str, "l": list, "t": set,
		"nested": tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"inner": list}}}}}
	inner := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"inner": list}}

	value := tftypes.NewValue(obj, map[string]tftypes.Value{
		"s": tftypes.NewValue(str, nil),
		"l": tftypes.NewValue(list, nil),
		"t": tftypes.NewValue(set, nil),
		"nested": tftypes.NewValue(tftypes.List{ElementType: inner}, []tftypes.Value{
			tftypes.NewValue(inner, map[string]tftypes.Value{"inner": tftypes.NewValue(list, nil)}),
		}),
	})
	want := tftypes.NewValue(obj, map[string]tftypes.Value{
		"s": tftypes.NewValue(str, nil),
		"l": tftypes.NewValue(list, []tftypes.Value{}),
		"t": tftypes.NewValue(set, []tftypes.Value{}),
		"nested": tftypes.NewValue(tftypes.List{ElementType: inner}, []tftypes.Value{
			tftypes.NewValue(inner, map[string]tftypes.Value{"inner": tftypes.NewValue(list, []tftypes.Value{})}),
		}),
	})
	if got := emptyListsForNull(value); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}

	t.Run("a null string stays null, and a null root stays null", func(t *testing.T) {
		if got := emptyListsForNull(tftypes.NewValue(obj, nil)); !got.IsNull() {
			t.Fatalf("a null root became %s", got)
		}
	})
}
