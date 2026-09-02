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
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
SQL Server credentials RSC uses to back up an Azure SQL Managed Instance
server.

RSC connects to the managed instance using the SQL Server credentials in the
´sql_credentials´ block and creates the user it uses to perform backups. These
administrator credentials are only used for that setup and are not stored by
RSC, which is why they are write-only arguments: they are sent to RSC but never
written to Terraform state, so they can be sourced from a secret store such as
Vault without leaking into state.

Use the ´rubrik_object´ data source with an object type of
´AzureSqlManagedInstanceServer´ to look up the ´server_id´ by name.

~> **Note:** Because the ´sql_credentials´ arguments are write-only, changing
them produces no difference in the plan. Change ´sql_credential_version´ to make
Terraform send the credentials again.

~> **Note:** The credentials are validated by the managed instance only once the
setup job runs, so invalid credentials surface as a failed job rather than as an
immediate error.

~> **Note:** Destroying the resource clears the backup credentials RSC holds for
the managed instance, which are the credentials of the user RSC created, not the
´sql_credentials´ administrator login. If the managed instance server itself no
longer exists in RSC, there is nothing left to clear, so the destroy succeeds and
the resource is simply removed from the Terraform state.
`

var (
	_ resource.Resource              = &azureSQLManagedInstanceCredentialsResource{}
	_ resource.ResourceWithConfigure = &azureSQLManagedInstanceCredentialsResource{}
)

type azureSQLManagedInstanceCredentialsResource struct {
	client *client
}

type azureSQLManagedInstanceCredentialsResourceModel struct {
	ID                   types.String         `tfsdk:"id"`
	ServerID             types.String         `tfsdk:"server_id"`
	SQLCredentials       *sqlCredentialsModel `tfsdk:"sql_credentials"`
	SQLCredentialVersion types.String         `tfsdk:"sql_credential_version"`
	Timeouts             timeouts.Value       `tfsdk:"timeouts"`
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
			keySQLCredentialVersion: schema.StringAttribute{
				Required: true,
				Description: "Arbitrary value identifying the version of the credentials. Change it to make " +
					"Terraform send the `sql_credentials` block again, e.g. after rotating the password. " +
					"Write-only arguments produce no difference in the plan on their own.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			keySQLCredentials: schema.SingleNestedBlock{
				Description: "Required. Credentials of a SQL Server user with permission to create the user " +
					"RSC uses to perform backups. Write-only, change `sql_credential_version` to send them " +
					"again. Note that Terraform lists the block as optional because the validator enforcing " +
					"it is not visible to the documentation generator.",
				Validators: []validator.Object{
					objectvalidator.IsRequired(),
				},
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
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
			}),
		},
	}
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
// only change which can reach this point is a new credentials_version, with or
// without new credentials.
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
	exists, existsErr := sqlManagedInstanceServerExists(ctx, polarisClient, serverID)
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

// sqlManagedInstanceServerExists reports whether the SQL Managed Instance
// server still exists in the RSC hierarchy.
//
// A hierarchy lookup of an object which does not exist returns
// graphql.ErrNotFound, which is reported as the server not existing. Note that
// RSC does not distinguish an object which is missing from one the service
// account is not authorized to see, so a server which exists but has become
// invisible to the service account is reported as not existing.
func sqlManagedInstanceServerExists(ctx context.Context, client *polaris.Client, serverID uuid.UUID) (bool, error) {
	_, err := hierarchy.ObjectByIDAndWorkload[hierarchy.Object](ctx, client.GQL, serverID,
		hierarchy.WorkloadAllSubHierarchyType)
	if err != nil {
		if errors.Is(err, graphql.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// setupBackup reads the write-only credentials from the configuration and runs
// the backup setup, blocking until the setup job finishes. Write-only arguments
// are null in the plan and never stored in state, so they must be read from the
// configuration.
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

	if config.SQLCredentials == nil {
		diags.AddError("Missing SQL credentials", "the sql_credentials block is required")
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

	err = azure.Wrap(polarisClient).SetupSQLManagedInstanceBackup(ctx, []uuid.UUID{serverID},
		gqlazure.LoginCredentials{
			Login:    config.SQLCredentials.SQLUsername.ValueString(),
			Password: secret.String(config.SQLCredentials.SQLPassword.ValueString()),
		}, 0)
	if err != nil {
		diags.AddError("Failed to set up SQL Managed Instance backup", err.Error())
		return diags, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
	}

	return diags, false
}
