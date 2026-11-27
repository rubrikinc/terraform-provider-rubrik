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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
	sdkdiag "github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdkterraform "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlexocompute "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/exocompute"
)

const (
	testKMSApplicationID = "d55592db-956a-45b6-b8b6-e1ce9495746b"
	testKMSHostID        = "18f586ae-968b-42c4-a60a-93d59a80dace"
	testKMSRolePolicy    = `{"Version":"2012-10-17","Statement":[]}`
	testKMSFormationURL  = "https://example.com/exocompute-kms-stack-update"
)

type awsExocomputeKMSTestProvider struct {
	FrameworkProvider
	rsc *client
}

func (p *awsExocomputeKMSTestProvider) Configure(_ context.Context, _ fwprovider.ConfigureRequest, res *fwprovider.ConfigureResponse) {
	res.ResourceData = p.rsc
}

type fakeExocomputeKMS struct {
	mu             sync.Mutex
	status         gqlexocompute.KMSMappingProvisioningStatus
	sharingMode    gqlexocompute.KMSSharingMode
	cft            bool
	enableConfirms int
	revokeConfirms int
}

func (f *fakeExocomputeKMS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/api/session" {
		// Test JWT with a long expiry. The SDK reads its claims; no server
		// ever accepts the token as an actual RSC credential.
		_, _ = fmt.Fprint(w, `{"client_id":"test","access_token":"eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjQ3NzgzNzUzMDZ9.signature","is_eula_accepted":true}`)
		return
	}

	var request struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	accountID := uuid.MustParse(testKMSApplicationID)
	var result any
	switch {
	case strings.Contains(request.Query, "result: mapCloudAccountExocomputeAccount"):
		if !strings.Contains(request.Query, "shouldEnableAutomatedKeySharing: true") {
			f.sharingMode = gqlexocompute.KMSSharingModeNone
			result = map[string]any{"isSuccess": true}
			break
		}
		f.sharingMode = gqlexocompute.KMSSharingModeSharingOnly
		f.status = gqlexocompute.KMSMappingProvisioningStatusConnecting
		action := gqlexocompute.AWSKMSMappingAction{AppCloudAccountID: accountID, ManualRolePolicy: testKMSRolePolicy}
		if f.cft {
			action.ManualRolePolicy = ""
			action.CloudFormationURL = testKMSFormationURL
			action.PendingApproval = true
		}
		result = map[string]any{"isSuccess": true, "results": []gqlexocompute.AWSKMSMappingAction{action}}
	case strings.Contains(request.Query, "result: allAwsExocomputeConfigs"):
		var hosts []map[string]any
		if f.sharingMode != "" && f.status != gqlexocompute.KMSMappingProvisioningStatusDisabling {
			hosts = append(hosts, map[string]any{
				"awsCloudAccount":     map[string]string{"id": testKMSHostID},
				"mappedCloudAccounts": []map[string]string{{"id": testKMSApplicationID}},
			})
		}
		result = hosts
	case strings.Contains(request.Query, "result: awsExocomputeKmsMappingStatus"):
		status := gqlexocompute.AWSKMSMappingStatus{
			AppCloudAccountID: accountID, SharingMode: f.sharingMode, ProvisioningStatus: f.status,
		}
		if status.SharingMode == "" {
			status.SharingMode = gqlexocompute.KMSSharingModeUnspecified
		}
		if status.ProvisioningStatus == "" {
			status.ProvisioningStatus = gqlexocompute.KMSMappingProvisioningStatusUnspecified
		}
		if f.cft {
			status.StackARN = "arn:aws:cloudformation:us-east-2:123456789012:stack/example/example"
		}
		result = map[string]any{"results": []gqlexocompute.AWSKMSMappingStatus{status}}
	case strings.Contains(request.Query, "result: unmapCloudAccountExocomputeAccount"):
		action := gqlexocompute.AWSKMSMappingAction{AppCloudAccountID: accountID}
		switch f.status {
		case gqlexocompute.KMSMappingProvisioningStatusConnected, gqlexocompute.KMSMappingProvisioningStatusDisabling:
			f.sharingMode = gqlexocompute.KMSSharingModeNone
			f.status = gqlexocompute.KMSMappingProvisioningStatusDisabling
			action.PendingApproval = true
			if f.cft {
				action.CloudFormationURL = testKMSFormationURL
			}
		default:
			f.sharingMode, f.status = "", ""
		}
		result = map[string]any{"isSuccess": true, "results": []gqlexocompute.AWSKMSMappingAction{action}}
	case strings.Contains(request.Query, "result: updateManualKmsMapping") && !f.cft:
		switch f.status {
		case gqlexocompute.KMSMappingProvisioningStatusConnecting:
			f.status = gqlexocompute.KMSMappingProvisioningStatusConnected
			f.enableConfirms++
		case gqlexocompute.KMSMappingProvisioningStatusDisabling:
			f.sharingMode, f.status = "", ""
			f.revokeConfirms++
		default:
			http.Error(w, "unexpected confirmation", http.StatusBadRequest)
			return
		}
		result = map[string]any{"results": []map[string]string{{"applicationCloudAccountId": testKMSApplicationID}}}
	default:
		http.Error(w, "unexpected operation", http.StatusBadRequest)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": result}})
}

func TestAWSExocomputeKMSMappingConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{"HostWithoutKMS", map[string]any{keyRegion: "us-east-2"}, false},
		{"HostWithKMS", map[string]any{keyRegion: "us-east-2", keyAutomatedKeySharing: true}, true},
		{"HostWithExplicitFalse", map[string]any{keyRegion: "us-east-2", keyAutomatedKeySharing: false}, true},
		{"ApplicationWithoutKMS", map[string]any{keyHostAccountID: testKMSHostID}, false},
		{"ApplicationWithKMS", map[string]any{keyHostAccountID: testKMSHostID, keyAutomatedKeySharing: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.config[keyAccountID] = testKMSApplicationID
			diags := Provider().ValidateResource("rubrik_aws_exocompute", sdkterraform.NewResourceConfigRaw(tc.config))
			if diags.HasError() != tc.wantErr {
				t.Fatalf("unexpected configuration diagnostics: %v", diags)
			}
		})
	}
}

func TestAWSExocomputeKMSCloudFormationRevoke(t *testing.T) {
	ctx := t.Context()
	fake := &fakeExocomputeKMS{cft: true}
	srv := httptest.NewTLSServer(fake)
	defer srv.Close()
	c := &client{polarisClient: &polaris.Client{GQL: graphql.NewTestClient(srv)}}
	d := schema.TestResourceDataRaw(t, resourceAwsExocompute().Schema, map[string]any{
		keyAccountID: testKMSApplicationID, keyHostAccountID: testKMSHostID, keyAutomatedKeySharing: true,
	})
	if diags := awsCreateExocompute(ctx, d, c); diags.HasError() {
		t.Fatal(diags)
	}
	if d.Get(keyCloudFormationURL) != testKMSFormationURL {
		t.Fatalf("unexpected CloudFormation URL: %v", d.Get(keyCloudFormationURL))
	}
	fake.mu.Lock()
	fake.status = gqlexocompute.KMSMappingProvisioningStatusConnected
	fake.mu.Unlock()
	if diags := awsDeleteExocompute(ctx, d, c); !diags.HasError() || diags[0].Summary != "Exocompute KMS revoke requires CloudFormation approval" {
		t.Fatalf("expected a pending revoke diagnostic, got %v", diags)
	}
	if diags := awsReadExocompute(ctx, d, c); diags.HasError() {
		t.Fatal(diags)
	}
	if d.Id() != appCloudAccountPrefix+testKMSApplicationID ||
		d.Get(keyKMSProvisioningStatus) != string(gqlexocompute.KMSMappingProvisioningStatusDisabling) {
		t.Fatalf("refresh lost the pending revoke: id=%q status=%v", d.Id(), d.Get(keyKMSProvisioningStatus))
	}
	fake.mu.Lock()
	fake.status, fake.sharingMode = "", ""
	fake.mu.Unlock()
	if diags := awsReadExocompute(ctx, d, c); diags.HasError() || d.Id() != "" {
		t.Fatalf("refresh did not clear the completed revoke: id=%q diagnostics=%v", d.Id(), diags)
	}
}

