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
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core/secret"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/hierarchy"
)

// defaultSQLManagedInstanceCredentialsTimeout is how long to wait for the
// backup setup job to finish before giving up.
const defaultSQLManagedInstanceCredentialsTimeout = 30 * time.Minute

const resourceAzureSQLManagedInstanceCredentialsDescription = `
The ´rubrik_azure_sql_managed_instance_credentials´ resource configures the
credentials RSC uses to back up an Azure SQL Managed Instance server.

There are two ways to create the user RSC backs up as, selected with
´setup_script_installed´:

* By default, RSC connects to the managed instance using the credentials in the
  ´sql_credentials´ block and creates the backup user itself. Those credentials
  are an administrator login, used only for that setup and not stored by RSC.
* When ´setup_script_installed´ is ´true´, the setup script has already been run
  against the managed instance and has created the backup user, so RSC only
  records which credentials to use. The ´sql_credentials´ block is then the
  backup user's own login, which must match the login and password the setup
  script was run with.

Whether ´sql_credentials´ is required depends on the authentication mechanisms
the managed instance supports. The ´rubrik_object´ data source reports them as
´auth_type´ for an object type of ´AzureSqlManagedInstanceServer´.

| ´auth_type´ | Default | ´setup_script_installed = true´ |
| --- | --- | --- |
| ´SQL_AUTH_ONLY´ | Required | Required |
| ´SQL_AUTH_AND_AAD´ | Required | Must not be set |
| ´AAD_ONLY´ | Not supported | Must not be set |
| ´AUTH_TYPE_UNSPECIFIED´ | Required | Required |

Where the table says the block must not be set, RSC authenticates using
Microsoft Entra ID instead and no credentials are sent at all.

´AUTH_TYPE_UNSPECIFIED´ means RSC holds no authentication type for the managed
instance, which is the case until its subscription is refreshed. Such a server
is treated as ´SQL_AUTH_ONLY´, since RSC accepts a SQL Server login for it.

Use the ´rubrik_object´ data source with an object type of
´AzureSqlManagedInstanceServer´ to look up the ´server_id´ by name.

~> **Note:** ´auth_type´ is only known once RSC has been queried, so a
combination which does not match the table above is reported when the resource
is applied, not when it is planned.

~> **Note:** Because the ´sql_credentials´ arguments are write-only, changing
them produces no difference in the plan. Change ´sql_credential_version´ to make
Terraform send the credentials again.

~> **Note:** When RSC creates the backup user, the credentials are validated by
the managed instance only once the setup job runs, so invalid credentials
surface as a failed job rather than as an immediate error. Registering
credentials for an already installed setup script is not a job and returns
immediately.

~> **Note:** Destroying the resource clears the backup credentials RSC holds for
the managed instance, which are the credentials of the backup user, not the
administrator login RSC may have created it with. If the managed instance server
itself no longer exists in RSC, there is nothing left to clear, so the destroy
succeeds and the resource is simply removed from the Terraform state.
`

var (
	_ resource.Resource                     = &azureSQLManagedInstanceCredentialsResource{}
	_ resource.ResourceWithConfigure        = &azureSQLManagedInstanceCredentialsResource{}
	_ resource.ResourceWithConfigValidators = &azureSQLManagedInstanceCredentialsResource{}
)

type azureSQLManagedInstanceCredentialsResource struct {
	client *client
}

type azureSQLManagedInstanceCredentialsResourceModel struct {
	ID       types.String `tfsdk:"id"`
	ServerID types.String `tfsdk:"server_id"`
	// SetupScriptInstalled selects which of the two ways of creating the backup
	// user RSC should assume has been used.
	SetupScriptInstalled types.Bool `tfsdk:"setup_script_installed"`
	// SQLCredentials holds at most one element, enforced by the schema. It is a
	// slice rather than a pointer because the block is a single-element list
	// block, see the schema for why.
	SQLCredentials       []sqlCredentialsModel `tfsdk:"sql_credentials"`
	SQLCredentialVersion types.String          `tfsdk:"sql_credential_version"`
	Timeouts             timeouts.Value        `tfsdk:"timeouts"`
}

type sqlCredentialsModel struct {
	SQLUsername types.String `tfsdk:"sql_username"`
	SQLPassword types.String `tfsdk:"sql_password"`
}

