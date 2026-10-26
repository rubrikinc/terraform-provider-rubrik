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

import "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"

// RSC feature flags gating workflows used by the provider and its tests.
const (
	// FeatureFlagAWSAppResilience gates AWS application resilience.
	FeatureFlagAWSAppResilience core.FeatureFlagName = "LIC_ENABLE_AWS_APP_RESILIENCE"

	// FeatureFlagAzureDevOpsProtection gates Azure DevOps protection.
	FeatureFlagAzureDevOpsProtection core.FeatureFlagName = "AZURE_DEVOPS_PROTECTION_ENABLED"

	// FeatureFlagAzureSQLSLARevamp gates the revamped Azure SQL SLA domains.
	FeatureFlagAzureSQLSLARevamp core.FeatureFlagName = "CNP_AZURE_SQL_SLA_REVAMP"

	// FeatureFlagDiscoveryOnboarding gates onboarding cloud accounts with the
	// Cloud Discovery feature.
	FeatureFlagDiscoveryOnboarding core.FeatureFlagName = "CNP_DISCOVERY_ONBOARDING_ENABLED"

	// FeatureFlagGCPPrivateContainerRegistry gates the private container
	// registry for GCP Exocompute.
	FeatureFlagGCPPrivateContainerRegistry core.FeatureFlagName = "REL_ENABLE_GCP_PRIVATE_CONTAINER_REGISTRY"

	// FeatureFlagGCPSQL gates the GCP Cloud SQL protection feature.
	FeatureFlagGCPSQL core.FeatureFlagName = "CNP_GCP_SQL_ENABLED"

	// FeatureFlagUnifiedEncryptionManagement gates unified encryption
	// management, i.e. the KMS key managers.
	FeatureFlagUnifiedEncryptionManagement core.FeatureFlagName = "UNIFIED_ENCRYPTION_MANAGEMENT_ENABLED"
)
