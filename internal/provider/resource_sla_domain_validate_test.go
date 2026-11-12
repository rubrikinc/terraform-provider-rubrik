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
	"maps"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// TestConfigHasLTRConfig verifies detection of a V1 (Azure-managed) config —
// an azure_sql_*_config block carrying a non-empty ltr_config.
func TestConfigHasLTRConfig(t *testing.T) {
	v1 := []any{map[string]any{
		keyLogRetention: 7,
		keyLTRConfig:    []any{map[string]any{keyWeeklyRetention: []any{}}},
	}}
	v2 := []any{map[string]any{
		keyLogRetention: 7,
		keyLTRConfig:    []any{},
	}}
	tests := []struct {
		name string
		in   any
		want bool
	}{
		{"V1 with ltr_config", v1, true},
		{"V2 without ltr_config", v2, false},
		{"empty block", []any{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configHasLTRConfig(tt.in); got != tt.want {
				t.Fatalf("configHasLTRConfig(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// unknownValue is how SDKv2 represents a value that is not known until apply in
// a raw configuration. The SDK's own constant is in an internal package.
const unknownValue = "74D93920-ED26-11E3-AC10-0800200C9A66"

// TestSLADomainCustomizeDiffGCPBigQuery runs the GCP BigQuery checks through
// SimpleDiff, the entry point PlanResourceChange uses, to cover how the planned
// schedules and blocks are read, including values that are not known during
// plan. Unknown values must never fail the plan, since CustomizeDiff runs again
// with known values when the plan is finalized during apply.
func TestSLADomainCustomizeDiffGCPBigQuery(t *testing.T) {
	backupLocation := []any{map[string]any{keyArchivalGroupID: "f6b1f4e8-5d1e-4f4e-9b6f-3a1c2d5e7f90"}}
	schedule := func(frequency any) []any {
		return []any{map[string]any{keyFrequency: frequency, keyRetention: 7}}
	}
	monthly := []any{map[string]any{keyFrequency: 1, keyRetention: 12, keyDayOfMonth: "FIRST_DAY"}}

	tests := []struct {
		name    string
		config  map[string]any
		wantErr string
	}{{
		name:   "Valid",
		config: map[string]any{keyDailySchedule: schedule(1), keyBackupLocation: backupLocation},
	}, {
		name:    "MissingBackupLocation",
		config:  map[string]any{keyDailySchedule: schedule(1)},
		wantErr: "requires a backup_location",
	}, {
		name:    "MinuteSchedule",
		config:  map[string]any{keyMinuteSchedule: schedule(15), keyBackupLocation: backupLocation},
		wantErr: "does not support a minute schedule",
	}, {
		name:    "WeeklyOverLimit",
		config:  map[string]any{keyWeeklySchedule: schedule(2), keyBackupLocation: backupLocation},
		wantErr: "at least every 7 days",
	}, {
		name:    "MonthlyOnly",
		config:  map[string]any{keyMonthlySchedule: monthly, keyBackupLocation: backupLocation},
		wantErr: "at least every 7 days",
	}, {
		name:   "UnknownFrequency",
		config: map[string]any{keyHourlySchedule: schedule(unknownValue), keyMonthlySchedule: monthly, keyBackupLocation: backupLocation},
	}, {
		name:   "UnknownBackupLocations",
		config: map[string]any{keyDailySchedule: schedule(1), keyBackupLocation: unknownValue},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]any{
				keyName:        "bigquery",
				keyObjectTypes: []any{string(gqlsla.ObjectGCPBigQuery)},
			}
			maps.Copy(config, tt.config)

			_, err := resourceSLADomain().SimpleDiff(context.Background(), &terraform.InstanceState{},
				terraform.NewResourceConfigRaw(config), nil)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// TestFromBackupLocation verifies the conversion of backup_location blocks into
// SLA-level backup location specs, preserving order.
func TestFromBackupLocation(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	block := func(ids ...uuid.UUID) []any {
		locations := make([]any, 0, len(ids))
		for _, id := range ids {
			locations = append(locations, map[string]any{keyArchivalGroupID: id.String()})
		}
		return locations
	}

	tests := []struct {
		name      string
		locations []any
		want      []uuid.UUID
	}{{
		name:      "None",
		locations: nil,
	}, {
		name:      "Empty",
		locations: []any{},
	}, {
		name:      "Single",
		locations: block(first),
		want:      []uuid.UUID{first},
	}, {
		name:      "MultiplePreservesOrder",
		locations: block(second, first),
		want:      []uuid.UUID{second, first},
	}, {
		// The archival_group_id schema validates the UUID, so this is not
		// reachable from a configuration. Documented to pin the behavior.
		name:      "InvalidUUIDDropsAll",
		locations: []any{map[string]any{keyArchivalGroupID: "not-a-uuid"}},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := fromBackupLocation(tt.locations)
			if len(specs) != len(tt.want) {
				t.Fatalf("got %d specs, want %d", len(specs), len(tt.want))
			}
			for i, want := range tt.want {
				if specs[i].ArchivalGroupID != want {
					t.Fatalf("spec %d = %v, want %v", i, specs[i].ArchivalGroupID, want)
				}
			}
		})
	}
}

// TestFromAWSS3Config verifies the conversion of a backup_location block into
// the legacy AWS S3 object specific config, which holds a single location.
func TestFromAWSS3Config(t *testing.T) {
	id := uuid.New()

	t.Run("NoneYieldsNoConfig", func(t *testing.T) {
		config, err := fromAWSS3Config(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config != nil {
			t.Fatalf("expected nil config, got %v", config)
		}
	})
	t.Run("Single", func(t *testing.T) {
		config, err := fromAWSS3Config([]any{map[string]any{keyArchivalGroupID: id.String()}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config == nil || config.ArchivalLocationID != id {
			t.Fatalf("got %v, want archival location %v", config, id)
		}
	})
	t.Run("MultipleRejected", func(t *testing.T) {
		_, err := fromAWSS3Config([]any{
			map[string]any{keyArchivalGroupID: id.String()},
			map[string]any{keyArchivalGroupID: uuid.New().String()},
		})
		if err == nil || !strings.Contains(err.Error(), "multiple backup locations not supported") {
			t.Fatalf("expected multiple backup locations error, got %v", err)
		}
	})
	t.Run("InvalidUUIDRejected", func(t *testing.T) {
		if _, err := fromAWSS3Config([]any{map[string]any{keyArchivalGroupID: "not-a-uuid"}}); err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}
