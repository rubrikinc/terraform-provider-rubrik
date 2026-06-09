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

import "testing"

func TestIsHigherVersion(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		current   string
		want      bool
	}{
		{
			name:      "empty current always loses",
			candidate: "9.3.3-p1-29518",
			current:   "",
			want:      true,
		},
		{
			name:      "higher patch level same mmp",
			candidate: "9.5.0-p2-36035",
			current:   "9.5.0-p1-35914",
			want:      true,
		},
		{
			name:      "lower patch level same mmp",
			candidate: "9.5.0-p1-35914",
			current:   "9.5.0-p2-36035",
			want:      false,
		},
		{
			name:      "patch release beats base release same mmp",
			candidate: "9.4.0-p2-30507",
			current:   "9.4.0-30189",
			want:      true,
		},
		{
			name:      "base release loses to its patch release",
			candidate: "9.4.0-30189",
			current:   "9.4.0-p2-30507",
			want:      false,
		},
		{
			name:      "higher minor wins regardless of build",
			candidate: "9.5.0-p1-35914",
			current:   "9.4.3-p1-31252",
			want:      true,
		},
		{
			name:      "equal full version is not higher",
			candidate: "9.4.2-p1-30914",
			current:   "9.4.2-p1-30914",
			want:      false,
		},
		{
			name:      "unparseable candidate never wins",
			candidate: "not-a-version",
			current:   "9.4.0-30189",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHigherVersion(tt.candidate, tt.current); got != tt.want {
				t.Errorf("isHigherVersion(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
			}
		})
	}
}

// TestFromReleaseDetailsLatest verifies latest_version is the highest full
// release name, not merely the highest major.minor.patch (which the SDK's
// version comparison collapses).
func TestFromReleaseDetailsLatest(t *testing.T) {
	names := []string{
		"9.3.3-p1-29518",
		"9.3.3-p2-29599",
		"9.3.3-p3-29693",
		"9.3.3-p4-29749",
		"9.3.3-p6-29818",
		"9.3.3-p7-29864",
		"9.4.0-30189",
		"9.4.0-p2-30507",
		"9.4.1-30557",
		"9.4.1-p1-30807",
		"9.4.2-30868",
		"9.4.2-p1-30914",
		"9.4.2-p3-31101",
		"9.4.3-31180",
		"9.4.3-p1-31252",
		"9.5.0-p1-35914",
		"9.5.0-p2-36035",
	}

	var latest string
	for _, n := range names {
		if isHigherVersion(n, latest) {
			latest = n
		}
	}

	const want = "9.5.0-p2-36035"
	if latest != want {
		t.Errorf("latest = %q, want %q", latest, want)
	}
}
