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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cluster"
)

const resourceClusterSettingsDescription = `
The ´rubrik_cluster_settings´ resource manages the CDM package download and
upgrade lifecycle of a single Rubrik cluster registered with RSC.

Setting ´version´ drives the cluster to that installed version: the provider
downloads the matching package (resolved from the Rubrik support portal, or
from ´package_url´/´package_md5´ for air-gapped environments) and then upgrades
the cluster, blocking until the cluster reports the target version. When the
target is more than one release ahead, the provider automatically upgrades
through the required intermediate releases one hop at a time. When
´package_url´/´package_md5´ are set the support portal is not queried, and they
require ´version´ or ´downloaded_version´ to be set as the download target; a
custom package can only drive a single direct hop, not a multi-hop upgrade.

Setting only ´downloaded_version´ pre-stages a package without upgrading. Both
may be set together to upgrade to ´version´ and pre-stage a newer
´downloaded_version´ for a future upgrade in the same apply; ´downloaded_version´
must not be older than ´version´.

Setting ´upgrade_mode´ toggles the cluster between FAST and ROLLING upgrades.

A multi-hop upgrade runs each hop sequentially within a single apply, so the
total time scales with the number of intermediate releases. The create and
update ´timeouts´ (default 6 hours) bound the whole chain, not a single hop;
increase ´timeouts.update´ when a target is several releases ahead.

Deleting the resource only removes it from Terraform state; the cluster and its
installed version are left untouched.
`

const defaultClusterUpgradeTimeout = 6 * time.Hour

var (
	_ resource.Resource                   = &clusterSettingsResource{}
	_ resource.ResourceWithConfigure      = &clusterSettingsResource{}
	_ resource.ResourceWithIdentity       = &clusterSettingsResource{}
	_ resource.ResourceWithImportState    = &clusterSettingsResource{}
	_ resource.ResourceWithValidateConfig = &clusterSettingsResource{}
)

type clusterSettingsResource struct {
	client *client
}

