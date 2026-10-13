// Copyright 2021 Rubrik, Inc.
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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const awsAccountOneRegionTmpl = `
provider "polaris" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "polaris_aws_account" "default" {
	name    = "{{ .Resource.AccountName }}"
	profile = "{{ .Resource.Profile }}"

	cloud_native_protection {
		permission_groups = [
			"BASIC",
		]
		regions = [
			"us-east-2",
		]
	}
}
`

const awsAccountTwoRegionsTmpl = `
provider "polaris" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "polaris_aws_account" "default" {
	name    = "{{ .Resource.AccountName }}"
	profile = "{{ .Resource.Profile }}"

	cloud_native_protection {
		permission_groups = [
			"BASIC",
		]
		regions = [
			"us-east-2",
			"us-west-2",
		]
	}
}
`

const awsCrossAccountOneRegionTmpl = `
provider "polaris" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "polaris_aws_account" "default" {
	assume_role = "{{ .Resource.CrossAccountRole }}"
	name        = "{{ .Resource.CrossAccountName }}"

	cloud_native_protection {
		permission_groups = [
			"BASIC",
		]
		regions = [
			"us-east-2",
		]
	}
}
`

const awsCrossAccountTwoRegionsTmpl = `
provider "polaris" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "polaris_aws_account" "default" {
	assume_role = "{{ .Resource.CrossAccountRole }}"
	name        = "{{ .Resource.CrossAccountName }}"

	cloud_native_protection {
		permission_groups = [
			"BASIC",
		]
		regions = [
			"us-east-2",
			"us-west-2",
		]
	}
}
`

func TestAccPolarisAWSAccount_basic(t *testing.T) {
	config, account := loadAWSTestConfig(t)
	accountOneRegion, err := makeTerraformConfig(config, awsAccountOneRegionTmpl)
	if err != nil {
		t.Fatal(err)
	}
	accountTwoRegions, err := makeTerraformConfig(config, awsAccountTwoRegionsTmpl)
	if err != nil {
		t.Fatal(err)
	}

	// Add and update account using a profile
	resource.Test(t, resource.TestCase{
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{{
			Config: accountOneRegion,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.AccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "profile", account.Profile),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "delete_snapshots_on_destroy", "false"),
				resource.TestCheckNoResourceAttr("polaris_aws_account.default", "assume_role"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-east-2"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.*", "BASIC"),
			),
		}, {
			Config: accountTwoRegions,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.AccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "profile", account.Profile),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "delete_snapshots_on_destroy", "false"),
				resource.TestCheckNoResourceAttr("polaris_aws_account.default", "assume_role"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.#", "2"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-east-2"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-west-2"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.*", "BASIC"),
			),
		}, {
			Config: accountOneRegion,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.AccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "profile", account.Profile),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "delete_snapshots_on_destroy", "false"),
				resource.TestCheckNoResourceAttr("polaris_aws_account.default", "assume_role"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-east-2"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.*", "BASIC"),
			),
		}},
	})

	crossAccountOneRegion, err := makeTerraformConfig(config, awsCrossAccountOneRegionTmpl)
	if err != nil {
		t.Fatal(err)
	}
	crossAccountTwoRegions, err := makeTerraformConfig(config, awsCrossAccountTwoRegionsTmpl)
	if err != nil {
		t.Fatal(err)
	}

	// Add and update account using cross account role. This test uses the
	// default profile to assume the role.
	resource.Test(t, resource.TestCase{
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{{
			Config: crossAccountOneRegion,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.CrossAccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "assume_role", account.CrossAccountRole),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "delete_snapshots_on_destroy", "false"),
				resource.TestCheckNoResourceAttr("polaris_aws_account.default", "profile"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-east-2"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.*", "BASIC"),
			),
		}, {
			Config: crossAccountTwoRegions,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.CrossAccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "assume_role", account.CrossAccountRole),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "delete_snapshots_on_destroy", "false"),
				resource.TestCheckNoResourceAttr("polaris_aws_account.default", "profile"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.#", "2"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-east-2"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-west-2"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.*", "BASIC"),
			),
		}, {
			Config: crossAccountOneRegion,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.CrossAccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "assume_role", account.CrossAccountRole),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "delete_snapshots_on_destroy", "false"),
				resource.TestCheckNoResourceAttr("polaris_aws_account.default", "profile"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.regions.*", "us-east-2"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_protection.0.permission_groups.*", "BASIC"),
			),
		}},
	})
}

const awsAccountUEMKeyManagementTmpl = `
provider "polaris" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "polaris_aws_account" "default" {
	name    = "{{ .Resource.AccountName }}"
	profile = "{{ .Resource.Profile }}"

	cloud_native_uem_key_management {
		permission_groups = [
			"BASIC",
		]
		regions = [
			"us-east-2",
		]
	}
}
`

// TestAccPolarisAWSAccount_uemKeyManagement onboards and offboards the Cloud
// Native UEM Key Management feature as the only feature of the account.
//
// The feature is gated in RSC behind the REL_ENABLE_AWS_EC2_UEM and
// REL_ENABLE_AWS_S3_UEM feature flags, either of which allows the feature to
// be onboarded. The test skips unless REL_ENABLE_AWS_EC2_UEM is enabled.
func TestAccPolarisAWSAccount_uemKeyManagement(t *testing.T) {
	skipUnlessFeatureEnabled(t, core.FeatureFlagAWSCloudAccountUEM)

	config, account := loadAWSTestConfig(t)
	accountUEM, err := makeTerraformConfig(config, awsAccountUEMKeyManagementTmpl)
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{{
			Config: accountUEM,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.AccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "profile", account.Profile),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_uem_key_management.0.status", "connected"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_uem_key_management.0.regions.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_uem_key_management.0.regions.*", "us-east-2"),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_uem_key_management.0.permission_groups.#", "1"),
				resource.TestCheckTypeSetElemAttr("polaris_aws_account.default", "cloud_native_uem_key_management.0.permission_groups.*", "BASIC"),
			),
		}},
	})
}

// TestAWSCFTFeatureBlockMap verifies that the feature blocks of the
// rubrik_aws_account resource cover exactly the features which can be onboarded
// using the AWS CloudFormation workflow.
func TestAWSCFTFeatureBlockMap(t *testing.T) {
	var got []string
	for _, features := range awsCFTFeatureBlockMap {
		got = append(got, core.FeatureNames(features)...)
	}
	slices.Sort(got)

	if want := core.AWSCloudFormationFeatureNames(); !slices.Equal(got, want) {
		t.Errorf("unexpected features, got: %s want: %s", got, want)
	}
}

// TestAWSCFTFeatureBlockPermissionGroups verifies that the permission_groups
// field of every feature block of the rubrik_aws_account resource has possible
// values, i.e. that the feature name of the block is known by the SDK.
func TestAWSCFTFeatureBlockPermissionGroups(t *testing.T) {
	for key, field := range resourceAwsAccount().Schema {
		block, ok := field.Elem.(*schema.Resource)
		if !ok {
			continue
		}
		groups, ok := block.Schema[keyPermissionGroups]
		if !ok {
			continue
		}
		if strings.Contains(groups.Description, possibleValues([]string{})) {
			t.Errorf("feature block %s has no possible permission groups", key)
		}
	}
}
