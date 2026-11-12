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

	"github.com/google/uuid"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

func TestToArchival(t *testing.T) {
	locationID1 := uuid.New()
	locationID2 := uuid.New()
	locationID3 := uuid.New()

	tests := []struct {
		name          string
		domain        gqlsla.Domain
		existing      []any
		expected      []any
		expectError   bool
		errorContains string
	}{
		{
			name:     "empty specs and existing",
			domain:   gqlsla.Domain{},
			existing: []any{},
			expected: []any{},
		}, {
			name: "new specs only",
			domain: gqlsla.Domain{
				ArchivalSpecs: []domainArchivalSpec{
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID1.String()}, Threshold: 30, ThresholdUnit: gqlsla.Days},
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID2.String()}, Threshold: 90, ThresholdUnit: gqlsla.Weeks},
				},
			},
			existing: []any{},
			expected: []any{
				map[string]any{keyArchivalLocationID: locationID1.String(), keyThreshold: 30, keyThresholdUnit: "DAYS"},
				map[string]any{keyArchivalLocationID: locationID2.String(), keyThreshold: 90, keyThresholdUnit: "WEEKS"},
			},
		}, {
			name: "preserve existing order",
			domain: gqlsla.Domain{
				ArchivalSpecs: []domainArchivalSpec{
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID1.String()}, Threshold: 30, ThresholdUnit: gqlsla.Days},
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID2.String()}, Threshold: 90, ThresholdUnit: gqlsla.Weeks},
				},
			},
			existing: []any{
				map[string]any{keyArchivalLocationID: locationID2.String()},
				map[string]any{keyArchivalLocationID: locationID1.String()},
			},
			expected: []any{
				map[string]any{keyArchivalLocationID: locationID2.String(), keyThreshold: 90, keyThresholdUnit: "WEEKS"},
				map[string]any{keyArchivalLocationID: locationID1.String(), keyThreshold: 30, keyThresholdUnit: "DAYS"},
			},
		}, {
			name: "add new specs to end",
			domain: gqlsla.Domain{
				ArchivalSpecs: []domainArchivalSpec{
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID1.String()}, Threshold: 30, ThresholdUnit: gqlsla.Days},
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID2.String()}, Threshold: 90, ThresholdUnit: gqlsla.Weeks},
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID3.String()}, Threshold: 12, ThresholdUnit: gqlsla.Months},
				},
			},
			existing: []any{
				map[string]any{keyArchivalLocationID: locationID2.String()},
			},
			expected: []any{
				map[string]any{keyArchivalLocationID: locationID2.String(), keyThreshold: 90, keyThresholdUnit: "WEEKS"},
				map[string]any{keyArchivalLocationID: locationID1.String(), keyThreshold: 30, keyThresholdUnit: "DAYS"},
				map[string]any{keyArchivalLocationID: locationID3.String(), keyThreshold: 12, keyThresholdUnit: "MONTHS"},
			},
		}, {
			name: "remove existing specs",
			domain: gqlsla.Domain{
				ArchivalSpecs: []domainArchivalSpec{
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID1.String()}, Threshold: 30, ThresholdUnit: gqlsla.Days},
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID3.String()}, Threshold: 12, ThresholdUnit: gqlsla.Months},
				},
			},
			existing: []any{
				map[string]any{keyArchivalLocationID: locationID3.String()},
				map[string]any{keyArchivalLocationID: locationID2.String()},
				map[string]any{keyArchivalLocationID: locationID1.String()},
			},
			expected: []any{
				map[string]any{keyArchivalLocationID: locationID3.String(), keyThreshold: 12, keyThresholdUnit: "MONTHS"},
				map[string]any{keyArchivalLocationID: locationID1.String(), keyThreshold: 30, keyThresholdUnit: "DAYS"},
			},
		}, {
			name: "duplicate location IDs",
			domain: gqlsla.Domain{
				ArchivalSpecs: []domainArchivalSpec{
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID1.String()}, Threshold: 30, ThresholdUnit: gqlsla.Days},
					{StorageSetting: struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{ID: locationID1.String()}, Threshold: 90, ThresholdUnit: gqlsla.Weeks},
				},
			},
			existing:      []any{},
			expectError:   true,
			errorContains: "used multiple times",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toArchival(tt.domain, tt.existing)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Fatalf("expected error to contain %q, got: %s", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				actual := result[i].(map[string]any)
				expectedMap := expected.(map[string]any)

				for key, expectedValue := range expectedMap {
					if actual[key] != expectedValue {
						t.Fatalf("item %d: expected %s=%v, got %v", i, key, expectedValue, actual[key])
					}
				}
			}
		})
	}
}

