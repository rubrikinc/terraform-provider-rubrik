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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

func TestToSnapshotWindowModels(t *testing.T) {
	tests := []struct {
		name          string
		backupWindows []gqlsla.BackupWindow
		expected      []snapshotWindowModel
	}{
		{
			name:          "empty windows",
			backupWindows: []gqlsla.BackupWindow{},
			expected:      nil,
		}, {
			name: "daily window",
			backupWindows: []gqlsla.BackupWindow{
				{StartTime: gqlsla.StartTime{Hour: 14, Minute: 30}, DurationInHours: 2},
			},
			expected: []snapshotWindowModel{
				{StartAt: types.StringValue("14:30"), Duration: types.Int64Value(2)},
			},
		}, {
			name: "weekly window",
			backupWindows: []gqlsla.BackupWindow{
				{
					StartTime: gqlsla.StartTime{
						Hour: 9, Minute: 0,
						DayOfWeek: gqlsla.DayOfWeek{Day: "MONDAY"},
					},
					DurationInHours: 4,
				},
			},
			expected: []snapshotWindowModel{
				{StartAt: types.StringValue("Mon, 09:00"), Duration: types.Int64Value(4)},
			},
		}, {
			name: "multiple windows",
			backupWindows: []gqlsla.BackupWindow{
				{StartTime: gqlsla.StartTime{Hour: 1, Minute: 0}, DurationInHours: 3},
				{StartTime: gqlsla.StartTime{Hour: 22, Minute: 45}, DurationInHours: 1},
			},
			expected: []snapshotWindowModel{
				{StartAt: types.StringValue("01:00"), Duration: types.Int64Value(3)},
				{StartAt: types.StringValue("22:45"), Duration: types.Int64Value(1)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toSnapshotWindowModels(tt.backupWindows)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if !result[i].StartAt.Equal(expected.StartAt) || !result[i].Duration.Equal(expected.Duration) {
					t.Errorf("item %d: expected %+v, got %+v", i, expected, result[i])
				}
			}
		})
	}
}

func TestToSnapshotWindowModelsInvalidDay(t *testing.T) {
	_, err := toSnapshotWindowModels([]gqlsla.BackupWindow{
		{StartTime: gqlsla.StartTime{DayOfWeek: gqlsla.DayOfWeek{Day: "FUNDAY"}}},
	})
	if err == nil {
		t.Fatal("expected an error for an invalid day")
	}
}

// TestFromSnapshotWindowModels covers the parsing of the start time, which the
// SDKv2 resource had no test for.
func TestFromSnapshotWindowModels(t *testing.T) {
	tests := []struct {
		name      string
		startAt   string
		wantDay   gqlsla.Day
		wantHour  int
		wantMin   int
		wantError string
	}{
		{name: "time only", startAt: "16:45", wantHour: 16, wantMin: 45},
		{name: "time only with zero", startAt: "00:00", wantHour: 0, wantMin: 0},
		{name: "monday", startAt: "Mon, 15:30", wantDay: "MONDAY", wantHour: 15, wantMin: 30},
		{name: "tuesday", startAt: "Tue, 01:05", wantDay: "TUESDAY", wantHour: 1, wantMin: 5},
		{name: "wednesday", startAt: "Wed, 02:00", wantDay: "WEDNESDAY", wantHour: 2},
		{name: "thursday", startAt: "Thu, 03:00", wantDay: "THURSDAY", wantHour: 3},
		{name: "friday", startAt: "Fri, 04:00", wantDay: "FRIDAY", wantHour: 4},
		{name: "saturday", startAt: "Sat, 05:00", wantDay: "SATURDAY", wantHour: 5},
		{name: "sunday", startAt: "Sun, 06:00", wantDay: "SUNDAY", wantHour: 6},
		{name: "day is case insensitive", startAt: "mON, 07:15", wantDay: "MONDAY", wantHour: 7, wantMin: 15},
		{name: "unknown day", startAt: "Fun, 15:30", wantError: "invalid day of week for start_at: Fun, 15:30"},
		{name: "full day name", startAt: "Monday, 15:30", wantError: "invalid day of week for start_at"},
		{name: "too many parts", startAt: "Mon, 15:30, extra", wantError: "invalid format for start_at: Mon, 15:30, extra"},
		{name: "no minute", startAt: "15", wantError: "invalid time format for start_at: 15"},
		{name: "too many time parts", startAt: "15:30:45", wantError: "invalid time format for start_at: 15:30:45"},
		{name: "hour not a number", startAt: "xx:30", wantError: "failed to parse hour for start_at"},
		{name: "minute not a number", startAt: "15:yy", wantError: "failed to parse minute for start_at"},
		{name: "empty", startAt: "", wantError: "invalid time format for start_at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fromSnapshotWindowModels([]snapshotWindowModel{
				{StartAt: types.StringValue(tt.startAt), Duration: types.Int64Value(3)},
			})
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected an error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 window, got %d", len(got))
			}
			window := got[0]
			if window.DurationInHours != 3 {
				t.Errorf("duration = %d, want 3", window.DurationInHours)
			}
			if window.StartTime.DayOfWeek.Day != tt.wantDay {
				t.Errorf("day = %q, want %q", window.StartTime.DayOfWeek.Day, tt.wantDay)
			}
			if window.StartTime.Hour != tt.wantHour || window.StartTime.Minute != tt.wantMin {
				t.Errorf("time = %d:%d, want %d:%d", window.StartTime.Hour, window.StartTime.Minute,
					tt.wantHour, tt.wantMin)
			}
		})
	}
}

