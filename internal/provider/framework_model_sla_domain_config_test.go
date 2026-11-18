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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

func TestVMwareVMConfigModel(t *testing.T) {
	if fromVMwareVMConfigModel(nil) != nil || toVMwareVMConfigModel(nil) != nil {
		t.Fatal("a missing block and a nil config must convert to nil")
	}

	// The log retention is in seconds, and is an int64 in the SDK.
	got := toVMwareVMConfigModel(&gqlsla.VMwareVMConfig{LogRetentionSeconds: 86400})
	if len(got) != 1 || got[0].LogRetention.ValueInt64() != 86400 {
		t.Fatalf("got %+v, want 86400 seconds", got)
	}
	back := fromVMwareVMConfigModel(got)
	if back == nil || back.LogRetentionSeconds != 86400 {
		t.Errorf("round trip: got %+v, want 86400", back)
	}
}

// TestLogRetentionConfigModels covers the three configs which hold only a log
// retention, which share a model: Managed Volume, Postgres DB Cluster and GCP
// Cloud SQL.
func TestLogRetentionConfigModels(t *testing.T) {
	retention := gqlsla.RetentionDuration{Duration: 7, Unit: gqlsla.Weeks}
	block := []logRetentionConfigModel{{LogRetention: types.Int64Value(7), LogRetentionUnit: types.StringValue("WEEKS")}}

	t.Run("managed volume", func(t *testing.T) {
		if fromManagedVolumeConfigModel(nil) != nil || toManagedVolumeConfigModel(nil) != nil {
			t.Fatal("nil must convert to nil")
		}
		if got := fromManagedVolumeConfigModel(block); got == nil || got.LogRetention != retention {
			t.Errorf("from: got %+v, want %+v", got, retention)
		}
		got := toManagedVolumeConfigModel(&gqlsla.ManagedVolumeSlaConfig{LogRetention: retention})
		if len(got) != 1 || !got[0].LogRetention.Equal(block[0].LogRetention) || !got[0].LogRetentionUnit.Equal(block[0].LogRetentionUnit) {
			t.Errorf("to: got %+v, want %+v", got, block)
		}
	})

	t.Run("postgres db cluster", func(t *testing.T) {
		if fromPostgresDBClusterConfigModel(nil) != nil || toPostgresDBClusterConfigModel(nil) != nil {
			t.Fatal("nil must convert to nil")
		}
		if got := fromPostgresDBClusterConfigModel(block); got == nil || got.LogRetention != retention {
			t.Errorf("from: got %+v, want %+v", got, retention)
		}
		got := toPostgresDBClusterConfigModel(&gqlsla.PostgresDbClusterSlaConfig{LogRetention: retention})
		if len(got) != 1 || got[0].LogRetention.ValueInt64() != 7 || got[0].LogRetentionUnit.ValueString() != "WEEKS" {
			t.Errorf("to: got %+v", got)
		}
	})

	t.Run("gcp cloud sql", func(t *testing.T) {
		if fromGCPCloudSQLConfigModel(nil) != nil || toGCPCloudSQLConfigModel(nil) != nil {
			t.Fatal("nil must convert to nil")
		}
		if got := fromGCPCloudSQLConfigModel(block); got == nil || got.LogRetention != retention {
			t.Errorf("from: got %+v, want %+v", got, retention)
		}
		got := toGCPCloudSQLConfigModel(&gqlsla.GcpCloudSqlConfig{LogRetention: retention})
		if len(got) != 1 || got[0].LogRetention.ValueInt64() != 7 || got[0].LogRetentionUnit.ValueString() != "WEEKS" {
			t.Errorf("to: got %+v", got)
		}
	})
}

func TestMSSQLConfigModel(t *testing.T) {
	if fromMSSQLConfigModel(nil) != nil || toMSSQLConfigModel(nil) != nil {
		t.Fatal("nil must convert to nil")
	}

	sdk := &gqlsla.MssqlConfig{
		Frequency:    gqlsla.RetentionDuration{Duration: 3, Unit: gqlsla.Days},
		LogRetention: gqlsla.RetentionDuration{Duration: 2, Unit: gqlsla.Weeks},
	}
	block := toMSSQLConfigModel(sdk)
	if len(block) != 1 || block[0].Frequency.ValueInt64() != 3 || block[0].FrequencyUnit.ValueString() != "DAYS" ||
		block[0].LogRetention.ValueInt64() != 2 || block[0].LogRetentionUnit.ValueString() != "WEEKS" {
		t.Fatalf("got %+v", block)
	}
	if back := fromMSSQLConfigModel(block); back == nil || *back != *sdk {
		t.Errorf("round trip: got %+v, want %+v", back, sdk)
	}
}

