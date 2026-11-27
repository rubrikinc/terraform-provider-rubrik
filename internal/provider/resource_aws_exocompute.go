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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	fwdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/exocompute"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqlexocompute "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/exocompute"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
)

const resourceAWSExocomputeDescription = `
The ´rubrik_aws_exocompute´ resource creates an RSC Exocompute configuration
for AWS workloads.

There are 3 types of Exocompute configurations:
 1. *RSC Managed Host* - When an RSC managed host configuration is created, RSC
    will automatically deploy the necessary resources in the specified AWS
    region to run the Exocompute service, including the AWS security groups.
 2. *Customer Managed Host* - When a customer managed host configuration is
    created, RSC will not deploy any resources. Instead it will use the AWS EKS
    cluster attached by the customer, using the
    ´rubrik_aws_exocompute_cluster_attachment´ resource, for all operations.
 3. *Application* - An application configuration is created by mapping the
    application cloud account to a host cloud account. The application cloud
    account will leverage the Exocompute resources deployed for the host
    configuration.

Items 1 and 2 above requires that the AWS account has been onboarded with the
´EXOCOMPUTE´ feature.

Since there are 3 types of Exocompute configurations, there are 3 ways to create
a ´rubrik_aws_exocompute´ resource:
 1. Using the ´account_id´, ´region´, ´vpc_id´ and ´subnets´ or ´subnet´ fields
    creates an RSC managed host configuration. Use the ´subnet´ block when pod
    subnets are needed. The ´cluster_access´ field can be used to configure
    private EKS cluster access.
 2. Using the ´account_id´ and ´region´ fields creates a customer managed host
    configuration. Note, the ´rubrik_aws_exocompute_cluster_attachment´
    resource must be used to attach an AWS EKS cluster to the Exocompute
    configuration.
 3. Using the ´account_id´ and ´host_account_id´ fields creates an
    application configuration. Set ´automated_key_sharing´ to ´true´ to
    request automated KMS key sharing for the application account.

-> **Note:** Customer managed Exocompute is sometimes referred to as Bring Your
   Own Kubernetes (BYOK). Using both host and application Exocompute
   configurations is sometimes referred to as shared Exocompute.
`

