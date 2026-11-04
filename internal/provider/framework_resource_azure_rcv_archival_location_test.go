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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAzureRCVArchivalLocationResource creates, updates and imports an RCV
// Azure archival location encrypted with a key RCV creates and manages.
func TestAccAzureRCVArchivalLocationResource(t *testing.T) {
	skipUnlessAzureRCVDefined(t)

	name := testUniqueName(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureRCVArchivalLocationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "tier" {
					type = string
				}
				variable "redundancy" {
					type = string
				}
				resource "rubrik_azure_rcv_archival_location" "location" {
					name       = var.name
					region     = var.region
					tier       = var.tier
					redundancy = var.redundancy
				}
			`,
			ConfigVariables: config.Variables{
				"name":       config.StringVariable(name),
				"region":     config.StringVariable(testAzureRCVRegion(t)),
				"tier":       config.StringVariable("BACKUP"),
				"redundancy": config.StringVariable("SINGLE_ZONE"),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyRegion),
					knownvalue.StringExact(testAzureRCVRegion(t))),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyTier),
					knownvalue.StringExact("BACKUP")),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyRedundancy), knownvalue.StringExact("SINGLE_ZONE")),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyLockDurationDays), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey), knownvalue.ListSizeExact(0)),
				statecheck.ExpectIdentity("rubrik_azure_rcv_archival_location.location", map[string]knownvalue.Check{
					keyID: NonNullUUID(),
				}),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Verify that the name is updated in place. The tier and the
			// redundancy are left unchanged, RSC converts them asynchronously.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "tier" {
					type = string
				}
				variable "redundancy" {
					type = string
				}
				resource "rubrik_azure_rcv_archival_location" "location" {
					name       = var.name
					region     = var.region
					tier       = var.tier
					redundancy = var.redundancy
				}
			`,
			ConfigVariables: config.Variables{
				"name":       config.StringVariable(name + " Update"),
				"region":     config.StringVariable(testAzureRCVRegion(t)),
				"tier":       config.StringVariable("BACKUP"),
				"redundancy": config.StringVariable("SINGLE_ZONE"),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_rcv_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyName),
					knownvalue.StringExact(name+" Update")),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyTier),
					knownvalue.StringExact("BACKUP")),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyRedundancy), knownvalue.StringExact("SINGLE_ZONE")),
				statecheck.ExpectIdentityValueMatchesState("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyID)),
			},
		}, {
			// Terraform import.
			ResourceName: "rubrik_azure_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":       config.StringVariable(name + " Update"),
				"region":     config.StringVariable(testAzureRCVRegion(t)),
				"tier":       config.StringVariable("BACKUP"),
				"redundancy": config.StringVariable("SINGLE_ZONE"),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_azure_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":       config.StringVariable(name + " Update"),
				"region":     config.StringVariable(testAzureRCVRegion(t)),
				"tier":       config.StringVariable("BACKUP"),
				"redundancy": config.StringVariable("SINGLE_ZONE"),
			},
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName: "rubrik_azure_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"name":       config.StringVariable(name + " Update"),
				"region":     config.StringVariable(testAzureRCVRegion(t)),
				"tier":       config.StringVariable("BACKUP"),
				"redundancy": config.StringVariable("SINGLE_ZONE"),
			},
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}

