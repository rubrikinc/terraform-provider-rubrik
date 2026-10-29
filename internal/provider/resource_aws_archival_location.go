// Copyright 2023 Rubrik, Inc.
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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
)

const (
	implicitPrefix = "rubrik-"
)

const resourceAWSArchivalLocationDescription = `
The ´rubrik_aws_archival_location´ resource creates an RSC archival location for
cloud-native workloads. This resource requires that the AWS account has been
onboarded with the ´CLOUD_NATIVE_ARCHIVAL´ feature.

When creating an archival location, the region where the snapshots are stored needs
to be specified:
  * ´SOURCE_REGION´ - Store snapshots in the same region to minimize data transfer
    charges. This is the default behaviour when the ´region´ field is not specified.
  * ´SPECIFIC_REGION´ - Storing snapshots in another region can increase total data
    transfer charges. The ´region´ field specifies the region.

The archived data is encrypted on the server side with an AWS KMS master key, or
on the client side with a KMS key managed by an RSC key manager:
  * ´kms_master_key´ - For ´SOURCE_REGION´ the key defaults to ´aws/s3´, the AWS
    managed S3 key, which is the same default the RSC UI uses. For
    ´SPECIFIC_REGION´ the key must be a customer managed key, given as a key ARN
    or key ID. AWS managed keys, such as ´aws/s3´, and key aliases, ´alias/...´,
    are rejected for ´SPECIFIC_REGION´, because they cannot encrypt data stored
    outside their own region.
  * ´kms_key´ - Client-side encryption with a KMS key in a key manager registered
    in RSC, see the ´rubrik_aws_kms_key_manager´ data source. Conflicts with
    ´kms_master_key´.

When ´region´ is specified, one of ´kms_master_key´ and ´kms_key´ must be given.

-> **Note:** The AWS bucket holding the archived data is not created until the first
   protected object is archived.

-> **Note:** RSC does not return the ´kms_key´ block, so it is not read back from
   RSC and cannot be imported. A location created with ´kms_key´ has no
   ´kms_master_key´.
`

