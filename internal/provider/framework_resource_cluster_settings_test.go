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

	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cluster"
)

func TestValidateClusterSettingsConfig(t *testing.T) {
	tests := []struct {
		name              string
		version           types.String
		downloadedVersion types.String
		packageURL        types.String
		packageMD5        types.String
		wantErr           bool
	}{
		{
			name:    "no fields set",
			wantErr: false,
		},
		{
			name:    "only version",
			version: types.StringValue("9.3.3"),
			wantErr: false,
		},
		{
			name:              "only downloaded_version",
			downloadedVersion: types.StringValue("9.3.3"),
			wantErr:           false,
		},
		{
			name:              "equal version and downloaded_version",
			version:           types.StringValue("9.3.3"),
			downloadedVersion: types.StringValue("9.3.3"),
			wantErr:           false,
		},
		{
			name:              "downloaded_version newer than version",
			version:           types.StringValue("9.2.0"),
			downloadedVersion: types.StringValue("9.3.3"),
			wantErr:           false,
		},
		{
			name:              "downloaded_version older than version",
			version:           types.StringValue("9.3.3"),
			downloadedVersion: types.StringValue("9.2.0"),
			wantErr:           true,
		},
		{
			name:              "build suffix ignored, treated as equal",
			version:           types.StringValue("9.3.3-p9"),
			downloadedVersion: types.StringValue("9.3.3-p8"),
			wantErr:           false,
		},
		{
			name:              "unparseable version",
			version:           types.StringValue("not-a-version"),
			downloadedVersion: types.StringValue("9.2.0"),
			wantErr:           true,
		},
		{
			name:    "only version unparseable",
			version: types.StringValue("garbage"),
			wantErr: true,
		},
		{
			name:              "only downloaded_version unparseable",
			downloadedVersion: types.StringValue("garbage"),
			wantErr:           true,
		},
		{
			name:              "unknown downloaded_version is skipped",
			version:           types.StringValue("9.3.3"),
			downloadedVersion: types.StringUnknown(),
			wantErr:           false,
		},
		{
			name:       "package_url without package_md5",
			packageURL: types.StringValue("https://example.com/pkg.tar"),
			packageMD5: types.StringNull(),
			wantErr:    true,
		},
		{
			name:       "package_md5 without package_url",
			packageURL: types.StringNull(),
			packageMD5: types.StringValue("abc123"),
			wantErr:    true,
		},
		{
			name:       "package_url and package_md5 both set",
			version:    types.StringValue("9.3.3"),
			packageURL: types.StringValue("https://example.com/pkg.tar"),
			packageMD5: types.StringValue("abc123"),
			wantErr:    false,
		},
		{
			name:       "package_url set, package_md5 unknown is skipped",
			version:    types.StringValue("9.3.3"),
			packageURL: types.StringValue("https://example.com/pkg.tar"),
			packageMD5: types.StringUnknown(),
			wantErr:    false,
		},
		{
			name:       "package without version or downloaded_version",
			packageURL: types.StringValue("https://example.com/pkg.tar"),
			packageMD5: types.StringValue("abc123"),
			wantErr:    true,
		},
		{
			name:       "package with version",
			version:    types.StringValue("9.3.3"),
			packageURL: types.StringValue("https://example.com/pkg.tar"),
			packageMD5: types.StringValue("abc123"),
			wantErr:    false,
		},
		{
			name:              "package with downloaded_version",
			downloadedVersion: types.StringValue("9.3.3"),
			packageURL:        types.StringValue("https://example.com/pkg.tar"),
			packageMD5:        types.StringValue("abc123"),
			wantErr:           false,
		},
		{
			name:       "package with unknown version is skipped",
			version:    types.StringUnknown(),
			packageURL: types.StringValue("https://example.com/pkg.tar"),
			packageMD5: types.StringValue("abc123"),
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := clusterSettingsResourceModel{
				Version:           tt.version,
				DownloadedVersion: tt.downloadedVersion,
				PackageURL:        tt.packageURL,
				PackageMD5:        tt.packageMD5,
			}
			diags := validateClusterSettingsConfig(config)
			if got := diags.HasError(); got != tt.wantErr {
				t.Errorf("validateClusterSettingsConfig() error = %v, wantErr %v: %v", got, tt.wantErr, diags)
			}
		})
	}
}

