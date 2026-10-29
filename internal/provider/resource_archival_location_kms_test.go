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
	"maps"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const testKeyManagerID = "e9895414-64b4-4e25-a10d-c7a76983953b"

// TestArchivalLocationKMSKeyConflicts checks that the client-side encryption
// key blocks of the cloud native archival locations conflict with the
// server-side encryption keys.
func TestArchivalLocationKMSKeyConflicts(t *testing.T) {
	awsKMSKey := []any{map[string]any{keyKeyManagerID: testKeyManagerID, keyKeyID: "key-id"}}
	azureKeyVaultKey := []any{map[string]any{keyKeyManagerID: testKeyManagerID, keyKeyName: "key", keyKeyVersion: "1"}}
	azureCMK := []any{map[string]any{keyName: "key", keyVaultName: "vault", keyRegion: "eastus2"}}
	gcpKMSKey := []any{map[string]any{keyKeyManagerID: testKeyManagerID, keyKeyName: "key", keyKeyVersion: "1"}}
	gcpCMK := []any{map[string]any{keyName: "key", keyRingName: "ring", keyRegion: "us-east1"}}

	awsBase := map[string]any{keyAccountID: testKeyManagerID, keyBucketPrefix: "prefix", keyName: "aws"}
	azureBase := map[string]any{keyCloudAccountID: testKeyManagerID, keyStorageAccountNamePrefix: "prefix",
		keyName: "azure"}
	gcpBase := map[string]any{keyCloudAccountID: testKeyManagerID, keyBucketPrefix: "prefix", keyName: "gcp"}

	testCases := []struct {
		name     string
		resource *schema.Resource
		base     map[string]any
		config   map[string]any
		conflict bool
	}{{
		name:     "AWSKMSKey",
		resource: resourceAwsArchivalLocation(),
		base:     awsBase,
		config:   map[string]any{keyKMSKey: awsKMSKey},
	}, {
		name:     "AWSKMSKeyAndKMSMasterKey",
		resource: resourceAwsArchivalLocation(),
		base:     awsBase,
		config:   map[string]any{keyKMSKey: awsKMSKey, keyKMSMasterKey: "aws/s3"},
		conflict: true,
	}, {
		name:     "AzureKeyVaultKey",
		resource: resourceAzureArchivalLocation(),
		base:     azureBase,
		config:   map[string]any{keyKeyVaultKey: azureKeyVaultKey},
	}, {
		name:     "AzureKeyVaultKeyAndCustomerManagedKey",
		resource: resourceAzureArchivalLocation(),
		base:     azureBase,
		config:   map[string]any{keyKeyVaultKey: azureKeyVaultKey, keyCustomerManagedKey: azureCMK},
		conflict: true,
	}, {
		name:     "GCPKMSKey",
		resource: resourceGcpArchivalLocation(),
		base:     gcpBase,
		config:   map[string]any{keyKMSKey: gcpKMSKey},
	}, {
		name:     "GCPKMSKeyAndCustomerManagedKey",
		resource: resourceGcpArchivalLocation(),
		base:     gcpBase,
		config:   map[string]any{keyKMSKey: gcpKMSKey, keyCustomerManagedKey: gcpCMK},
		conflict: true,
	}}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			config := maps.Clone(testCase.base)
			maps.Copy(config, testCase.config)
			diags := testCase.resource.Validate(terraform.NewResourceConfigRaw(config))
			if diags.HasError() != testCase.conflict {
				t.Fatalf("expected conflict %t, got diagnostics %v", testCase.conflict, diags)
			}
		})
	}
}

// TestAWSArchivalLocationCustomizeDiff checks that a specific region AWS
// archival location requires a KMS master key or a KMS key when it is
// created, since there is no default KMS master key for a specific region.
func TestAWSArchivalLocationCustomizeDiff(t *testing.T) {
	kmsKey := cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
		keyKeyManagerID: cty.StringVal(testKeyManagerID),
		keyKeyID:        cty.StringVal("key-id"),
	})})

	testCases := []struct {
		name    string
		id      string
		config  map[string]cty.Value
		wantErr string
	}{{
		name:   "SourceRegionWithoutKey",
		config: map[string]cty.Value{},
	}, {
		name:    "SpecificRegionWithoutKey",
		config:  map[string]cty.Value{keyRegion: cty.StringVal("us-east-2")},
		wantErr: "one of kms_master_key and kms_key must be specified",
	}, {
		name: "SpecificRegionWithKMSMasterKey",
		config: map[string]cty.Value{
			keyRegion:       cty.StringVal("us-east-2"),
			keyKMSMasterKey: cty.StringVal("1234abcd-12ab-34cd-56ef-1234567890ab"),
		},
	}, {
		name:   "SpecificRegionWithKMSKey",
		config: map[string]cty.Value{keyRegion: cty.StringVal("us-east-2"), keyKMSKey: kmsKey},
	}, {
		name:   "UnknownRegion",
		config: map[string]cty.Value{keyRegion: cty.UnknownVal(cty.String)},
	}, {
		name: "UnknownKMSMasterKey",
		config: map[string]cty.Value{
			keyRegion:       cty.StringVal("us-east-2"),
			keyKMSMasterKey: cty.UnknownVal(cty.String),
		},
	}, {
		name:   "ExistingLocation",
		id:     "c5d2a3e1-4b6f-4f0e-9a8d-7b1c2e3f4a5b",
		config: map[string]cty.Value{keyRegion: cty.StringVal("us-east-2")},
	}}

	res := resourceAwsArchivalLocation()
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			attrs := map[string]cty.Value{
				keyAccountID:    cty.StringVal(testKeyManagerID),
				keyBucketPrefix: cty.StringVal("prefix"),
				keyName:         cty.StringVal("aws"),
			}
			maps.Copy(attrs, testCase.config)
			rawConfig, err := res.CoreConfigSchema().CoerceValue(cty.ObjectVal(attrs))
			if err != nil {
				t.Fatal(err)
			}

			// The legacy configuration only drives the attribute diff, the
			// check reads the raw configuration.
			config := map[string]any{keyAccountID: testKeyManagerID, keyBucketPrefix: "prefix", keyName: "aws"}
			state := &terraform.InstanceState{ID: testCase.id, RawConfig: rawConfig}
			if testCase.id != "" {
				state.Attributes = map[string]string{keyAccountID: testKeyManagerID, keyBucketPrefix: "prefix",
					keyName: "aws", keyRegion: "us-east-2"}
				config[keyRegion] = "us-east-2"
			}

			_, err = res.SimpleDiff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
			switch {
			case testCase.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case testCase.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", testCase.wantErr)
			case testCase.wantErr != "" && !strings.Contains(err.Error(), testCase.wantErr):
				t.Fatalf("error %q does not contain %q", err, testCase.wantErr)
			}
		})
	}
}