// TestAccAzureRCVArchivalLocationRSAKeyResource creates an RCV Azure archival
// location encrypted with an RSA key, generated by the test, and verifies that
// changing the key version replaces the archival location.
func TestAccAzureRCVArchivalLocationRSAKeyResource(t *testing.T) {
	skipUnlessAzureRCVDefined(t)

	name := testUniqueName(t)

	key1, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key1InPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key1),
	})
	key2, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key2InPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key2),
	})

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy:             azureRCVArchivalLocationCheckDestroy(t),
		Steps: []resource.TestStep{{
			// Verify that the resource can be created with an RSA key, and that
			// the write-only key never reaches the state.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "key" {
					type      = string
					sensitive = true
				}
				variable "key_version" {
					type = string
				}
				resource "rubrik_azure_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
		
					rsa_key {
						key     = var.key
						version = var.key_version
					}
				}
			`,
			ConfigVariables: config.Variables{
				"name":        config.StringVariable(name),
				"region":      config.StringVariable(testAzureRCVRegion(t)),
				"key":         config.StringVariable(string(key1InPEM)),
				"key_version": config.StringVariable("1"),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyEncryptionType), knownvalue.StringExact("RSA_KEY_BASED")),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyRSAKey).AtSliceIndex(0).AtMapKey(keyKey), knownvalue.Null()),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyRSAKey).AtSliceIndex(0).AtMapKey(keyVersion), knownvalue.StringExact("1")),
			},
		}, {
			// Verify that a new key version replaces the archival location,
			// since RSC only accepts the key when the location is created.
			Config: `
				variable "name" {
					type = string
				}
				variable "region" {
					type = string
				}
				variable "key" {
					type      = string
					sensitive = true
				}
				variable "key_version" {
					type = string
				}
				resource "rubrik_azure_rcv_archival_location" "location" {
					name   = var.name
					region = var.region
		
					rsa_key {
						key     = var.key
						version = var.key_version
					}
				}
			`,
			ConfigVariables: config.Variables{
				"name":        config.StringVariable(name),
				"region":      config.StringVariable(testAzureRCVRegion(t)),
				"key":         config.StringVariable(string(key2InPEM)),
				"key_version": config.StringVariable("2"),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_rcv_archival_location.location",
						plancheck.ResourceActionReplace),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyRSAKey).AtSliceIndex(0).AtMapKey(keyVersion), knownvalue.StringExact("2")),
			},
		}},
	})
}

// TestAccAzureRCVArchivalLocationKMSKeyResource onboards the Azure subscription
// with the UEM key management feature, registers a key manager, creates an
// RCV Azure archival location encrypted with the first key and rekeys it to
// the second key.
func TestAccAzureRCVArchivalLocationKMSKeyResource(t *testing.T) {
	skipUnlessAzureRCVDefined(t)
	skipUnlessAzureKMSDefined(t)
	skipUnlessFeatureEnabled(t, FeatureFlagUnifiedEncryptionManagement)

	name := testUniqueName(t)

	const conf = `
		variable "credentials" {
			type      = string
			sensitive = true
		}
		variable "tenant_domain" {
			type = string
		}
		variable "subscription_id" {
			type = string
		}
		variable "subscription_name" {
			type = string
		}
		variable "name" {
			type = string
		}
		variable "kms_region" {
			type = string
		}
		variable "vault_name" {
			type = string
		}
		variable "rcv_region" {
			type = string
		}
		variable "key_name" {
			type = string
		}
		variable "key_version" {
			type = string
		}

		resource "rubrik_azure_service_principal" "service_principal" {
			credentials   = var.credentials
			tenant_domain = var.tenant_domain
		}

		resource "rubrik_azure_subscription" "subscription" {
			subscription_id   = var.subscription_id
			subscription_name = var.subscription_name
			tenant_domain     = var.tenant_domain

			cloud_native_uem_key_management {
				permission_groups = ["BASIC"]
			}

			depends_on = [rubrik_azure_service_principal.service_principal]
		}

		resource "rubrik_azure_kms_key_manager" "key_manager" {
			name             = var.name
			cloud_account_id = rubrik_azure_subscription.subscription.id
			region           = var.kms_region
			vault_name       = var.vault_name
		}

		resource "rubrik_azure_rcv_archival_location" "location" {
			name   = var.name
			region = var.rcv_region

			key_vault_key {
				key_manager_id = rubrik_azure_kms_key_manager.key_manager.id
				key_name       = var.key_name
				key_version    = var.key_version
			}
		}
	`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			azureRCVArchivalLocationCheckDestroy(t),
			azureKMSKeyManagerCheckDestroy(t),
			azureSubscriptionCheckDestroy(t),
		),
		Steps: []resource.TestStep{{
			// Verify that the subscription can be onboarded with the UEM key
			// management feature, that a key manager can be registered, and
			// that an archival location can be created with the first key.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"name":              config.StringVariable(name),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"rcv_region":        config.StringVariable(testAzureRCVRegion(t)),
				"key_name":          config.StringVariable(testAzureKMSKeys(t)[0].Name),
				"key_version":       config.StringVariable(testAzureKMSKeys(t)[0].Version),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_subscription.subscription",
					tfjsonpath.New(keyCloudNativeUEMKeyManagement).AtSliceIndex(0).AtMapKey(keyStatus),
					knownvalue.StringExact("CONNECTED")),
				statecheck.CompareValuePairs(
					"rubrik_azure_subscription.subscription", tfjsonpath.New(keyID),
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyCloudAccountID),
					compare.ValuesSame()),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location", tfjsonpath.New(keyID),
					NonNullUUID()),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testAzureKMSKeys(t)[0].Name)),
				statecheck.CompareValuePairs(
					"rubrik_azure_kms_key_manager.key_manager", tfjsonpath.New(keyID),
					"rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyManagerID),
					compare.ValuesSame()),
			},
		}, {
			// Verify that changing the key rekeys the archival location in
			// place, keeping its ID.
			Config: conf,
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"name":              config.StringVariable(name),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"rcv_region":        config.StringVariable(testAzureRCVRegion(t)),
				"key_name":          config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":       config.StringVariable(testAzureKMSKeys(t)[1].Version),
			},
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("rubrik_azure_rcv_archival_location.location",
						plancheck.ResourceActionUpdate),
				},
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyName),
					knownvalue.StringExact(testAzureKMSKeys(t)[1].Name)),
				statecheck.ExpectKnownValue("rubrik_azure_rcv_archival_location.location",
					tfjsonpath.New(keyKeyVaultKey).AtSliceIndex(0).AtMapKey(keyKeyVersion),
					knownvalue.StringExact(testAzureKMSKeys(t)[1].Version)),
			},
		}, {
			// Terraform import, verifying that the key vault key is read back.
			ResourceName: "rubrik_azure_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"name":              config.StringVariable(name),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"rcv_region":        config.StringVariable(testAzureRCVRegion(t)),
				"key_name":          config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":       config.StringVariable(testAzureKMSKeys(t)[1].Version),
			},
			ImportStateKind:   resource.ImportCommandWithID,
			ImportState:       true,
			ImportStateVerify: true,
		}, {
			// import {} block with id attribute.
			ResourceName: "rubrik_azure_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"name":              config.StringVariable(name),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"rcv_region":        config.StringVariable(testAzureRCVRegion(t)),
				"key_name":          config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":       config.StringVariable(testAzureKMSKeys(t)[1].Version),
			},
			ImportStateKind: resource.ImportBlockWithID,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}, {
			// import {} block with identity attribute.
			ResourceName: "rubrik_azure_rcv_archival_location.location",
			ConfigVariables: config.Variables{
				"credentials":       config.StringVariable(testAzureCredentials(t)),
				"tenant_domain":     config.StringVariable(testAzureTenantDomain(t)),
				"subscription_id":   config.StringVariable(testAzureSubscriptionID(t)),
				"subscription_name": config.StringVariable(testAzureSubscriptionName(t)),
				"name":              config.StringVariable(name),
				"kms_region":        config.StringVariable(testAzureKMSRegion(t)),
				"vault_name":        config.StringVariable(testAzureKMSVaultName(t)),
				"rcv_region":        config.StringVariable(testAzureRCVRegion(t)),
				"key_name":          config.StringVariable(testAzureKMSKeys(t)[1].Name),
				"key_version":       config.StringVariable(testAzureKMSKeys(t)[1].Version),
			},
			ImportStateKind: resource.ImportBlockWithResourceIdentity,
			ImportState:     true,
			ImportPlanChecks: resource.ImportPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		}},
	})
}
