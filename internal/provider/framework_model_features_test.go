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
	"slices"
	"testing"

	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

func TestAwsCnpTrackedFeatures(t *testing.T) {
	cnp := aws.Feature{Feature: core.FeatureCloudNativeProtection.WithPermissionGroups(core.PermissionGroupBasic)}
	ccr := aws.Feature{Feature: core.FeatureCloudCostReport.WithPermissionGroups(core.PermissionGroupBasic)}
	cloudAccounts := aws.Feature{Feature: core.FeatureCloudAccounts}

	tt := []struct {
		name     string
		features []aws.Feature
		declared []core.Feature
		want     []aws.Feature
	}{{
		name: "NoFeatures",
	}, {
		// CLOUDACCOUNTS cannot be declared and is never tracked.
		name:     "UndeclarableFeatureDropped",
		features: []aws.Feature{cnp, cloudAccounts},
		declared: []core.Feature{cnp.Feature},
		want:     []aws.Feature{cnp},
	}, {
		// Cost reporting enabled by RSC on its own is left to RSC.
		name:     "UndeclaredCostReportDropped",
		features: []aws.Feature{cnp, ccr},
		declared: []core.Feature{cnp.Feature},
		want:     []aws.Feature{cnp},
	}, {
		// Nothing has been declared on import or when generating configuration,
		// so cost reporting is not tracked.
		name:     "NothingDeclaredDropsCostReport",
		features: []aws.Feature{cnp, ccr},
		want:     []aws.Feature{cnp},
	}, {
		// A declared cost report is tracked like any other feature. The
		// permission groups come from RSC, not from the declaration.
		name:     "DeclaredCostReportKept",
		features: []aws.Feature{cnp, ccr},
		declared: []core.Feature{cnp.Feature, core.FeatureCloudCostReport},
		want:     []aws.Feature{cnp, ccr},
	}, {
		// A declared cost report RSC does not have on the account is simply
		// absent, there is nothing to keep.
		name:     "DeclaredCostReportMissingOnAccount",
		features: []aws.Feature{cnp},
		declared: []core.Feature{cnp.Feature, core.FeatureCloudCostReport},
		want:     []aws.Feature{cnp},
	}}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			features := slices.Clone(tc.features)

			tracked := awsCnpTrackedFeatures(features, tc.declared)

			if !slices.EqualFunc(tracked, tc.want, func(a, b aws.Feature) bool { return a.DeepEqual(b.Feature) }) {
				t.Errorf("tracked: got %v, want %v", tracked, tc.want)
			}
			if !slices.EqualFunc(features, tc.features, func(a, b aws.Feature) bool { return a.DeepEqual(b.Feature) }) {
				t.Errorf("features were modified: got %v, want %v", features, tc.features)
			}
		})
	}
}
