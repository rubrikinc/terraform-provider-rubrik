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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const objectAWSAccountTmpl = `
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

data "polaris_object" "aws_account" {
	name        = "{{ .Resource.AccountName }}"
	object_type = "AwsNativeAccount"

	depends_on = [polaris_aws_account.default]
}
`

func TestAccPolarisAwsAccountObject(t *testing.T) {
	config, account := loadAWSTestConfig(t)
	objectAWSAccount, err := makeTerraformConfig(config, objectAWSAccountTmpl)
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: objectAWSAccount,
			// Verify the AWS account resource was created.
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_aws_account.default", "name", account.AccountName),
				resource.TestCheckResourceAttr("polaris_aws_account.default", "cloud_native_protection.0.status", "connected"),
			),
			// Verify the object data source returns the correct values.
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.polaris_object.aws_account", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_object.aws_account", tfjsonpath.New(keyName),
					knownvalue.StringExact(account.AccountName)),
				statecheck.ExpectKnownValue("data.polaris_object.aws_account", tfjsonpath.New(keyObjectType),
					knownvalue.StringExact("AwsNativeAccount")),
			},
		}},
	})
}

const objectAzureSubscriptionTmpl = `
provider "polaris" {
	credentials = "{{ .Provider.Credentials }}"
}

resource "polaris_azure_service_principal" "default" {
	credentials   = "{{ .Resource.Credentials }}"
	tenant_domain = "{{ .Resource.TenantDomain }}"
}

resource "polaris_azure_subscription" "default" {
	subscription_id   = "{{ .Resource.SubscriptionID }}"
	subscription_name = "{{ .Resource.SubscriptionName }}"
	tenant_domain     = "{{ .Resource.TenantDomain }}"

	cloud_native_protection {
		resource_group_name   = "{{ .Resource.CloudNativeProtection.ResourceGroupName }}"
		resource_group_region = "{{ .Resource.CloudNativeProtection.ResourceGroupRegion }}"

		regions = [
			"eastus2",
		]
	}
  
	depends_on = [polaris_azure_service_principal.default]
}

data "polaris_object" "azure_subscription" {
	name        = "{{ .Resource.SubscriptionName }}"
	object_type = "AzureNativeSubscription"

	depends_on = [polaris_azure_subscription.default]
}
`

func TestAccPolarisAzureSubscriptionObject(t *testing.T) {
	config, subscription := loadAzureTestConfig(t)
	objectAzureSubscription, err := makeTerraformConfig(config, objectAzureSubscriptionTmpl)
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: objectAzureSubscription,
			// Verify the Azure subscription resource was created.
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("polaris_azure_subscription.default", "subscription_name", subscription.SubscriptionName),
				resource.TestCheckResourceAttr("polaris_azure_subscription.default", "cloud_native_protection.0.status", "CONNECTED"),
			),
			// Verify the object data source returns the correct values.
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.polaris_object.azure_subscription", tfjsonpath.New(keyID),
					knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.polaris_object.azure_subscription", tfjsonpath.New(keyName),
					knownvalue.StringExact(subscription.SubscriptionName)),
				statecheck.ExpectKnownValue("data.polaris_object.azure_subscription", tfjsonpath.New(keyObjectType),
					knownvalue.StringExact("AzureNativeSubscription")),
			},
		}},
	})
}
