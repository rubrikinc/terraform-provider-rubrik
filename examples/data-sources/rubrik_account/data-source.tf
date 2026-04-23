data "rubrik_account" "current" {}

# Account name and fully qualified domain name.
output "name" {
  value = data.rubrik_account.current.name
}

output "fqdn" {
  value = data.rubrik_account.current.fqdn
}

# Features enabled for the RSC account.
output "features" {
  value = data.rubrik_account.current.features
}

# Cloud vendor features and their permission groups.
output "aws" {
  value = data.rubrik_account.current.aws
}

output "azure" {
  value = data.rubrik_account.current.azure
}

output "gcp" {
  value = data.rubrik_account.current.gcp
}

# Create maps of operations and workloads for easy lookup.
locals {
  operations = {
    for op in data.rubrik_account.current.operations : op => op
  }
  workloads = {
    for w in data.rubrik_account.current.workloads : w => w
  }
}

resource "rubrik_custom_role" "azure_admin" {
  name        = "RSC Azure Admin"
  description = "Custom role for Azure admin."

  permission {
    operation = local.operations.VIEW_INVENTORY
    hierarchy {
      snappable_type = local.workloads.AllSubHierarchyType
      object_ids = [
        "GlobalResource"
      ]
    }
  }

  permission {
    operation = local.operations.RESTORE_TO_ORIGIN
    hierarchy {
      snappable_type = local.workloads.AwsNativeRdsInstance
      object_ids = [
        "AWSNATIVE_ROOT"
      ]
    }
    hierarchy {
      snappable_type = local.workloads.AllSubHierarchyType
      object_ids = [
        "ORACLE_ROOT"
      ]
    }
  }
}

# Using the fqdn field from the account data source to create an Azure
# AD application.
resource "azuread_application" "app" {
  display_name = "Rubrik Security Cloud Integration"
  web {
    homepage_url = "https://${data.rubrik_account.current.fqdn}/setup_azure"
  }
}
