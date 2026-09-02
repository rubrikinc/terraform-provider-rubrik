# Configures the credentials RSC uses to back up an Azure SQL Managed Instance
# server.
#
# There are two ways to create the user RSC backs up as, selected with
# setup_script_installed. By default RSC connects to the managed instance using
# the credentials in the sql_credentials block and creates the backup user
# itself. With setup_script_installed set to true, the setup script has already
# been run against the managed instance and created that user, so RSC only
# records which credentials to use.
#
# Whether sql_credentials is required depends on the authentication mechanisms
# the managed instance supports, reported as auth_type by the rubrik_object
# data source. The three examples below cover each case.

# Look up the managed instance server by name to get its RSC object ID.
data "rubrik_object" "sql_mi" {
  name        = "my-sql-managed-instance"
  object_type = "AzureSqlManagedInstanceServer"

  # Optional, set to the RSC cloud account ID of the parent subscription when
  # the same server name exists in more than one subscription.
  # subscription_id = "00000000-0000-0000-0000-000000000000"
}

variable "sql_username" {
  type = string
}

variable "sql_password" {
  type      = string
  sensitive = true
}

# RSC creates the backup user. The credentials are an administrator login with
# permission to do so, used only for the setup and not stored by RSC. Works for
# a managed instance whose auth_type is SQL_AUTH_ONLY or SQL_AUTH_AND_AAD.
resource "rubrik_azure_sql_managed_instance_credentials" "rsc_creates_user" {
  server_id = data.rubrik_object.sql_mi.id

  # Write-only, so these never reach Terraform state and can be sourced from a
  # secret store such as Vault.
  sql_credentials {
    sql_username = var.sql_username
    sql_password = var.sql_password
  }

  # Because sql_credentials is write-only, changing the username or password
  # produces no difference in the plan. Change sql_credential_version to send
  # them again, for example after rotating the password.
  sql_credential_version = "1"
}

# The setup script has already been run against a managed instance whose
# auth_type is SQL_AUTH_ONLY. The credentials are the backup user's own login
# and must match the login and password the script was run with.
resource "rubrik_azure_sql_managed_instance_credentials" "script_sql_auth" {
  server_id              = data.rubrik_object.sql_mi.id
  setup_script_installed = true

  sql_credentials {
    sql_username = var.sql_username
    sql_password = var.sql_password
  }

  sql_credential_version = "1"
}

# The setup script has already been run against a managed instance which
# supports Microsoft Entra ID, so auth_type is SQL_AUTH_AND_AAD or AAD_ONLY.
# RSC authenticates using Entra ID, so no credentials are sent at all and the
# sql_credentials block must be left out.
resource "rubrik_azure_sql_managed_instance_credentials" "script_entra_id" {
  server_id              = data.rubrik_object.sql_mi.id
  setup_script_installed = true
}