// TestSnapshotWindowModelsRoundTrip verifies that formatting a window and
// parsing the result gives the same window back.
func TestSnapshotWindowModelsRoundTrip(t *testing.T) {
	windows := []gqlsla.BackupWindow{
		{StartTime: gqlsla.StartTime{Hour: 1, Minute: 5}, DurationInHours: 3},
		{
			StartTime: gqlsla.StartTime{
				Hour: 23, Minute: 59, DayOfWeek: gqlsla.DayOfWeek{Day: "SUNDAY"},
			},
			DurationInHours: 8,
		},
	}

	models, err := toSnapshotWindowModels(windows)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fromSnapshotWindowModels(models)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(windows) {
		t.Fatalf("expected %d windows, got %d", len(windows), len(got))
	}
	for i := range windows {
		if got[i] != windows[i] {
			t.Errorf("window %d: got %+v, want %+v", i, got[i], windows[i])
		}
	}
}

func TestToLocalRetentionModel(t *testing.T) {
	tests := []struct {
		name           string
		localRetention *gqlsla.RetentionDuration
		expected       *localRetentionModel
	}{
		{name: "nil retention", localRetention: nil, expected: nil},
		{
			name:           "days retention",
			localRetention: &gqlsla.RetentionDuration{Duration: 7, Unit: gqlsla.Days},
			expected:       &localRetentionModel{Retention: types.Int64Value(7), RetentionUnit: types.StringValue("DAYS")},
		}, {
			name:           "weeks retention",
			localRetention: &gqlsla.RetentionDuration{Duration: 4, Unit: gqlsla.Weeks},
			expected:       &localRetentionModel{Retention: types.Int64Value(4), RetentionUnit: types.StringValue("WEEKS")},
		}, {
			name:           "months retention",
			localRetention: &gqlsla.RetentionDuration{Duration: 12, Unit: gqlsla.Months},
			expected:       &localRetentionModel{Retention: types.Int64Value(12), RetentionUnit: types.StringValue("MONTHS")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLocalRetentionModel(tt.localRetention)
			if tt.expected == nil {
				if result != nil {
					t.Fatalf("expected nil, got %+v", result)
				}
				return
			}
			if result == nil || !result.Retention.Equal(tt.expected.Retention) ||
				!result.RetentionUnit.Equal(tt.expected.RetentionUnit) {
				t.Fatalf("expected %+v, got %+v", tt.expected, result)
			}

			// Converting back gives the same SDK value.
			back := fromLocalRetentionModel(result)
			if back == nil || *back != *tt.localRetention {
				t.Errorf("round trip: got %+v, want %+v", back, tt.localRetention)
			}
		})
	}

	if fromLocalRetentionModel(nil) != nil {
		t.Error("a nil local retention must convert to nil")
	}
}
