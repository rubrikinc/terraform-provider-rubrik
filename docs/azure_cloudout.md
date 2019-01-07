## rubrik_azure_cloudout

Configures a new Azure archive target.

## Example Usage

```hcl
resource "rubrik_azure_cloudout" "example" {
  container            = "rubriktfdemo"
  azure_access_key     = "q9eFAW9eawMe/fekai5GoFfKjH0UH31dceuY7LDIh3IbDFwqGekSBelJMZ90/S6deVfmw/TZaiKBk5lw0DXgw=+"
  storage_account_name = "rubriktf"
  archive_name         = "TF-Demo"
  rsa_key              = "${var.rsakey}"
}
```

## Argument Reference

The following arguments are supported:

* `container` - (Required) The name of the Azure storage container you wish to use as an archive.
* `azure_access_key` - (Required) The access key for the Azure storage account.
* `storage_account_name` - (Required) The name of the Storage Account that the container belongs to.
* `archive_name` - (Required) The name of the archive location used in the Rubrik GUI.
* `instance_type` - (Optional) The Cloud Platform type of the archival location. Valid choices are `default`, `china`, `germany`, and `government` with `default` being the default choice.
* `rsa_key` - (Required) The RSA key that will be used to encrypt the archive data.
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is 15.