func TestToReplicationSpec(t *testing.T) {
	tests := []struct {
		name             string
		replicationSpecs []gqlsla.ReplicationSpec
		expected         []any
	}{
		{
			name:             "empty specs",
			replicationSpecs: []gqlsla.ReplicationSpec{},
			expected:         nil,
		}, {
			name: "AWS replication",
			replicationSpecs: []gqlsla.ReplicationSpec{
				{
					AWSRegion:  gqlaws.RegionFromName("us-east-1").ToRegionForReplicationEnum(),
					AWSAccount: "123456789012",
					RetentionDuration: &gqlsla.RetentionDuration{
						Duration: 30,
						Unit:     gqlsla.Days,
					},
				},
			},
			expected: []any{
				map[string]any{
					keyAWSRegion:       "us-east-1",
					keyAWSCrossAccount: "123456789012",
					keyAzureRegion:     "",
					keyRetention:       30,
					keyRetentionUnit:   gqlsla.Days,
				},
			},
		}, {
			name: "Azure replication",
			replicationSpecs: []gqlsla.ReplicationSpec{
				{
					AzureRegion: gqlazure.RegionFromName("eastus").ToRegionForReplicationEnum(),
					RetentionDuration: &gqlsla.RetentionDuration{
						Duration: 90,
						Unit:     gqlsla.Weeks,
					},
				},
			},
			expected: []any{
				map[string]any{
					keyAWSRegion:       "",
					keyAWSCrossAccount: "",
					keyAzureRegion:     "eastus",
					keyRetention:       90,
					keyRetentionUnit:   gqlsla.Weeks,
				},
			},
		}, {
			name: "multiple replications",
			replicationSpecs: []gqlsla.ReplicationSpec{
				{
					AWSRegion: gqlaws.RegionFromName("us-west-2").ToRegionForReplicationEnum(),
					RetentionDuration: &gqlsla.RetentionDuration{
						Duration: 7,
						Unit:     gqlsla.Days,
					},
				},
				{
					AzureRegion: gqlazure.RegionFromName("westeurope").ToRegionForReplicationEnum(),
					RetentionDuration: &gqlsla.RetentionDuration{
						Duration: 14,
						Unit:     gqlsla.Days,
					},
				},
			},
			expected: []any{
				map[string]any{
					keyAWSRegion:       "us-west-2",
					keyAWSCrossAccount: "",
					keyAzureRegion:     "",
					keyRetention:       7,
					keyRetentionUnit:   gqlsla.Days,
				},
				map[string]any{
					keyAWSRegion:       "",
					keyAWSCrossAccount: "",
					keyAzureRegion:     "westeurope",
					keyRetention:       14,
					keyRetentionUnit:   gqlsla.Days,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toReplicationSpec(tt.replicationSpecs)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				actual := result[i].(map[string]any)
				expectedMap := expected.(map[string]any)

				for key, expectedValue := range expectedMap {
					if actual[key] != expectedValue {
						t.Fatalf("item %d: expected %s=%v, got %v", i, key, expectedValue, actual[key])
					}
				}
			}
		})
	}
}

func TestToSnapshotWindow(t *testing.T) {
	tests := []struct {
		name          string
		backupWindows []gqlsla.BackupWindow
		expected      []any
	}{
		{
			name:          "empty windows",
			backupWindows: []gqlsla.BackupWindow{},
			expected:      nil,
		}, {
			name: "daily window",
			backupWindows: []gqlsla.BackupWindow{
				{
					StartTime:       gqlsla.StartTime{Hour: 14, Minute: 30},
					DurationInHours: 2,
				},
			},
			expected: []any{
				map[string]any{
					keyStartAt:  "14:30",
					keyDuration: 2,
				},
			},
		}, {
			name: "weekly window",
			backupWindows: []gqlsla.BackupWindow{
				{
					StartTime: gqlsla.StartTime{
						Hour:      9,
						Minute:    0,
						DayOfWeek: gqlsla.DayOfWeek{Day: "MONDAY"},
					},
					DurationInHours: 4,
				},
			},
			expected: []any{
				map[string]any{
					keyStartAt:  "Mon, 09:00",
					keyDuration: 4,
				},
			},
		}, {
			name: "multiple windows",
			backupWindows: []gqlsla.BackupWindow{
				{
					StartTime:       gqlsla.StartTime{Hour: 1, Minute: 0},
					DurationInHours: 3,
				},
				{
					StartTime:       gqlsla.StartTime{Hour: 22, Minute: 45},
					DurationInHours: 1,
				},
			},
			expected: []any{
				map[string]any{
					keyStartAt:  "01:00",
					keyDuration: 3,
				},
				map[string]any{
					keyStartAt:  "22:45",
					keyDuration: 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toSnapshotWindow(tt.backupWindows)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				actual := result[i].(map[string]any)
				expectedMap := expected.(map[string]any)

				for key, expectedValue := range expectedMap {
					if actual[key] != expectedValue {
						t.Fatalf("item %d: expected %s=%v, got %v", i, key, expectedValue, actual[key])
					}
				}
			}
		})
	}
}