func newAzureSQLManagedInstanceCredentialsResource() resource.Resource {
	return &azureSQLManagedInstanceCredentialsResource{}
}

func (r *azureSQLManagedInstanceCredentialsResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyAzureSQLManagedInstanceCredentials
}

func (r *azureSQLManagedInstanceCredentialsResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAzureSQLManagedInstanceCredentialsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC object ID of the SQL Managed Instance server (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyServerID: schema.StringAttribute{
				Required: true,
				Description: "RSC object ID of the SQL Managed Instance server (UUID). Changing this forces a " +
					"new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keySetupScriptInstalled: schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Whether the setup script has already been run against the managed instance. When " +
					"`false`, the default, RSC connects to the managed instance using `sql_credentials` and " +
					"creates the backup user itself. When `true`, the script has already created the backup " +
					"user and RSC only records which credentials to use.",
			},
			keySQLCredentialVersion: schema.StringAttribute{
				Optional: true,
				Description: "Arbitrary value identifying the version of the credentials. Change it to make " +
					"Terraform send the `sql_credentials` block again, e.g. after rotating the password. " +
					"Write-only arguments produce no difference in the plan on their own. Required when " +
					"`sql_credentials` is set and must not be set otherwise.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			// A single-element list block rather than a SingleNestedBlock. The
			// two are written identically in a configuration, but a
			// SingleNestedBlock cannot be left out when its attributes are
			// required: the framework still demands a value for each of them
			// and rejects the configuration, which would make the cases where
			// no credentials are sent impossible to express.
			keySQLCredentials: schema.ListNestedBlock{
				Description: "SQL Server credentials. When `setup_script_installed` is `false`, these are the " +
					"credentials of a user with permission to create the user RSC uses to perform backups. " +
					"When it is `true`, these are the credentials of the backup user the setup script created. " +
					"Whether the block is required depends on the authentication mechanisms the managed " +
					"instance supports, see the resource description. May be specified at most once. " +
					"Write-only, change `sql_credential_version` to send them again.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keySQLUsername: schema.StringAttribute{
							Required:    true,
							WriteOnly:   true,
							Description: "SQL Server login.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keySQLPassword: schema.StringAttribute{
							Required:    true,
							Sensitive:   true,
							WriteOnly:   true,
							Description: "Password for `sql_username`.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
			}),
		},
	}
}

func (r *azureSQLManagedInstanceCredentialsResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.ConfigValidators")

	return []resource.ConfigValidator{
		// The version is a nonce which exists only to make Terraform send the
		// write-only credentials again, so neither is meaningful alone.
		resourcevalidator.RequiredTogether(
			path.MatchRoot(keySQLCredentials),
			path.MatchRoot(keySQLCredentialVersion),
		),
		sqlCredentialsRequiredValidator{},
	}
}

// sqlCredentialsRequiredValidator enforces that the sql_credentials block is set
// when RSC is the one creating the backup user, which it cannot do without an
// administrator login to connect as.
//
// The remaining combinations depend on the authentication mechanisms the managed
// instance supports, which is only known once RSC has been queried, so they are
// checked when the resource is applied instead.
type sqlCredentialsRequiredValidator struct{}

func (v sqlCredentialsRequiredValidator) Description(_ context.Context) string {
	return "the sql_credentials block is required unless setup_script_installed is true"
}

func (v sqlCredentialsRequiredValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v sqlCredentialsRequiredValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	var config azureSQLManagedInstanceCredentialsResourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(validateSQLCredentialsRequired(config)...)
}

// validateSQLCredentialsRequired holds the plan-time, client-free validation
// rule for the resource so it can be unit-tested in isolation.
func validateSQLCredentialsRequired(config azureSQLManagedInstanceCredentialsResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// An unknown value comes from another resource and is not resolved until
	// the configuration is applied, so there is nothing to check yet. The apply
	// still rejects the combination.
	if config.SetupScriptInstalled.IsUnknown() || config.SetupScriptInstalled.ValueBool() {
		return diags
	}
	if len(config.SQLCredentials) == 0 {
		diags.AddError("Missing SQL credentials", "the sql_credentials block is required unless "+
			"setup_script_installed is true, because RSC needs an administrator login to connect to the "+
			"managed instance and create the backup user")
	}

	return diags
}

