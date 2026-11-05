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

	// RCV is optional. Region is the AWS region for RCV archival locations,
	// e.g. us-east-2. RCV must be entitled for the RSC account.
	RCV struct {
		Region string `json:"region"`
	} `json:"rcv"`

	// Archival is optional. Region is the AWS region for cloud native archival
	// locations.
	Archival struct {
		Region string `json:"region"`
	} `json:"archival"`

	// KMS is optional. KeyIDs are the IDs of two pre-existing KMS keys in
	// Region. The keys must use the default key policy.
	KMS struct {
		Region string   `json:"region"`
		KeyIDs []string `json:"keyIds"`
	} `json:"kms"`
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

// testKMSKey holds a pre-existing key, and its version, in an Azure key vault
// or a GCP key ring.
type testKMSKey struct {
	Name    string `json:"name"`
	Version string `json:"version"`
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

	// PostgresFlexibleServer is optional. ManagedIdentityName and
	// ManagedIdentityPrincipalID identify a pre-existing user-assigned managed
	// identity in the resource group, located in ManagedIdentityRegion.
	PostgresFlexibleServer struct {
		Regions                    []string `json:"regions"`
		ResourceGroupName          string   `json:"resourceGroupName"`
		ResourceGroupRegion        string   `json:"resourceGroupRegion"`
		ManagedIdentityName        string   `json:"managedIdentityName"`
		ManagedIdentityPrincipalID string   `json:"managedIdentityPrincipalId"`
		ManagedIdentityRegion      string   `json:"managedIdentityRegion"`
	} `json:"postgresFlexibleServer"`

	// RCV is optional. Region is the Azure region for RCV archival locations,
	// e.g. eastus2. RCV must be entitled for the RSC account.
	RCV struct {
		Region string `json:"region"`
	} `json:"rcv"`

	// Archival is optional. Regions are the Azure regions for the Cloud Native
	// Archival feature. ResourceGroupName and ResourceGroupRegion are the
	// resource group for the feature. The section uses the same format as the
	// Go SDK test configuration.
	Archival struct {
		Regions             []string `json:"regions"`
		ResourceGroupName   string   `json:"resourceGroupName"`
		ResourceGroupRegion string   `json:"resourceGroupRegion"`
	} `json:"archival"`

	// KMS is optional. Keys are two pre-existing keys, or two versions of one
	// key, in the pre-existing key vault VaultName in Region. The key vault
	// must use Azure RBAC, and the service principal must be assigned the Key
	// Vault Crypto User role on it.
	KMS struct {
		Region    string       `json:"region"`
		VaultName string       `json:"vaultName"`
		Keys      []testKMSKey `json:"keys"`
	} `json:"kms"`

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

	// Optional for acceptance tests, if omitted the private container registry
	// tests are skipped.
	PCR struct {
		RegistryURL string `json:"registryUrl"`
	} `json:"pcr"`

	// RCV is optional. Region is the GCP region for RCV archival locations,
	// e.g. us-west1. RCV must be entitled for the RSC account.
	RCV struct {
		Region string `json:"region"`
	} `json:"rcv"`

	// Archival is optional. Region is the GCP region for cloud native archival
	// locations.
	Archival struct {
		Region string `json:"region"`
	} `json:"archival"`

	// KMS is optional. Keys are two pre-existing keys, or two versions of one
	// key, in the key ring KeyRing in Region. The key ring must be pre-existing,
	// since key rings can't be deleted in GCP.
	KMS struct {
		Region  string       `json:"region"`
		KeyRing string       `json:"keyRing"`
		Keys    []testKMSKey `json:"keys"`
	} `json:"kms"`
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

	// Optional for acceptance tests, if omitted the tests are skipped. RSC
	// cloud account IDs of two AWS accounts onboarded with the
	// CLOUD_NATIVE_CONFIG_PROTECTION feature, including the RECOVERY
	// permission groups, in a connected state and in the same region. An
	// application can only be created in a connected account, so these must
	// refer to pre-existing cloud accounts.
	CloudApplicationAccountID       string `json:"cloudApplicationAccountId"`
	CloudApplicationTargetAccountID string `json:"cloudApplicationTargetAccountId"`
	CloudApplicationRegion          string `json:"cloudApplicationRegion"`
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
