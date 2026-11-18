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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

func TestFromBackupLocationModels(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	block := func(ids ...uuid.UUID) []backupLocationModel {
		locations := make([]backupLocationModel, 0, len(ids))
		for _, id := range ids {
			locations = append(locations, backupLocationModel{ArchivalGroupID: types.StringValue(id.String())})
		}
		return locations
	}

	tests := []struct {
		name      string
		locations []backupLocationModel
		want      []uuid.UUID
	}{
		{name: "None", locations: nil},
		{name: "Empty", locations: []backupLocationModel{}},
		{name: "Single", locations: block(first), want: []uuid.UUID{first}},
		{name: "MultiplePreservesOrder", locations: block(second, first), want: []uuid.UUID{second, first}},
		{
			// The archival_group_id schema validates the UUID, so this is not
			// reachable from a configuration. Documented to pin the behavior.
			name:      "InvalidUUIDDropsAll",
			locations: []backupLocationModel{{ArchivalGroupID: types.StringValue("not-a-uuid")}},
		}, {
			// The drop is all or nothing: a valid location before an invalid one
			// is dropped too.
			name: "InvalidUUIDDropsValidOnesToo",
			locations: append(block(first),
				backupLocationModel{ArchivalGroupID: types.StringValue("not-a-uuid")}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := fromBackupLocationModels(tt.locations)
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

// domainBackupLocationSpec is the type of an element of the backup location
// specs of an SDK domain, which is an anonymous struct.
type domainBackupLocationSpec = struct {
	ArchivalGroup struct {
		ID string `json:"id"`
	} `json:"archivalGroup"`
}

func newDomainBackupLocationSpec(id string) domainBackupLocationSpec {
	var spec domainBackupLocationSpec
	spec.ArchivalGroup.ID = id
	return spec
}

func TestToBackupLocationModels(t *testing.T) {
	a, b, c := uuid.NewString(), uuid.NewString(), uuid.NewString()
	existing := func(ids ...string) []backupLocationModel {
		models := make([]backupLocationModel, 0, len(ids))
		for _, id := range ids {
			models = append(models, backupLocationModel{ArchivalGroupID: types.StringValue(id)})
		}
		return models
	}
	ids := func(models []backupLocationModel) []string {
		out := make([]string, 0, len(models))
		for _, m := range models {
			out = append(out, m.ArchivalGroupID.ValueString())
		}
		return out
	}
	s3 := uuid.New()

	tests := []struct {
		name      string
		domain    gqlsla.Domain
		existing  []backupLocationModel
		want      []string
		wantError string
	}{
		{name: "none", domain: gqlsla.Domain{}},
		{
			name:   "new specs only",
			domain: gqlsla.Domain{BackupLocationSpecs: []domainBackupLocationSpec{newDomainBackupLocationSpec(a), newDomainBackupLocationSpec(b)}},
			want:   []string{a, b},
		}, {
			name:     "preserve existing order",
			domain:   gqlsla.Domain{BackupLocationSpecs: []domainBackupLocationSpec{newDomainBackupLocationSpec(a), newDomainBackupLocationSpec(b)}},
			existing: existing(b, a),
			want:     []string{b, a},
		}, {
			name: "new specs last",
			domain: gqlsla.Domain{BackupLocationSpecs: []domainBackupLocationSpec{
				newDomainBackupLocationSpec(a), newDomainBackupLocationSpec(b), newDomainBackupLocationSpec(c),
			}},
			existing: existing(b),
			want:     []string{b, a, c},
		}, {
			name:     "removed specs are dropped",
			domain:   gqlsla.Domain{BackupLocationSpecs: []domainBackupLocationSpec{newDomainBackupLocationSpec(a)}},
			existing: existing(c, b, a),
			want:     []string{a},
		}, {
			name:      "duplicate",
			domain:    gqlsla.Domain{BackupLocationSpecs: []domainBackupLocationSpec{newDomainBackupLocationSpec(a), newDomainBackupLocationSpec(a)}},
			wantError: "used multiple times",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toBackupLocationModels(tt.domain, tt.existing)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected an error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if gotIDs := ids(got); strings.Join(gotIDs, ",") != strings.Join(tt.want, ",") {
				t.Errorf("got %v, want %v", gotIDs, tt.want)
			}
		})
	}

	// The SDKv2 function had no test for the AWS S3 fallback.
	t.Run("AWS S3 fallback", func(t *testing.T) {
		domain := gqlsla.Domain{}
		domain.ObjectSpecificConfigs.AWSS3Config = &struct {
			gqlsla.AWSS3Config
			ArchivalLocationName string `json:"archivalLocationName"`
		}{AWSS3Config: gqlsla.AWSS3Config{ArchivalLocationID: s3}}

		got, err := toBackupLocationModels(domain, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ArchivalGroupID.ValueString() != s3.String() {
			t.Errorf("got %+v, want the S3 archival location %s", got, s3)
		}

		// The fallback is only used when there are no specs.
		domain.BackupLocationSpecs = []domainBackupLocationSpec{newDomainBackupLocationSpec(a)}
		got, err = toBackupLocationModels(domain, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ArchivalGroupID.ValueString() != a {
			t.Errorf("got %+v, want only the spec %s", got, a)
		}
	})
}

func TestAWSDynamoDBConfigModel(t *testing.T) {
	t.Run("to", func(t *testing.T) {
		tests := []struct {
			name   string
			config *gqlsla.AWSDynamoDBConfig
			want   []awsDynamoDBConfigModel
		}{
			{name: "nil config", config: nil, want: nil},
			{
				name:   "with KMS alias",
				config: &gqlsla.AWSDynamoDBConfig{KMSAliasForPrimaryBackup: "alias/my-key"},
				want:   []awsDynamoDBConfigModel{{KMSAlias: types.StringValue("alias/my-key")}},
			}, {
				name:   "empty KMS alias",
				config: &gqlsla.AWSDynamoDBConfig{KMSAliasForPrimaryBackup: ""},
				want:   []awsDynamoDBConfigModel{{KMSAlias: types.StringValue("")}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := toAWSDynamoDBConfigModel(tt.config)
				if len(got) != len(tt.want) {
					t.Fatalf("got %d, want %d", len(got), len(tt.want))
				}
				for i := range got {
					if !got[i].KMSAlias.Equal(tt.want[i].KMSAlias) {
						t.Errorf("got %+v, want %+v", got[i], tt.want[i])
					}
				}
			})
		}
	})

	t.Run("from", func(t *testing.T) {
		if got := fromAWSDynamoDBConfigModel(nil); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
		got := fromAWSDynamoDBConfigModel([]awsDynamoDBConfigModel{{KMSAlias: types.StringValue("alias/k")}})
		if got == nil || got.KMSAliasForPrimaryBackup != "alias/k" {
			t.Errorf("got %+v", got)
		}
		// An alias which isn't set gives a config with an empty alias.
		got = fromAWSDynamoDBConfigModel([]awsDynamoDBConfigModel{{KMSAlias: types.StringNull()}})
		if got == nil || got.KMSAliasForPrimaryBackup != "" {
			t.Errorf("got %+v, want a config with an empty alias", got)
		}
	})
}

func TestAWSRDSConfigModel(t *testing.T) {
	t.Run("to", func(t *testing.T) {
		tests := []struct {
			name     string
			config   *gqlsla.AWSRDSConfig
			wantNil  bool
			wantDur  int64
			wantUnit string
		}{
			{name: "nil config", config: nil, wantNil: true},
			{
				name:    "days",
				config:  &gqlsla.AWSRDSConfig{LogRetention: gqlsla.RetentionDuration{Duration: 7, Unit: gqlsla.Days}},
				wantDur: 7, wantUnit: "DAYS",
			}, {
				name:    "weeks",
				config:  &gqlsla.AWSRDSConfig{LogRetention: gqlsla.RetentionDuration{Duration: 2, Unit: gqlsla.Weeks}},
				wantDur: 2, wantUnit: "WEEKS",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := toAWSRDSConfigModel(tt.config)
				if tt.wantNil {
					if got != nil {
						t.Fatalf("expected nil, got %+v", got)
					}
					return
				}
				if len(got) != 1 || got[0].LogRetention.ValueInt64() != tt.wantDur ||
					got[0].LogRetentionUnit.ValueString() != tt.wantUnit {
					t.Errorf("got %+v, want %d %s", got, tt.wantDur, tt.wantUnit)
				}
				// Converting back gives the same SDK config.
				if back := fromAWSRDSConfigModel(got); back == nil || *back != *tt.config {
					t.Errorf("round trip: got %+v, want %+v", back, tt.config)
				}
			})
		}
	})

	t.Run("from nothing", func(t *testing.T) {
		if got := fromAWSRDSConfigModel(nil); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

func TestFromAWSS3ConfigModels(t *testing.T) {
	id := uuid.New()
	loc := func(s string) backupLocationModel { return backupLocationModel{ArchivalGroupID: types.StringValue(s)} }

	t.Run("NoneYieldsNoConfig", func(t *testing.T) {
		config, err := fromAWSS3ConfigModels(nil)
		if err != nil || config != nil {
			t.Fatalf("got %v, %v, want nil, nil", config, err)
		}
	})
	t.Run("Single", func(t *testing.T) {
		config, err := fromAWSS3ConfigModels([]backupLocationModel{loc(id.String())})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config == nil || config.ArchivalLocationID != id {
			t.Fatalf("got %v, want archival location %v", config, id)
		}
	})
	t.Run("MultipleRejected", func(t *testing.T) {
		_, err := fromAWSS3ConfigModels([]backupLocationModel{loc(id.String()), loc(uuid.NewString())})
		if err == nil || !strings.Contains(err.Error(), "multiple backup locations not supported") {
			t.Fatalf("expected multiple backup locations error, got %v", err)
		}
	})
	t.Run("InvalidUUIDRejected", func(t *testing.T) {
		if _, err := fromAWSS3ConfigModels([]backupLocationModel{loc("not-a-uuid")}); err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
	t.Run("NullIDRejected", func(t *testing.T) {
		if _, err := fromAWSS3ConfigModels([]backupLocationModel{{ArchivalGroupID: types.StringNull()}}); err == nil {
			t.Fatal("expected an error for a null archival group ID, got nil")
		}
	})
}

func TestFromAzureBlobConfigModel(t *testing.T) {
	id := uuid.New()

	if got, err := fromAzureBlobConfigModel(nil); got != nil || err != nil {
		t.Fatalf("got %v, %v, want nil, nil", got, err)
	}

	got, err := fromAzureBlobConfigModel([]azureBlobConfigModel{{ArchivalLocationID: types.StringValue(id.String())}})
	if err != nil {
		t.Fatal(err)
	}
	// The continuous backup retention is always one day.
	if got == nil || got.BackupLocationID != id || got.ContinuousBackupRetentionInDays != 1 {
		t.Errorf("got %+v, want location %s and 1 day of continuous backup", got, id)
	}

	if _, err := fromAzureBlobConfigModel([]azureBlobConfigModel{{ArchivalLocationID: types.StringValue("nope")}}); err == nil {
		t.Error("expected an error for an invalid archival location ID")
	}
}

// TestBackupModelsMatchAttributes decodes the models from the attribute names
// the SLA domain schema uses.
func TestBackupModelsMatchAttributes(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }

	t.Run("backup location", func(t *testing.T) {
		var m backupLocationModel
		decodeModel(t, map[string]attr.Type{keyArchivalGroupID: types.StringType},
			map[string]attr.Value{keyArchivalGroupID: str("g")}, &m)
		if m.ArchivalGroupID.ValueString() != "g" {
			t.Errorf("unexpected model: %+v", m)
		}
	})
	t.Run("aws dynamodb config", func(t *testing.T) {
		var m awsDynamoDBConfigModel
		decodeModel(t, map[string]attr.Type{keyKMSAlias: types.StringType},
			map[string]attr.Value{keyKMSAlias: str("alias/k")}, &m)
		if m.KMSAlias.ValueString() != "alias/k" {
			t.Errorf("unexpected model: %+v", m)
		}
	})
	t.Run("aws rds config", func(t *testing.T) {
		var m awsRDSConfigModel
		decodeModel(t, map[string]attr.Type{keyLogRetention: types.Int64Type, keyLogRetentionUnit: types.StringType},
			map[string]attr.Value{keyLogRetention: types.Int64Value(7), keyLogRetentionUnit: str("DAYS")}, &m)
		if m.LogRetention.ValueInt64() != 7 || m.LogRetentionUnit.ValueString() != "DAYS" {
			t.Errorf("unexpected model: %+v", m)
		}
	})
	t.Run("azure blob config", func(t *testing.T) {
		var m azureBlobConfigModel
		decodeModel(t, map[string]attr.Type{keyArchivalLocationID: types.StringType},
			map[string]attr.Value{keyArchivalLocationID: str("l")}, &m)
		if m.ArchivalLocationID.ValueString() != "l" {
			t.Errorf("unexpected model: %+v", m)
		}
	})
}