func (r *azureSQLManagedInstanceCredentialsResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureSQLManagedInstanceCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.Create")

	var plan azureSQLManagedInstanceCredentialsResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, defaultSQLManagedInstanceCredentialsTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	diags, credentialsMayExist := r.setupBackup(ctx, req.Config, plan, timeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		// RSC keeps running the setup job after the provider stops waiting for
		// it, so a timeout can still end with credentials on the server. Record
		// the resource anyway, alongside the error, so a later destroy clears
		// them. Without this the credentials would be left in RSC with nothing
		// in Terraform tracking them.
		if credentialsMayExist {
			plan.ID = plan.ServerID
			res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
		}
		return
	}

	plan.ID = plan.ServerID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

// Read is intentionally a no-op. RSC does not expose the backup credentials of
// a SQL Managed Instance server, so there is nothing to read back and no way to
// detect drift. Removing the resource from state here would make every plan
// recreate it.
func (r *azureSQLManagedInstanceCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.Read")

	var state azureSQLManagedInstanceCredentialsResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

// Update re-runs the backup setup. Since server_id requires replacement, the
// changes which can reach this point are a new sql_credential_version, with or
// without new credentials, and a change of setup_script_installed.
func (r *azureSQLManagedInstanceCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.Update")

	var plan azureSQLManagedInstanceCredentialsResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Update(ctx, defaultSQLManagedInstanceCredentialsTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Unlike Create, a failure here cannot leave credentials untracked: the
	// resource is already in state, so Terraform keeps the previous state and a
	// destroy still clears the server.
	diags, _ = r.setupBackup(ctx, req.Config, plan, timeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.ServerID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *azureSQLManagedInstanceCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureSQLManagedInstanceCredentialsResource.Delete")

	var state azureSQLManagedInstanceCredentialsResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	serverID, err := uuid.Parse(state.ServerID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid server ID", err.Error())
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	clearErr := azure.Wrap(polarisClient).ClearSQLManagedInstanceBackupCredentials(ctx, []uuid.UUID{serverID})
	if clearErr == nil {
		return
	}

	// The clear failed. If the managed instance server itself is gone from RSC
	// — deleted in Azure, or its subscription offboarded — then the backup
	// credentials went with it, so there is nothing left to clear and the
	// destroy should still succeed. Failing here would strand the resource in
	// state, with no way to remove it other than `terraform state rm`, and
	// these are only credentials: they can be set up again if the server ever
	// comes back.
	//
	// The clear error cannot be classified on its own. RSC answers a clear for
	// an object it cannot find with a MANAGE_PROTECTION permission error rather
	// than anything indicating the object is missing, so the error looks
	// identical whether the server is gone or the service account genuinely
	// lacks the permission. Look the server up in the hierarchy instead, and
	// only ignore the failure once the server is confirmed gone.
	_, exists, existsErr := sqlManagedInstanceServer(ctx, polarisClient, serverID)
	switch {
	case existsErr != nil:
		// Whether the server still exists could not be established, so assume
		// the clear failure is real. Both errors are reported: the lookup
		// failure is the reason the clear failure could not be dismissed.
		res.Diagnostics.AddError("Failed to clear SQL Managed Instance backup credentials", fmt.Sprintf(
			"%s\n\nLooking up SQL Managed Instance server %s to determine whether it still exists also "+
				"failed: %s", clearErr, serverID, existsErr))
	case exists:
		res.Diagnostics.AddError("Failed to clear SQL Managed Instance backup credentials", clearErr.Error())
	default:
		tflog.Warn(ctx, "SQL Managed Instance server no longer exists in RSC, ignoring the failure to clear "+
			"its backup credentials", map[string]any{
			"server_id": serverID.String(),
			"err":       clearErr.Error(),
		})
	}
}

// sqlManagedInstanceServer looks up a SQL Managed Instance server in the RSC
// hierarchy. The second return value reports whether the server exists, and the
// server is only valid when it does.
//
// A hierarchy lookup of an object which does not exist returns
// graphql.ErrNotFound, which is reported as the server not existing. Note that
// RSC does not distinguish an object which is missing from one the service
// account is not authorized to see, so a server which exists but has become
// invisible to the service account is reported as not existing.
func sqlManagedInstanceServer(ctx context.Context, client *polaris.Client, serverID uuid.UUID) (hierarchy.AzureSQLManagedInstanceServer, bool, error) {
	server, err := hierarchy.ObjectByIDAndWorkload[hierarchy.AzureSQLManagedInstanceServer](ctx, client.GQL,
		serverID, hierarchy.WorkloadAllSubHierarchyType)
	if err != nil {
		if errors.Is(err, graphql.ErrNotFound) {
			return hierarchy.AzureSQLManagedInstanceServer{}, false, nil
		}
		return hierarchy.AzureSQLManagedInstanceServer{}, false, err
	}

	return server, true, nil
}

// setupBackup reads the write-only credentials from the configuration and
// configures the backup credentials RSC uses for the managed instance.
// Write-only arguments are null in the plan and never stored in state, so they
// must be read from the configuration.
//
// The second return value reports whether the setup may have left credentials on
// the server even though it failed. It is true when the wait was interrupted, by
// the configured timeout or by cancellation, because RSC keeps running the setup
// job after the provider stops waiting for it. Every other failure happens
// before RSC accepts the work, or after it has already reported the job failed,
// and leaves nothing behind.
func (r *azureSQLManagedInstanceCredentialsResource) setupBackup(ctx context.Context, cfg tfsdk.Config, plan azureSQLManagedInstanceCredentialsResourceModel, timeout time.Duration) (diag.Diagnostics, bool) {
	var diags diag.Diagnostics

	var config azureSQLManagedInstanceCredentialsResourceModel
	diags.Append(cfg.Get(ctx, &config)...)
	if diags.HasError() {
		return diags, false
	}

	serverID, err := uuid.Parse(plan.ServerID.ValueString())
	if err != nil {
		diags.AddError("Invalid server ID", err.Error())
		return diags, false
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		diags.AddError("RSC client error", err.Error())
		return diags, false
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// The authentication mechanisms the managed instance supports decide which
	// credentials RSC accepts for it, and are only known once the server has
	// been looked up. The lookup doubles as a check that the server exists,
	// turning a stale server_id into a clear error rather than a failed job.
	server, exists, err := sqlManagedInstanceServer(ctx, polarisClient, serverID)
	switch {
	case err != nil:
		diags.AddError("Failed to look up SQL Managed Instance server", err.Error())
		return diags, false
	case !exists:
		diags.AddError("SQL Managed Instance server not found", fmt.Sprintf("no SQL Managed Instance server "+
			"with ID %s exists in RSC, or the service account is not authorized to see it", serverID))
		return diags, false
	}
	authType := gqlazure.AzureSQLAuthenticationType(server.AuthType)
	if authType == gqlazure.AzureSQLAuthTypeUnspecified {
		// RSC records no authentication type for a server whose subscription has
		// not been refreshed since RSC started tracking the field. RSC accepts a
		// SQL Server login for such a server, so proceed rather than block a
		// setup which would succeed. Logged because the provider is assuming
		// rather than reading the authentication type.
		tflog.Warn(ctx, "RSC reports no authentication type for the SQL Managed Instance server, "+
			"assuming SQL Server authentication", map[string]any{"server_id": serverID.String()})
	}

	var credentials *gqlazure.LoginCredentials
	if len(config.SQLCredentials) > 0 {
		credentials = &gqlazure.LoginCredentials{
			Login:    config.SQLCredentials[0].SQLUsername.ValueString(),
			Password: secret.String(config.SQLCredentials[0].SQLPassword.ValueString()),
		}
	}

	if !plan.SetupScriptInstalled.ValueBool() {
		return r.createBackupUser(ctx, polarisClient, serverID, authType, credentials)
	}

	return r.registerBackupCredentials(ctx, polarisClient, serverID, authType, credentials), false
}

// createBackupUser has RSC connect to the managed instance as the given
// administrator login and create the user it backs up as. The task chain
// started for the server is waited for, so this blocks until the setup either
// finishes or the context deadline passes.
func (r *azureSQLManagedInstanceCredentialsResource) createBackupUser(ctx context.Context, client *polaris.Client, serverID uuid.UUID, authType gqlazure.AzureSQLAuthenticationType, credentials *gqlazure.LoginCredentials) (diag.Diagnostics, bool) {
	var diags diag.Diagnostics

	switch authType {
	// All three accept a SQL Server login for RSC to connect with, an
	// unspecified authentication type by assumption, warned about above.
	case gqlazure.AzureSQLAuthTypeSQLOnly, gqlazure.AzureSQLAuthTypeSQLAndEntraID,
		gqlazure.AzureSQLAuthTypeUnspecified:
	case gqlazure.AzureSQLAuthTypeEntraIDOnly:
		diags.AddError("SQL Server authentication not supported", fmt.Sprintf("SQL Managed Instance server "+
			"%s only supports Microsoft Entra ID authentication, so RSC cannot connect to it with a SQL "+
			"Server login to create the backup user. Run the setup script against the managed instance "+
			"and set setup_script_installed to true instead", serverID))
		return diags, false
	default:
		diags.AddError("Unknown authentication type", fmt.Sprintf("RSC reports the authentication type of "+
			"SQL Managed Instance server %s as %q, which this provider does not recognize", serverID,
			authType))
		return diags, false
	}

	// Guarded by sqlCredentialsRequiredValidator, except when
	// setup_script_installed is only known at apply time.
	if credentials == nil {
		diags.AddError("Missing SQL credentials", "the sql_credentials block is required unless "+
			"setup_script_installed is true")
		return diags, false
	}

	if err := azure.Wrap(client).SetupSQLManagedInstanceBackup(ctx, []uuid.UUID{serverID}, *credentials, 0); err != nil {
		diags.AddError("Failed to set up SQL Managed Instance backup", err.Error())
		return diags, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
	}

	return diags, false
}

// registerBackupCredentials records which credentials RSC should use for a
// managed instance whose backup user the setup script has already created.
//
// Whether credentials are needed is decided by the managed instance rather than
// by the practitioner: a server which only supports SQL Server authentication
// needs the backup user's login, and one which supports Microsoft Entra ID uses
// that instead and takes no credentials at all. Unlike the setup this is
// synchronous, so there is no task chain to wait for.
func (r *azureSQLManagedInstanceCredentialsResource) registerBackupCredentials(ctx context.Context, client *polaris.Client, serverID uuid.UUID, authType gqlazure.AzureSQLAuthenticationType, credentials *gqlazure.LoginCredentials) diag.Diagnostics {
	var diags diag.Diagnostics

	var err error
	switch authType {
	// An unspecified authentication type is assumed to be SQL Server
	// authentication, warned about above, so the setup script RSC generated
	// created a SQL Server login just as it does for SQL_AUTH_ONLY.
	case gqlazure.AzureSQLAuthTypeSQLOnly, gqlazure.AzureSQLAuthTypeUnspecified:
		if credentials == nil {
			diags.AddError("Missing SQL credentials", fmt.Sprintf("RSC generates the SQL Server setup "+
				"script for SQL Managed Instance server %s, and that script creates a SQL Server login, "+
				"so the sql_credentials block is required. It must hold the login and password the setup "+
				"script was run with", serverID))
			return diags
		}
		err = azure.Wrap(client).AddSQLManagedInstanceBackupCredentials(ctx, []uuid.UUID{serverID}, *credentials)
	case gqlazure.AzureSQLAuthTypeSQLAndEntraID, gqlazure.AzureSQLAuthTypeEntraIDOnly:
		if credentials != nil {
			diags.AddError("Unexpected SQL credentials", fmt.Sprintf("SQL Managed Instance server %s "+
				"supports Microsoft Entra ID authentication, so RSC returns an Entra ID setup script "+
				"for it and that script creates no SQL Server login to authenticate as. Remove the "+
				"sql_credentials block along with sql_credential_version", serverID))
			return diags
		}
		err = azure.Wrap(client).AddSQLManagedInstanceBackupCredentialsUsingEntraID(ctx, []uuid.UUID{serverID})
	default:
		diags.AddError("Unknown authentication type", fmt.Sprintf("RSC reports the authentication type of "+
			"SQL Managed Instance server %s as %q, which this provider does not recognize", serverID,
			authType))
		return diags
	}
	if err != nil {
		diags.AddError("Failed to add SQL Managed Instance backup credentials", err.Error())
	}

	return diags
}