func resourceAwsArchivalLocation() *schema.Resource {
	return &schema.Resource{
		CreateContext: awsCreateArchivalLocation,
		ReadContext:   awsReadArchivalLocation,
		UpdateContext: awsUpdateArchivalLocation,
		DeleteContext: awsDeleteArchivalLocation,
		CustomizeDiff: awsCustomizeDiffArchivalLocation,

		Description: description(resourceAWSArchivalLocationDescription),
		Schema: map[string]*schema.Schema{
			keyID: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Cloud native archival location ID (UUID).",
			},
			keyAccountID: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "RSC cloud account ID (UUID). Changing this forces a new resource to be created.",
				ValidateFunc: validation.IsUUID,
			},
			keyBucketPrefix: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "AWS bucket prefix. The prefix cannot be longer than 19 characters. Note that `rubrik-` " +
					"will always be prepended to the prefix. Changing this forces a new resource to be created.",
				ValidateFunc: validation.StringLenBetween(1, 19),
			},
			keyBucketTags: {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "AWS bucket tags. Each tag will be added to the bucket created by RSC.",
			},
			keyConnectionStatus: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Connection status of the cloud native archival location.",
			},
			keyKMSKey: {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						keyKeyManagerID: {
							Type:         schema.TypeString,
							Required:     true,
							ForceNew:     true,
							Description:  "KMS key manager ID (UUID).",
							ValidateFunc: validation.IsUUID,
						},
						keyKeyID: {
							Type:         schema.TypeString,
							Required:     true,
							ForceNew:     true,
							Description:  "KMS key ID.",
							ValidateFunc: validation.StringIsNotWhiteSpace,
						},
					},
				},
				MaxItems:      1,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{keyKMSMasterKey},
				Description: "AWS KMS key used for client-side encryption of the archived data. The key manager " +
					"must be registered in RSC, see the `rubrik_aws_kms_key_manager` data source. RSC does not " +
					"check the key manager's region against the archival location's region, but the RSC UI " +
					"requires an Exocompute mapping covering the key manager's region. Conflicts with " +
					"`kms_master_key`. RSC does not return the key, so it is not read back from RSC and cannot be " +
					"imported. May be specified at most once. Changing this forces a new resource to be created.",
			},
			keyKMSMasterKey: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				Sensitive:     true,
				ConflictsWith: []string{keyKMSKey},
				Description: "AWS KMS master key ARN, ID or alias, used for server-side encryption of the archived " +
					"data. When `region` is not specified (`SOURCE_REGION`), the key defaults to `aws/s3`, the AWS " +
					"managed S3 key. When `region` is specified (`SPECIFIC_REGION`), the key must be the ARN or ID " +
					"of a customer managed key, and one of `kms_master_key` and `kms_key` must be given. Conflicts " +
					"with `kms_key`.",
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			keyLocationTemplate: {
				Type:     schema.TypeString,
				Computed: true,
				Description: "Location template. If a region was specified, it will be `SPECIFIC_REGION`, otherwise " +
					"`SOURCE_REGION`.",
			},
			keyName: {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "Name of the cloud native archival location.",
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			keyRegion: {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Description: "AWS region to store the snapshots in. If not specified, the snapshots will be " +
					"stored in the same region as the workload. Changing this forces a new resource to be created.",
				ValidateFunc: validation.StringInSlice(aws.AllRegionNames(), false),
			},
			keyStorageClass: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "STANDARD_IA",
				Description: "AWS bucket storage class. Possible values are `STANDARD`, `STANDARD_IA`, `ONEZONE_IA`, " +
					"`GLACIER_INSTANT_RETRIEVAL`, `GLACIER_DEEP_ARCHIVE` and `GLACIER_FLEXIBLE_RETRIEVAL`. Default " +
					"value is `STANDARD_IA`.",
				ValidateFunc: validation.StringInSlice([]string{
					"STANDARD", "STANDARD_IA", "ONEZONE_IA", "GLACIER_INSTANT_RETRIEVAL", "GLACIER_DEEP_ARCHIVE",
					"GLACIER_FLEXIBLE_RETRIEVAL",
				}, false),
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func awsCreateArchivalLocation(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	tflog.Trace(ctx, "awsCreateArchivalLocation")

	client, err := m.(*client).polaris()
	if err != nil {
		return diag.FromErr(err)
	}

	cloudAccountID, err := uuid.Parse(d.Get(keyAccountID).(string))
	if err != nil {
		return diag.FromErr(err)
	}

	createParams := gqlarchival.CreateAWSStorageSettingParams{
		CloudAccountID: cloudAccountID,
		Name:           d.Get(keyName).(string),
		BucketPrefix:   d.Get(keyBucketPrefix).(string),
		StorageClass:   d.Get(keyStorageClass).(string),
		Region:         aws.RegionFromName(d.Get(keyRegion).(string)).ToRegionEnum(),
		KmsMasterKey:   d.Get(keyKMSMasterKey).(string),
		BucketTags:     toAWSBucketTags(d.Get(keyBucketTags).(map[string]any)),
	}

	// A KMS key selects client-side encryption, otherwise a source region
	// location without a KMS master key falls back to the AWS managed S3 key,
	// like the RSC UI does. RSC rejects a create request without a key.
	if kmsKey := d.Get(keyKMSKey).([]any); len(kmsKey) > 0 {
		block := kmsKey[0].(map[string]any)
		keyManagerID, err := uuid.Parse(block[keyKeyManagerID].(string))
		if err != nil {
			return diag.FromErr(err)
		}
		createParams.KMSKey = &gqlarchival.AWSKMSKey{
			KeyManagerID: keyManagerID,
			KeyID:        block[keyKeyID].(string),
		}
	} else if createParams.KmsMasterKey == "" && d.Get(keyRegion).(string) == "" {
		createParams.KmsMasterKey = "aws/s3"
	}

	// Create the AWS cloud native archival location.
	targetMappingID, err := archival.Wrap(client).CreateAWSStorageSetting(ctx, createParams)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(targetMappingID.String())
	awsReadArchivalLocation(ctx, d, m)
	return nil
}

func awsReadArchivalLocation(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	tflog.Trace(ctx, "awsReadArchivalLocation")

	client, err := m.(*client).polaris()
	if err != nil {
		return diag.FromErr(err)
	}

	targetMappingID, err := uuid.Parse(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	// Read the AWS cloud native archival location. If the archival location
	// isn't found we remove it from the local state and return.
	targetMapping, err := archival.Wrap(client).AWSTargetMappingByID(ctx, targetMappingID)
	if errors.Is(err, graphql.ErrNotFound) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}

	targetTemplate := targetMapping.TargetTemplate
	if err := d.Set(keyBucketPrefix, strings.TrimPrefix(targetTemplate.BucketPrefix, implicitPrefix)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(keyAccountID, targetTemplate.CloudAccount.ID.String()); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(keyConnectionStatus, targetMapping.ConnectionStatus.Status); err != nil {
		return diag.FromErr(err)
	}
	// RSC only returns the KMS master key of a server-side encrypted location.
	if len(d.Get(keyKMSKey).([]any)) == 0 {
		if err := d.Set(keyKMSMasterKey, targetTemplate.KMSMasterKey); err != nil {
			return diag.FromErr(err)
		}
	}
	if err := d.Set(keyLocationTemplate, targetTemplate.LocTemplate); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(keyName, targetMapping.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(keyRegion, targetTemplate.Region.Name()); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(keyStorageClass, targetTemplate.StorageClass); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(keyBucketTags, fromAWSBucketTags(targetTemplate.BucketTags)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func awsUpdateArchivalLocation(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	tflog.Trace(ctx, "awsUpdateArchivalLocation")

	client, err := m.(*client).polaris()
	if err != nil {
		return diag.FromErr(err)
	}

	targetMappingID, err := uuid.Parse(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	bucketTags := toAWSBucketTags(d.Get(keyBucketTags).(map[string]any))
	updateParams := gqlarchival.UpdateAWSStorageSettingParams{
		Name:                d.Get(keyName).(string),
		StorageClass:        d.Get(keyStorageClass).(string),
		DeleteAllBucketTags: bucketTags == nil,
		BucketTags:          bucketTags,
	}

	// RSC rejects a KMS master key for a client-side encrypted location.
	if len(d.Get(keyKMSKey).([]any)) == 0 {
		updateParams.KmsMasterKey = d.Get(keyKMSMasterKey).(string)
	}

	// Update the AWS cloud native archival location. Note, the API doesn't
	// support updating all arguments.
	err = archival.Wrap(client).UpdateAWSStorageSetting(ctx, targetMappingID, updateParams)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func awsDeleteArchivalLocation(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	tflog.Trace(ctx, "awsDeleteArchivalLocation")

	client, err := m.(*client).polaris()
	if err != nil {
		return diag.FromErr(err)
	}

	targetMappingID, err := uuid.Parse(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	// Delete the AWS cloud native archival location.
	if err := archival.Wrap(client).DeleteTargetMapping(ctx, targetMappingID); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// awsCustomizeDiffArchivalLocation validates changes to the AWS archival
// location resource. A specific region location needs a customer managed key,
// so it gets no default KMS master key.
func awsCustomizeDiffArchivalLocation(ctx context.Context, diff *schema.ResourceDiff, m any) error {
	tflog.Trace(ctx, "awsCustomizeDiffArchivalLocation")

	// Only a new archival location is checked. The KMS master key of an
	// existing location is computed, and the region forces a new resource.
	if diff.Id() != "" {
		return nil
	}

	// Values unknown during plan are checked again when the plan is applied.
	config := diff.GetRawConfig()
	if config.IsNull() || !config.IsKnown() {
		return nil
	}
	region := config.GetAttr(keyRegion)
	kmsMasterKey := config.GetAttr(keyKMSMasterKey)
	kmsKey := config.GetAttr(keyKMSKey)
	if !region.IsKnown() || !kmsMasterKey.IsKnown() || !kmsKey.IsKnown() {
		return nil
	}
	if region.IsNull() || region.AsString() == "" {
		return nil
	}
	if (kmsMasterKey.IsNull() || kmsMasterKey.AsString() == "") && (kmsKey.IsNull() || kmsKey.LengthInt() == 0) {
		return fmt.Errorf("one of %s and %s must be specified when %s is specified, the KMS master key must be a "+
			"customer managed key", keyKMSMasterKey, keyKMSKey, keyRegion)
	}

	return nil
}

// toAWSBucketTags converts from the bucket tags argument to an archival AWS
// tags. If the bucket tags argument is empty, nil is returned.
func toAWSBucketTags(tags map[string]any) *core.Tags {
	tagList := make([]core.Tag, 0, len(tags))
	for key, value := range tags {
		tagList = append(tagList, core.Tag{Key: key, Value: value.(string)})
	}
	if len(tagList) > 0 {
		return &core.Tags{TagList: tagList}
	}

	return nil
}

// fromAWSBucketTags converts to the bucket tags argument from a slice of AWS
// tags.
func fromAWSBucketTags(tags []core.Tag) map[string]any {
	bucketTags := make(map[string]any, len(tags))
	for _, tag := range tags {
		bucketTags[tag.Key] = tag.Value
	}

	return bucketTags
}