// TestLogScheduleConfigModels covers the Mongo and MySQL configs, which name
// their log frequency and log retention frequency and retention, and not
// log_retention.
func TestLogScheduleConfigModels(t *testing.T) {
	frequency := gqlsla.RetentionDuration{Duration: 4, Unit: gqlsla.Days}
	retention := gqlsla.RetentionDuration{Duration: 9, Unit: gqlsla.Months}

	// The frequency must not be swapped with the retention. The values and the
	// units are all different.
	check := func(t *testing.T, got []logScheduleConfigModel) {
		t.Helper()
		if len(got) != 1 || got[0].Frequency.ValueInt64() != 4 || got[0].FrequencyUnit.ValueString() != "DAYS" ||
			got[0].Retention.ValueInt64() != 9 || got[0].RetentionUnit.ValueString() != "MONTHS" {
			t.Errorf("got %+v, want frequency 4 DAYS and retention 9 MONTHS", got)
		}
	}
	block := []logScheduleConfigModel{{
		Frequency: types.Int64Value(4), FrequencyUnit: types.StringValue("DAYS"),
		Retention: types.Int64Value(9), RetentionUnit: types.StringValue("MONTHS"),
	}}

	t.Run("mongo", func(t *testing.T) {
		if fromMongoConfigModel(nil) != nil || toMongoConfigModel(nil) != nil {
			t.Fatal("nil must convert to nil")
		}
		got := fromMongoConfigModel(block)
		if got == nil || got.LogFrequency != frequency || got.LogRetention != retention {
			t.Errorf("from: got %+v", got)
		}
		check(t, toMongoConfigModel(&gqlsla.MongoConfig{LogFrequency: frequency, LogRetention: retention}))
	})

	t.Run("mysql", func(t *testing.T) {
		if fromMySQLDBConfigModel(nil) != nil || toMySQLDBConfigModel(nil) != nil {
			t.Fatal("nil must convert to nil")
		}
		got := fromMySQLDBConfigModel(block)
		if got == nil || got.LogFrequency != frequency || got.LogRetention != retention {
			t.Errorf("from: got %+v", got)
		}
		check(t, toMySQLDBConfigModel(&gqlsla.MysqldbSlaConfig{LogFrequency: frequency, LogRetention: retention}))
	})
}

func TestAzurePostgresFlexibleServerConfigModel(t *testing.T) {
	if fromAzurePostgresFlexibleServerConfigModel(nil) != nil || toAzurePostgresFlexibleServerConfigModel(nil) != nil {
		t.Fatal("nil must convert to nil")
	}

	got := toAzurePostgresFlexibleServerConfigModel(&gqlsla.AzurePostgresFlexibleServerConfig{BackupRetentionInDays: 14})
	if len(got) != 1 || got[0].BackupRetentionInDays.ValueInt64() != 14 {
		t.Fatalf("got %+v", got)
	}
	if back := fromAzurePostgresFlexibleServerConfigModel(got); back == nil || back.BackupRetentionInDays != 14 {
		t.Errorf("round trip: got %+v", back)
	}
}

// TestConfigModelsMatchAttributes decodes the models from the attribute names
// the SLA domain schema uses.
func TestConfigModelsMatchAttributes(t *testing.T) {
	str := func(s string) attr.Value { return types.StringValue(s) }

	t.Run("vmware vm config", func(t *testing.T) {
		var m vmwareVMConfigModel
		decodeModel(t, map[string]attr.Type{keyLogRetention: types.Int64Type},
			map[string]attr.Value{keyLogRetention: types.Int64Value(60)}, &m)
		if m.LogRetention.ValueInt64() != 60 {
			t.Errorf("unexpected model: %+v", m)
		}
	})
	t.Run("log retention config", func(t *testing.T) {
		var m logRetentionConfigModel
		decodeModel(t, map[string]attr.Type{keyLogRetention: types.Int64Type, keyLogRetentionUnit: types.StringType},
			map[string]attr.Value{keyLogRetention: types.Int64Value(7), keyLogRetentionUnit: str("DAYS")}, &m)
		if m.LogRetention.ValueInt64() != 7 || m.LogRetentionUnit.ValueString() != "DAYS" {
			t.Errorf("unexpected model: %+v", m)
		}
	})
	t.Run("mssql config", func(t *testing.T) {
		var m mssqlConfigModel
		decodeModel(t, map[string]attr.Type{
			keyFrequency: types.Int64Type, keyFrequencyUnit: types.StringType,
			keyLogRetention: types.Int64Type, keyLogRetentionUnit: types.StringType,
		}, map[string]attr.Value{
			keyFrequency: types.Int64Value(1), keyFrequencyUnit: str("DAYS"),
			keyLogRetention: types.Int64Value(2), keyLogRetentionUnit: str("WEEKS"),
		}, &m)
		if m.Frequency.ValueInt64() != 1 || m.LogRetention.ValueInt64() != 2 {
			t.Errorf("unexpected model: %+v", m)
		}
	})
	t.Run("log schedule config", func(t *testing.T) {
		var m logScheduleConfigModel
		decodeModel(t, map[string]attr.Type{
			keyFrequency: types.Int64Type, keyFrequencyUnit: types.StringType,
			keyRetention: types.Int64Type, keyRetentionUnit: types.StringType,
		}, map[string]attr.Value{
			keyFrequency: types.Int64Value(1), keyFrequencyUnit: str("DAYS"),
			keyRetention: types.Int64Value(2), keyRetentionUnit: str("WEEKS"),
		}, &m)
		if m.Frequency.ValueInt64() != 1 || m.Retention.ValueInt64() != 2 {
			t.Errorf("unexpected model: %+v", m)
		}
	})
	t.Run("azure postgres flexible server config", func(t *testing.T) {
		var m azurePostgresFlexibleServerConfigModel
		decodeModel(t, map[string]attr.Type{keyBackupRetentionInDays: types.Int64Type},
			map[string]attr.Value{keyBackupRetentionInDays: types.Int64Value(7)}, &m)
		if m.BackupRetentionInDays.ValueInt64() != 7 {
			t.Errorf("unexpected model: %+v", m)
		}
	})
}
