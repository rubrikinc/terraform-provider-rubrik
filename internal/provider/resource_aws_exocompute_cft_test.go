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
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlexocompute "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/exocompute"
)

type fakeExocomputeStackFunc func(context.Context, string) error

func (f fakeExocomputeStackFunc) Update(ctx context.Context, templateURL string) error {
	return f(ctx, templateURL)
}

func TestAWSExocomputeKMSCloudFormationAutomaticLifecycle(t *testing.T) {
	rsc := &fakeExocomputeKMS{cft: true}
	srv := httptest.NewTLSServer(rsc)
	defer srv.Close()

	const profile = "example"
	const role = "arn:aws:iam::123456789012:role/example"
	var updates int
	revokeError := true
	c := &client{
		polarisClient: &polaris.Client{GQL: graphql.NewTestClient(srv)},
		openAWSExocomputeStack: func(_ context.Context, stackARN, awsProfile, awsRole string) (awsExocomputeStack, error) {
			if stackARN != testKMSStackARN || awsProfile != profile || awsRole != role {
				t.Fatalf("invalid CloudFormation target or credentials: %q / %q / %q", stackARN, awsProfile, awsRole)
			}
			return fakeExocomputeStackFunc(func(_ context.Context, templateURL string) error {
				if templateURL != testKMSTemplateURL {
					t.Fatalf("unexpected CloudFormation template URL")
				}
				updates++
				rsc.mu.Lock()
				defer rsc.mu.Unlock()
				switch rsc.status {
				case gqlexocompute.KMSMappingProvisioningStatusConnecting:
					rsc.status = gqlexocompute.KMSMappingProvisioningStatusConnected
				case gqlexocompute.KMSMappingProvisioningStatusDisabling:
					if revokeError {
						return errors.New("stack update failed")
					}
					rsc.status, rsc.sharingMode = "", ""
				default:
					t.Fatalf("updated the stack without a pending mapping: %s", rsc.status)
				}
				return nil
			}), nil
		},
	}
	d := schema.TestResourceDataRaw(t, resourceAwsExocompute().Schema, map[string]any{
		keyAccountID: testKMSApplicationID, keyHostAccountID: testKMSHostID, keyAutomatedKeySharing: true,
		keyProfile: profile, keyAssumeRole: role,
	})
	if diags := awsCreateExocompute(t.Context(), d, c); diags.HasError() {
		t.Fatal(diags)
	}
	if updates != 1 || d.Get(keyKMSProvisioningStatus) != string(gqlexocompute.KMSMappingProvisioningStatusConnected) ||
		d.Get(keyCloudFormationURL) != "" {
		t.Fatal("create did not complete the stack update and clear the launch URL")
	}

	// RSC must acknowledge the add before we can safely ask it to revoke.
	rsc.mu.Lock()
	rsc.status = gqlexocompute.KMSMappingProvisioningStatusConnecting
	rsc.mu.Unlock()
	if diags := awsDeleteExocompute(t.Context(), d, c); !diags.HasError() || !strings.Contains(diags[0].Summary, "still CONNECTING") {
		t.Fatalf("delete attempted to cancel a pending CloudFormation add: %v", diags)
	}
	rsc.mu.Lock()
	if rsc.status != gqlexocompute.KMSMappingProvisioningStatusConnecting {
		rsc.mu.Unlock()
		t.Fatal("delete mutated the pending add")
	}
	rsc.status = gqlexocompute.KMSMappingProvisioningStatusConnected
	rsc.mu.Unlock()

	// An AWS error leaves RSC's pending revoke and the Terraform resource intact.
	if diags := awsDeleteExocompute(t.Context(), d, c); !diags.HasError() {
		t.Fatal("expected a failed stack update to stop the delete")
	}
	if diags := awsReadExocompute(t.Context(), d, c); diags.HasError() {
		t.Fatal(diags)
	}
	if d.Id() == "" || d.Get(keyKMSProvisioningStatus) != string(gqlexocompute.KMSMappingProvisioningStatusDisabling) {
		t.Fatal("refresh did not retain the pending revoke")
	}
	revokeError = false
	if diags := awsDeleteExocompute(t.Context(), d, c); diags.HasError() {
		t.Fatal(diags)
	}
	if diags := awsReadExocompute(t.Context(), d, c); diags.HasError() || d.Id() != "" || updates != 3 {
		t.Fatalf("retry did not finish removing the mapping: ID %q, updates %d, diagnostics %v", d.Id(), updates, diags)
	}
}

func TestAWSExocomputeKMSCloudFormationPreflight(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cft     bool
		wantErr string
	}{
		{name: "ManualApplication", wantErr: "single-account CloudFormation"},
		{name: "WrongCredentials", cft: true, wantErr: "incorrect AWS account"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rsc := &fakeExocomputeKMS{cft: tc.cft}
			srv := httptest.NewTLSServer(rsc)
			defer srv.Close()
			c := &client{
				polarisClient: &polaris.Client{GQL: graphql.NewTestClient(srv)},
				openAWSExocomputeStack: func(context.Context, string, string, string) (awsExocomputeStack, error) {
					return nil, errors.New("incorrect AWS account")
				},
			}
			d := schema.TestResourceDataRaw(t, resourceAwsExocompute().Schema, map[string]any{
				keyAccountID: testKMSApplicationID, keyHostAccountID: testKMSHostID, keyAutomatedKeySharing: true, keyProfile: "example",
			})
			if diags := awsCreateExocompute(t.Context(), d, c); !diags.HasError() || !strings.Contains(diags[0].Summary, tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, diags)
			}
			rsc.mu.Lock()
			defer rsc.mu.Unlock()
			if rsc.status != "" || d.Id() != "" {
				t.Fatal("preflight failure created a mapping")
			}
		})
	}
}

func TestAWSExocomputeKMSRejectsDifferentCloudFormationStack(t *testing.T) {
	stack := fakeExocomputeStackFunc(func(context.Context, string) error {
		t.Fatal("submitted an update to a different stack")
		return nil
	})
	action := gqlexocompute.AWSKMSMappingAction{
		CloudFormationURL: strings.Replace(testKMSFormationURL, testKMSStackARN, testKMSStackARN+"-other", 1),
		TemplateURL:       testKMSTemplateURL,
	}
	if err := awsApplyExocomputeStackUpdate(t.Context(), stack, testKMSStackARN, action); err == nil {
		t.Fatal("accepted a launch URL for a different stack")
	}
}
