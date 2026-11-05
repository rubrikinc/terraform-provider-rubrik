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
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// snapshotWindowModel is an element of the snapshot_window and
// first_full_snapshot blocks.
type snapshotWindowModel struct {
	Duration types.Int64  `tfsdk:"duration"`
	StartAt  types.String `tfsdk:"start_at"`
}

// localRetentionModel is the local_retention block of a replication spec.
type localRetentionModel struct {
	Retention     types.Int64  `tfsdk:"retention"`
	RetentionUnit types.String `tfsdk:"retention_unit"`
}

// fromSnapshotWindowModels returns the SDK backup windows of the snapshot
// windows. The start time has the format "Mon, 15:30", or "16:45" when no day
// of the week is given.
func fromSnapshotWindowModels(windows []snapshotWindowModel) ([]gqlsla.BackupWindow, error) {
	var snapshotWindows []gqlsla.BackupWindow
	for _, window := range windows {
		startAt := window.StartAt.ValueString()

		// Parse start time, e.g. "Mon, 15:30" or "16:45".
		var day string
		var timeParts []string
		parts := strings.Split(startAt, ", ")
		switch len(parts) {
		case 1:
			// No day of week specified.
			timeParts = strings.Split(parts[0], ":")
		case 2:
			// Day of week specified.
			switch strings.ToUpper(parts[0]) {
			case "MON":
				day = "MONDAY"
			case "TUE":
				day = "TUESDAY"
			case "WED":
				day = "WEDNESDAY"
			case "THU":
				day = "THURSDAY"
			case "FRI":
				day = "FRIDAY"
			case "SAT":
				day = "SATURDAY"
			case "SUN":
				day = "SUNDAY"
			default:
				return nil, fmt.Errorf("invalid day of week for %s: %s", keyStartAt, startAt)
			}
			timeParts = strings.Split(parts[1], ":")
		default:
			return nil, fmt.Errorf("invalid format for %s: %s", keyStartAt, startAt)
		}

		if len(timeParts) != 2 {
			return nil, fmt.Errorf("invalid time format for %s: %s", keyStartAt, startAt)
		}
		h, err := strconv.Atoi(timeParts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse hour for %s: %s", keyStartAt, err)
		}
		m, err := strconv.Atoi(timeParts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse minute for %s: %s", keyStartAt, err)
		}

		snapshotWindows = append(snapshotWindows, gqlsla.BackupWindow{
			DurationInHours: int(window.Duration.ValueInt64()),
			StartTime:       gqlsla.StartTime{DayOfWeek: gqlsla.DayOfWeek{Day: gqlsla.Day(day)}, Hour: h, Minute: m},
		})
	}

	return snapshotWindows, nil
}

// toSnapshotWindowModels returns the snapshot windows of the SDK backup
// windows. The start time is formatted as "Mon, 15:30", or "16:45" when the
// backup window has no day of the week.
func toSnapshotWindowModels(backupWindows []gqlsla.BackupWindow) ([]snapshotWindowModel, error) {
	var windows []snapshotWindowModel
	for _, backupWindow := range backupWindows {
		startAt := fmt.Sprintf("%02d:%02d", backupWindow.StartTime.Hour, backupWindow.StartTime.Minute)
		if day := backupWindow.StartTime.DayOfWeek.Day; day != "" {
			wd, err := day.ToWeekday()
			if err != nil {
				return nil, err
			}
			startAt = wd.String()[:3] + ", " + startAt
		}
		windows = append(windows, snapshotWindowModel{
			Duration: types.Int64Value(int64(backupWindow.DurationInHours)),
			StartAt:  types.StringValue(startAt),
		})
	}

	return windows, nil
}

// fromLocalRetentionModel returns the SDK retention duration of the local
// retention, or nil if the local retention is not configured.
func fromLocalRetentionModel(m *localRetentionModel) *gqlsla.RetentionDuration {
	if m == nil {
		return nil
	}
	return &gqlsla.RetentionDuration{
		Duration: int(m.Retention.ValueInt64()),
		Unit:     gqlsla.RetentionUnit(m.RetentionUnit.ValueString()),
	}
}

// toLocalRetentionModel returns the local retention of the SDK retention
// duration, or nil if the retention duration is nil.
func toLocalRetentionModel(localRetention *gqlsla.RetentionDuration) *localRetentionModel {
	if localRetention == nil {
		return nil
	}
	return &localRetentionModel{
		Retention:     types.Int64Value(int64(localRetention.Duration)),
		RetentionUnit: types.StringValue(string(localRetention.Unit)),
	}
}