func TestToLocalRetention(t *testing.T) {
	tests := []struct {
		name           string
		localRetention *gqlsla.RetentionDuration
		expected       []any
	}{
		{
			name:           "nil retention",
			localRetention: nil,
			expected:       nil,
		}, {
			name: "days retention",
			localRetention: &gqlsla.RetentionDuration{
				Duration: 7,
				Unit:     gqlsla.Days,
			},
			expected: []any{
				map[string]any{
					keyRetention:     7,
					keyRetentionUnit: "DAYS",
				},
			},
		}, {
			name: "weeks retention",
			localRetention: &gqlsla.RetentionDuration{
				Duration: 4,
				Unit:     gqlsla.Weeks,
			},
			expected: []any{
				map[string]any{
					keyRetention:     4,
					keyRetentionUnit: "WEEKS",
				},
			},
		}, {
			name: "months retention",
			localRetention: &gqlsla.RetentionDuration{
				Duration: 12,
				Unit:     gqlsla.Months,
			},
			expected: []any{
				map[string]any{
					keyRetention:     12,
					keyRetentionUnit: "MONTHS",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLocalRetention(tt.localRetention)

			if tt.expected == nil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}

			actual := result[0].(map[string]any)
			expectedMap := tt.expected[0].(map[string]any)

			for key, expectedValue := range expectedMap {
				if actual[key] != expectedValue {
					t.Fatalf("expected %s=%v, got %v", key, expectedValue, actual[key])
				}
			}
		})
	}
}

func TestToAWSDynamoDBConfig(t *testing.T) {
	tests := []struct {
		name           string
		dynamoDBConfig *gqlsla.AWSDynamoDBConfig
		expected       []any
	}{
		{
			name:           "nil config",
			dynamoDBConfig: nil,
			expected:       nil,
		}, {
			name: "with KMS alias",
			dynamoDBConfig: &gqlsla.AWSDynamoDBConfig{
				KMSAliasForPrimaryBackup: "alias/my-key",
			},
			expected: []any{
				map[string]any{
					keyKMSAlias: "alias/my-key",
				},
			},
		}, {
			name: "empty KMS alias",
			dynamoDBConfig: &gqlsla.AWSDynamoDBConfig{
				KMSAliasForPrimaryBackup: "",
			},
			expected: []any{
				map[string]any{
					keyKMSAlias: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toAWSDynamoDBConfig(tt.dynamoDBConfig)

			if tt.expected == nil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}

			actual := result[0].(map[string]any)
			expectedMap := tt.expected[0].(map[string]any)

			for key, expectedValue := range expectedMap {
				if actual[key] != expectedValue {
					t.Fatalf("expected %s=%v, got %v", key, expectedValue, actual[key])
				}
			}
		})
	}
}

func TestToAWSRDSConfig(t *testing.T) {
	tests := []struct {
		name      string
		rdsConfig *gqlsla.AWSRDSConfig
		expected  []any
	}{
		{
			name:      "nil config",
			rdsConfig: nil,
			expected:  nil,
		}, {
			name: "with log retention in days",
			rdsConfig: &gqlsla.AWSRDSConfig{
				LogRetention: gqlsla.RetentionDuration{
					Duration: 7,
					Unit:     gqlsla.Days,
				},
			},
			expected: []any{
				map[string]any{
					keyLogRetention:     7,
					keyLogRetentionUnit: gqlsla.Days,
				},
			},
		}, {
			name: "with log retention in weeks",
			rdsConfig: &gqlsla.AWSRDSConfig{
				LogRetention: gqlsla.RetentionDuration{
					Duration: 2,
					Unit:     gqlsla.Weeks,
				},
			},
			expected: []any{
				map[string]any{
					keyLogRetention:     2,
					keyLogRetentionUnit: gqlsla.Weeks,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toAWSRDSConfig(tt.rdsConfig)

			if tt.expected == nil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}

			actual := result[0].(map[string]any)
			expectedMap := tt.expected[0].(map[string]any)

			for key, expectedValue := range expectedMap {
				if actual[key] != expectedValue {
					t.Fatalf("expected %s=%v, got %v", key, expectedValue, actual[key])
				}
			}
		})
	}
}

func TestToAzureSQLConfig(t *testing.T) {
	tests := []struct {
		name      string
		sqlConfig *gqlsla.AzureDBConfig
		expected  []any
	}{
		{
			name:      "nil config",
			sqlConfig: nil,
			expected:  nil,
		}, {
			name: "with log retention",
			sqlConfig: &gqlsla.AzureDBConfig{
				LogRetentionInDays: 14,
			},
			expected: []any{
				map[string]any{
					keyLogRetention: 14,
				},
			},
		}, {
			name: "with zero log retention",
			sqlConfig: &gqlsla.AzureDBConfig{
				LogRetentionInDays: 0,
			},
			expected: []any{
				map[string]any{
					keyLogRetention: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toAzureSQLConfig(tt.sqlConfig)

			if tt.expected == nil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}

			actual := result[0].(map[string]any)
			expectedMap := tt.expected[0].(map[string]any)

			for key, expectedValue := range expectedMap {
				if actual[key] != expectedValue {
					t.Fatalf("expected %s=%v, got %v", key, expectedValue, actual[key])
				}
			}
		})
	}
}
