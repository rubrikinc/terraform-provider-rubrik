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
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// slaDomainDataSourceFromRemote converts an SLA domain read from RSC to the state
// of the data source.
//
// The SDKv2 data source returned less than its schema declares, and the
// Framework data source returns the same, so that the migration doesn't change
// what a configuration reads. The blocks of the database configs and the
// retention lock are declared and documented, but were never set, so they are
// always empty. The replication specs only have the regions, the account, the
// retention and the replication pairs, and not the local retention or the
// cascading archival, which the resource reads. All of it is left out here and
// nowhere else, so a change which returns it only has to remove this function's
// reductions.
func slaDomainDataSourceFromRemote(ctx context.Context, domain gqlsla.Domain) (slaDomainDataSourceModel, diag.Diagnostics) {
	model, diags := slaDomainModelFromRemote(ctx, domain, slaDomainModel{})
	if diags.HasError() {
		return slaDomainDataSourceModel{}, diags
	}

	// The conversion of the resource only keeps the archival frequencies which are
	// configured, to avoid a diff against a derived value. The data source has no
	// configuration, and returned the frequencies of RSC, so they are put back.
	for i, spec := range domain.ArchivalSpecs {
		if i >= len(model.Archival) || model.Archival[i].ArchivalLocationID.ValueString() != spec.StorageSetting.ID {
			continue
		}
		model.Archival[i].Frequency = nil
		for _, frequency := range spec.Frequencies {
			model.Archival[i].Frequency = append(model.Archival[i].Frequency, string(frequency))
		}
		slices.Sort(model.Archival[i].Frequency)
	}

	// What the SDKv2 data source didn't set.
	for i := range model.ReplicationSpec {
		model.ReplicationSpec[i].LocalRetention = nil
		model.ReplicationSpec[i].CascadingArchival = nil
	}

	return slaDomainDataSourceModel{
		ID:                                model.ID,
		Name:                              model.Name,
		Description:                       model.Description,
		ObjectTypes:                       model.ObjectTypes,
		BackupType:                        model.BackupType,
		Archival:                          model.Archival,
		AWSDynamoDBConfig:                 model.AWSDynamoDBConfig,
		AWSRDSConfig:                      model.AWSRDSConfig,
		AzureBlobConfig:                   model.AzureBlobConfig,
		AzurePostgresFlexibleServerConfig: model.AzurePostgresFlexibleServerConfig,
		AzureSQLDatabaseConfig:            model.AzureSQLDatabaseConfig,
		AzureSQLManagedInstanceConfig:     model.AzureSQLManagedInstanceConfig,
		BackupLocation:                    model.BackupLocation,
		DailySchedule:                     model.DailySchedule,
		FirstFullSnapshot:                 model.FirstFullSnapshot,
		HourlySchedule:                    model.HourlySchedule,
		LocalRetention:                    model.LocalRetention,
		MinuteSchedule:                    model.MinuteSchedule,
		MonthlySchedule:                   model.MonthlySchedule,
		QuarterlySchedule:                 model.QuarterlySchedule,
		ReplicationSpec:                   model.ReplicationSpec,
		SnapshotWindow:                    model.SnapshotWindow,
		VMwareVMConfig:                    model.VMwareVMConfig,
		WeeklySchedule:                    model.WeeklySchedule,
		YearlySchedule:                    model.YearlySchedule,
	}, diags
}

// emptyListsForNull returns the value with every null list or set set to an
// empty one. The SDKv2 data source returned an empty list for a block which the
// SLA domain doesn't have, so length() of it is 0 in a configuration, and it is
// an error for the null list the framework otherwise stores for a nil slice.
func emptyListsForNull(value tftypes.Value) tftypes.Value {
	result, err := tftypes.Transform(value, func(_ *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if !v.IsNull() {
			return v, nil
		}
		switch v.Type().(type) {
		case tftypes.List:
			return tftypes.NewValue(v.Type(), []tftypes.Value{}), nil
		case tftypes.Set:
			return tftypes.NewValue(v.Type(), []tftypes.Value{}), nil
		}
		return v, nil
	})
	if err != nil {
		return value
	}
	return result
}

// slaDomainDataSourceState converts an SLA domain read from RSC to the state of
// the data source. A block which the SLA domain doesn't have is an empty list.
func slaDomainDataSourceState(ctx context.Context, domain gqlsla.Domain) (tfsdk.State, diag.Diagnostics) {
	s := slaDomainDataSourceSchema()
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}

	model, diags := slaDomainDataSourceFromRemote(ctx, domain)
	if diags.HasError() {
		return state, diags
	}
	diags.Append(state.Set(ctx, &model)...)
	if diags.HasError() {
		return state, diags
	}

	state.Raw = emptyListsForNull(state.Raw)
	return state, diags
}