// This resource uses a template for its documentation, remember to update the
// template if the documentation for any field changes.
func resourceAwsExocompute() *schema.Resource {
	return &schema.Resource{
		CreateContext: awsCreateExocompute,
		ReadContext:   awsReadExocompute,
		UpdateContext: awsUpdateExocompute,
		DeleteContext: awsDeleteExocompute,

		Description: description(resourceAWSExocomputeDescription),
		Schema: map[string]*schema.Schema{
			keyID: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Exocompute configuration ID (UUID).",
			},
			keyAccountID: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "RSC cloud account ID (UUID). Changing this forces a new resource to be created.",
				ValidateFunc: validation.IsUUID,
			},
			keyAutomatedKeySharing: {
				Type:          schema.TypeBool,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				RequiredWith:  []string{keyHostAccountID},
				ConflictsWith: []string{keyRegion},
				Description: "Request automated KMS key sharing for an application configuration. New mappings " +
					"default to disabled. When omitted, the existing setting is kept. Changing this forces a new " +
					"resource to be created.",
			},
			keyCloudFormationURL: {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "CloudFormation launch URL returned by RSC when enabling KMS key sharing requires a stack update.",
			},
			keyKMSProvisioningStatus: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Current RSC provisioning status of the application account's KMS mapping.",
			},
			keyKMSRolePolicy: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "IAM policy JSON returned by RSC for a manually onboarded application account.",
			},
			keyClusterSecurityGroupID: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{keyHostAccountID},
				RequiredWith:  []string{keyNodeSecurityGroupID},
				Description:   "AWS security group ID for the cluster. Changing this forces a new resource to be created.",
				ValidateFunc:  validation.StringIsNotWhiteSpace,
			},
			keyHostAccountID: {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				AtLeastOneOf: []string{keyHostAccountID, keyRegion},
				Description:  "Exocompute host cloud account ID. Changing this forces a new resource to be created.",
				ValidateFunc: validation.IsUUID,
			},
			keyNodeSecurityGroupID: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{keyHostAccountID},
				RequiredWith:  []string{keyClusterSecurityGroupID},
				Description:   "AWS security group ID for the nodes. Changing this forces a new resource to be created.",
				ValidateFunc:  validation.StringIsNotWhiteSpace,
			},
			keyPolarisManaged: {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "If true the security groups are managed by RSC.",
			},
			keyClusterAccess: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{keyHostAccountID},
				RequiredWith:  []string{keyVPCID},
				Description: "EKS cluster access type. Possible values are " +
					"`EKS_CLUSTER_ACCESS_TYPE_PUBLIC` and `EKS_CLUSTER_ACCESS_TYPE_PRIVATE`. Can only be used with " +
					"RSC managed configurations.",
				ValidateFunc: validation.StringInSlice([]string{
					string(gqlexocompute.EKSClusterAccessPrivate),
					string(gqlexocompute.EKSClusterAccessPublic),
				}, false),
			},
			keyRegion: {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				AtLeastOneOf:  []string{keyHostAccountID, keyRegion},
				ConflictsWith: []string{"host_account_id"},
				Description: "AWS region to run the Exocompute instance in. Changing this forces a new resource " +
					"to be created.",
				ValidateFunc: validation.StringInSlice(gqlaws.AllRegionNames(), false),
			},
			keyTriggerHealthCheck: {
				Type:          schema.TypeBool,
				Optional:      true,
				ConflictsWith: []string{keyHostAccountID},
				Description: "Trigger a health check of the Exocompute configuration when it's created, or when the " +
					"field is changed to `true`, and wait for the result. A health check which fails or doesn't " +
					"finish is reported as a warning. Ignored for customer managed configurations. Defaults to " +
					"`false`.",
			},
			keySubnet: {
				Type:          schema.TypeSet,
				Elem:          awsSubnetResource(),
				MinItems:      2,
				MaxItems:      2,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{keyHostAccountID, keySubnets},
				RequiredWith:  []string{keyVPCID},
				Description: "AWS subnet for the cluster. Each subnet block accepts a `subnet_id` and an optional " +
					"`pod_subnet_id`. Changing this forces a new resource to be created.",
			},
			keySubnets: {
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringIsNotWhiteSpace,
				},
				MinItems:      2,
				MaxItems:      2,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{keyHostAccountID, keySubnet},
				RequiredWith:  []string{keyVPCID},
				Description: "AWS subnet IDs for the cluster subnets. Changing this forces a new resource to be " +
					"created.",
			},
			keyVPCID: {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{keyHostAccountID},
				Description:   "AWS VPC ID for the cluster network. Changing this forces a new resource to be created.",
				ValidateFunc:  validation.StringIsNotWhiteSpace,
			},
		},
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, m any) error {
			if _, ok := diff.GetOk(keyVPCID); ok {
				subnet := diff.Get(keySubnet).(*schema.Set)
				subnets := diff.Get(keySubnets).(*schema.Set)
				if subnet.Len() == 0 && subnets.Len() == 0 {
					return errors.New("vpc_id requires either subnet or subnets to be specified")
				}
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
		},
	}
}

