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
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// slaDomainSnapshotBlocks are the blocks of which an SLA domain needs at least
// one. An AWS RDS SLA doesn't need a snapshot frequency, and neither does a V1
// (Azure-managed) Azure SQL SLA, so their configs count as well.
var slaDomainSnapshotBlocks = []string{
	keyDailySchedule,
	keyHourlySchedule,
	keyMinuteSchedule,
	keyMonthlySchedule,
	keyQuarterlySchedule,
	keyWeeklySchedule,
	keyYearlySchedule,
	keyAWSRDSConfig,
	keyAzureSQLDatabaseConfig,
	keyAzureSQLManagedInstanceConfig,
}

// slaDomainAtLeastOneBlock requires at least one of the snapshot blocks of an
// SLA domain to be configured. The SDKv2 resource had this rule on each of the
// seven schedule blocks, which reported the same error seven times.
func slaDomainAtLeastOneBlock() resource.ConfigValidator {
	expressions := make([]path.Expression, 0, len(slaDomainSnapshotBlocks))
	for _, name := range slaDomainSnapshotBlocks {
		expressions = append(expressions, path.MatchRoot(name))
	}
	return resourcevalidator.AtLeastOneOf(expressions...)
}
