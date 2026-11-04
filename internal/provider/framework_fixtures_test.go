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
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/access"
	gqlaccess "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/access"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/hierarchy"
)

// testAWSAccountID returns the AWS account ID from the AWS test configuration.
func testAWSAccountID(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAWSTestConfig(t)
	return conf.AccountID
}

// testAWSAccountName returns the AWS account name from the AWS test
// configuration.
func testAWSAccountName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAWSTestConfig(t)
	return conf.AccountName
}

// testAWSProfile returns the AWS profile from the AWS test configuration.
func testAWSProfile(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAWSTestConfig(t)
	return conf.Profile
}

// testAzureCredentials returns the Azure service principal credentials from the
// Azure test configuration.
func testAzureCredentials(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	return conf.Credentials
}

// testAzureTenantDomain returns the Azure tenant domain from the Azure test
// configuration.
func testAzureTenantDomain(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	return conf.TenantDomain
}

// testAzureSubscriptionID returns the Azure subscription ID from the Azure test
// configuration.
func testAzureSubscriptionID(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	return conf.SubscriptionID
}

// testAzureSubscriptionName returns the Azure subscription name from the Azure
// test configuration.
func testAzureSubscriptionName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	return conf.SubscriptionName
}

// testAzureResourceGroupName returns the cloud native protection resource group
// name from the Azure test configuration.
func testAzureResourceGroupName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	return conf.CloudNativeProtection.ResourceGroupName
}

// testAzureResourceGroupRegion returns the cloud native protection resource
// group region from the Azure test configuration.
func testAzureResourceGroupRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	return conf.CloudNativeProtection.ResourceGroupRegion
}

// testGCPCredentials returns the GCP service account credentials from the GCP
// test configuration.
func testGCPCredentials(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	return conf.Credentials
}

// testGCPProjectID returns the GCP project ID from the GCP test configuration.
func testGCPProjectID(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	return conf.ProjectID
}

// testGCPProjectName returns the GCP project name from the GCP test
// configuration.
func testGCPProjectName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	return conf.ProjectName
}

// testGCPProjectNumber returns the GCP project number from the GCP test
// configuration. The project number is a string in the schema, guarded by a
// numeric validator.
func testGCPProjectNumber(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	return strconv.FormatInt(conf.ProjectNumber, 10)
}

// testGCPOrganizationName returns the GCP organization name from the GCP test
// configuration.
func testGCPOrganizationName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	return conf.OrganizationName
}

// testUniqueName returns a unique name for a test resource, suffixed with a
// random UUID so repeated or concurrent runs do not collide.
func testUniqueName(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf("Test Name %s", id)
}

// testUniqueTagKey returns a unique custom tag or custom label key, suffixed
// with a random UUID so repeated or concurrent runs do not collide.
func testUniqueTagKey(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf("terraform-acc-%s", id)
}

// testClient returns an RSC client for testing outside the Terraform provider.
// E.g. checking if resources have been destroyed in a check destroy function.
func testClient(t *testing.T) *polaris.Client {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	c, err := newClient(context.Background(), testCredentials(t), polaris.CacheParams{})
	if err != nil {
		t.Fatalf("failed to create test client: %s", err)
	}

	pc, err := c.polaris()
	if err != nil {
		t.Fatalf("failed to create test client: %s", err)
	}

	return pc
}

// testCredentials returns the RSC credentials from the environment.
//
// Note, this function returns the actual credentials and not just the path to
// the credentials file.
func testCredentials(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	credentials, err := loadTestCredentials(rscCredentialsEnv)
	if err != nil {
		t.Fatal(err)
	}

	return credentials
}

// testUserEmail returns the new user email from the RSC test configuration.
func testUserEmail(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadRSCTestConfig(t)
	return conf.NewUserEmail
}

// testSSOGroupName returns the new SSO group name from the RSC test
// configuration. Used by tests that exercise the SSO group resource lifecycle.
// The test is skipped if the value is not set in the RSC test configuration.
func testSSOGroupName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadRSCTestConfig(t)
	if conf.NewSSOGroupName == "" {
		t.Skip("SSO group fixture not available: newSsoGroupName not set")
	}

	return conf.NewSSOGroupName
}

