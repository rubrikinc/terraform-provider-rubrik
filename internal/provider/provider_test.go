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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var sdkProvider *schema.Provider = Provider()

var providerFactories = map[string]func() (*schema.Provider, error){
	"polaris": func() (*schema.Provider, error) {
		return sdkProvider, nil
	},
}

// loadTestCredentials returns the content of the file pointed to by the
// credentialsEnv parameter.
func loadTestCredentials(credentialsEnv string) (string, error) {
	credentials := os.Getenv(credentialsEnv)
	if credentials == "" {
		return "", fmt.Errorf("%s is empty", credentialsEnv)
	}

	buf, err := os.ReadFile(credentials)
	if err != nil {
		return "", fmt.Errorf("failed to read file pointed to by %s: %v", credentialsEnv, err)
	}

	return string(buf), nil
}

// testConfig holds the configuration for a test, i.e. the actual values to
// give to a Terraform template.
type testConfig struct {
	Provider struct {
		Credentials string
	}
	Resource            any
	Timestamp           string
	DiscoveryOnboarding bool
}

// loadTestConfig returns a new testConfig initialized from the file pointed
// to by the environmental variable in resourceFileEnv. Note that it must be
// possible to unmarshal the file to the resource type and that resource must
// be of pointer type.
func loadTestConfig(t *testing.T, credentialsEnv, resourceFileEnv string, resource any) testConfig {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	credentials := os.Getenv(credentialsEnv)
	if credentials == "" {
		t.Fatalf("%s is empty", credentialsEnv)
	}

	buf, err := os.ReadFile(os.Getenv(resourceFileEnv))
	if err != nil {
		t.Fatalf("failed to read file %s: %s", resourceFileEnv, err)
	}

	if err := json.Unmarshal(buf, resource); err != nil {
		t.Fatalf("failed to unmarshal JSON from %s: %s", resourceFileEnv, err)
	}

	config := testConfig{
		Provider: struct{ Credentials string }{
			Credentials: credentials,
		},
		Resource: resource,
	}

	return config
}

// makeTerraformConfig returns a Terraform configuration given a test
// configuration and a Terraform template.
func makeTerraformConfig(config testConfig, terraformTemplate string) (string, error) {
	tmpl, err := template.New("resource").Parse(terraformTemplate)
	if err != nil {
		return "", err
	}

	out := &strings.Builder{}
	if err := tmpl.Execute(out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

// testAWSAccount holds information about an AWS account used in one or more
// acceptance tests.
type testAWSAccount struct {
	Profile          string `json:"profile"`
	AccountID        string `json:"accountId"`
	AccountName      string `json:"accountName"`
	CrossAccountID   string `json:"crossAccountId"`
	CrossAccountName string `json:"crossAccountName"`
	CrossAccountRole string `json:"crossAccountRole"`

	Exocompute struct {
		VPCID   string `json:"vpcId"`
		Subnets []struct {
			ID               string `json:"id"`
			AvailabilityZone string `json:"availabilityZone"`
		} `json:"subnets"`
	} `json:"exocompute"`
}

// loadAWSTestConfig loads an AWS test configuration using the default
// environment variables.
func loadAWSTestConfig(t *testing.T) (testConfig, testAWSAccount) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	account := testAWSAccount{}
	config := loadTestConfig(t, rscCredentialsEnv, awsAccountFileEnv, &account)

	// Note that this will update both project and config.
	if account.Profile == "" {
		account.Profile = "default"
	}

	return config, account
}

// testAzureSubscription holds information about an Azure subscription used in
// one or more acceptance tests.
type testAzureSubscription struct {
	Credentials      string `json:"credentials"`
	SubscriptionID   string `json:"subscriptionId"`
	SubscriptionName string `json:"subscriptionName"`
	TenantID         string `json:"tenantId"`
	TenantDomain     string `json:"tenantDomain"`
	PrincipalID      string `json:"principalId"`
	PrincipalName    string `json:"principalName"`
	PrincipalSecret  string `json:"principalSecret"`

	CloudNativeProtection struct {
		Regions             []string `json:"regions"`
		ResourceGroupName   string   `json:"resourceGroupName"`
		ResourceGroupRegion string   `json:"resourceGroupRegion"`
	} `json:"cloudNativeProtection"`

	Exocompute struct {
		Regions             []string `json:"regions"`
		ResourceGroupName   string   `json:"resourceGroupName"`
		ResourceGroupRegion string   `json:"resourceGroupRegion"`
		SubnetID            string   `json:"subnetId"`
	} `json:"exocompute"`

	VMName string `json:"vmName"`
}

// loadAzureTestConfig loads an Azure test configuration using the default
// environment variables.
func loadAzureTestConfig(t *testing.T) (testConfig, testAzureSubscription) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	subscription := testAzureSubscription{}
	config := loadTestConfig(t, rscCredentialsEnv, azureSubscriptionFileEnv, &subscription)

	if subscription.Credentials == "" {
		subscription.Credentials = os.Getenv(azureCredentialsEnv)
	}

	return config, subscription
}

// testGCPProject holds information about a GCP project used in one or more
// acceptance tests.
type testGCPProject struct {
	Credentials      string `json:"credentials"`
	ProjectID        string `json:"projectId"`
	ProjectName      string `json:"projectName"`
	ProjectNumber    int64  `json:"projectNumber"`
	OrganizationName string `json:"organizationName"`
	Exocompute       struct {
		Region     string `json:"region"`
		SubnetName string `json:"subnetName"`
		VPCName    string `json:"vpcNetworkName"`
	} `json:"exocompute"`
}

// loadGCPTestConfig loads a GCP test configuration using the default
// environment variables.
func loadGCPTestConfig(t *testing.T) (testConfig, testGCPProject) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	project := testGCPProject{}
	config := loadTestConfig(t, rscCredentialsEnv, gcpProjectFileEnv, &project)

	// Note that this will update both project and config.
	if project.Credentials == "" {
		project.Credentials = os.Getenv(gcpCredentialsEnv)
	}

	return config, project
}

// testRSCConfig holds RSC configuration information used in one or more
// acceptance tests.
type testRSCConfig struct {
	NewUserEmail string `json:"newUserEmail"`

	// Optional for acceptance tests, if omitted the tests are skipped.
	AuthDomainID    string `json:"authDomainId"`
	NewSSOGroupName string `json:"newSsoGroupName"`
}

// loadRSCTestConfig loads an RSC test configuration using the default
// environment variables.
func loadRSCTestConfig(t *testing.T) (testConfig, testRSCConfig) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	rsc := testRSCConfig{}
	conf := loadTestConfig(t, rscCredentialsEnv, rscConfigFileEnv, &rsc)

	return conf, rsc
}
