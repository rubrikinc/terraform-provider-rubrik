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
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// slaDomainPlanValue returns a plan of the SLA domain with the given object
// types, where the given attributes have the given values. An attribute which
// isn't given is null, as the framework delivers a block which isn't configured.
func slaDomainPlanValue(t *testing.T, objectTypes tftypes.Value, set map[string]tftypes.Value) tftypes.Value {
	t.Helper()

	values := map[string]tftypes.Value{keyObjectTypes: objectTypes}
	for k, v := range set {
		values[k] = v
	}
	return slaDomainConfig(t, values).Raw
}

func objectTypesValue(t *testing.T, types ...gqlsla.ObjectType) tftypes.Value {
	t.Helper()

	typ := slaDomainSchema(t.Context()).Type().TerraformType(t.Context()).(tftypes.Object).AttributeTypes[keyObjectTypes]
	elements := make([]tftypes.Value, 0, len(types))
	for _, objectType := range types {
		elements = append(elements, tftypes.NewValue(tftypes.String, string(objectType)))
	}
	return tftypes.NewValue(typ, elements)
}

// scheduleBlock returns a schedule block with the given frequency, which is
// unknown for a nil frequency.
func scheduleBlock(t *testing.T, name string, frequency any) tftypes.Value {
	t.Helper()

	if frequency == nil {
		frequency = tftypes.UnknownValue
	}
	return oneBlockWith(t, name, map[string]tftypes.Value{keyFrequency: tftypes.NewValue(tftypes.Number, frequency)})
}