// testAuthDomainID returns the RSC-side auth domain ID from the RSC test
// configuration. The test is skipped if the value is not set.
func testAuthDomainID(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadRSCTestConfig(t)
	if conf.AuthDomainID == "" {
		t.Skip("SSO group fixture not available: authDomainId not set")
	}

	return conf.AuthDomainID
}

// testAWSRCVRegion returns the RCV region from the AWS test configuration. The
// test is skipped if the value is not set.
func testAWSRCVRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAWSTestConfig(t)
	if conf.RCV.Region == "" {
		t.Skip("AWS RCV fixture not available: rcv.region not set")
	}

	return conf.RCV.Region
}

// testAWSArchivalRegion returns the archival region from the AWS test
// configuration. The test is skipped if the value is not set.
func testAWSArchivalRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAWSTestConfig(t)
	if conf.Archival.Region == "" {
		t.Skip("AWS archival fixture not available: archival.region not set")
	}

	return conf.Archival.Region
}

// testAWSKMSRegion returns the KMS region from the AWS test configuration. The
// test is skipped if the value is not set.
func testAWSKMSRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAWSTestConfig(t)
	if conf.KMS.Region == "" {
		t.Skip("AWS KMS fixture not available: kms.region not set")
	}

	return conf.KMS.Region
}

// testAWSKMSKeyIDs returns the KMS key IDs from the AWS test configuration.
// The test is skipped if fewer than two keys are set.
func testAWSKMSKeyIDs(t *testing.T) []string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAWSTestConfig(t)
	if len(conf.KMS.KeyIDs) < 2 {
		t.Skip("AWS KMS fixture not available: kms.keyIds must hold two keys")
	}

	return conf.KMS.KeyIDs
}

// testAzureRCVRegion returns the RCV region from the Azure test configuration.
// The test is skipped if the value is not set.
func testAzureRCVRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	if conf.RCV.Region == "" {
		t.Skip("Azure RCV fixture not available: rcv.region not set")
	}

	return conf.RCV.Region
}

// testAzureArchivalRegion returns the first archival region from the Azure test
// configuration. The test is skipped if no region is set.
func testAzureArchivalRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	if len(conf.Archival.Regions) == 0 {
		t.Skip("Azure archival fixture not available: archival.regions not set")
	}

	return conf.Archival.Regions[0]
}

// testAzureArchivalResourceGroupName returns the archival resource group name
// from the Azure test configuration. The test is skipped if the value is not
// set.
func testAzureArchivalResourceGroupName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	if conf.Archival.ResourceGroupName == "" {
		t.Skip("Azure archival fixture not available: archival.resourceGroupName not set")
	}

	return conf.Archival.ResourceGroupName
}

// testAzureArchivalResourceGroupRegion returns the archival resource group
// region from the Azure test configuration. The test is skipped if the value
// is not set.
func testAzureArchivalResourceGroupRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	if conf.Archival.ResourceGroupRegion == "" {
		t.Skip("Azure archival fixture not available: archival.resourceGroupRegion not set")
	}

	return conf.Archival.ResourceGroupRegion
}

// testAzureKMSRegion returns the KMS region from the Azure test configuration.
// The test is skipped if the value is not set.
func testAzureKMSRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	if conf.KMS.Region == "" {
		t.Skip("Azure KMS fixture not available: kms.region not set")
	}

	return conf.KMS.Region
}

// testAzureKMSVaultName returns the key vault name from the Azure test
// configuration. The test is skipped if the value is not set.
func testAzureKMSVaultName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	if conf.KMS.VaultName == "" {
		t.Skip("Azure KMS fixture not available: kms.vaultName not set")
	}

	return conf.KMS.VaultName
}

