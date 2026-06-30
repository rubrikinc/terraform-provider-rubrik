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
	"testing"

	"github.com/google/uuid"
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

	conf, err := loadAWSTestConf()
	if err != nil {
		t.Fatal(err)
	}

	return conf.AccountID
}

// testAWSAccountName returns the AWS account name from the AWS test
// configuration.
func testAWSAccountName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	conf, err := loadAWSTestConf()
	if err != nil {
		t.Fatal(err)
	}

	return conf.AccountName
}

// testAWSProfile returns the AWS profile from the AWS test configuration.
func testAWSProfile(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	conf, err := loadAWSTestConf()
	if err != nil {
		t.Fatal(err)
	}

	return conf.Profile
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
func testCredentials(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	credentials, err := loadTestCredentials("RUBRIK_SERVICEACCOUNT_FILE")
	if err != nil {
		t.Fatal(err)
	}

	return credentials
}

// testUserEmail returns the new user email from the RSC test configuration.
func testUserEmail(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	conf, err := loadRSCTestConf()
	if err != nil {
		t.Fatal(err)
	}

	return conf.NewUserEmail
}

// testSSOGroupName returns the new SSO group name from the RSC test
// configuration. Used by tests that exercise the SSO group resource lifecycle.
// The test is skipped if the value is not set in the RSC test configuration.
func testSSOGroupName(t *testing.T) string {
	t.Helper()
	skipUnlessAcceptanceTest(t)

	conf, err := loadRSCTestConf()
	if err != nil {
		t.Fatal(err)
	}
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

	conf, err := loadRSCTestConf()
	if err != nil {
		t.Fatal(err)
	}
	if conf.AuthDomainID == "" {
		t.Skip("SSO group fixture not available: authDomainId not set")
	}

	return conf.AuthDomainID
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