func awsCreateExocompute(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	tflog.Trace(ctx, "awsCreateExocompute")

	client, err := m.(*client).polaris()
	if err != nil {
		return diag.FromErr(err)
	}

	accountID, err := uuid.Parse(d.Get(keyAccountID).(string))
	if err != nil {
		return diag.FromErr(err)
	}

	if host, ok := d.GetOk(keyHostAccountID); ok {
		hostID, err := uuid.Parse(host.(string))
		if err != nil {
			return diag.FromErr(err)
		}
		api := exocompute.Wrap(client)
		if d.Get(keyAutomatedKeySharing).(bool) {
			if accountID == hostID {
				return diag.Errorf("automated_key_sharing requires distinct application and host cloud accounts")
			}
			action, err := api.MapAWSCloudAccountWithKMSSharing(ctx, accountID, hostID)
			if err != nil {
				return diag.FromErr(err)
			}
			d.SetId(appCloudAccountPrefix + accountID.String())

			if err := d.Set(keyKMSRolePolicy, action.ManualRolePolicy); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set(keyCloudFormationURL, action.CloudFormationURL); err != nil {
				return diag.FromErr(err)
			}
			status, err := api.AWSKMSMappingStatus(ctx, accountID)
			if err != nil {
				return diag.Diagnostics{{
					Severity: diag.Warning,
					Summary:  "Could not read Exocompute KMS mapping status",
					Detail:   "The mapping was created. The provider will read its status on the next refresh: " + err.Error(),
				}}
			}
			return diag.FromErr(d.Set(keyKMSProvisioningStatus, string(status.ProvisioningStatus)))
		}

		if err := api.MapAWSCloudAccount(ctx, accountID, hostID); err != nil {
			return diag.FromErr(err)
		}
		d.SetId(appCloudAccountPrefix + accountID.String())
	} else {
		clusterSecurityGroupID := d.Get(keyClusterSecurityGroupID).(string)
		nodeSecurityGroupID := d.Get(keyNodeSecurityGroupID).(string)
		region := d.Get(keyRegion).(string)
		vpcID := d.Get(keyVPCID).(string)
		clusterAccess := d.Get(keyClusterAccess).(string)

		// Read subnets from whichever field is set.
		var subnets []gqlexocompute.AWSSubnet
		if v := d.Get(keySubnet).(*schema.Set); v.Len() > 0 {
			for _, s := range v.List() {
				subnets = append(subnets, fromAWSSubnetResource(s.(map[string]any)))
			}
		} else {
			for _, s := range d.Get(keySubnets).(*schema.Set).List() {
				subnets = append(subnets, gqlexocompute.AWSSubnet{ID: s.(string)})
			}
		}

		var config exocompute.AWSConfig
		switch {
		case region != "" && vpcID != "" && len(subnets) > 0:
			cfg := exocompute.AWSConfigParams{
				Region:                 gqlaws.RegionFromName(region),
				VPCID:                  vpcID,
				Subnets:                subnets,
				ClusterSecurityGroupID: clusterSecurityGroupID,
				NodeSecurityGroupID:    nodeSecurityGroupID,
			}
			if clusterAccess != "" {
				cfg.OptionalConfig = &gqlexocompute.AWSOptionalConfig{
					AWSClusterAccess: gqlexocompute.AWSClusterAccess(clusterAccess),
				}
			}
			config = cfg
		case region != "":
			config = exocompute.AWSConfigParams{Region: gqlaws.RegionFromName(region)}
		default:
			return diag.Errorf("invalid exocompute configuration")
		}

		id, err := exocompute.Wrap(client).AddAWSConfiguration(ctx, accountID, config)
		if err != nil {
			return diag.FromErr(err)
		}
		d.SetId(id.String())
	}

	var diags diag.Diagnostics
	if d.Get(keyTriggerHealthCheck).(bool) {
		// The health check returns Framework diagnostics, convert them. This
		// goes away when the resource is migrated.
		for _, fd := range exocomputeRunHealthCheck(ctx, client, core.CloudVendorAWS, d.Id(), d.Get(keyVPCID).(string) == "") {
			severity := diag.Warning
			if fd.Severity() == fwdiag.SeverityError {
				severity = diag.Error
			}
			diags = append(diags, diag.Diagnostic{Severity: severity, Summary: fd.Summary(), Detail: fd.Detail()})
		}
	}

	awsReadExocompute(ctx, d, m)
	return diags
}

func awsUpdateExocompute(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	tflog.Trace(ctx, "awsUpdateExocompute")

	var diags diag.Diagnostics
	if d.HasChange(keyTriggerHealthCheck) && d.Get(keyTriggerHealthCheck).(bool) {
		client, err := m.(*client).polaris()
		if err != nil {
			return diag.FromErr(err)
		}
		// The health check returns Framework diagnostics, convert them. This
		// goes away when the resource is migrated.
		for _, fd := range exocomputeRunHealthCheck(ctx, client, core.CloudVendorAWS, d.Id(), d.Get(keyVPCID).(string) == "") {
			severity := diag.Warning
			if fd.Severity() == fwdiag.SeverityError {
				severity = diag.Error
			}
			diags = append(diags, diag.Diagnostic{Severity: severity, Summary: fd.Summary(), Detail: fd.Detail()})
		}
	}

	return append(diags, awsReadExocompute(ctx, d, m)...)
}