// clusterSettingsResourceModel holds the configurable cluster settings:
// version, downloaded_version and upgrade_mode, plus the download-source
// override and operation timeouts. id and name are computed.
type clusterSettingsResourceModel struct {
	ClusterID         types.String `tfsdk:"cluster_id"`
	Version           types.String `tfsdk:"version"`
	DownloadedVersion types.String `tfsdk:"downloaded_version"`
	UpgradeMode       types.String `tfsdk:"upgrade_mode"`
	PackageURL        types.String `tfsdk:"package_url"`
	PackageMD5        types.String `tfsdk:"package_md5"`

	ID       types.String   `tfsdk:"id"`
	Name     types.String   `tfsdk:"name"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

type clusterSettingsIdentityModel struct {
	ClusterID types.String `tfsdk:"cluster_id"`
}

func newClusterSettingsResource() resource.Resource {
	return &clusterSettingsResource{}
}

func (r *clusterSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyClusterSettings
}

func (r *clusterSettingsResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.Schema")

	useState := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}

	res.Schema = schema.Schema{
		Description: description(resourceClusterSettingsDescription),
		Attributes: map[string]schema.Attribute{
			keyClusterID: schema.StringAttribute{
				Required: true,
				Description: "Cluster ID. Changing this forces a new resource to be " +
					"created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyVersion: schema.StringAttribute{
				Optional: true,
				Description: "Desired installed CDM version. When set, the cluster is " +
					"downloaded (if needed) and upgraded to this version, automatically " +
					"upgrading through any required intermediate releases. Leave unset to " +
					"not manage the installed version.",
			},
			keyDownloadedVersion: schema.StringAttribute{
				Optional: true,
				Description: "Desired staged CDM version. Set this without `version` to " +
					"pre-stage a package without upgrading.",
			},
			keyUpgradeMode: schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Upgrade mode for the cluster. One of `FAST` or `ROLLING`.",
				PlanModifiers: useState,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(gqlcluster.UpgradeTypeFast),
						string(gqlcluster.UpgradeTypeRolling),
					),
				},
			},
			keyPackageURL: schema.StringAttribute{
				Optional: true,
				Description: "Override URL for the CDM package tarball. When set together " +
					"with `package_md5`, the support portal lookup is skipped and these are " +
					"passed directly to the download. Use this for air-gapped environments.",
			},
			keyPackageMD5: schema.StringAttribute{
				Optional:    true,
				Description: "MD5 checksum of the package at `package_url`. Required when `package_url` is set.",
			},

			keyID: schema.StringAttribute{
				Computed:      true,
				Description:   "Cluster ID.",
				PlanModifiers: useState,
			},
			keyName: schema.StringAttribute{
				Computed:      true,
				Description:   "Cluster name.",
				PlanModifiers: useState,
			},
			keyTimeouts: timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Update: true,
			}),
		},
	}
}

func (r *clusterSettingsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.ValidateConfig")

	var config clusterSettingsResourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(validateClusterSettingsConfig(config)...)
}

// validateClusterSettingsConfig holds the plan-time, client-free validation
// rules for the resource so they can be unit-tested in isolation.
func validateClusterSettingsConfig(config clusterSettingsResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// version and downloaded_version must be valid CDM versions. Parse them up
	// front so a bad format fails at plan time with a clear message instead of
	// late, when a download fires.
	versionOK := false
	if config.Version.ValueString() != "" {
		if _, err := cluster.ParseCDMVersion(config.Version.ValueString()); err != nil {
			diags.AddAttributeError(
				path.Root(keyVersion),
				"Invalid version",
				fmt.Sprintf("version %q is not a valid CDM version: %s", config.Version.ValueString(), err),
			)
		} else {
			versionOK = true
		}
	}

	var downloadedCDM cluster.CDMVersion
	downloadedOK := false
	if config.DownloadedVersion.ValueString() != "" {
		v, err := cluster.ParseCDMVersion(config.DownloadedVersion.ValueString())
		if err != nil {
			diags.AddAttributeError(
				path.Root(keyDownloadedVersion),
				"Invalid downloaded_version",
				fmt.Sprintf("downloaded_version %q is not a valid CDM version: %s", config.DownloadedVersion.ValueString(), err),
			)
		} else {
			downloadedCDM, downloadedOK = v, true
		}
	}

	// version is the package that gets downloaded and installed; downloaded_version
	// is staged for a future upgrade. Staging a package older than the install
	// target is contradictory, so reject that. An equal or newer downloaded_version
	// is allowed (equal is a no-op pre-stage, newer stages the next upgrade).
	if versionOK && downloadedOK && downloadedCDM.LessThan(config.Version.ValueString()) {
		diags.AddAttributeError(
			path.Root(keyDownloadedVersion),
			"Conflicting version and downloaded_version",
			fmt.Sprintf("downloaded_version %q must not be older than version %q. "+
				"Set only version to download and upgrade, set only downloaded_version to "+
				"pre-stage a package, or set downloaded_version to a release newer than "+
				"version to pre-stage the next upgrade.",
				config.DownloadedVersion.ValueString(), config.Version.ValueString()),
		)
	}

	// package_url and package_md5 are an air-gapped download override and must be
	// set together. Validating here gives plan-time feedback instead of failing
	// late, only when a download fires.
	urlSet := config.PackageURL.ValueString() != ""
	md5Set := config.PackageMD5.ValueString() != ""
	if urlSet && !md5Set && !config.PackageMD5.IsUnknown() {
		diags.AddAttributeError(
			path.Root(keyPackageMD5),
			"package_md5 required",
			"`package_md5` must be set together with `package_url`.",
		)
	}
	if md5Set && !urlSet && !config.PackageURL.IsUnknown() {
		diags.AddAttributeError(
			path.Root(keyPackageURL),
			"package_url required",
			"`package_url` must be set together with `package_md5`.",
		)
	}

	// The package override only takes effect while downloading a target, so it is
	// meaningless without a version to upgrade to or a downloaded_version to
	// pre-stage. Reject it so the package is never silently ignored.
	// unknown is treated as possibly-set so validation defers until the value
	// resolves, matching the package_url/package_md5 unknown-guards above.
	versionSet := config.Version.IsUnknown() || config.Version.ValueString() != ""
	downloadedSet := config.DownloadedVersion.IsUnknown() || config.DownloadedVersion.ValueString() != ""
	if (urlSet || md5Set) && !versionSet && !downloadedSet {
		diags.AddAttributeError(
			path.Root(keyPackageURL),
			"version or downloaded_version required",
			"`package_url` and `package_md5` only apply when downloading a package; "+
				"set `version` to download and upgrade, or `downloaded_version` to pre-stage.",
		)
	}

	return diags
}

func (r *clusterSettingsResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyClusterID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Cluster ID.",
			},
		},
	}
}

func (r *clusterSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *clusterSettingsResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.Create")

	var plan clusterSettingsResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, defaultClusterUpgradeTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.reconcile(ctx, &plan, &res.Diagnostics)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, clusterSettingsIdentityModel{ClusterID: plan.ClusterID})...)
}

func (r *clusterSettingsResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.Read")

	var state clusterSettingsResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	clusterUUID, err := uuid.Parse(state.ClusterID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cluster UUID", err.Error())
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	details, err := cluster.Wrap(polarisClient).ClusterUpgrade(ctx, clusterUUID)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read cluster settings", err.Error())
		return
	}

	state.applyComputed(details)
	state.Version = refreshedVersion(state.Version, details)

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, clusterSettingsIdentityModel{ClusterID: state.ClusterID})...)
}

func (r *clusterSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.Update")

	var plan clusterSettingsResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Update(ctx, defaultClusterUpgradeTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.reconcile(ctx, &plan, &res.Diagnostics)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, clusterSettingsIdentityModel{ClusterID: plan.ClusterID})...)
}

// Delete only removes the resource from Terraform state. The cluster outlives
// its Terraform declaration, so no API calls are made.
func (r *clusterSettingsResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.Delete")
}

func (r *clusterSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "clusterSettingsResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyClusterID), path.Root(keyClusterID), req, res)
}

// reconcile drives the cluster toward the plan's desired download/upgrade
// state, then refreshes plan's computed fields from the resulting cluster
// state. Mutates plan in place. Blocks on the SDK wait loops, so ctx must
// carry a deadline.
func (r *clusterSettingsResource) reconcile(ctx context.Context, plan *clusterSettingsResourceModel, diags *diag.Diagnostics) {
	clusterUUID, err := uuid.Parse(plan.ClusterID.ValueString())
	if err != nil {
		diags.AddError("Invalid cluster UUID", err.Error())
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		diags.AddError("RSC client error", err.Error())
		return
	}

	// The rubrik_cluster_settings resource relies on the RSC upgrade v2 APIs,
	// which are only available when the RSC_UI_UPGRADE_V2_ENABLED feature flag is
	// enabled for the account.
	if !r.client.flag(ctx, "RSC_UI_UPGRADE_V2_ENABLED") {
		diags.AddError("Feature not enabled",
			"The rubrik_cluster_settings resource requires the RSC_UI_UPGRADE_V2_ENABLED feature flag to be enabled for the RSC account.")
		return
	}

	api := cluster.Wrap(polarisClient)

	details, err := api.ClusterUpgrade(ctx, clusterUUID)
	if err != nil {
		diags.AddError("Failed to read cluster settings", err.Error())
		return
	}
	info := details.CDMInfo

	// 1. Upgrade mode preference.
	upgradeType := currentUpgradeType(info)
	if mode := plan.UpgradeMode.ValueString(); mode != "" {
		desired := gqlcluster.UpgradeType(mode)
		if desired != upgradeType {
			if _, err := api.SetUpgradeType(ctx, clusterUUID, desired); err != nil {
				diags.AddError("Failed to set upgrade mode", err.Error())
				return
			}
		}
		upgradeType = desired
	}

	// 2. Block on an in-flight rollback before staging anything new.
	if info != nil && info.UpgradeStatusV2 != nil {
		switch info.UpgradeStatusV2.RSCClusterUpgradeStatus {
		case gqlcluster.RSCUpgradeStatusRollingBack:
			if _, err := api.WaitForRollback(ctx, clusterUUID, info.PreviousVersion); err != nil {
				diags.AddError("Failed waiting for cluster rollback", err.Error())
				return
			}
			if details, err = api.ClusterUpgrade(ctx, clusterUUID); err != nil {
				diags.AddError("Failed to read cluster settings", err.Error())
				return
			}
			info = details.CDMInfo
		case gqlcluster.RSCUpgradeStatusRollingBackFailed:
			diags.AddError("Cluster rollback failed",
				fmt.Sprintf("cluster %q is in ROLLINGBACK_FAILED and must be recovered manually before Terraform can manage it", clusterUUID))
			return
		}
	}

	// 3. Reach the desired installed version. When the target is more than one
	// release ahead, walk the intermediate hops in turn, downloading (unless
	// already staged) and upgrading to each. Each hop is a direct upgrade from
	// the previous one, so resolveDownload resolves it from the support portal.
	if target := plan.Version.ValueString(); target != "" && (info == nil || info.Version != target) {
		hops, ok := r.upgradePath(ctx, polarisClient, clusterUUID, info, target, *plan, diags)
		if !ok {
			return
		}
		for i, hop := range hops {
			tflog.Info(ctx, "upgrading cluster", map[string]any{
				"cluster": clusterUUID.String(),
				"hop":     hop,
				"step":    fmt.Sprintf("%d/%d", i+1, len(hops)),
				"target":  target,
			})
			if !alreadyStaged(info, hop) {
				url, md5, ok := r.resolveDownload(ctx, polarisClient, clusterUUID, *plan, hop, diags)
				if !ok {
					return
				}
				if _, err := api.DownloadPackageAndWait(ctx, clusterUUID, url, md5, hop); err != nil {
					diags.AddError("Failed to download cluster package", err.Error())
					return
				}
			}
			if _, err := api.Upgrade(ctx, clusterUUID, upgradeType, hop); err != nil {
				diags.AddError("Failed to start cluster upgrade", err.Error())
				return
			}
			if _, err := api.WaitForUpgrade(ctx, clusterUUID, hop); err != nil {
				diags.AddError("Failed waiting for cluster upgrade", err.Error())
				return
			}
			// Refresh so the next hop (and the pre-stage step below) sees the
			// new installed version.
			if details, err = api.ClusterUpgrade(ctx, clusterUUID); err != nil {
				diags.AddError("Failed to read cluster settings", err.Error())
				return
			}
			info = details.CDMInfo
		}
	}

	// 4. Pre-stage downloaded_version when it differs from the installed
	// version. ValidateConfig guarantees it is not older than version, so this
	// only ever stages a newer package for a future upgrade. A downloaded_version
	// equal to the installed version is already satisfied (alreadyStaged).
	if stage := plan.DownloadedVersion.ValueString(); stage != "" && !alreadyStaged(info, stage) {
		url, md5, ok := r.resolveDownload(ctx, polarisClient, clusterUUID, *plan, stage, diags)
		if !ok {
			return
		}
		if _, err := api.DownloadPackageAndWait(ctx, clusterUUID, url, md5, stage); err != nil {
			diags.AddError("Failed to download cluster package", err.Error())
			return
		}
	}

	// 5. Refresh computed state.
	if details, err = api.ClusterUpgrade(ctx, clusterUUID); err != nil {
		diags.AddError("Failed to read cluster settings", err.Error())
		return
	}
	plan.applyComputed(details)
}

// upgradePath returns the ordered list of CDM versions to upgrade through to
// reach targetVersion from the cluster's currently installed version, excluding
// the installed version itself. A single-hop upgrade yields just
// [targetVersion]; a multi-release jump yields each intermediate version
// followed by the target, as computed by MultiHopUpgradePath.
//
// When package_url is set the user supplies a single explicit package, which can
// only drive one direct hop; a computed multi-hop path is rejected.
func (r *clusterSettingsResource) upgradePath(ctx context.Context, polarisClient *polaris.Client, clusterUUID uuid.UUID, info *gqlcluster.CDMInfo, targetVersion string, plan clusterSettingsResourceModel, diags *diag.Diagnostics) ([]string, bool) {
	// With no observed install version the source is unknown; fall back to a
	// single direct hop and let resolveDownload validate it.
	if info == nil {
		return []string{targetVersion}, true
	}

	// Request full release names (including patch and build, e.g.
	// "9.4.0-p2-30507") so each hop matches the cluster's reported
	// info.Version, the support-portal release names used by resolveDownload,
	// and the exact-match comparison in WaitForUpgrade. Short names would never
	// match and the installed-version drop in upgradeHops would fail.
	path, err := gqlcluster.MultiHopUpgradePath(ctx, polarisClient.GQL, clusterUUID, "", targetVersion, true)
	if err != nil {
		diags.AddError("Failed to resolve upgrade path", err.Error())
		return nil, false
	}

	hops := upgradeHops(path, info.Version, targetVersion)

	// A custom package can only drive a single direct hop: we have one URL and
	// MD5, not one per intermediate version.
	if plan.PackageURL.ValueString() != "" && len(hops) > 1 {
		diags.AddError("package_url cannot drive a multi-hop upgrade",
			fmt.Sprintf("upgrading cluster %q from %q to %q requires %d hops (%s); package_url/package_md5 can only supply a single direct upgrade package. Remove package_url to stage each hop from the support portal, or set version to an intermediate release and upgrade one hop at a time.",
				clusterUUID, info.Version, targetVersion, len(hops), strings.Join(hops, " -> ")))
		return nil, false
	}

	return hops, true
}

// upgradeHops reduces a MultiHopUpgradePath result to the ordered hops to apply.
// The path is source→target inclusive and already ordered for sequential
// application, so the hops are simply everything after the installed (source)
// version. If the installed version is not present, or it is the last element,
// the path carries no actionable hops; fall back to a single direct hop to the
// target and let resolveDownload surface a precise error for an unknown or
// unreachable target.
func upgradeHops(path []string, installed, target string) []string {
	for i, v := range path {
		if v == installed {
			if hops := path[i+1:]; len(hops) > 0 {
				return hops
			}
			return []string{target}
		}
	}
	return []string{target}
}

// resolveDownload returns the package URL and MD5 to stage targetVersion. The
// package_url/package_md5 override is used verbatim when set; otherwise the
// release is resolved from the Rubrik support portal via ListUpgrades.
func (r *clusterSettingsResource) resolveDownload(ctx context.Context, polarisClient *polaris.Client, clusterUUID uuid.UUID, plan clusterSettingsResourceModel, targetVersion string, diags *diag.Diagnostics) (string, string, bool) {
	if url := plan.PackageURL.ValueString(); url != "" {
		md5 := plan.PackageMD5.ValueString()
		// ValidateConfig already rejects package_url without package_md5, except
		// when package_md5 is unknown at plan time (e.g. sourced from another
		// resource). This guards that deferred case, where the value resolves to
		// empty only at apply.
		if md5 == "" {
			diags.AddError("package_md5 required", "`package_md5` must be set together with `package_url`.")
			return "", "", false
		}
		return url, md5, true
	}

	releases, err := gqlcluster.ListUpgrades(ctx, polarisClient.GQL, []uuid.UUID{clusterUUID}, gqlcluster.ListUpgradesOptions{
		FilterVersion: targetVersion,
		FetchLinks:    true,
	})
	if err != nil {
		diags.AddError("Failed to look up release metadata", err.Error())
		return "", "", false
	}

	for _, release := range releases {
		if release.Name != targetVersion {
			continue
		}
		if !release.Upgradable {
			diags.AddError("Version not directly upgradable",
				fmt.Sprintf("version %q is not a direct upgrade target for cluster %q; it cannot be staged in a single hop from the current version", targetVersion, clusterUUID))
			return "", "", false
		}
		if release.URL == "" || release.MD5Sum == "" {
			diags.AddError("Release has no download link",
				fmt.Sprintf("version %q has no download link or checksum available from the support portal", targetVersion))
			return "", "", false
		}
		return release.URL, release.MD5Sum, true
	}

	diags.AddError("Version not found",
		fmt.Sprintf("version %q was not found in the support portal release listing for cluster %q; set `package_url` and `package_md5` to download from a custom source", targetVersion, clusterUUID))
	return "", "", false
}

// applyComputed copies the cluster's observed state from details onto the
// resource model's computed fields (id and name).
//
// downloaded_version is intentionally left untouched: it is a declared-intent
// input (the package to pre-stage), not an observation, and refreshing it from
// the cluster would produce a perpetual diff once it is removed from config.
// version is refreshed separately in Read (see refreshedVersion) while it is
// managed, so out-of-band upgrades surface as drift. upgrade_mode is
// Optional+Computed, so it keeps the configured value when set and otherwise
// reflects the cluster's current mode.
func (m *clusterSettingsResourceModel) applyComputed(details gqlcluster.UpgradeDetails) {
	m.ID = types.StringValue(details.ID.String())
	m.Name = types.StringValue(details.Name)

	if m.UpgradeMode.IsNull() || m.UpgradeMode.IsUnknown() {
		mode := gqlcluster.UpgradeTypeRolling
		if info := details.CDMInfo; info != nil && info.FastUpgradePreferred {
			mode = gqlcluster.UpgradeTypeFast
		}
		m.UpgradeMode = types.StringValue(string(mode))
	}
}

// refreshedVersion returns the version to store in state during Read. While
// version is managed (prior is non-empty) and the cluster reports an installed
// version, it mirrors that observed version so out-of-band upgrades surface as
// drift in the next plan. When version is unset, or the cluster reports no
// version, the prior value is preserved unchanged: refreshing an unmanaged
// version would otherwise produce a perpetual diff after it is removed from
// config.
func refreshedVersion(prior types.String, details gqlcluster.UpgradeDetails) types.String {
	if prior.ValueString() == "" {
		return prior
	}
	if info := details.CDMInfo; info != nil && info.Version != "" {
		return types.StringValue(info.Version)
	}
	return prior
}

// currentUpgradeType reports the cluster's configured upgrade type, defaulting
// to ROLLING when the info is absent.
func currentUpgradeType(info *gqlcluster.CDMInfo) gqlcluster.UpgradeType {
	if info != nil && info.FastUpgradePreferred {
		return gqlcluster.UpgradeTypeFast
	}
	return gqlcluster.UpgradeTypeRolling
}

// alreadyStaged reports whether targetVersion is already installed or staged
// on the cluster.
func alreadyStaged(info *gqlcluster.CDMInfo, targetVersion string) bool {
	if info == nil {
		return false
	}
	return info.Version == targetVersion || info.IsStaged(targetVersion)
}