// testAzureKMSKeys returns the key vault keys from the Azure test
// configuration. The test is skipped if fewer than two keys are set.
func testAzureKMSKeys(t *testing.T) []testKMSKey {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadAzureTestConfig(t)
	if len(conf.KMS.Keys) < 2 {
		t.Skip("Azure KMS fixture not available: kms.keys must hold two keys")
	}

	return conf.KMS.Keys
}

// testGCPRCVRegion returns the RCV region from the GCP test configuration. The
// test is skipped if the value is not set.
func testGCPRCVRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	if conf.RCV.Region == "" {
		t.Skip("GCP RCV fixture not available: rcv.region not set")
	}

	return conf.RCV.Region
}

// testGCPArchivalRegion returns the archival region from the GCP test
// configuration. The test is skipped if the value is not set.
func testGCPArchivalRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	if conf.Archival.Region == "" {
		t.Skip("GCP archival fixture not available: archival.region not set")
	}

	return conf.Archival.Region
}

// testGCPKMSRegion returns the KMS region from the GCP test configuration. The
// test is skipped if the value is not set.
func testGCPKMSRegion(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	if conf.KMS.Region == "" {
		t.Skip("GCP KMS fixture not available: kms.region not set")
	}

	return conf.KMS.Region
}

// testGCPKMSKeyRing returns the key ring from the GCP test configuration. The
// test is skipped if the value is not set.
func testGCPKMSKeyRing(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	if conf.KMS.KeyRing == "" {
		t.Skip("GCP KMS fixture not available: kms.keyRing not set")
	}

	return conf.KMS.KeyRing
}

// testGCPKMSKeys returns the Cloud KMS keys from the GCP test configuration.
// The test is skipped if fewer than two keys are set.
func testGCPKMSKeys(t *testing.T) []testKMSKey {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	_, conf := loadGCPTestConfig(t)
	if len(conf.KMS.Keys) < 2 {
		t.Skip("GCP KMS fixture not available: kms.keys must hold two keys")
	}

	return conf.KMS.Keys
}

// createTestRole creates a custom role via the SDK and registers a cleanup
// function to delete it. The role will have the VIEW_CLUSTER and
// VIEW_CLUSTER_REFERENCE permissions on the CLUSTER_ROOT. Returns the role ID.
func createTestRole(t *testing.T, name string) uuid.UUID {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	polarisClient := testClient(t)

	desc := "Test Role: Delete Me!"
	roleID, err := access.Wrap(polarisClient).CreateRole(t.Context(), name, desc, []gqlaccess.Permission{{
		Operation: string(gqlaccess.OperationViewCluster),
		ObjectsForHierarchyTypes: []gqlaccess.ObjectsForHierarchyType{{
			SnappableType: "AllSubHierarchyType",
			ObjectIDs:     []string{hierarchy.ClusterRoot},
		}},
	}, {
		// RSC adds VIEW_CLUSTER_REFERENCE automatically with VIEW_CLUSTER, so
		// grant it explicitly to keep the role free of drift.
		Operation: string(gqlaccess.OperationViewClusterReference),
		ObjectsForHierarchyTypes: []gqlaccess.ObjectsForHierarchyType{{
			SnappableType: "AllSubHierarchyType",
			ObjectIDs:     []string{hierarchy.ClusterRoot},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := access.Wrap(polarisClient).DeleteRole(context.Background(), roleID); err != nil {
			t.Fatalf("failed to delete test role %q: %s", roleID, err)
		}
	})

	return roleID
}

// createTestRoleWithUniqueName creates a custom role with a unqiue name via
// the SDK and registers a cleanup function to delete it. The role will have
// the VIEW_CLUSTER and VIEW_CLUSTER_REFERENCE permissions on the CLUSTER_ROOT.
// Returns the role ID.
func createTestRoleWithUniqueName(t *testing.T) uuid.UUID {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}

	return createTestRole(t, fmt.Sprintf("Test Role %s", id.String()))
}

// createTestUser creates a user with the specified email and role via the SDK.
// Registers a cleanup function to delete the user. Returns the user ID.
func createTestUser(t *testing.T, email string, roleID uuid.UUID) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	polarisClient := testClient(t)

	userID, err := access.Wrap(polarisClient).CreateUser(t.Context(), email, []uuid.UUID{roleID})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := access.Wrap(polarisClient).DeleteUser(context.Background(), userID); err != nil {
			t.Fatalf("failed to delete test user %q: %s", userID, err)
		}
	})

	return userID
}