func TestValidateSLADomainPlan(t *testing.T) {
	typ := slaDomainSchema(t.Context()).Type().TerraformType(t.Context()).(tftypes.Object)
	pg := objectTypesValue(t, gqlsla.ObjectAzurePostgresFlexibleServer)
	awsConfig := objectTypesValue(t, gqlsla.ObjectAWSConfig)
	bigQuery := objectTypesValue(t, gqlsla.ObjectGCPBigQuery)
	two := func(name string) tftypes.Value {
		block := oneBlock(t, name)
		var elems []tftypes.Value
		if err := block.As(&elems); err != nil {
			t.Fatal(err)
		}
		return tftypes.NewValue(block.Type(), []tftypes.Value{elems[0], elems[0]})
	}

	tests := []struct {
		name        string
		objectTypes tftypes.Value
		set         map[string]tftypes.Value
		wantErr     string
	}{
		// Azure Postgres Flexible Server config.
		{name: "Postgres config with its object type", objectTypes: pg, set: map[string]tftypes.Value{keyAzurePostgresFlexibleServerConfig: oneBlock(t, keyAzurePostgresFlexibleServerConfig)}},
		{name: "Postgres config with another object type", objectTypes: objectTypesValue(t, gqlsla.ObjectAzureSQLDatabase), set: map[string]tftypes.Value{keyAzurePostgresFlexibleServerConfig: oneBlock(t, keyAzurePostgresFlexibleServerConfig)}, wantErr: "is only valid when object_types is"},
		{name: "no Postgres config with another object type", objectTypes: objectTypesValue(t, gqlsla.ObjectVSphereVM)},

		// AWS Config.
		{name: "AWS Config daily only", objectTypes: awsConfig, set: map[string]tftypes.Value{keyDailySchedule: scheduleBlock(t, keyDailySchedule, 1)}},
		{name: "AWS Config hourly 6", objectTypes: awsConfig, set: map[string]tftypes.Value{keyHourlySchedule: scheduleBlock(t, keyHourlySchedule, 6)}},
		{name: "AWS Config hourly 5", objectTypes: awsConfig, set: map[string]tftypes.Value{keyHourlySchedule: scheduleBlock(t, keyHourlySchedule, 5)}, wantErr: "at least 6 hours"},
		{name: "AWS Config minute schedule", objectTypes: awsConfig, set: map[string]tftypes.Value{keyMinuteSchedule: scheduleBlock(t, keyMinuteSchedule, 15)}, wantErr: "does not support a minute schedule"},
		{name: "AWS Config two backup locations", objectTypes: awsConfig, set: map[string]tftypes.Value{keyBackupLocation: two(keyBackupLocation)}, wantErr: "at most one backup_location"},
		{name: "AWS Config combined", objectTypes: objectTypesValue(t, gqlsla.ObjectAWSConfig, gqlsla.ObjectAWSEC2EBS), wantErr: "cannot be combined with other object types"},
		{name: "AWS Config unknown hourly frequency", objectTypes: awsConfig, set: map[string]tftypes.Value{keyHourlySchedule: scheduleBlock(t, keyHourlySchedule, nil)}},

		// GCP BigQuery, ported from TestSLADomainCustomizeDiffGCPBigQuery.
		{name: "BigQuery valid", objectTypes: bigQuery, set: map[string]tftypes.Value{keyDailySchedule: scheduleBlock(t, keyDailySchedule, 1), keyBackupLocation: oneBlock(t, keyBackupLocation)}},
		{name: "BigQuery missing backup location", objectTypes: bigQuery, set: map[string]tftypes.Value{keyDailySchedule: scheduleBlock(t, keyDailySchedule, 1)}, wantErr: "requires a backup_location"},
		{name: "BigQuery minute schedule", objectTypes: bigQuery, set: map[string]tftypes.Value{keyMinuteSchedule: scheduleBlock(t, keyMinuteSchedule, 15), keyBackupLocation: oneBlock(t, keyBackupLocation)}, wantErr: "does not support a minute schedule"},
		{name: "BigQuery weekly over the limit", objectTypes: bigQuery, set: map[string]tftypes.Value{keyWeeklySchedule: scheduleBlock(t, keyWeeklySchedule, 2), keyBackupLocation: oneBlock(t, keyBackupLocation)}, wantErr: "at least every 7 days"},
		{name: "BigQuery monthly only", objectTypes: bigQuery, set: map[string]tftypes.Value{keyMonthlySchedule: oneBlock(t, keyMonthlySchedule), keyBackupLocation: oneBlock(t, keyBackupLocation)}, wantErr: "at least every 7 days"},
		{name: "BigQuery unknown frequency", objectTypes: bigQuery, set: map[string]tftypes.Value{keyHourlySchedule: scheduleBlock(t, keyHourlySchedule, nil), keyMonthlySchedule: oneBlock(t, keyMonthlySchedule), keyBackupLocation: oneBlock(t, keyBackupLocation)}},
		{name: "BigQuery unknown backup locations", objectTypes: bigQuery, set: map[string]tftypes.Value{keyDailySchedule: scheduleBlock(t, keyDailySchedule, 1), keyBackupLocation: tftypes.NewValue(typ.AttributeTypes[keyBackupLocation], tftypes.UnknownValue)}},
		{name: "BigQuery with archival", objectTypes: bigQuery, set: map[string]tftypes.Value{keyDailySchedule: scheduleBlock(t, keyDailySchedule, 1), keyBackupLocation: oneBlock(t, keyBackupLocation), keyArchival: oneBlock(t, keyArchival)}, wantErr: "remove the archival block"},

		// Unknown object types decide nothing.
		{name: "unknown object types", objectTypes: tftypes.NewValue(typ.AttributeTypes[keyObjectTypes], tftypes.UnknownValue), set: map[string]tftypes.Value{keyAzurePostgresFlexibleServerConfig: oneBlock(t, keyAzurePostgresFlexibleServerConfig)}},
		{name: "an unknown object type", objectTypes: tftypes.NewValue(typ.AttributeTypes[keyObjectTypes], []tftypes.Value{tftypes.NewValue(tftypes.String, tftypes.UnknownValue)}), set: map[string]tftypes.Value{keyAzurePostgresFlexibleServerConfig: oneBlock(t, keyAzurePostgresFlexibleServerConfig)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := slaDomainPlanValue(t, tt.objectTypes, tt.set)
			err := validateSLADomainPlan(plan, tftypes.NewValue(typ, nil))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected an error containing %q, got none", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSLADomainPlanBackupServiceChange(t *testing.T) {
	typ := slaDomainSchema(t.Context()).Type().TerraformType(t.Context()).(tftypes.Object)
	types := objectTypesValue(t, gqlsla.ObjectAzureSQLDatabase)

	v1 := func(name string) tftypes.Value {
		return oneBlockWith(t, name, map[string]tftypes.Value{keyLTRConfig: oneBlock2(t, name, keyLTRConfig)})
	}
	v2 := func(name string) tftypes.Value { return oneBlock(t, name) }

	plan := func(set map[string]tftypes.Value) tftypes.Value { return slaDomainPlanValue(t, types, set) }
	create := tftypes.NewValue(typ, nil)

	for _, key := range []string{keyAzureSQLDatabaseConfig, keyAzureSQLManagedInstanceConfig} {
		t.Run(key, func(t *testing.T) {
			tests := []struct {
				name        string
				state, plan tftypes.Value
				wantErr     bool
			}{
				{"create V1", create, plan(map[string]tftypes.Value{key: v1(key)}), false},
				{"create V2", create, plan(map[string]tftypes.Value{key: v2(key)}), false},
				{"V1 stays V1", plan(map[string]tftypes.Value{key: v1(key)}), plan(map[string]tftypes.Value{key: v1(key)}), false},
				{"V2 stays V2", plan(map[string]tftypes.Value{key: v2(key)}), plan(map[string]tftypes.Value{key: v2(key)}), false},
				{"V1 to V2", plan(map[string]tftypes.Value{key: v1(key)}), plan(map[string]tftypes.Value{key: v2(key)}), true},
				{"V2 to V1", plan(map[string]tftypes.Value{key: v2(key)}), plan(map[string]tftypes.Value{key: v1(key)}), true},
				{"config removed", plan(map[string]tftypes.Value{key: v1(key)}), plan(nil), false},
				{"config added", plan(nil), plan(map[string]tftypes.Value{key: v1(key)}), false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := validateSLADomainPlan(tt.plan, tt.state)
					if tt.wantErr && !errors.Is(err, errSLADomainBackupServiceChange) {
						t.Fatalf("error = %v, want errSLADomainBackupServiceChange", err)
					}
					if !tt.wantErr && err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
				})
			}
		})
	}
}

// oneBlock2 returns the block inside a block, with a single empty element.
func oneBlock2(t *testing.T, parent, name string) tftypes.Value {
	t.Helper()

	return nestedBlock(t, parent, name, nil)
}