// Run against a local GraphQL fake, not AWS or an RSC deployment.
func TestAWSExocomputeKMSConfirmationTerraform(t *testing.T) {
	bin, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("Terraform CLI is required for the local lifecycle test")
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", bin)

	fake := &fakeExocomputeKMS{}
	srv := httptest.NewTLSServer(fake)
	defer srv.Close()
	c := &client{polarisClient: &polaris.Client{GQL: graphql.NewTestClient(srv)}}

	mux := func() (tfprotov6.ProviderServer, error) {
		sdkProvider := Provider()
		sdkProvider.ConfigureContextFunc = func(context.Context, *schema.ResourceData) (any, sdkdiag.Diagnostics) {
			return c, nil
		}
		legacy, err := tf5to6server.UpgradeServer(t.Context(), sdkProvider.GRPCProvider)
		if err != nil {
			return nil, err
		}
		server, err := tf6muxserver.NewMuxServer(t.Context(),
			providerserver.NewProtocol6(&awsExocomputeKMSTestProvider{FrameworkProvider: FrameworkProvider{Version: "test"}, rsc: c}),
			func() tfprotov6.ProviderServer { return legacy })
		if err != nil {
			return nil, err
		}
		return server.ProviderServer(), nil
	}

	mappingConfig := fmt.Sprintf(`
resource "rubrik_aws_exocompute" "app" {
  account_id            = %q
  host_account_id       = %q
  automated_key_sharing = true
}
`, testKMSApplicationID, testKMSHostID)
	config := mappingConfig + `
resource "terraform_data" "policy" {
  input = rubrik_aws_exocompute.app.kms_role_policy
}
resource "rubrik_aws_exocompute_kms_confirmation" "app" {
  account_id = rubrik_aws_exocompute.app.account_id
  depends_on = [terraform_data.policy]
}
`
	const confirmation = "rubrik_aws_exocompute_kms_confirmation.app"
	const mapping = "rubrik_aws_exocompute.app"

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0)},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"rubrik": mux},
		CheckDestroy: func(_ *terraform.State) error {
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if fake.sharingMode != "" || fake.status != "" || fake.enableConfirms != 1 || fake.revokeConfirms != 1 {
				return fmt.Errorf("expected confirmation on create and revoke after unmap, got status %s and %d/%d confirms",
					fake.status, fake.enableConfirms, fake.revokeConfirms)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			// Create a mapping, attach the returned policy, then confirm it.
			Config: config,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(mapping, tfjsonpath.New(keyAutomatedKeySharing), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue(mapping, tfjsonpath.New(keyKMSRolePolicy), knownvalue.StringExact(testKMSRolePolicy)),
				statecheck.ExpectIdentityValueMatchesState(confirmation, tfjsonpath.New(keyID)),
			},
		}, {
			// Refresh the mapping after the separate confirmation has connected it.
			Config: config,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(mapping, tfjsonpath.New(keyKMSProvisioningStatus),
					knownvalue.StringExact(string(gqlexocompute.KMSMappingProvisioningStatusConnected))),
			},
		}, {
			// The sharing mode and status are read back. RSC does not return
			// the create-only policy or CloudFormation URL on import.
			ResourceName:            mapping,
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{keyKMSRolePolicy, keyCloudFormationURL},
		}, {
			// Import confirmation using the CLI.
			ResourceName:      confirmation,
			ImportState:       true,
			ImportStateKind:   resource.ImportCommandWithID,
			ImportStateVerify: true,
		}, {
			// Import confirmation using an ID block.
			ResourceName:    confirmation,
			ImportState:     true,
			ImportStateKind: resource.ImportBlockWithID,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		}, {
			// Import confirmation by resource identity.
			ResourceName:    confirmation,
			ImportState:     true,
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		}, {
			// Omitting the opt-in must not replace an existing KMS mapping.
			Config: strings.Replace(config, "  automated_key_sharing = true\n", "", 1),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		}, {
			// Remove the dependent policy and confirmation and replace the KMS mapping with a plain one.
			Config: strings.Replace(mappingConfig, "automated_key_sharing = true", "automated_key_sharing = false", 1),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(mapping, plancheck.ResourceActionDestroyBeforeCreate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(mapping, tfjsonpath.New(keyAutomatedKeySharing), knownvalue.Bool(false)),
			},
		}},
	})
}