func awsReadExocompute(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	tflog.Trace(ctx, "awsReadExocompute")

	client, err := m.(*client).polaris()
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	if strings.HasPrefix(d.Id(), appCloudAccountPrefix) {
		appID, err := uuid.Parse(strings.TrimPrefix(id, appCloudAccountPrefix))
		if err != nil {
			return diag.FromErr(err)
		}
		api := exocompute.Wrap(client)
		hostID, err := api.AWSHostCloudAccount(ctx, appID)
		if err != nil && !errors.Is(err, graphql.ErrNotFound) {
			return diag.FromErr(err)
		}
		notFound := errors.Is(err, graphql.ErrNotFound)
		if notFound && !d.Get(keyAutomatedKeySharing).(bool) {
			d.SetId("")
			return nil
		}

		status, err := api.AWSKMSMappingStatus(ctx, appID)
		if errors.Is(err, graphql.ErrNotFound) && notFound {
			d.SetId("")
			return nil
		}
		if err != nil {
			return diag.FromErr(err)
		}

		if notFound {
			// RSC hides mappings undergoing revoke from the ordinary mapping
			// list. Keep the identity so a failed or interrupted delete can be
			// retried after the required AWS change has been applied.
			switch status.ProvisioningStatus {
			case gqlexocompute.KMSMappingProvisioningStatusConnecting, gqlexocompute.KMSMappingProvisioningStatusDisabling:
				if err := d.Set(keyKMSProvisioningStatus, string(status.ProvisioningStatus)); err != nil {
					return diag.FromErr(err)
				}
			default:
				d.SetId("")
			}
			return nil
		}

		if err := d.Set(keyAccountID, appID.String()); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(keyHostAccountID, hostID.String()); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(keyKMSProvisioningStatus, string(status.ProvisioningStatus)); err != nil {
			return diag.FromErr(err)
		}
		if status.ProvisioningStatus == gqlexocompute.KMSMappingProvisioningStatusDisabling {
			// A revoke may have started between the two reads. It reports
			// mode NONE but still needs KMS-aware cleanup.
			return diag.FromErr(d.Set(keyAutomatedKeySharing, true))
		}

		switch status.SharingMode {
		case gqlexocompute.KMSSharingModeNone, gqlexocompute.KMSSharingModeUnspecified:
			if err := d.Set(keyAutomatedKeySharing, false); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set(keyKMSRolePolicy, nil); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set(keyCloudFormationURL, nil); err != nil {
				return diag.FromErr(err)
			}
		case gqlexocompute.KMSSharingModeSharingOnly, gqlexocompute.KMSSharingModeSharingAndGateway:
			if err := d.Set(keyAutomatedKeySharing, true); err != nil {
				return diag.FromErr(err)
			}
		default:
			return diag.Errorf("unrecognized Exocompute KMS sharing mode %q for application cloud account %s", status.SharingMode, appID)
		}
	} else {
		configID, err := uuid.Parse(id)
		if err != nil {
			return diag.FromErr(err)
		}
		exoConfig, err := exocompute.Wrap(client).AWSConfigurationByID(ctx, configID)
		if errors.Is(err, graphql.ErrNotFound) {
			d.SetId("")
			return nil
		}
		if err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set(keyAccountID, exoConfig.CloudAccountID.String()); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(keyRegion, exoConfig.Region.Name()); err != nil {
			return diag.FromErr(err)
		}

		// Rubrik managed cluster
		if err := d.Set(keyClusterSecurityGroupID, exoConfig.ClusterSecurityGroupID); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(keyNodeSecurityGroupID, exoConfig.NodeSecurityGroupID); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(keyPolarisManaged, exoConfig.IsManagedByRubrik); err != nil {
			return diag.FromErr(err)
		}
		// Populate keySubnets (string set of IDs).
		stringSubnets := schema.Set{F: schema.HashString}
		if exoConfig.Subnet1.ID != "" {
			stringSubnets.Add(exoConfig.Subnet1.ID)
		}
		if exoConfig.Subnet2.ID != "" {
			stringSubnets.Add(exoConfig.Subnet2.ID)
		}
		if err := d.Set(keySubnets, &stringSubnets); err != nil {
			return diag.FromErr(err)
		}

		// Populate keySubnet (block set with full details including
		// pod_subnet_id).
		subnetSet := schema.Set{F: schema.HashResource(awsSubnetResource())}
		if exoConfig.Subnet1.ID != "" {
			subnetSet.Add(toAWSSubnetResource(exoConfig.Subnet1))
		}
		if exoConfig.Subnet2.ID != "" {
			subnetSet.Add(toAWSSubnetResource(exoConfig.Subnet2))
		}
		if err := d.Set(keySubnet, &subnetSet); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set(keyVPCID, exoConfig.VPCID); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(keyClusterAccess, awsClusterAccess(exoConfig.OptionalConfig)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func awsDeleteExocompute(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	tflog.Trace(ctx, "awsDeleteExocompute")

	client, err := m.(*client).polaris()
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	if strings.HasPrefix(id, appCloudAccountPrefix) {
		appID, err := uuid.Parse(strings.TrimPrefix(id, appCloudAccountPrefix))
		if err != nil {
			return diag.FromErr(err)
		}
		api := exocompute.Wrap(client)
		if !d.Get(keyAutomatedKeySharing).(bool) {
			return diag.FromErr(api.UnmapAWSCloudAccount(ctx, appID))
		}

		action, err := api.UnmapAWSCloudAccountWithKMSSharing(ctx, appID)
		if errors.Is(err, graphql.ErrNotFound) {
			return nil
		}
		if err != nil {
			return diag.FromErr(err)
		}
		if !action.PendingApproval {
			return nil
		}
		if action.CloudFormationURL != "" || action.TemplateURL != "" {
			return diag.Diagnostics{{
				Severity: diag.Error,
				Summary:  "Exocompute KMS revoke requires CloudFormation approval",
				Detail: fmt.Sprintf("Complete the pending KMS key sharing revoke for application account %s in RSC by "+
					"applying the AWS CloudFormation stack update, then rerun Terraform.", appID),
			}}
		}
		if action.ManualRolePolicy != "" {
			return diag.Errorf("apply the pending IAM policy change for application account %s in RSC, confirm the revoke, and rerun Terraform", appID)
		}
		// The policy attachment must depend on the mapping, so Terraform
		// removes it before reaching this Delete. RSC does not verify removal.
		return diag.FromErr(api.ConfirmAWSManualKMSMapping(ctx, appID))
	} else {
		configID, err := uuid.Parse(id)
		if err != nil {
			return diag.FromErr(err)
		}
		if err = exocompute.Wrap(client).RemoveAWSConfiguration(ctx, configID); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

// exocomputeRunHealthCheck triggers a health check of the AWS or Azure
// Exocompute configuration with the specified ID and waits for the result.
// The configuration has already been created when this is called, so a
// failure to trigger the health check, a health check which doesn't report
// the configuration as healthy, or a timeout, is returned as a warning. An
// error would taint the resource, which would replace a working configuration
// on the next apply. For a customer managed configuration no health check is
// triggered, since RSC runs its health checks when it reconciles the attached
// cluster.
//
// The triggerHealthCheck parameter of the create mutations isn't used, since
// RSC ignores any error from it and returns no job ID, so there is no way to
// tell which health check result belongs to the configuration.
func exocomputeRunHealthCheck(ctx context.Context, client *polaris.Client, cloudVendor core.CloudVendor, id string, customerManaged bool) fwdiag.Diagnostics {
	if customerManaged {
		return fwdiag.Diagnostics{fwdiag.NewWarningDiagnostic(
			"Exocompute health check ignored",
			"trigger_health_check is ignored for customer managed Exocompute configurations, RSC runs "+
				"their health checks when it reconciles the attached cluster.")}
	}

	configID, err := uuid.Parse(id)
	if err != nil {
		return fwdiag.Diagnostics{fwdiag.NewErrorDiagnostic("Invalid Exocompute configuration ID", err.Error())}
	}

	jobID, err := exocompute.Wrap(client).TriggerHealthCheck(ctx, cloudVendor, configID)
	if err != nil {
		return fwdiag.Diagnostics{fwdiag.NewWarningDiagnostic("Exocompute health check not started", err.Error())}
	}

	status, err := exocompute.Wrap(client).WaitForHealthCheck(ctx, cloudVendor, configID, jobID)
	if err != nil {
		return fwdiag.Diagnostics{fwdiag.NewWarningDiagnostic(
			"Exocompute health check result not available",
			fmt.Sprintf("Health check job %s did not finish: %s. Check the result in the RSC UI.", jobID, err))}
	}
	if status.Status != gqlexocompute.HealthCheckStatusHealthy {
		detail := fmt.Sprintf("Health check job %s finished with status %s.", jobID, status.Status)
		if status.FailureReason != "" {
			detail += " " + status.FailureReason
		}
		detail += " See the Exocompute events in the RSC activity log for details."
		return fwdiag.Diagnostics{fwdiag.NewWarningDiagnostic("Exocompute health check failed", detail)}
	}

	return nil
}

// awsClusterAccess returns the cluster access type from the optional config.
// Returns the default public access type if the config is nil.
func awsClusterAccess(config *gqlexocompute.AWSOptionalConfig) string {
	if config == nil {
		return ""
	}
	return string(config.AWSClusterAccess)
}

func awsSubnetResource() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			keySubnetID: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "AWS subnet ID.",
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			keyPodSubnetID: {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Description:  "AWS subnet ID for the pods.",
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
		},
	}
}

func fromAWSSubnetResource(block map[string]any) gqlexocompute.AWSSubnet {
	return gqlexocompute.AWSSubnet{
		ID:          block[keySubnetID].(string),
		PodSubnetID: block[keyPodSubnetID].(string),
	}
}

func toAWSSubnetResource(subnet gqlexocompute.AWSSubnet) map[string]any {
	return map[string]any{
		keySubnetID:    subnet.ID,
		keyPodSubnetID: subnet.PodSubnetID,
	}
}