// customTagsImportID returns the import ID of a custom tags resource scoped to
// a cloud account, which is the cloud account ID.
func customTagsImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}

		return rs.Primary.Attributes[keyCloudAccountID], nil
	}
}

// skipUnlessAcceptanceTest skips the test if the TF_ACC environment variable is
// not set.
func skipUnlessAcceptanceTest(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}
}

// skipUnlessSSOGroupDefined skips the test if fixtures required for SSO group
// tests are not set.
func skipUnlessSSOGroupDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testAuthDomainID(t)
	testSSOGroupName(t)
}

// skipUnlessAWSRCVDefined skips the test if fixtures required for
// AWS RCV tests, the rcv section of TEST_AWSACCOUNT_FILE, are not set.
func skipUnlessAWSRCVDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testAWSRCVRegion(t)
}

// skipUnlessAWSArchivalDefined skips the test if fixtures required for
// AWS Archival tests, the archival section of TEST_AWSACCOUNT_FILE, are not set.
func skipUnlessAWSArchivalDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testAWSArchivalRegion(t)
}

// skipUnlessAWSKMSDefined skips the test if fixtures required for
// AWS KMS tests, the kms section of TEST_AWSACCOUNT_FILE, are not set.
func skipUnlessAWSKMSDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testAWSKMSRegion(t)
	testAWSKMSKeyIDs(t)
}

// skipUnlessAzureRCVDefined skips the test if fixtures required for
// Azure RCV tests, the rcv section of TEST_AZURESUBSCRIPTION_FILE, are not set.
func skipUnlessAzureRCVDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testAzureRCVRegion(t)
}

// skipUnlessAzureArchivalDefined skips the test if fixtures required for
// Azure Archival tests, the archival section of TEST_AZURESUBSCRIPTION_FILE, are not set.
func skipUnlessAzureArchivalDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testAzureArchivalRegion(t)
	testAzureArchivalResourceGroupName(t)
	testAzureArchivalResourceGroupRegion(t)
}

// skipUnlessAzureKMSDefined skips the test if fixtures required for
// Azure KMS tests, the kms section of TEST_AZURESUBSCRIPTION_FILE, are not set.
func skipUnlessAzureKMSDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testAzureKMSRegion(t)
	testAzureKMSVaultName(t)
	testAzureKMSKeys(t)
}

// skipUnlessGCPRCVDefined skips the test if fixtures required for
// GCP RCV tests, the rcv section of TEST_GCPPROJECT_FILE, are not set.
func skipUnlessGCPRCVDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testGCPRCVRegion(t)
}

// skipUnlessGCPArchivalDefined skips the test if fixtures required for
// GCP Archival tests, the archival section of TEST_GCPPROJECT_FILE, are not set.
func skipUnlessGCPArchivalDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testGCPArchivalRegion(t)
}

// skipUnlessGCPKMSDefined skips the test if fixtures required for
// GCP KMS tests, the kms section of TEST_GCPPROJECT_FILE, are not set.
func skipUnlessGCPKMSDefined(t *testing.T) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	testGCPKMSRegion(t)
	testGCPKMSKeyRing(t)
	testGCPKMSKeys(t)
}

// skipUnlessFeatureEnabled skips the test if the specified feature is not
// enabled.
func skipUnlessFeatureEnabled(t *testing.T, featureFlag core.FeatureFlagName) {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	ctx := context.Background()
	c, err := newClient(ctx, testCredentials(t), polaris.CacheParams{})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if !c.flag(ctx, featureFlag) {
		t.Skipf("Feature flag %q not enabled", featureFlag)
	}
}
