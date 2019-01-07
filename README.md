# Terraform Provider for Rubrik

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)

<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">

## Installation

_Note: We assume [Terraform has already been installed](https://learn.hashicorp.com/terraform/getting-started/install.html) on your machine._

1. Download the latest [Release version](https://github.com/rubrikinc/rubrik-provider-for-terraform/releases)

* macOS: `terraform-provider-rubrik-darwin-amd64`
* Linux: `terraform-provider-rubrik-linux-amd64`
* Windows: `terraform-provider-rubrik-windows-amd64.exe`

2. [Sideload](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins) the Rubrik provder into the correct Terraform plugin directory

_Note: You may need to manually create the folder first._

* macOS: `~/.terraform.d/plugins/darwin_amd64`
* Linux: `~/.terraform.d/plugins/linux_amd64`
* Windows: `%APPDATA%\terraform.d\plugins\windows_amd64`

3. Rename the downloaded file to `terraform-provider-rubrik`

4. Run `terraform init` in the directory that contains your Terraform configuration fiile (`main.tf`)

## Quick Start

[Quick Start Guide](https://github.com/rubrikinc/rubrik-provider-for-terraform/blob/master/docs/quickstart.md)

## Documentation

[Provider Documentatinon](https://rubrik.gitbook.io/terraform-provider-for-rubrik/)

## Example 

```hcl
provider "rubrik" {}

resource "rubrik_configure_timezone" "LA-Timezone" {
  timezone = "America/Los_Angeles"
}
```

## Author Information

<p></p>
<p align="center">
  <img src="https://user-images.githubusercontent.com/8610203/37415009-6f9cf416-2778-11e8-8b56-052a8e41c3c8.png" alt="Rubrik Ranger Logo"/>
</p>