func TestRefreshedVersion(t *testing.T) {
	tests := []struct {
		name    string
		prior   types.String
		details gqlcluster.UpgradeDetails
		want    types.String
	}{
		{
			name:    "unset is left untouched",
			prior:   types.StringNull(),
			details: gqlcluster.UpgradeDetails{CDMInfo: &gqlcluster.CDMInfo{Version: "9.4.0"}},
			want:    types.StringNull(),
		},
		{
			name:    "empty is left untouched",
			prior:   types.StringValue(""),
			details: gqlcluster.UpgradeDetails{CDMInfo: &gqlcluster.CDMInfo{Version: "9.4.0"}},
			want:    types.StringValue(""),
		},
		{
			name:    "managed and in sync stays the same",
			prior:   types.StringValue("9.4.0"),
			details: gqlcluster.UpgradeDetails{CDMInfo: &gqlcluster.CDMInfo{Version: "9.4.0"}},
			want:    types.StringValue("9.4.0"),
		},
		{
			name:    "managed and drifted mirrors the cluster",
			prior:   types.StringValue("9.4.0"),
			details: gqlcluster.UpgradeDetails{CDMInfo: &gqlcluster.CDMInfo{Version: "9.3.0"}},
			want:    types.StringValue("9.3.0"),
		},
		{
			name:    "managed but cluster reports no version keeps prior",
			prior:   types.StringValue("9.4.0"),
			details: gqlcluster.UpgradeDetails{CDMInfo: &gqlcluster.CDMInfo{Version: ""}},
			want:    types.StringValue("9.4.0"),
		},
		{
			name:    "managed but no cdm info keeps prior",
			prior:   types.StringValue("9.4.0"),
			details: gqlcluster.UpgradeDetails{CDMInfo: nil},
			want:    types.StringValue("9.4.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := refreshedVersion(tt.prior, tt.details); !got.Equal(tt.want) {
				t.Errorf("refreshedVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlreadyStaged(t *testing.T) {
	tests := []struct {
		name   string
		info   *gqlcluster.CDMInfo
		target string
		want   bool
	}{
		{
			name:   "nil info",
			info:   nil,
			target: "9.3.3",
			want:   false,
		},
		{
			name:   "installed version matches",
			info:   &gqlcluster.CDMInfo{Version: "9.3.3"},
			target: "9.3.3",
			want:   true,
		},
		{
			name:   "different version, not staged",
			info:   &gqlcluster.CDMInfo{Version: "9.2.0"},
			target: "9.3.3",
			want:   false,
		},
		{
			name: "staged via legacy downloaded_version",
			info: &gqlcluster.CDMInfo{
				Version:           "9.2.0",
				DownloadedVersion: "9.3.3",
				ClusterJobStatus:  gqlcluster.ClusterJobStatusReadyForUpgrade,
			},
			target: "9.3.3",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alreadyStaged(tt.info, tt.target); got != tt.want {
				t.Errorf("alreadyStaged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpgradeHops(t *testing.T) {
	tests := []struct {
		name      string
		path      []string
		installed string
		target    string
		want      []string
	}{
		{
			name:      "single direct hop",
			path:      []string{"9.2.0", "9.3.0"},
			installed: "9.2.0",
			target:    "9.3.0",
			want:      []string{"9.3.0"},
		},
		{
			name:      "multi hop returns rest after installed",
			path:      []string{"9.1.0", "9.2.0", "9.3.0"},
			installed: "9.1.0",
			target:    "9.3.0",
			want:      []string{"9.2.0", "9.3.0"},
		},
		{
			name:      "installed mid-path ignores earlier entries",
			path:      []string{"9.0.0", "9.1.0", "9.2.0", "9.3.0"},
			installed: "9.1.0",
			target:    "9.3.0",
			want:      []string{"9.2.0", "9.3.0"},
		},
		{
			name:      "empty path falls back to target",
			path:      nil,
			installed: "9.1.0",
			target:    "9.3.0",
			want:      []string{"9.3.0"},
		},
		{
			name:      "installed is last element falls back to target",
			path:      []string{"9.1.0"},
			installed: "9.1.0",
			target:    "9.3.0",
			want:      []string{"9.3.0"},
		},
		{
			name:      "installed not in path falls back to target",
			path:      []string{"9.2.0", "9.3.0"},
			installed: "9.1.0",
			target:    "9.3.0",
			want:      []string{"9.3.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := upgradeHops(tt.path, tt.installed, tt.target)
			if len(got) != len(tt.want) {
				t.Fatalf("upgradeHops() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("upgradeHops()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCurrentUpgradeType(t *testing.T) {
	tests := []struct {
		name string
		info *gqlcluster.CDMInfo
		want gqlcluster.UpgradeType
	}{
		{
			name: "nil info defaults to rolling",
			info: nil,
			want: gqlcluster.UpgradeTypeRolling,
		},
		{
			name: "fast preferred",
			info: &gqlcluster.CDMInfo{FastUpgradePreferred: true},
			want: gqlcluster.UpgradeTypeFast,
		},
		{
			name: "fast not preferred",
			info: &gqlcluster.CDMInfo{FastUpgradePreferred: false},
			want: gqlcluster.UpgradeTypeRolling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := currentUpgradeType(tt.info); got != tt.want {
				t.Errorf("currentUpgradeType() = %v, want %v", got, tt.want)
			}
		})
	}
}
