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


## ## Argument Reference
